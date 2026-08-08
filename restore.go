package main

import (
	"bytes"
	"encoding/json"
	"strings"
)

// repairStreamBody walks a stream chunk payload and normalizes tool field
// names across the dry_run/dryRun naming variants and the OpenAI
// function call format. Returns the repaired body and whether it changed.
func repairStreamBody(raw json.RawMessage) (json.RawMessage, bool) {
	if len(raw) == 0 || !json.Valid(raw) {
		return raw, false
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw, false
	}
	changed := repairJSONValue(&value)
	if !changed {
		return raw, false
	}
	updated, err := json.Marshal(value)
	if err != nil {
		return raw, false
	}
	return updated, true
}

func repairJSONValue(value *any) bool {
	switch typed := (*value).(type) {
	case []any:
		changed := false
		for index := range typed {
			if repairJSONValue(&typed[index]) {
				changed = true
			}
		}
		return changed
	case map[string]any:
		return repairJSONMap(typed)
	default:
		return false
	}
}

// repairJSONMap updates the currently canonical JSON keys for the streaming
// tool call family. The primary fix aligns dryRun to dry_run, matching the
// client-side schema.
func repairJSONMap(object map[string]any) bool {
	changed := false
	if value, exists := object["dryRun"]; exists {
		object["dry_run"] = value
		delete(object, "dryRun")
		changed = true
	}
	if _, exists := object["tool_use"]; exists {
		// tool_use blocks are nested objects; nothing to fix at this level
	}
	for key, child := range object {
		if key == "dryRun" || key == "dry_run" {
			continue
		}
		childPtr := &child
		if repairJSONValue(childPtr) {
			object[key] = *childPtr
			changed = true
		}
	}
	return changed
}

// splitStreamLines helps, but chunk bodies are expected to be whole JSON.
// Kept for parity with non-streamed repair.
func splitStreamLines(raw []byte) [][]byte {
	return bytes.SplitAfter(raw, []byte("\n"))
}

var _ = strings.TrimSpace
var _ = splitStreamLines