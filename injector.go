package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const identityPrompt = "You are Claude Code, Anthropic's official CLI for Claude."

// Claude Code CLI fingerprint constants. Captured from claude-cli 2.1.220.
const (
	// betaMinimum is the smallest anthropic-beta value that upstream accepts.
	// Removing context-1m-2025-08-07 produces an explicit HTTP 400.
	betaMinimum = "context-1m-2025-08-07"

	// betaFull mirrors the complete beta list sent by the real CLI.
	betaFull = "claude-code-20250219,context-1m-2025-08-07,interleaved-thinking-2025-05-14," +
		"redact-thinking-2026-02-12,thinking-token-count-2026-05-13,context-management-2025-06-27," +
		"prompt-caching-scope-2026-01-05,mid-conversation-system-2026-04-07,effort-2025-11-24," +
		"fallback-credit-2026-06-01"

	// claudeUserAgent is a hard signal: replacing or dropping it yields HTTP 520.
	claudeUserAgent  = "claude-cli/2.1.220 (external, cli)"
	anthropicVersion = "2023-06-01"

	// headerSessionID carries the session identifier in the CLI fingerprint.
	headerSessionID = "X-Claude-Code-Session-Id"

	// headerClientRequestID is injected by other proxies and must be removed to
	// avoid contradicting the CLI fingerprint.
	headerClientRequestID = "x-client-request-id"
)

// minimumHeaders is the smallest set verified to pass upstream validation.
var minimumHeaders = map[string]string{
	"anthropic-beta":    betaMinimum,
	"Anthropic-Version": anthropicVersion,
	"Content-Type":      "application/json",
	"User-Agent":        claudeUserAgent,
}

// fullHeaders is the complete Claude Code CLI capture. The Stainless metadata
// is not strictly required by upstream but is kept for a faithful fingerprint.
var fullHeaders = map[string]string{
	"Accept":          "application/json",
	"Accept-Encoding": "gzip, deflate, br, zstd",
	"anthropic-beta":  betaFull,
	"Anthropic-Dangerous-Direct-Browser-Access": "true",
	"Anthropic-Version":           anthropicVersion,
	"Connection":                  "keep-alive",
	"Content-Type":                "application/json",
	"User-Agent":                  claudeUserAgent,
	"X-App":                       "cli",
	"X-Stainless-Arch":            "x64",
	"X-Stainless-Lang":            "js",
	"X-Stainless-OS":              "Windows",
	"X-Stainless-Package-Version": "0.94.0",
	"X-Stainless-Retry-Count":     "0",
	"X-Stainless-Runtime":         "node",
	"X-Stainless-Runtime-Version": "v26.3.0",
	"X-Stainless-Timeout":         "600",
}

type systemBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// injectIdentity rewrites the upstream request body with the identity system
// prompt and identity metadata. It returns the new body and any header
// adjustments. Full strict-mode completeness is guaranteed by the caller
// failing open on error: the request is passed through unmodified.
func injectIdentity(req *requestInterceptRequest, cfg config, profile string) (json.RawMessage, map[string][]string, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &body); err != nil || body == nil {
		return nil, nil, errors.New("body must be a JSON object")
	}
	if modified, err := injectSystemPrompt(body, cfg.SystemPrompt); err != nil || !modified {
		if err != nil {
			return nil, nil, err
		}
	}
	sessionID := resolveSessionID(req, cfg)
	injectMetadata(body, cfg, sessionID)
	updated, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	headers := injectHeaders(req, cfg, profile, sessionID)
	return updated, headers, nil
}

// injectSystemPrompt prepends the identity block to the system field. When
// system is absent, it is created as an array containing the identity block.
func injectSystemPrompt(body map[string]json.RawMessage, prompt string) (bool, error) {
	if prompt == "" {
		return false, nil
	}
	identityRaw, _ := json.Marshal(systemBlock{Type: "text", Text: prompt})
	rawSystem, exists := body["system"]
	if !exists || string(rawSystem) == "null" {
		body["system"] = json.RawMessage("[" + string(identityRaw) + "]")
		return true, nil
	}
	var systemText string
	if json.Unmarshal(rawSystem, &systemText) == nil {
		blocks := []json.RawMessage{identityRaw}
		if systemText != "" {
			original, _ := json.Marshal(systemBlock{Type: "text", Text: systemText})
			blocks = append(blocks, original)
		}
		replaced, _ := json.Marshal(blocks)
		body["system"] = replaced
		return true, nil
	}
	var blocks []json.RawMessage
	if err := json.Unmarshal(rawSystem, &blocks); err != nil {
		return false, errors.New("system must be a string or array")
	}
	if len(blocks) > 0 {
		var first systemBlock
		if json.Unmarshal(blocks[0], &first) == nil && first.Text == prompt {
			// already present; still record metadata
			return false, nil
		}
	}
	blocks = append([]json.RawMessage{identityRaw}, blocks...)
	replaced, _ := json.Marshal(blocks)
	body["system"] = replaced
	return true, nil
}

// injectMetadata sets user_id and session_id on the request metadata object
// (created if missing). The session_id is written both here and as a header.
func injectMetadata(body map[string]json.RawMessage, cfg config, sessionID string) {
	var metadata map[string]any
	if rawMeta, exists := body["metadata"]; exists && len(rawMeta) > 0 {
		if err := json.Unmarshal(rawMeta, &metadata); err != nil {
			metadata = map[string]any{}
		}
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	if cfg.UserID != "" {
		if existing, ok := metadata["user_id"].(string); !ok || existing == "" {
			metadata["user_id"] = encodeIdentity(cfg.UserID, cfg.DeviceID, sessionID)
		}
	}
	raw, _ := json.Marshal(metadata)
	body["metadata"] = raw
}

// encodeIdentity composes user_id with device/session markers.
func encodeIdentity(userID, deviceID, sessionID string) string {
	if deviceID != "" && sessionID != "" {
		return userID + "-" + deviceID + "-" + sessionID
	}
	if deviceID != "" {
		return userID + "-" + deviceID
	}
	return userID
}

// resolveSessionID determines the session ID for the current request.
//
// Priority:
//  1. cfg.SessionID (static config value)
//  2. Derived from auth_index + auth_id via SHA-256 → UUID v4-shaped hex
//  3. Falls back to RequestID to avoid all-anonymous requests sharing one ID
func resolveSessionID(req *requestInterceptRequest, cfg config) string {
	if cfg.SessionID != "" {
		return cfg.SessionID
	}

	authIndex := strconv.Itoa(req.AuthIndex)
	authID := strings.TrimSpace(req.AuthID)
	if authID == "" {
		if v, ok := req.Metadata["auth_id"].(string); ok {
			authID = strings.TrimSpace(v)
		}
	}

	var seed string
	if authID != "" {
		seed = authIndex + "\x00" + authID + "\x00claude-code-session"
	} else {
		// Fall back to request-scoped seed so anonymous requests are distinct.
		seed = req.RequestID + "\x00claude-code-session"
	}
	return deriveSessionUUID(seed)
}

// deriveSessionUUID hashes seed with SHA-256 and formats the first 16 bytes
// as a lowercase UUID (xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx layout).
func deriveSessionUUID(seed string) string {
	h := sha256.Sum256([]byte(seed))
	// Force version 4 and variant bits as per RFC 4122.
	b := h[:16]
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", raw[0:8], raw[8:12], raw[12:16], raw[16:20], raw[20:32])
}

// injectHeaders builds the header map to be set by the host, based on the
// effective profile. The session ID is injected for minimum/full/custom
// profiles when present.
func injectHeaders(req *requestInterceptRequest, cfg config, profile string, sessionID string) map[string][]string {
	out := map[string][]string{}

	switch profile {
	case profilePreserve, "":
		// Nothing to add; caller handles User-Agent removal separately.
		return out

	case profileBeta:
		out["anthropic-beta"] = []string{betaFull}
		return out

	case profileMinimum:
		for k, v := range minimumHeaders {
			out[k] = []string{v}
		}
		if sessionID != "" {
			out[headerSessionID] = []string{sessionID}
		}
		return out

	case profileFull:
		for k, v := range fullHeaders {
			out[k] = []string{v}
		}
		if sessionID != "" {
			out[headerSessionID] = []string{sessionID}
		}
		return out

	case profileCustom:
		for k, v := range cfg.CustomHeaders {
			out[k] = []string{v}
		}
		if sessionID != "" {
			if _, alreadySet := cfg.CustomHeaders[headerSessionID]; !alreadySet {
				out[headerSessionID] = []string{sessionID}
			}
		}
		return out
	}

	return out
}

// clearHeadersForProfile returns the list of header names that should be
// deleted from the upstream request for the given profile.
func clearHeadersForProfile(profile string, clearUserAgent bool) []string {
	var list []string
	// For full fingerprint profiles the proxy may have added its own request
	// ID which contradicts the CLI identity; strip it.
	if profile == profileFull || profile == profileMinimum {
		list = append(list, headerClientRequestID)
	}
	if clearUserAgent && profile == profilePreserve {
		list = append(list, "User-Agent")
	}
	return list
}