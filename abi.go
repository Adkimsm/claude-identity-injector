package main

import "encoding/json"

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type lifecycleRequest struct {
	ConfigYAML    []byte `json:"config_yaml"`
	SchemaVersion uint32 `json:"schema_version"`
}

type registration struct {
	SchemaVersion uint32               `json:"schema_version"`
	Metadata      registrationMetadata `json:"metadata"`
	Capabilities  registrationCaps     `json:"capabilities"`
}

type registrationMetadata struct {
	Name             string `json:"Name"`
	Version          string `json:"Version"`
	Author           string `json:"Author"`
	GitHubRepository string `json:"GitHubRepository"`
}

type registrationCaps struct {
	RequestInterceptor       bool `json:"request_interceptor"`
	ResponseInterceptor      bool `json:"response_interceptor"`
	StreamChunkInterceptor   bool `json:"stream_chunk_interceptor"`
	ManagementAPI            bool `json:"management_api"`
	RequestLifecyclePlugin   bool `json:"request_lifecycle_plugin"`
}

type requestInterceptRequest struct {
	RequestID      string         `json:"RequestID"`
	TraceID        string         `json:"TraceID"`
	SourceFormat   string         `json:"SourceFormat"`
	ToFormat       string         `json:"ToFormat"`
	Model          string         `json:"Model"`
	RequestedModel string         `json:"RequestedModel"`
	Stream         bool           `json:"Stream"`
	Headers        map[string][]string `json:"Headers"`
	Body           json.RawMessage `json:"Body"`
	Metadata       map[string]any `json:"Metadata"`
}

type requestInterceptResponse struct {
	Headers        map[string][]string `json:"Headers,omitempty"`
	Body           json.RawMessage     `json:"Body,omitempty"`
	ClearHeaders   []string            `json:"ClearHeaders,omitempty"`
}

type streamChunkInterceptRequest struct {
	RequestID       string             `json:"RequestID"`
	SourceFormat    string             `json:"SourceFormat"`
	Model           string             `json:"Model"`
	RequestedModel  string             `json:"RequestedModel"`
	Body            json.RawMessage    `json:"Body"`
	HistoryChunks   []json.RawMessage  `json:"HistoryChunks"`
	ChunkIndex      int                `json:"ChunkIndex"`
	Metadata        map[string]any     `json:"Metadata"`
	ResponseHeaders map[string][]string `json:"ResponseHeaders"`
}

type streamChunkInterceptResponse struct {
	Headers      map[string][]string `json:"Headers,omitempty"`
	Body         json.RawMessage     `json:"Body,omitempty"`
	ClearHeaders []string            `json:"ClearHeaders,omitempty"`
	DropChunk    bool                `json:"DropChunk,omitempty"`
}

// Management / resource route registration
type managementRegistration struct {
	Routes    []managementRoute    `json:"routes,omitempty"`
	Resources []managementResource `json:"resources,omitempty"`
}

type managementRoute struct {
	Method      string `json:"Method"`
	Path        string `json:"Path"`
	Menu        string `json:"Menu,omitempty"`
	Description string `json:"Description,omitempty"`
}

type managementResource struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu"`
	Description string `json:"Description"`
}

type managementRequest struct {
	Method  string              `json:"Method"`
	Path    string              `json:"Path"`
	Headers map[string][]string `json:"Headers"`
	Query   map[string][]string `json:"Query"`
	Body    json.RawMessage     `json:"Body"`
}

type managementResponse struct {
	StatusCode int                 `json:"StatusCode"`
	Headers    map[string][]string `json:"Headers,omitempty"`
	Body       []byte              `json:"Body,omitempty"`
}