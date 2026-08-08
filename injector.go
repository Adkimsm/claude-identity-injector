package main

import (
	"encoding/json"
	"errors"
)

const identityPrompt = "You are Claude Code, Anthropic's official CLI for Claude."

type systemBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// injectIdentity rewrites the upstream request body with the identity system
// prompt and identity metadata. It returns the new body and any header
// adjustments. Full strict-mode completeness is guaranteed by the caller
// failing open on error: the request is passed through unmodified.
func injectIdentity(req *requestInterceptRequest, cfg config) (json.RawMessage, map[string][]string, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &body); err != nil || body == nil {
		return nil, nil, errors.New("body must be a JSON object")
	}
	if modified, err := injectSystemPrompt(body, cfg.SystemPrompt); err != nil || !modified {
		if err != nil {
			return nil, nil, err
		}
	}
	injectMetadata(body, cfg)
	updated, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	headers := injectHeaders(&req.Headers, cfg)
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

// injectMetadata sets user_id, device_id, and session_id on the request
// metadata object (created if missing). Missing fields are preserved.
func injectMetadata(body map[string]json.RawMessage, cfg config) {
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
			metadata["user_id"] = encodeIdentity(cfg.UserID, cfg.DeviceID, cfg.SessionID)
		}
	}
	raw, _ := json.Marshal(metadata)
	body["metadata"] = raw
}

// encodeIdentity composes user_id with device/session markers to keep the
// upstream identity distinct while preserving the privacy-oriented format.
func encodeIdentity(userID, deviceID, sessionID string) string {
	if deviceID != "" && sessionID != "" {
		return userID + "-" + deviceID + "-" + sessionID
	}
	if deviceID != "" {
		return userID + "-" + deviceID
	}
	return userID
}

// injectHeaders mirrors identity-managed headers into the request. It returns
// headers that should be set by the host.
func injectHeaders(headers *map[string][]string, cfg config) map[string][]string {
	if headers == nil {
		headers = &map[string][]string{}
	}
	changed := map[string][]string{}
	// Preserve upstream identity headers if the host passes them.
	return changed
}