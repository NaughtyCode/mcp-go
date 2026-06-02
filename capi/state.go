package capi

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ServerSnapshotJSON returns a JSON object describing server state, counters,
// registered public API object counts, and transport state.
func ServerSnapshotJSON(handle ServerHandle) ([]byte, error) {
	entry, err := LookupServer(handle)
	if err != nil {
		return nil, err
	}
	return json.Marshal(entry.snapshot())
}

// ServerMetricsJSON returns only the server metrics counters as JSON.
func ServerMetricsJSON(handle ServerHandle) ([]byte, error) {
	entry, err := LookupServer(handle)
	if err != nil {
		return nil, err
	}
	return json.Marshal(entry.metricsSnapshot())
}

// TransportSnapshotJSON returns a JSON object describing one transport.
func TransportSnapshotJSON(handle TransportHandle) ([]byte, error) {
	entry, ok := transportRegistry.get(uint64(handle))
	if !ok {
		return nil, ErrInvalidTransportHandle
	}
	snapshot := entry.snapshot(handle)
	return json.Marshal(snapshot)
}

func (s *Server) snapshot() ServerSnapshot {
	s.mu.Lock()
	state := s.state
	templateCount := s.templateCount
	transportHandles := make([]TransportHandle, 0, len(s.transports))
	for handle := range s.transports {
		transportHandles = append(transportHandles, handle)
	}
	s.mu.Unlock()

	transports := make([]TransportSnapshot, 0, len(transportHandles))
	for _, handle := range transportHandles {
		if transport, ok := transportRegistry.get(uint64(handle)); ok {
			transports = append(transports, transport.snapshot(handle))
		}
	}

	registered := s.registeredSnapshot()
	registered.ResourceTemplates = templateCount

	return ServerSnapshot{
		Handle:     uint64(s.handle),
		State:      state,
		CreatedAt:  s.createdAt.UTC().Format(time.RFC3339Nano),
		UptimeMS:   time.Since(s.createdAt).Milliseconds(),
		Registered: registered,
		Metrics:    s.metricsSnapshot(),
		Transports: transports,
	}
}

func (s *Server) registeredSnapshot() RegisteredSnapshot {
	var registered RegisteredSnapshot
	if tools := s.server.ListTools(); tools != nil {
		registered.Tools = len(tools)
	}
	if resources := s.server.ListResources(); resources != nil {
		registered.Resources = len(resources)
	}
	if prompts := s.server.ListPrompts(); prompts != nil {
		registered.Prompts = len(prompts)
	}
	return registered
}

func (s *Server) metricsSnapshot() ServerMetrics {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	return s.metrics
}

func (s *Server) setState(state string) {
	s.mu.Lock()
	s.state = state
	s.mu.Unlock()
}

func (s *Server) recordMessage(method string, hasID bool) {
	now := nowString()
	s.metricsMu.Lock()
	s.metrics.RequestsTotal++
	s.metrics.LastRequestAt = now
	if !hasID {
		s.metrics.NotificationsTotal++
	}
	switch mcp.MCPMethod(method) {
	case mcp.MethodToolsCall:
		s.metrics.ToolCallsTotal++
	case mcp.MethodResourcesRead:
		s.metrics.ResourceReadsTotal++
	case mcp.MethodPromptsGet:
		s.metrics.PromptGetsTotal++
	}
	s.metricsMu.Unlock()
}

func (s *Server) recordResponse(responseJSON []byte) {
	s.metricsMu.Lock()
	s.metrics.ResponsesTotal++
	if bytes.Contains(responseJSON, []byte(`"error"`)) {
		s.metrics.ErrorsTotal++
		s.metrics.LastErrorAt = nowString()
	}
	s.metricsMu.Unlock()
}

func (s *Server) recordError() {
	s.metricsMu.Lock()
	s.metrics.ErrorsTotal++
	s.metrics.LastErrorAt = nowString()
	s.metricsMu.Unlock()
}

func (s *Server) recordCallbackError() {
	s.metricsMu.Lock()
	s.metrics.CallbackErrorsTotal++
	s.metrics.ErrorsTotal++
	s.metrics.LastErrorAt = nowString()
	s.metricsMu.Unlock()
}

func (s *Server) recordTransportStarted() {
	s.metricsMu.Lock()
	s.metrics.TransportsStarted++
	s.metrics.LastTransportStartAt = nowString()
	s.metricsMu.Unlock()
}

func (s *Server) recordTransportStopped() {
	s.metricsMu.Lock()
	s.metrics.TransportsStopped++
	s.metrics.LastTransportStopAt = nowString()
	s.metricsMu.Unlock()
}

func (s *Server) recordToolRegistration() {
	s.metricsMu.Lock()
	s.metrics.ToolsRegistered++
	s.metricsMu.Unlock()
}

func (s *Server) recordResourceRegistration() {
	s.metricsMu.Lock()
	s.metrics.ResourcesRegistered++
	s.metricsMu.Unlock()
}

func (s *Server) recordTemplateRegistration() {
	s.metricsMu.Lock()
	s.metrics.TemplatesRegistered++
	s.metricsMu.Unlock()

	s.mu.Lock()
	s.templateCount++
	s.mu.Unlock()
}

func (s *Server) recordPromptRegistration() {
	s.metricsMu.Lock()
	s.metrics.PromptsRegistered++
	s.metricsMu.Unlock()
}

func (s *Server) recordNotificationSent() {
	s.metricsMu.Lock()
	s.metrics.NotificationsSent++
	s.metricsMu.Unlock()
}

func (t *Transport) snapshot(handle TransportHandle) TransportSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()

	snapshot := TransportSnapshot{
		Handle:        uint64(handle),
		Server:        uint64(t.serverHandle),
		Kind:          t.kind,
		State:         t.state,
		StartedAt:     t.startedAt.UTC().Format(time.RFC3339Nano),
		ShutdownCount: t.shutdownCount,
	}
	if t.stoppedAt != nil {
		snapshot.StoppedAt = t.stoppedAt.UTC().Format(time.RFC3339Nano)
	}
	if t.lastErr != nil {
		snapshot.LastError = t.lastErr.Error()
	}
	if t.lastErrorAt != nil {
		snapshot.LastErrorAt = t.lastErrorAt.UTC().Format(time.RFC3339Nano)
	}
	return snapshot
}

func (t *Transport) markStopped() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.state = "stopped"
	t.stoppedAt = &now
	t.shutdownCount++
}
