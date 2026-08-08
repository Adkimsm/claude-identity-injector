package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type config struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	Priority      int      `yaml:"priority" json:"priority"`
	Active        bool     `yaml:"active" json:"active"`
	StrictMode    bool     `yaml:"strict_mode" json:"strict_mode"`
	ProviderMatch string   `yaml:"provider" json:"provider"`
	SystemPrompt  string   `yaml:"system_prompt" json:"system_prompt"`
	UserID        string   `yaml:"user_id" json:"user_id"`
	DeviceID      string   `yaml:"device_id" json:"device_id"`
	SessionID     string   `yaml:"session_id" json:"session_id"`
	ClearUserAgent bool    `yaml:"clear_user_agent" json:"clear_user_agent"`
	Rules         []rule   `yaml:"rules" json:"rules"`
}

type rule struct {
	ID                   string   `yaml:"id" json:"id"`
	Enabled              bool     `yaml:"enabled" json:"enabled"`
	Providers            []string `yaml:"providers" json:"providers"`
	ProviderAuthIndexes  []string `yaml:"provider_auth_indexes" json:"provider_auth_indexes"`
	RequestedModels      []string `yaml:"requested_models" json:"requested_models"`
	UpstreamModels       []string `yaml:"upstream_models" json:"upstream_models"`

	requestedPatterns []*regexp.Regexp
	upstreamPatterns  []*regexp.Regexp
}

var configState struct {
	sync.RWMutex
	value config
}

func defaultConfig() config {
	return config{
		Enabled:          true,
		Priority:         100,
		Active:           false,
		StrictMode:       true,
		SystemPrompt:     identityPrompt,
		ClearUserAgent:   false,
		Rules:            []rule{},
	}
}

func currentConfig() config {
	configState.RLock()
	defer configState.RUnlock()
	return configState.value
}

func handleLifecycle(method string, raw []byte) ([]byte, error) {
	var req lifecycleRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("decode lifecycle request: %w", err)
		}
	}
	next, errParse := parseConfig(req.ConfigYAML)
	if errParse != nil {
		logHost("", "error", "claude-identity-injector-v2 rejected invalid configuration", map[string]any{"error": errParse.Error()})
		return okEnvelope(pluginRegistration())
	}
	configState.Lock()
	configState.value = next
	configState.Unlock()
	logHost("", "info", "claude-identity-injector-v2 configuration applied", map[string]any{
		"active":      next.Active,
		"strict_mode": next.StrictMode,
		"provider":    next.ProviderMatch,
		"rules":       len(next.Rules),
		"method":      method,
	})
	return okEnvelope(pluginRegistration())
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: schemaVersion,
		Metadata: registrationMetadata{
			Name:             "Claude Identity Injector v2",
			Version:          pluginVersion,
			Author:           "Octobersama",
			GitHubRepository: "https://github.com/Octobersama/claude-identity-injector",
		},
		Capabilities: registrationCaps{
			RequestInterceptor:       true,
			ResponseInterceptor:      true,
			StreamChunkInterceptor:   true,
			ManagementAPI:            true,
		},
	}
}

func parseConfig(raw json.RawMessage) (config, error) {
	next := defaultConfig()
	if len(strings.TrimSpace(string(raw))) == 0 {
		return next, nil
	}
	if err := yaml.Unmarshal(raw, &next); err != nil {
		return config{}, fmt.Errorf("decode config YAML: %w", err)
	}
	next.ProviderMatch = strings.ToLower(strings.TrimSpace(next.ProviderMatch))
	next.SystemPrompt = strings.TrimSpace(next.SystemPrompt)
	if next.SystemPrompt == "" {
		next.SystemPrompt = defaultConfig().SystemPrompt
	}

	seen := make(map[string]struct{}, len(next.Rules))
	for index := range next.Rules {
		r := &next.Rules[index]
		r.ID = strings.TrimSpace(r.ID)
		if r.ID == "" {
			return config{}, fmt.Errorf("rules[%d].id is required", index)
		}
		if _, exists := seen[r.ID]; exists {
			return config{}, fmt.Errorf("duplicate rule id %q", r.ID)
		}
		seen[r.ID] = struct{}{}
		r.Providers = cleanList(r.Providers)
		r.ProviderAuthIndexes = cleanList(r.ProviderAuthIndexes)
		r.RequestedModels = cleanList(r.RequestedModels)
		r.UpstreamModels = cleanList(r.UpstreamModels)
		var errCompile error
		r.requestedPatterns, errCompile = compileGlobs(r.RequestedModels)
		if errCompile != nil {
			return config{}, fmt.Errorf("rule %q requested_models: %w", r.ID, errCompile)
		}
		r.upstreamPatterns, errCompile = compileGlobs(r.UpstreamModels)
		if errCompile != nil {
			return config{}, fmt.Errorf("rule %q upstream_models: %w", r.ID, errCompile)
		}
	}
	return next, nil
}

func cleanList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func compileGlobs(patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		var expr strings.Builder
		expr.WriteString("^")
		for _, char := range pattern {
			switch char {
			case '*':
				expr.WriteString(".*")
			case '?':
				expr.WriteString(".")
			default:
				expr.WriteString(regexp.QuoteMeta(string(char)))
			}
		}
		expr.WriteString("$")
		compiled, err := regexp.Compile(expr.String())
		if err != nil {
			return nil, err
		}
		out = append(out, compiled)
	}
	return out, nil
}

func firstMatchingRule(rules []rule, req *requestInterceptRequest) *rule {
	for index := range rules {
		r := &rules[index]
		if !r.Enabled || !matchesRule(*r, req) {
			continue
		}
		return r
	}
	return nil
}

// deriveProvider extracts a provider-ish key from the request context available
// in the v2 intercept seam. ParentRequest provider resolution is limited to
// the upstream model prefix and metadata hints.
func deriveProvider(req *requestInterceptRequest) string {
	if v, ok := req.Metadata["provider"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := req.Metadata["auth_provider"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}
	model := strings.ToLower(req.Model)
	for _, prefix := range []string{"claude-", "anthropic"} {
		if strings.HasPrefix(model, prefix) {
			return prefix
		}
	}
	return ""
}

func matchesRule(r rule, req *requestInterceptRequest) bool {
	if len(r.Providers) > 0 {
		provider := deriveProvider(req)
		if !matchesFold(r.Providers, provider) {
			return false
		}
	}
	if len(r.RequestedModels) > 0 && !matchesPatterns(r.requestedPatterns, req.RequestedModel) {
		return false
	}
	if len(r.UpstreamModels) > 0 && !matchesPatterns(r.upstreamPatterns, req.Model) {
		return false
	}
	return true
}

func matchesFold(values []string, actual string) bool {
	if len(values) == 0 {
		return true
	}
	for _, value := range values {
		if strings.EqualFold(value, actual) {
			return true
		}
	}
	return false
}

func matchesPatterns(patterns []*regexp.Regexp, actual string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if pattern.MatchString(actual) {
			return true
		}
	}
	return false
}