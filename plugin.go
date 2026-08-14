package main

import (
	"encoding/json"
	"errors"
)

const (
	methodPluginRegister        = "plugin.register"
	methodPluginReconfigure     = "plugin.reconfigure"
	methodPluginShutdown        = "plugin.shutdown"
	methodRequestInterceptBefore = "request.intercept_before"
	methodRequestInterceptAfter  = "request.intercept_after"
	methodResponseIntercept      = "response.intercept_after"
	methodResponseStreamChunk    = "response.intercept_stream_chunk"
	methodRequestComplete        = "request.complete"
	methodManagementRegister     = "management.register"
	methodManagementHandle       = "management.handle"
)

func handleMethod(method string, raw []byte) ([]byte, error) {
	switch method {
	case methodPluginRegister, methodPluginReconfigure:
		return handleLifecycle(method, raw)
	case methodPluginShutdown:
		handleShutdown()
		return okEnvelope(map[string]any{})
	case methodRequestInterceptBefore, methodRequestInterceptAfter:
		return handleRequestInterceptAfter(raw)
	case methodResponseIntercept:
		return handleResponseIntercept(raw)
	case methodResponseStreamChunk:
		return handleStreamChunk(raw)
	case methodRequestComplete:
		return handleRequestComplete(raw)
	case methodManagementRegister:
		return okEnvelope(pluginManagementRegistration())
	case methodManagementHandle:
		return handleManagement(raw)
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func handleShutdown() {
	clearRequestMetrics()
}

func handleRequestInterceptAfter(raw []byte) ([]byte, error) {
	var req requestInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if len(req.Body) == 0 {
		return okEnvelope(requestInterceptResponse{})
	}
	cfg := currentConfig()
	if !cfg.Active {
		return okEnvelope(requestInterceptResponse{})
	}
	matched := firstMatchingRule(cfg.Rules, &req)
	if matched == nil {
		recordRequestMetric(&req, "unmatched")
		return okEnvelope(requestInterceptResponse{})
	}

	profile := effectiveHeaderProfile(cfg, matched)

	updated, headers, errInject := injectIdentity(&req, cfg, profile)
	if errInject != nil {
		recordRequestMetric(&req, "error")
		logHost("", "error", "claude-identity-injector-v2 injection failed", map[string]any{
			"error": errInject.Error(),
		})
		return okEnvelope(requestInterceptResponse{})
	}
	clearHeaders := clearHeadersForProfile(profile, cfg.ClearUserAgent)
	recordRequestMetric(&req, "injected")
	if cfg.LogEnabled {
		logHost("", "info", "claude-identity-injector-v2 injected identity", map[string]any{
			"request_id":      req.RequestID,
			"source_format":   req.SourceFormat,
			"to_format":       req.ToFormat,
			"model":           req.Model,
			"requested_model":  req.RequestedModel,
			"provider":        cfg.ProviderMatch,
			"rule":            matched.ID,
			"header_profile":  profile,
		})
	}
	return okEnvelope(requestInterceptResponse{
		Body:         updated,
		Headers:      headers,
		ClearHeaders: clearHeaders,
		ForceHTTP1:   profileForcesHTTP1(profile),
	})
}

// handleResponseIntercept handles non-streaming response interception.
// For the identity-only profile, responses need no structural repair.
func handleResponseIntercept(raw []byte) ([]byte, error) {
	var req streamChunkInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	jsonBody, err := decodeBase64Body(req.Body)
	if err != nil {
		return okEnvelope(struct {
			Body json.RawMessage `json:"Body,omitempty"`
		}{})
	}
	updated, changed := repairStreamBody(json.RawMessage(jsonBody))
	if !changed {
		return okEnvelope(struct {
			Body json.RawMessage `json:"Body,omitempty"`
		}{})
	}
	encoded, err := encodeBase64Body(updated)
	if err != nil {
		return okEnvelope(struct {
			Body json.RawMessage `json:"Body,omitempty"`
		}{})
	}
	return okEnvelope(struct {
		Body json.RawMessage `json:"Body"`
	}{Body: encoded})
}

func handleStreamChunk(raw []byte) ([]byte, error) {
	var req streamChunkInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	jsonBody, err := decodeBase64Body(req.Body)
	if err != nil {
		return okEnvelope(streamChunkInterceptResponse{})
	}
	updated, changed := repairStreamBody(json.RawMessage(jsonBody))
	if !changed {
		return okEnvelope(streamChunkInterceptResponse{})
	}
	recordToolRepair(&req)
	encoded, err := encodeBase64Body(updated)
	if err != nil {
		return okEnvelope(streamChunkInterceptResponse{})
	}
	return okEnvelope(streamChunkInterceptResponse{Body: encoded})
}

func handleRequestComplete(raw []byte) ([]byte, error) {
	var req struct {
		RequestID string `json:"RequestID"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	_ = req.RequestID
	return okEnvelope(map[string]any{})
}

func pluginManagementRegistration() managementRegistration {
	return managementRegistration{
		Resources: []managementResource{{
			Path:        "/status",
			Menu:        "Claude Identity Injector",
			Description: "状态与配置",
		}, {
			Path:        "/status.json",
			Menu:        "",
			Description: "",
		}},
	}
}

var errUnsupported = errors.New("unsupported operation")