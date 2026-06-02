package capi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// StatusOK is returned by the C ABI when an operation succeeds.
const StatusOK = 0

// StatusError is returned by the C ABI when an operation fails. Callers should
// retrieve the detailed message through the C ABI's last-error function.
const StatusError = -1

// ErrInvalidServerHandle is returned when a server handle does not reference a
// live server in this process.
var ErrInvalidServerHandle = errors.New("invalid server handle")

// ErrInvalidTransportHandle is returned when a transport handle does not
// reference a live transport in this process.
var ErrInvalidTransportHandle = errors.New("invalid transport handle")

// ServerHandle is an opaque process-local identifier for an MCP server.
type ServerHandle uint64

// TransportHandle is an opaque process-local identifier for a running
// transport owned by an MCP server.
type TransportHandle uint64

// JSONCallback is the language-neutral callback shape used by the C API.
// The request JSON is the MCP request object for the tool/resource/prompt
// handler. The returned JSON must be the corresponding MCP result object.
type JSONCallback func(requestJSON []byte) ([]byte, error)

// ServerConfig is the JSON-decoded configuration accepted by NewServer.
type ServerConfig struct {
	// Name is the MCP implementation name returned during initialization.
	Name string `json:"name"`
	// Version is the MCP implementation version returned during initialization.
	Version string `json:"version"`
	// Title is an optional human-readable implementation title.
	Title string `json:"title,omitempty"`
	// Description is an optional implementation description.
	Description string `json:"description,omitempty"`
	// WebsiteURL is an optional implementation website URL.
	WebsiteURL string `json:"websiteUrl,omitempty"`
	// Icons contains optional visual identifiers for the implementation.
	Icons []mcp.Icon `json:"icons,omitempty"`
	// Instructions are server instructions returned to clients at initialize.
	Instructions string `json:"instructions,omitempty"`
	// Capabilities explicitly enables server capabilities.
	Capabilities ServerCapabilitiesConfig `json:"capabilities,omitempty"`
	// Experimental stores non-standard capability data.
	Experimental map[string]any `json:"experimental,omitempty"`
	// PaginationLimit configures the maximum number of items per list response.
	PaginationLimit int `json:"paginationLimit,omitempty"`
	// Recovery enables panic recovery for tool handlers.
	Recovery bool `json:"recovery,omitempty"`
	// ResourceRecovery enables panic recovery for resource handlers.
	ResourceRecovery bool `json:"resourceRecovery,omitempty"`
	// InputSchemaValidation validates tool arguments against input schemas.
	InputSchemaValidation bool `json:"inputSchemaValidation,omitempty"`
	// OutputSchemaValidation validates structured tool results against output schemas.
	OutputSchemaValidation bool `json:"outputSchemaValidation,omitempty"`
	// StrictInputSchemaDefault sets additionalProperties:false for structured schemas.
	StrictInputSchemaDefault bool `json:"strictInputSchemaDefault,omitempty"`
	// MaxConcurrentTasks limits concurrently running task-augmented tool calls.
	MaxConcurrentTasks int `json:"maxConcurrentTasks,omitempty"`
}

// ServerCapabilitiesConfig is the capabilities section of ServerConfig.
type ServerCapabilitiesConfig struct {
	// Tools explicitly enables tools/list and tools/call.
	Tools *ToolCapabilitiesConfig `json:"tools,omitempty"`
	// Resources explicitly enables resource operations.
	Resources *ResourceCapabilitiesConfig `json:"resources,omitempty"`
	// Prompts explicitly enables prompt operations.
	Prompts *PromptCapabilitiesConfig `json:"prompts,omitempty"`
	// Logging enables logging/setLevel support.
	Logging bool `json:"logging,omitempty"`
	// Elicitation enables elicitation support.
	Elicitation bool `json:"elicitation,omitempty"`
	// Roots enables roots/list support from the server to clients.
	Roots bool `json:"roots,omitempty"`
	// Tasks enables task operations.
	Tasks *TaskCapabilitiesConfig `json:"tasks,omitempty"`
	// Completions enables completion/complete support.
	Completions bool `json:"completions,omitempty"`
}

// ToolCapabilitiesConfig configures tool-related capabilities.
type ToolCapabilitiesConfig struct {
	// ListChanged advertises notifications/tools/list_changed support.
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourceCapabilitiesConfig configures resource-related capabilities.
type ResourceCapabilitiesConfig struct {
	// Subscribe advertises resources/subscribe support.
	Subscribe bool `json:"subscribe,omitempty"`
	// ListChanged advertises notifications/resources/list_changed support.
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptCapabilitiesConfig configures prompt-related capabilities.
type PromptCapabilitiesConfig struct {
	// ListChanged advertises notifications/prompts/list_changed support.
	ListChanged bool `json:"listChanged,omitempty"`
}

// TaskCapabilitiesConfig configures task-related capabilities.
type TaskCapabilitiesConfig struct {
	// List advertises tasks/list support.
	List bool `json:"list,omitempty"`
	// Cancel advertises tasks/cancel support.
	Cancel bool `json:"cancel,omitempty"`
	// ToolCallTasks advertises task augmentation for tools/call.
	ToolCallTasks bool `json:"toolCallTasks,omitempty"`
}

// StreamableHTTPConfig is the JSON-decoded configuration accepted by
// StartStreamableHTTP.
type StreamableHTTPConfig struct {
	// Addr is the listen address, such as ":8080" or "127.0.0.1:8080".
	Addr string `json:"addr,omitempty"`
	// EndpointPath is the MCP endpoint path. The default is "/mcp".
	EndpointPath string `json:"endpointPath,omitempty"`
	// Stateless disables session ID generation and validation.
	Stateless bool `json:"stateless,omitempty"`
	// Stateful enables local stateful session validation.
	Stateful bool `json:"stateful,omitempty"`
	// DisableStreaming rejects GET streaming and keeps POST responses buffered.
	DisableStreaming bool `json:"disableStreaming,omitempty"`
	// HeartbeatIntervalMS enables GET stream heartbeat events when positive.
	HeartbeatIntervalMS int64 `json:"heartbeatIntervalMs,omitempty"`
	// SessionIdleTTLMS removes idle per-session state when positive.
	SessionIdleTTLMS int64 `json:"sessionIdleTtlMs,omitempty"`
	// TLSCertFile is the PEM certificate path for HTTPS.
	TLSCertFile string `json:"tlsCertFile,omitempty"`
	// TLSKeyFile is the PEM private key path for HTTPS.
	TLSKeyFile string `json:"tlsKeyFile,omitempty"`
}

// SSEConfig is the JSON-decoded configuration accepted by StartSSE.
type SSEConfig struct {
	// Addr is the listen address, such as ":8080" or "127.0.0.1:8080".
	Addr string `json:"addr,omitempty"`
	// BaseURL is the public base URL used to construct message endpoints.
	BaseURL string `json:"baseUrl,omitempty"`
	// BasePath is the static base path for SSE routes.
	BasePath string `json:"basePath,omitempty"`
	// MessageEndpoint is the POST message endpoint path. The default is "/message".
	MessageEndpoint string `json:"messageEndpoint,omitempty"`
	// SSEEndpoint is the SSE connection endpoint path. The default is "/sse".
	SSEEndpoint string `json:"sseEndpoint,omitempty"`
	// AppendQueryToMessageEndpoint preserves the initial SSE query string.
	AppendQueryToMessageEndpoint bool `json:"appendQueryToMessageEndpoint,omitempty"`
	// UseFullURLForMessageEndpoint controls whether the endpoint event contains a full URL.
	UseFullURLForMessageEndpoint *bool `json:"useFullUrlForMessageEndpoint,omitempty"`
	// KeepAlive enables SSE keep-alive events.
	KeepAlive bool `json:"keepAlive,omitempty"`
	// KeepAliveIntervalMS sets the SSE keep-alive interval when positive.
	KeepAliveIntervalMS int64 `json:"keepAliveIntervalMs,omitempty"`
}

// Server is the Go-side owner for a server handle.
type Server struct {
	handle    ServerHandle
	server    *mcpserver.MCPServer
	createdAt time.Time

	mu            sync.Mutex
	state         string
	templateCount int
	transports    map[TransportHandle]struct{}

	metricsMu sync.Mutex
	metrics   ServerMetrics
}

// Transport is the Go-side owner for a transport handle.
type Transport struct {
	kind         string
	serverHandle ServerHandle
	shutdown     func(context.Context) error

	mu            sync.Mutex
	state         string
	startedAt     time.Time
	stoppedAt     *time.Time
	lastErr       error
	lastErrorAt   *time.Time
	shutdownCount uint64
}

type registry[T any] struct {
	mu     sync.RWMutex
	next   uint64
	values map[uint64]T
}

func newRegistry[T any]() *registry[T] {
	return &registry[T]{
		next:   1,
		values: make(map[uint64]T),
	}
}

func (r *registry[T]) add(value T) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	handle := r.next
	r.next++
	r.values[handle] = value
	return handle
}

func (r *registry[T]) get(handle uint64) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.values[handle]
	return value, ok
}

func (r *registry[T]) remove(handle uint64) (T, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[handle]
	if ok {
		delete(r.values, handle)
	}
	return value, ok
}

var serverRegistry = newRegistry[*Server]()
var transportRegistry = newRegistry[*Transport]()

// NewServer creates an MCP server from a JSON ServerConfig document and returns
// an opaque handle that can be passed to other capi functions.
func NewServer(configJSON []byte) (handle ServerHandle, err error) {
	defer capturePanic(&err)

	cfg := ServerConfig{
		Name:    "mcp-go-capi",
		Version: "0.0.0",
	}
	if len(bytes.TrimSpace(configJSON)) > 0 {
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return 0, fmt.Errorf("decode server config: %w", err)
		}
	}
	if cfg.Name == "" {
		return 0, fmt.Errorf("server name is required")
	}
	if cfg.Version == "" {
		return 0, fmt.Errorf("server version is required")
	}

	opts := serverOptions(cfg)
	s := mcpserver.NewMCPServer(cfg.Name, cfg.Version, opts...)
	entry := &Server{
		createdAt:  time.Now(),
		server:     s,
		state:      "created",
		transports: make(map[TransportHandle]struct{}),
	}
	handle = ServerHandle(serverRegistry.add(entry))
	entry.handle = handle
	RecordLog(logLevelInfo, handle, 0, "server.created", "server created", map[string]any{
		"name":    cfg.Name,
		"version": cfg.Version,
	})
	return handle, nil
}

// FreeServer releases a server handle and shuts down transports started through
// this package. Handles are process-local and must not be reused after freeing.
func FreeServer(handle ServerHandle) (err error) {
	defer capturePanic(&err)

	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}

	entry.mu.Lock()
	transports := make([]TransportHandle, 0, len(entry.transports))
	for transport := range entry.transports {
		transports = append(transports, transport)
	}
	entry.transports = make(map[TransportHandle]struct{})
	entry.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, transport := range transports {
		_ = shutdownAndRemoveTransport(ctx, transport)
	}
	_, _ = serverRegistry.remove(uint64(handle))
	entry.setState("freed")
	RecordLog(logLevelInfo, handle, 0, "server.freed", "server freed", nil)
	return nil
}

// ShutdownServer gracefully stops all transports for a server without freeing
// the server handle. New transports may be started again later.
func ShutdownServer(handle ServerHandle, timeout time.Duration) (err error) {
	defer capturePanic(&err)

	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}
	entry.mu.Lock()
	transports := make([]TransportHandle, 0, len(entry.transports))
	for transport := range entry.transports {
		transports = append(transports, transport)
	}
	entry.mu.Unlock()

	for _, transport := range transports {
		if err := ShutdownTransport(transport, timeout); err != nil {
			entry.recordError()
			RecordLog(logLevelError, handle, transport, "server.shutdown.error", err.Error(), nil)
			return err
		}
	}
	entry.setState("stopped")
	RecordLog(logLevelInfo, handle, 0, "server.stopped", "server transports stopped", map[string]any{
		"transportCount": len(transports),
	})
	return nil
}

// LookupServer returns the Go server owner for a handle.
func LookupServer(handle ServerHandle) (*Server, error) {
	entry, ok := serverRegistry.get(uint64(handle))
	if !ok {
		return nil, ErrInvalidServerHandle
	}
	return entry, nil
}

// HandleMessage sends a raw JSON-RPC message through MCPServer.HandleMessage.
// The boolean return is false when the message was a notification or another
// protocol message that does not produce a response.
func HandleMessage(handle ServerHandle, messageJSON []byte) (responseJSON []byte, hasResponse bool, err error) {
	defer capturePanic(&err)

	entry, err := LookupServer(handle)
	if err != nil {
		return nil, false, err
	}
	if len(bytes.TrimSpace(messageJSON)) == 0 {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "message.invalid", "message JSON is required", nil)
		return nil, false, fmt.Errorf("message JSON is required")
	}
	method, hasID := parseMessageMetadata(messageJSON)
	entry.recordMessage(method, hasID)
	RecordLog(logLevelInfo, handle, 0, "message.received", "JSON-RPC message received", map[string]any{
		"method": method,
		"hasID":  hasID,
	})
	response := entry.server.HandleMessage(context.Background(), json.RawMessage(messageJSON))
	if response == nil {
		RecordLog(logLevelInfo, handle, 0, "message.completed", "JSON-RPC message completed without response", map[string]any{
			"method": method,
		})
		return nil, false, nil
	}
	data, err := json.Marshal(response)
	if err != nil {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "message.marshal_error", err.Error(), map[string]any{
			"method": method,
		})
		return nil, false, fmt.Errorf("marshal response: %w", err)
	}
	entry.recordResponse(data)
	RecordLog(logLevelInfo, handle, 0, "message.responded", "JSON-RPC response produced", map[string]any{
		"method": method,
		"bytes":  len(data),
	})
	return data, true, nil
}

// AddToolJSON registers a tool definition and a JSON callback. The toolJSON
// payload is the MCP Tool object as it appears in tools/list. The callback
// receives a CallToolRequest JSON object and must return a CallToolResult JSON
// object.
func AddToolJSON(handle ServerHandle, toolJSON []byte, callback JSONCallback) (err error) {
	defer capturePanic(&err)

	if callback == nil {
		RecordLog(logLevelError, handle, 0, "tool.register.error", "tool callback is required", nil)
		return fmt.Errorf("tool callback is required")
	}
	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}
	tool, err := decodeToolJSON(toolJSON)
	if err != nil {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "tool.register.error", err.Error(), nil)
		return err
	}
	entry.server.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		requestJSON, err := json.Marshal(request)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "tool.callback.error", err.Error(), map[string]any{
				"tool": request.Params.Name,
			})
			return nil, fmt.Errorf("marshal tool request: %w", err)
		}
		resultJSON, err := callback(requestJSON)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "tool.callback.error", err.Error(), map[string]any{
				"tool": request.Params.Name,
			})
			return nil, err
		}
		var result mcp.CallToolResult
		if err := json.Unmarshal(resultJSON, &result); err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "tool.callback.decode_error", err.Error(), map[string]any{
				"tool": request.Params.Name,
			})
			return nil, fmt.Errorf("decode CallToolResult JSON: %w", err)
		}
		RecordLog(logLevelInfo, handle, 0, "tool.callback.completed", "tool callback completed", map[string]any{
			"tool": request.Params.Name,
		})
		return &result, nil
	})
	entry.recordToolRegistration()
	RecordLog(logLevelInfo, handle, 0, "tool.registered", "tool registered from full JSON", map[string]any{
		"tool": tool.Name,
	})
	return nil
}

// AddToolWithSchemaJSON registers a tool from simple metadata plus raw input
// and output schema JSON documents.
func AddToolWithSchemaJSON(
	handle ServerHandle,
	name string,
	description string,
	inputSchemaJSON []byte,
	outputSchemaJSON []byte,
	callback JSONCallback,
) (err error) {
	defer capturePanic(&err)

	toolJSON, err := buildToolJSON(name, description, inputSchemaJSON, outputSchemaJSON)
	if err != nil {
		RecordLog(logLevelError, handle, 0, "tool.register.error", err.Error(), map[string]any{
			"tool": name,
		})
		return err
	}
	return AddToolJSON(handle, toolJSON, callback)
}

// AddSimpleTool registers a tool with an empty object input schema.
func AddSimpleTool(handle ServerHandle, name string, description string, callback JSONCallback) (err error) {
	defer capturePanic(&err)

	return AddToolWithSchemaJSON(handle, name, description, nil, nil, callback)
}

// AddStaticTextTool registers a tool that always returns the same text result
// and does not require a callback from the host language.
func AddStaticTextTool(handle ServerHandle, name string, description string, text string) (err error) {
	defer capturePanic(&err)

	return AddSimpleTool(handle, name, description, func(requestJSON []byte) ([]byte, error) {
		return ToolResultTextJSON(text)
	})
}

// AddResourceJSON registers a resource definition and a JSON callback. The
// resourceJSON payload is the MCP Resource object as it appears in
// resources/list. The callback receives a ReadResourceRequest JSON object and
// may return either a ReadResourceResult JSON object or a raw array of resource
// contents.
func AddResourceJSON(handle ServerHandle, resourceJSON []byte, callback JSONCallback) (err error) {
	defer capturePanic(&err)

	if callback == nil {
		RecordLog(logLevelError, handle, 0, "resource.register.error", "resource callback is required", nil)
		return fmt.Errorf("resource callback is required")
	}
	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}
	var resource mcp.Resource
	if err := json.Unmarshal(resourceJSON, &resource); err != nil {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "resource.register.error", err.Error(), nil)
		return fmt.Errorf("decode Resource JSON: %w", err)
	}
	if resource.URI == "" {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "resource.register.error", "resource uri is required", nil)
		return fmt.Errorf("resource uri is required")
	}
	if resource.Name == "" {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "resource.register.error", "resource name is required", map[string]any{
			"uri": resource.URI,
		})
		return fmt.Errorf("resource name is required")
	}
	entry.server.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		requestJSON, err := json.Marshal(request)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "resource.callback.error", err.Error(), map[string]any{
				"uri": request.Params.URI,
			})
			return nil, fmt.Errorf("marshal resource request: %w", err)
		}
		resultJSON, err := callback(requestJSON)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "resource.callback.error", err.Error(), map[string]any{
				"uri": request.Params.URI,
			})
			return nil, err
		}
		contents, err := decodeResourceContentsJSON(resultJSON)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "resource.callback.decode_error", err.Error(), map[string]any{
				"uri": request.Params.URI,
			})
			return nil, err
		}
		RecordLog(logLevelInfo, handle, 0, "resource.callback.completed", "resource callback completed", map[string]any{
			"uri": request.Params.URI,
		})
		return contents, nil
	})
	entry.recordResourceRegistration()
	RecordLog(logLevelInfo, handle, 0, "resource.registered", "resource registered", map[string]any{
		"uri": resource.URI,
	})
	return nil
}

// AddResourceTemplateJSON registers a resource template definition and a JSON
// callback. The callback receives a ReadResourceRequest JSON object and may
// return either a ReadResourceResult JSON object or a raw array of resource
// contents.
func AddResourceTemplateJSON(handle ServerHandle, templateJSON []byte, callback JSONCallback) (err error) {
	defer capturePanic(&err)

	if callback == nil {
		RecordLog(logLevelError, handle, 0, "resource_template.register.error", "resource template callback is required", nil)
		return fmt.Errorf("resource template callback is required")
	}
	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}
	var template mcp.ResourceTemplate
	if err := json.Unmarshal(templateJSON, &template); err != nil {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "resource_template.register.error", err.Error(), nil)
		return fmt.Errorf("decode ResourceTemplate JSON: %w", err)
	}
	if template.URITemplate == nil {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "resource_template.register.error", "resource template uriTemplate is required", nil)
		return fmt.Errorf("resource template uriTemplate is required")
	}
	if template.Name == "" {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "resource_template.register.error", "resource template name is required", nil)
		return fmt.Errorf("resource template name is required")
	}
	entry.server.AddResourceTemplate(template, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		requestJSON, err := json.Marshal(request)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "resource_template.callback.error", err.Error(), map[string]any{
				"uri": request.Params.URI,
			})
			return nil, fmt.Errorf("marshal resource template request: %w", err)
		}
		resultJSON, err := callback(requestJSON)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "resource_template.callback.error", err.Error(), map[string]any{
				"uri": request.Params.URI,
			})
			return nil, err
		}
		contents, err := decodeResourceContentsJSON(resultJSON)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "resource_template.callback.decode_error", err.Error(), map[string]any{
				"uri": request.Params.URI,
			})
			return nil, err
		}
		RecordLog(logLevelInfo, handle, 0, "resource_template.callback.completed", "resource template callback completed", map[string]any{
			"uri": request.Params.URI,
		})
		return contents, nil
	})
	entry.recordTemplateRegistration()
	RecordLog(logLevelInfo, handle, 0, "resource_template.registered", "resource template registered", map[string]any{
		"uriTemplate": template.URITemplate.Raw(),
	})
	return nil
}

// AddPromptJSON registers a prompt definition and a JSON callback. The prompt
// JSON payload is the MCP Prompt object as it appears in prompts/list. The
// callback receives a GetPromptRequest JSON object and must return a
// GetPromptResult JSON object.
func AddPromptJSON(handle ServerHandle, promptJSON []byte, callback JSONCallback) (err error) {
	defer capturePanic(&err)

	if callback == nil {
		RecordLog(logLevelError, handle, 0, "prompt.register.error", "prompt callback is required", nil)
		return fmt.Errorf("prompt callback is required")
	}
	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}
	var prompt mcp.Prompt
	if err := json.Unmarshal(promptJSON, &prompt); err != nil {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "prompt.register.error", err.Error(), nil)
		return fmt.Errorf("decode Prompt JSON: %w", err)
	}
	if prompt.Name == "" {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "prompt.register.error", "prompt name is required", nil)
		return fmt.Errorf("prompt name is required")
	}
	entry.server.AddPrompt(prompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		requestJSON, err := json.Marshal(request)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "prompt.callback.error", err.Error(), map[string]any{
				"prompt": request.Params.Name,
			})
			return nil, fmt.Errorf("marshal prompt request: %w", err)
		}
		resultJSON, err := callback(requestJSON)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "prompt.callback.error", err.Error(), map[string]any{
				"prompt": request.Params.Name,
			})
			return nil, err
		}
		result, err := decodePromptResultJSON(resultJSON)
		if err != nil {
			entry.recordCallbackError()
			RecordLog(logLevelError, handle, 0, "prompt.callback.decode_error", err.Error(), map[string]any{
				"prompt": request.Params.Name,
			})
			return nil, err
		}
		RecordLog(logLevelInfo, handle, 0, "prompt.callback.completed", "prompt callback completed", map[string]any{
			"prompt": request.Params.Name,
		})
		return result, nil
	})
	entry.recordPromptRegistration()
	RecordLog(logLevelInfo, handle, 0, "prompt.registered", "prompt registered", map[string]any{
		"prompt": prompt.Name,
	})
	return nil
}

// DeleteTool removes a registered global tool by name.
func DeleteTool(handle ServerHandle, name string) (err error) {
	defer capturePanic(&err)

	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}
	if name == "" {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "tool.delete.error", "tool name is required", nil)
		return fmt.Errorf("tool name is required")
	}
	entry.server.DeleteTools(name)
	RecordLog(logLevelInfo, handle, 0, "tool.deleted", "tool deleted", map[string]any{
		"tool": name,
	})
	return nil
}

// DeleteResource removes a registered global resource by URI.
func DeleteResource(handle ServerHandle, uri string) (err error) {
	defer capturePanic(&err)

	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}
	if uri == "" {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "resource.delete.error", "resource uri is required", nil)
		return fmt.Errorf("resource uri is required")
	}
	entry.server.DeleteResources(uri)
	RecordLog(logLevelInfo, handle, 0, "resource.deleted", "resource deleted", map[string]any{
		"uri": uri,
	})
	return nil
}

// DeletePrompt removes a registered global prompt by name.
func DeletePrompt(handle ServerHandle, name string) (err error) {
	defer capturePanic(&err)

	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}
	if name == "" {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "prompt.delete.error", "prompt name is required", nil)
		return fmt.Errorf("prompt name is required")
	}
	entry.server.DeletePrompts(name)
	RecordLog(logLevelInfo, handle, 0, "prompt.deleted", "prompt deleted", map[string]any{
		"prompt": name,
	})
	return nil
}

// SendNotificationToAllClients sends an MCP notification to all initialized
// client sessions attached to this server. paramsJSON may be empty to send a
// notification without params.
func SendNotificationToAllClients(handle ServerHandle, method string, paramsJSON []byte) (err error) {
	defer capturePanic(&err)

	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}
	if method == "" {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "notification.send.error", "notification method is required", nil)
		return fmt.Errorf("notification method is required")
	}
	var params map[string]any
	if len(bytes.TrimSpace(paramsJSON)) > 0 {
		if err := json.Unmarshal(paramsJSON, &params); err != nil {
			entry.recordError()
			RecordLog(logLevelError, handle, 0, "notification.send.error", err.Error(), map[string]any{
				"method": method,
			})
			return fmt.Errorf("decode notification params JSON: %w", err)
		}
	}
	entry.server.SendNotificationToAllClients(method, params)
	entry.recordNotificationSent()
	RecordLog(logLevelInfo, handle, 0, "notification.sent", "notification sent to all clients", map[string]any{
		"method": method,
	})
	return nil
}

// ServeStdio runs the server over standard input and standard output. It
// blocks until the stdio transport exits.
func ServeStdio(handle ServerHandle) (err error) {
	defer capturePanic(&err)

	entry, err := LookupServer(handle)
	if err != nil {
		return err
	}
	RecordLog(logLevelInfo, handle, 0, "stdio.start", "stdio transport starting", nil)
	entry.setState("running")
	if err := mcpserver.ServeStdio(entry.server); err != nil {
		entry.recordError()
		RecordLog(logLevelError, handle, 0, "stdio.error", err.Error(), nil)
		return err
	}
	RecordLog(logLevelInfo, handle, 0, "stdio.stopped", "stdio transport stopped", nil)
	return nil
}

// StartStreamableHTTP starts the streamable HTTP transport in the background
// and returns a transport handle that can be shut down later.
func StartStreamableHTTP(handle ServerHandle, configJSON []byte) (transport TransportHandle, err error) {
	defer capturePanic(&err)

	entry, err := LookupServer(handle)
	if err != nil {
		return 0, err
	}
	cfg := StreamableHTTPConfig{Addr: ":8080"}
	if len(bytes.TrimSpace(configJSON)) > 0 {
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return 0, fmt.Errorf("decode streamable HTTP config: %w", err)
		}
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}

	opts := streamableHTTPOptions(cfg)
	httpServer := mcpserver.NewStreamableHTTPServer(entry.server, opts...)
	transportEntry := &Transport{
		kind:         "streamable-http",
		serverHandle: handle,
		shutdown:     httpServer.Shutdown,
		state:        "running",
		startedAt:    time.Now(),
	}
	transport = TransportHandle(transportRegistry.add(transportEntry))
	entry.addTransport(transport)
	entry.recordTransportStarted()
	entry.setState("running")
	RecordLog(logLevelInfo, handle, transport, "transport.started", "streamable HTTP transport started", map[string]any{
		"kind": "streamable-http",
		"addr": cfg.Addr,
	})

	go func() {
		if startErr := httpServer.Start(cfg.Addr); startErr != nil && !errors.Is(startErr, http.ErrServerClosed) {
			transportEntry.setLastError(startErr)
			entry.recordError()
			RecordLog(logLevelError, handle, transport, "transport.error", startErr.Error(), map[string]any{
				"kind": "streamable-http",
				"addr": cfg.Addr,
			})
		}
	}()
	return transport, nil
}

// StartSSE starts the legacy SSE transport in the background and returns a
// transport handle that can be shut down later.
func StartSSE(handle ServerHandle, configJSON []byte) (transport TransportHandle, err error) {
	defer capturePanic(&err)

	entry, err := LookupServer(handle)
	if err != nil {
		return 0, err
	}
	cfg := SSEConfig{Addr: ":8080"}
	if len(bytes.TrimSpace(configJSON)) > 0 {
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return 0, fmt.Errorf("decode SSE config: %w", err)
		}
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}

	opts := sseOptions(cfg)
	sseServer := mcpserver.NewSSEServer(entry.server, opts...)
	transportEntry := &Transport{
		kind:         "sse",
		serverHandle: handle,
		shutdown:     sseServer.Shutdown,
		state:        "running",
		startedAt:    time.Now(),
	}
	transport = TransportHandle(transportRegistry.add(transportEntry))
	entry.addTransport(transport)
	entry.recordTransportStarted()
	entry.setState("running")
	RecordLog(logLevelInfo, handle, transport, "transport.started", "SSE transport started", map[string]any{
		"kind": "sse",
		"addr": cfg.Addr,
	})

	go func() {
		if startErr := sseServer.Start(cfg.Addr); startErr != nil && !errors.Is(startErr, http.ErrServerClosed) {
			transportEntry.setLastError(startErr)
			entry.recordError()
			RecordLog(logLevelError, handle, transport, "transport.error", startErr.Error(), map[string]any{
				"kind": "sse",
				"addr": cfg.Addr,
			})
		}
	}()
	return transport, nil
}

// ShutdownTransport gracefully stops a running transport. The handle remains
// valid until FreeTransport is called.
func ShutdownTransport(handle TransportHandle, timeout time.Duration) (err error) {
	defer capturePanic(&err)

	entry, ok := transportRegistry.get(uint64(handle))
	if !ok {
		return ErrInvalidTransportHandle
	}
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := entry.shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		entry.setLastError(err)
		if serverEntry, ok := serverRegistry.get(uint64(entry.serverHandle)); ok {
			serverEntry.recordError()
		}
		RecordLog(logLevelError, entry.serverHandle, handle, "transport.shutdown.error", err.Error(), map[string]any{
			"kind": entry.kind,
		})
		return err
	}
	entry.markStopped()
	if serverEntry, ok := serverRegistry.get(uint64(entry.serverHandle)); ok {
		serverEntry.recordTransportStopped()
	}
	RecordLog(logLevelInfo, entry.serverHandle, handle, "transport.stopped", "transport stopped", map[string]any{
		"kind": entry.kind,
	})
	return nil
}

// FreeTransport shuts down and releases a transport handle.
func FreeTransport(handle TransportHandle, timeout time.Duration) (err error) {
	defer capturePanic(&err)

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return shutdownAndRemoveTransport(ctx, handle)
}

// TransportLastError returns the last asynchronous transport start error, if
// any. It is primarily useful for background transports started through C.
func TransportLastError(handle TransportHandle) (string, error) {
	entry, ok := transportRegistry.get(uint64(handle))
	if !ok {
		return "", ErrInvalidTransportHandle
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.lastErr == nil {
		return "", nil
	}
	return entry.lastErr.Error(), nil
}

// ToolResultTextJSON builds a CallToolResult JSON object containing one text
// content item.
func ToolResultTextJSON(text string) ([]byte, error) {
	return json.Marshal(mcp.NewToolResultText(text))
}

// ToolResultErrorJSON builds a CallToolResult JSON object marked as a tool
// execution error.
func ToolResultErrorJSON(text string) ([]byte, error) {
	return json.Marshal(mcp.NewToolResultError(text))
}

// ToolResultStructuredJSON builds a CallToolResult JSON object containing
// structuredContent and a text fallback.
func ToolResultStructuredJSON(structuredJSON []byte, fallbackText string) ([]byte, error) {
	var structured any
	if len(bytes.TrimSpace(structuredJSON)) > 0 {
		if err := json.Unmarshal(structuredJSON, &structured); err != nil {
			return nil, fmt.Errorf("decode structured JSON: %w", err)
		}
	}
	return json.Marshal(mcp.NewToolResultStructured(structured, fallbackText))
}

// ResourceResultTextJSON builds a ReadResourceResult JSON object containing one
// text resource content item.
func ResourceResultTextJSON(uri, mimeType, text string) ([]byte, error) {
	result := mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: mimeType,
				Text:     text,
			},
		},
	}
	return json.Marshal(result)
}

// ResourceResultBlobJSON builds a ReadResourceResult JSON object containing one
// base64-encoded binary resource content item.
func ResourceResultBlobJSON(uri, mimeType, base64Blob string) ([]byte, error) {
	result := mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			mcp.BlobResourceContents{
				URI:      uri,
				MIMEType: mimeType,
				Blob:     base64Blob,
			},
		},
	}
	return json.Marshal(result)
}

// PromptResultTextJSON builds a GetPromptResult JSON object containing one text
// prompt message.
func PromptResultTextJSON(description, role, text string) ([]byte, error) {
	if role == "" {
		role = string(mcp.RoleUser)
	}
	result := mcp.GetPromptResult{
		Description: description,
		Messages: []mcp.PromptMessage{
			{
				Role:    mcp.Role(role),
				Content: mcp.NewTextContent(text),
			},
		},
	}
	return json.Marshal(result)
}

func serverOptions(cfg ServerConfig) []mcpserver.ServerOption {
	var opts []mcpserver.ServerOption
	if cfg.Title != "" {
		opts = append(opts, mcpserver.WithTitle(cfg.Title))
	}
	if cfg.Description != "" {
		opts = append(opts, mcpserver.WithDescription(cfg.Description))
	}
	if cfg.WebsiteURL != "" {
		opts = append(opts, mcpserver.WithWebsiteURL(cfg.WebsiteURL))
	}
	if len(cfg.Icons) > 0 {
		opts = append(opts, mcpserver.WithIcons(cfg.Icons...))
	}
	if cfg.Instructions != "" {
		opts = append(opts, mcpserver.WithInstructions(cfg.Instructions))
	}
	if cfg.PaginationLimit > 0 {
		opts = append(opts, mcpserver.WithPaginationLimit(cfg.PaginationLimit))
	}
	if cfg.Recovery {
		opts = append(opts, mcpserver.WithRecovery())
	}
	if cfg.ResourceRecovery {
		opts = append(opts, mcpserver.WithResourceRecovery())
	}
	if cfg.InputSchemaValidation {
		opts = append(opts, mcpserver.WithInputSchemaValidation())
	}
	if cfg.OutputSchemaValidation {
		opts = append(opts, mcpserver.WithOutputSchemaValidation())
	}
	if cfg.StrictInputSchemaDefault {
		opts = append(opts, mcpserver.WithStrictInputSchemaDefault())
	}
	if cfg.MaxConcurrentTasks > 0 {
		opts = append(opts, mcpserver.WithMaxConcurrentTasks(cfg.MaxConcurrentTasks))
	}
	if cfg.Capabilities.Tools != nil {
		opts = append(opts, mcpserver.WithToolCapabilities(cfg.Capabilities.Tools.ListChanged))
	}
	if cfg.Capabilities.Resources != nil {
		opts = append(opts, mcpserver.WithResourceCapabilities(
			cfg.Capabilities.Resources.Subscribe,
			cfg.Capabilities.Resources.ListChanged,
		))
	}
	if cfg.Capabilities.Prompts != nil {
		opts = append(opts, mcpserver.WithPromptCapabilities(cfg.Capabilities.Prompts.ListChanged))
	}
	if cfg.Capabilities.Logging {
		opts = append(opts, mcpserver.WithLogging())
	}
	if cfg.Capabilities.Elicitation {
		opts = append(opts, mcpserver.WithElicitation())
	}
	if cfg.Capabilities.Roots {
		opts = append(opts, mcpserver.WithRoots())
	}
	if cfg.Capabilities.Tasks != nil {
		opts = append(opts, mcpserver.WithTaskCapabilities(
			cfg.Capabilities.Tasks.List,
			cfg.Capabilities.Tasks.Cancel,
			cfg.Capabilities.Tasks.ToolCallTasks,
		))
	}
	if cfg.Capabilities.Completions {
		opts = append(opts, mcpserver.WithCompletions())
	}
	if cfg.Experimental != nil {
		opts = append(opts, mcpserver.WithExperimental(cfg.Experimental))
	}
	return opts
}

func streamableHTTPOptions(cfg StreamableHTTPConfig) []mcpserver.StreamableHTTPOption {
	var opts []mcpserver.StreamableHTTPOption
	if cfg.EndpointPath != "" {
		opts = append(opts, mcpserver.WithEndpointPath(cfg.EndpointPath))
	}
	if cfg.Stateless {
		opts = append(opts, mcpserver.WithStateLess(true))
	}
	if cfg.Stateful {
		opts = append(opts, mcpserver.WithStateful(true))
	}
	if cfg.DisableStreaming {
		opts = append(opts, mcpserver.WithDisableStreaming(true))
	}
	if cfg.HeartbeatIntervalMS > 0 {
		opts = append(opts, mcpserver.WithHeartbeatInterval(time.Duration(cfg.HeartbeatIntervalMS)*time.Millisecond))
	}
	if cfg.SessionIdleTTLMS > 0 {
		opts = append(opts, mcpserver.WithSessionIdleTTL(time.Duration(cfg.SessionIdleTTLMS)*time.Millisecond))
	}
	if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
		opts = append(opts, mcpserver.WithTLSCert(cfg.TLSCertFile, cfg.TLSKeyFile))
	}
	return opts
}

func sseOptions(cfg SSEConfig) []mcpserver.SSEOption {
	var opts []mcpserver.SSEOption
	if cfg.BaseURL != "" {
		opts = append(opts, mcpserver.WithBaseURL(cfg.BaseURL))
	}
	if cfg.BasePath != "" {
		opts = append(opts, mcpserver.WithStaticBasePath(cfg.BasePath))
	}
	if cfg.MessageEndpoint != "" {
		opts = append(opts, mcpserver.WithMessageEndpoint(cfg.MessageEndpoint))
	}
	if cfg.SSEEndpoint != "" {
		opts = append(opts, mcpserver.WithSSEEndpoint(cfg.SSEEndpoint))
	}
	if cfg.AppendQueryToMessageEndpoint {
		opts = append(opts, mcpserver.WithAppendQueryToMessageEndpoint())
	}
	if cfg.UseFullURLForMessageEndpoint != nil {
		opts = append(opts, mcpserver.WithUseFullURLForMessageEndpoint(*cfg.UseFullURLForMessageEndpoint))
	}
	if cfg.KeepAlive {
		opts = append(opts, mcpserver.WithKeepAlive(true))
	}
	if cfg.KeepAliveIntervalMS > 0 {
		opts = append(opts, mcpserver.WithKeepAliveInterval(time.Duration(cfg.KeepAliveIntervalMS)*time.Millisecond))
	}
	return opts
}

func decodeToolJSON(data []byte) (mcp.Tool, error) {
	var raw struct {
		Meta              *mcp.Meta          `json:"_meta,omitempty"`
		Name              string             `json:"name"`
		Title             string             `json:"title,omitempty"`
		Description       string             `json:"description,omitempty"`
		InputSchema       json.RawMessage    `json:"inputSchema,omitempty"`
		OutputSchema      json.RawMessage    `json:"outputSchema,omitempty"`
		Annotations       mcp.ToolAnnotation `json:"annotations,omitempty"`
		DeferLoading      bool               `json:"defer_loading,omitempty"`
		DeferLoadingCamel bool               `json:"deferLoading,omitempty"`
		Icons             []mcp.Icon         `json:"icons,omitempty"`
		Execution         *mcp.ToolExecution `json:"execution,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return mcp.Tool{}, fmt.Errorf("decode Tool JSON: %w", err)
	}
	if raw.Name == "" {
		return mcp.Tool{}, fmt.Errorf("tool name is required")
	}

	tool := mcp.Tool{
		Meta:         raw.Meta,
		Name:         raw.Name,
		Title:        raw.Title,
		Description:  raw.Description,
		Annotations:  raw.Annotations,
		DeferLoading: raw.DeferLoading || raw.DeferLoadingCamel,
		Icons:        raw.Icons,
		Execution:    raw.Execution,
	}
	if len(bytes.TrimSpace(raw.InputSchema)) > 0 {
		tool.RawInputSchema = copyRawMessage(raw.InputSchema)
	} else {
		tool.InputSchema = mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]any{},
		}
	}
	if len(bytes.TrimSpace(raw.OutputSchema)) > 0 {
		tool.RawOutputSchema = copyRawMessage(raw.OutputSchema)
	}
	return tool, nil
}

func buildToolJSON(name, description string, inputSchemaJSON, outputSchemaJSON []byte) ([]byte, error) {
	if name == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	var inputSchema any = map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	if len(bytes.TrimSpace(inputSchemaJSON)) > 0 {
		if err := json.Unmarshal(inputSchemaJSON, &inputSchema); err != nil {
			return nil, fmt.Errorf("decode input schema JSON: %w", err)
		}
	}

	raw := map[string]any{
		"name":        name,
		"inputSchema": inputSchema,
	}
	if description != "" {
		raw["description"] = description
	}
	if len(bytes.TrimSpace(outputSchemaJSON)) > 0 {
		var outputSchema any
		if err := json.Unmarshal(outputSchemaJSON, &outputSchema); err != nil {
			return nil, fmt.Errorf("decode output schema JSON: %w", err)
		}
		raw["outputSchema"] = outputSchema
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode tool JSON: %w", err)
	}
	return data, nil
}

func parseMessageMetadata(messageJSON []byte) (string, bool) {
	var base struct {
		Method string          `json:"method"`
		ID     json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(messageJSON, &base); err != nil {
		return "", false
	}
	return base.Method, len(base.ID) > 0 && string(base.ID) != "null"
}

func decodeResourceContentsJSON(data []byte) ([]mcp.ResourceContents, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("resource callback returned empty JSON")
	}

	var rawContents []json.RawMessage
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &rawContents); err != nil {
			return nil, fmt.Errorf("decode resource contents array: %w", err)
		}
	} else {
		var result struct {
			Contents []json.RawMessage `json:"contents"`
		}
		if err := json.Unmarshal(trimmed, &result); err != nil {
			return nil, fmt.Errorf("decode ReadResourceResult JSON: %w", err)
		}
		rawContents = result.Contents
	}

	contents := make([]mcp.ResourceContents, 0, len(rawContents))
	for i, raw := range rawContents {
		content, err := decodeResourceContentJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("decode resource content %d: %w", i, err)
		}
		contents = append(contents, content)
	}
	return contents, nil
}

func decodeResourceContentJSON(data []byte) (mcp.ResourceContents, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if _, ok := fields["text"]; ok {
		var content mcp.TextResourceContents
		if err := json.Unmarshal(data, &content); err != nil {
			return nil, err
		}
		return content, nil
	}
	if _, ok := fields["blob"]; ok {
		var content mcp.BlobResourceContents
		if err := json.Unmarshal(data, &content); err != nil {
			return nil, err
		}
		return content, nil
	}
	return nil, fmt.Errorf("resource content must contain either text or blob")
}

func decodePromptResultJSON(data []byte) (*mcp.GetPromptResult, error) {
	var raw struct {
		Meta        *mcp.Meta `json:"_meta,omitempty"`
		Description string    `json:"description,omitempty"`
		Messages    []struct {
			Role    mcp.Role        `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode GetPromptResult JSON: %w", err)
	}
	result := &mcp.GetPromptResult{
		Result: mcp.Result{
			Meta: raw.Meta,
		},
		Description: raw.Description,
		Messages:    make([]mcp.PromptMessage, 0, len(raw.Messages)),
	}
	for i, message := range raw.Messages {
		content, err := mcp.UnmarshalContent(message.Content)
		if err != nil {
			return nil, fmt.Errorf("decode prompt message %d content: %w", i, err)
		}
		result.Messages = append(result.Messages, mcp.PromptMessage{
			Role:    message.Role,
			Content: content,
		})
	}
	return result, nil
}

func copyRawMessage(raw json.RawMessage) json.RawMessage {
	copied := make(json.RawMessage, len(raw))
	copy(copied, raw)
	return copied
}

func (s *Server) addTransport(handle TransportHandle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transports[handle] = struct{}{}
}

func (s *Server) removeTransport(handle TransportHandle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.transports, handle)
}

func (t *Transport) setLastError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastErr = err
	now := time.Now()
	t.lastErrorAt = &now
	t.state = "failed"
}

func shutdownAndRemoveTransport(ctx context.Context, handle TransportHandle) error {
	entry, ok := transportRegistry.remove(uint64(handle))
	if !ok {
		return ErrInvalidTransportHandle
	}
	if serverEntry, ok := serverRegistry.get(uint64(entry.serverHandle)); ok {
		serverEntry.removeTransport(handle)
	}
	if err := entry.shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		entry.setLastError(err)
		if serverEntry, ok := serverRegistry.get(uint64(entry.serverHandle)); ok {
			serverEntry.recordError()
		}
		RecordLog(logLevelError, entry.serverHandle, handle, "transport.free.error", err.Error(), map[string]any{
			"kind": entry.kind,
		})
		return err
	}
	entry.markStopped()
	if serverEntry, ok := serverRegistry.get(uint64(entry.serverHandle)); ok {
		serverEntry.recordTransportStopped()
	}
	RecordLog(logLevelInfo, entry.serverHandle, handle, "transport.freed", "transport freed", map[string]any{
		"kind": entry.kind,
	})
	return nil
}

func capturePanic(err *error) {
	if recovered := recover(); recovered != nil {
		*err = fmt.Errorf("panic recovered in C API wrapper: %v", recovered)
	}
}
