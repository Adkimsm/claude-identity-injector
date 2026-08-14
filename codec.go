package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// decodeBase64Body extracts the JSON payload from a CPA intercept Body field.
// The host encodes all Body fields as base64 strings inside a JSON string
// literal, so raw is expected to be a quoted base64 string like
// "eyJtb2RlbCI6...".
func decodeBase64Body(raw json.RawMessage) ([]byte, error) {
	var b64 string
	if err := json.Unmarshal(raw, &b64); err != nil {
		return nil, fmt.Errorf("body must be a base64 string: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("body base64 decode: %w", err)
	}
	return decoded, nil
}

// encodeBase64Body wraps a JSON payload back into the CPA intercept Body
// format: a base64 string inside a JSON string literal.
func encodeBase64Body(data []byte) (json.RawMessage, error) {
	b64 := base64.StdEncoding.EncodeToString(data)
	raw, err := json.Marshal(b64)
	if err != nil {
		return nil, fmt.Errorf("body base64 encode: %w", err)
	}
	return raw, nil
}
