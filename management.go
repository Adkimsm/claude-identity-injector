package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type aggregatedHost struct {
	Version          string             `json:"version"`
	PluginEnabled    bool               `json:"plugin_enabled"`
	VersionSeconds   int64              `json:"uptime_seconds"`
	Counters         map[string]uint64  `json:"counters"`
	Tool             configSnapshot     `json:"config"`
	RepairedTools    int64              `json:"repaired_tools"`
}

type configSnapshot struct {
	Active        bool           `json:"active"`
	StrictMode    bool           `json:"strict_mode"`
	Provider      string         `json:"provider"`
	ClearUserAgent bool          `json:"clear_user_agent"`
	RuleCount     int            `json:"rule_count"`
	Rules         []snapshotRule `json:"rules"`
}

type snapshotRule struct {
	ID        string   `json:"id"`
	Enabled   bool     `json:"enabled"`
	Providers []string `json:"providers,omitempty"`
	Requested []string `json:"requested_models,omitempty"`
	Upstream  []string `json:"upstream_models,omitempty"`
}

func handleManagement(raw []byte) ([]byte, error) {
	var req managementRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	path := strings.TrimSpace(req.Path)
	switch {
	case strings.HasSuffix(path, "/status.json"):
		body, _ := json.Marshal(currentStatusReport())
		return okEnvelope(managementResponse{
			StatusCode: http.StatusOK,
			Headers: map[string][]string{
				"content-type": {"application/json; charset=utf-8"},
			},
			Body: body,
		})
	case strings.HasSuffix(path, "/status"):
		page, _ := statusPageHTML()
		return okEnvelope(managementResponse{
			StatusCode: http.StatusOK,
			Headers: map[string][]string{
				"content-type": {"text/html; charset=utf-8"},
			},
			Body: page,
		})
	}
	return okEnvelope(managementResponse{StatusCode: http.StatusNotFound})
}

func currentStatusReport() aggregatedHost {
	cfg := currentConfig()
	state := currentMetrics()
	snap := configSnapshot{
		Active:         cfg.Active,
		StrictMode:     cfg.StrictMode,
		Provider:       cfg.ProviderMatch,
		ClearUserAgent: cfg.ClearUserAgent,
		RuleCount:      len(cfg.Rules),
	}
	for index := range cfg.Rules {
		r := &cfg.Rules[index]
		snap.Rules = append(snap.Rules, snapshotRule{
			ID:        r.ID,
			Enabled:   r.Enabled,
			Providers: r.Providers,
			Requested: r.RequestedModels,
			Upstream:  r.UpstreamModels,
		})
	}
	total := map[string]uint64{
		"seen":      state.seen,
		"matched":   state.matched,
		"injected":  state.injected,
		"unmatched": state.unmatched,
		"errors":    state.errors,
	}
	return aggregatedHost{
		Version:        pluginVersion,
		PluginEnabled:  cfg.Enabled,
		VersionSeconds: state.uptime,
		Counters:       total,
		Tool:           snap,
		RepairedTools:  int64(state.repairedTools),
	}
}