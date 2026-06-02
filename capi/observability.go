package capi

import (
	"encoding/json"
	"sync"
	"time"
)

const (
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
)

// LogEntry is one structured C API log record.
type LogEntry struct {
	Sequence  uint64         `json:"sequence"`
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Event     string         `json:"event"`
	Message   string         `json:"message"`
	Server    uint64         `json:"server,omitempty"`
	Transport uint64         `json:"transport,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// LogCallback receives each structured log record after it is stored.
type LogCallback func(LogEntry)

// ServerMetrics contains counters that make the C API observable.
type ServerMetrics struct {
	RequestsTotal        uint64 `json:"requestsTotal"`
	NotificationsTotal   uint64 `json:"notificationsTotal"`
	ResponsesTotal       uint64 `json:"responsesTotal"`
	ErrorsTotal          uint64 `json:"errorsTotal"`
	ToolCallsTotal       uint64 `json:"toolCallsTotal"`
	ResourceReadsTotal   uint64 `json:"resourceReadsTotal"`
	PromptGetsTotal      uint64 `json:"promptGetsTotal"`
	CallbackErrorsTotal  uint64 `json:"callbackErrorsTotal"`
	TransportsStarted    uint64 `json:"transportsStarted"`
	TransportsStopped    uint64 `json:"transportsStopped"`
	ToolsRegistered      uint64 `json:"toolsRegistered"`
	ResourcesRegistered  uint64 `json:"resourcesRegistered"`
	TemplatesRegistered  uint64 `json:"templatesRegistered"`
	PromptsRegistered    uint64 `json:"promptsRegistered"`
	NotificationsSent    uint64 `json:"notificationsSent"`
	LastRequestAt        string `json:"lastRequestAt,omitempty"`
	LastErrorAt          string `json:"lastErrorAt,omitempty"`
	LastTransportStartAt string `json:"lastTransportStartAt,omitempty"`
	LastTransportStopAt  string `json:"lastTransportStopAt,omitempty"`
}

// ServerSnapshot is a point-in-time view of an MCP server exposed through the C
// API.
type ServerSnapshot struct {
	Handle     uint64              `json:"handle"`
	State      string              `json:"state"`
	CreatedAt  string              `json:"createdAt"`
	UptimeMS   int64               `json:"uptimeMs"`
	Registered RegisteredSnapshot  `json:"registered"`
	Metrics    ServerMetrics       `json:"metrics"`
	Transports []TransportSnapshot `json:"transports"`
}

// RegisteredSnapshot contains current registered object counts.
type RegisteredSnapshot struct {
	Tools             int `json:"tools"`
	Resources         int `json:"resources"`
	ResourceTemplates int `json:"resourceTemplates"`
	Prompts           int `json:"prompts"`
}

// TransportSnapshot is a point-in-time view of a started transport.
type TransportSnapshot struct {
	Handle        uint64 `json:"handle"`
	Server        uint64 `json:"server"`
	Kind          string `json:"kind"`
	State         string `json:"state"`
	StartedAt     string `json:"startedAt"`
	StoppedAt     string `json:"stoppedAt,omitempty"`
	LastError     string `json:"lastError,omitempty"`
	LastErrorAt   string `json:"lastErrorAt,omitempty"`
	ShutdownCount uint64 `json:"shutdownCount"`
}

var logs = struct {
	sync.Mutex
	next     uint64
	capacity int
	entries  []LogEntry
	callback LogCallback
}{
	next:     1,
	capacity: 1024,
}

// SetLogCapacity configures the in-memory structured log ring capacity.
func SetLogCapacity(capacity int) {
	logs.Lock()
	defer logs.Unlock()
	if capacity <= 0 {
		capacity = 1
	}
	logs.capacity = capacity
	if len(logs.entries) > capacity {
		logs.entries = logs.entries[len(logs.entries)-capacity:]
	}
}

// SetLogCallback registers a process-wide callback for new structured logs.
func SetLogCallback(callback LogCallback) {
	logs.Lock()
	logs.callback = callback
	logs.Unlock()
}

// LogsSnapshotJSON returns stored structured logs as JSON. When server is
// non-zero, only entries for that server and global entries are returned.
func LogsSnapshotJSON(server ServerHandle) ([]byte, error) {
	logs.Lock()
	defer logs.Unlock()
	entries := make([]LogEntry, 0, len(logs.entries))
	for _, entry := range logs.entries {
		if server == 0 || entry.Server == 0 || entry.Server == uint64(server) {
			entries = append(entries, entry)
		}
	}
	return json.Marshal(struct {
		Logs []LogEntry `json:"logs"`
	}{
		Logs: entries,
	})
}

// ClearLogs clears stored structured logs. When server is non-zero, only that
// server's entries are removed; global entries are retained.
func ClearLogs(server ServerHandle) {
	logs.Lock()
	defer logs.Unlock()
	if server == 0 {
		logs.entries = nil
		return
	}
	filtered := logs.entries[:0]
	for _, entry := range logs.entries {
		if entry.Server != uint64(server) {
			filtered = append(filtered, entry)
		}
	}
	logs.entries = filtered
}

// RecordLog stores one structured log and invokes the optional log callback.
func RecordLog(level string, server ServerHandle, transport TransportHandle, event, message string, fields map[string]any) {
	if level == "" {
		level = logLevelInfo
	}
	if fields != nil {
		copied := make(map[string]any, len(fields))
		for k, v := range fields {
			copied[k] = v
		}
		fields = copied
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Event:     event,
		Message:   message,
		Server:    uint64(server),
		Transport: uint64(transport),
		Fields:    fields,
	}

	var callback LogCallback
	logs.Lock()
	entry.Sequence = logs.next
	logs.next++
	logs.entries = append(logs.entries, entry)
	if len(logs.entries) > logs.capacity {
		logs.entries = logs.entries[len(logs.entries)-logs.capacity:]
	}
	callback = logs.callback
	logs.Unlock()

	if callback != nil {
		callback(entry)
	}
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
