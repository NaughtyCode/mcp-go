#ifndef MCPGO_CAPI_H
#define MCPGO_CAPI_H

#include <stdint.h>

#include "callback.h"

#ifdef __cplusplus
extern "C" {
#endif

/* Operation completed successfully. */
#define MCPGO_OK 0

/* Operation failed. Call mcpgo_last_error() for a detailed message. */
#define MCPGO_ERROR -1

/*
 * String input parameters are declared as char * to match the header generated
 * by Go's c-shared build mode. mcp-go treats all input strings as read-only and
 * never mutates them.
 */

/*
 * Opaque process-local server handle.
 *
 * Values of this type are returned by mcpgo_server_new(). A non-zero handle is
 * valid until mcpgo_server_free() succeeds. Handles are process-local, are not
 * stable across runs, and must not be serialized or fabricated by callers.
 */
typedef uint64_t mcpgo_server;

/*
 * Opaque process-local transport handle.
 *
 * Values of this type are returned by mcpgo_streamable_http_start() and
 * mcpgo_sse_start(). A non-zero handle is valid until mcpgo_transport_free()
 * succeeds. Handles are process-local and must not be serialized or fabricated.
 */
typedef uint64_t mcpgo_transport;

/*
 * Release a string allocated by mcp-go.
 *
 * Parameters:
 *   value (char *): A pointer returned by mcp-go, such as a char ** output
 *     value or mcpgo_last_error(). Passing NULL is allowed and has no effect.
 *
 * Returns:
 *   void: No status is returned. The pointer is considered consumed after this
 *     function returns.
 *
 * Boundary cases:
 *   Passing a stack pointer, string literal, pointer allocated by the caller, or
 *   a pointer already released with mcpgo_free_string() is undefined behavior.
 */
void mcpgo_free_string(char *value);

/*
 * Return the last error message observed by the C ABI.
 *
 * Parameters:
 *   none.
 *
 * Returns:
 *   char *: A newly allocated UTF-8, NUL-terminated diagnostic string. The
 *     string may be empty when no error has been recorded. The caller must
 *     release the returned pointer with mcpgo_free_string().
 *
 * Boundary cases:
 *   The error slot is process-global. Concurrent callers should treat this as
 *   best-effort diagnostic text, not as structured per-thread state.
 */
char *mcpgo_last_error(void);

/*
 * Set the in-memory structured log ring capacity.
 *
 * Parameters:
 *   capacity (int): Maximum number of log records retained in memory. Values
 *     less than or equal to zero are coerced to one.
 *
 * Returns:
 *   int: MCPGO_OK when the capacity is accepted. This function currently has no
 *     failure path.
 *
 * Boundary cases:
 *   Reducing capacity may discard older records. The setting is process-wide
 *   and affects all servers created through this DLL.
 */
int mcpgo_logs_set_capacity(int capacity);

/*
 * Register or clear a process-wide structured log callback.
 *
 * Parameters:
 *   callback (mcpgo_log_callback): Function invoked for each new log record, or
 *     NULL to clear the current callback.
 *   user_data (void *): Caller-owned pointer passed back to callback. It may be
 *     NULL. It is ignored when callback is NULL.
 *
 * Returns:
 *   int: MCPGO_OK when the callback state is updated. This function currently
 *     has no failure path.
 *
 * Boundary cases:
 *   The callback is process-wide and may be invoked concurrently. user_data
 *   must remain valid until the callback is cleared or process exit. Do not pass
 *   Go pointers as user_data.
 */
int mcpgo_logs_set_callback(mcpgo_log_callback callback, void *user_data);

/*
 * Return stored structured logs as JSON.
 *
 * Parameters:
 *   server (mcpgo_server): Server handle to filter by. Passing 0 returns all
 *     stored logs. Passing a non-zero handle returns global logs plus logs tied
 *     to that server.
 *   out_json (char **): Required output location. On success it receives a
 *     newly allocated UTF-8 JSON string that must be released with
 *     mcpgo_free_string().
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_json is NULL or another
 *     error occurs. Use mcpgo_last_error() for details on failure.
 *
 * Boundary cases:
 *   *out_json is set to NULL before work begins. An invalid non-zero server
 *   handle is reported as MCPGO_ERROR.
 */
int mcpgo_logs_snapshot(mcpgo_server server, char **out_json);

/*
 * Clear stored structured logs.
 *
 * Parameters:
 *   server (mcpgo_server): Server handle to clear logs for. Passing 0 clears
 *     all stored logs. Passing a non-zero handle clears only logs tied to that
 *     server and keeps global logs.
 *
 * Returns:
 *   int: MCPGO_OK after the clear operation. This function currently has no
 *     failure path.
 *
 * Boundary cases:
 *   Clearing logs does not unregister callbacks and does not reset server or
 *   transport metrics.
 */
int mcpgo_logs_clear(mcpgo_server server);

/*
 * Create a new MCP server.
 *
 * Parameters:
 *   config_json (char *): Optional UTF-8 JSON ServerConfig object. Passing NULL
 *     or an empty string uses defaults. Non-empty input must be valid JSON.
 *   out_server (mcpgo_server *): Required output location. On success it
 *     receives a non-zero server handle. On failure it is set to 0.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_server is NULL, the JSON
 *     configuration is invalid, or server creation fails. Use
 *     mcpgo_last_error() for details on failure.
 *
 * Boundary cases:
 *   The returned handle must be released with mcpgo_server_free(). Handles are
 *   not thread-affine, but callers must coordinate their own concurrent use of
 *   application data referenced by callbacks.
 */
int mcpgo_server_new(char *config_json, mcpgo_server *out_server);

/*
 * Free a server handle.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when the handle is invalid or the
 *     server cannot be shut down cleanly. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Any transports started through this C API for the server are shut down
 *   before the handle is released. The handle must not be used again after
 *   successful free.
 */
int mcpgo_server_free(mcpgo_server server);

/*
 * Stop all running transports for a server without freeing the server handle.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   timeout_ms (int): Graceful shutdown timeout in milliseconds. Values less
 *     than or equal to zero mean no explicit timeout.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when the server handle is invalid
 *     or shutdown fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   The server handle remains valid after shutdown and can be used to start new
 *   transports later. Already stopped transports are tolerated.
 */
int mcpgo_server_shutdown(mcpgo_server server, int timeout_ms);

/*
 * Process one raw MCP JSON-RPC message.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   message_json (char *): Required UTF-8 JSON-RPC request, notification, or
 *     response encoded as a NUL-terminated string.
 *   out_response_json (char **): Required output location. If a response is
 *     produced, receives a newly allocated JSON string. If the message is a
 *     notification or otherwise produces no response, receives NULL.
 *
 * Returns:
 *   int: MCPGO_OK on successful handling, or MCPGO_ERROR when any required
 *     pointer is NULL, the server handle is invalid, the message is invalid, or
 *     handling fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   *out_response_json is set to NULL before work begins. Non-NULL response
 *   strings must be released with mcpgo_free_string().
 */
int mcpgo_server_handle_message(
	mcpgo_server server,
	char *message_json,
	char **out_response_json
);

/*
 * Register a tool backed by a C/C++ callback.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   tool_json (char *): Required UTF-8 JSON MCP Tool definition object.
 *   callback (mcpgo_json_callback): Required function that receives a
 *     CallToolRequest JSON object and returns a CallToolResult JSON object.
 *   user_data (void *): Caller-owned pointer passed to callback. It may be
 *     NULL and must remain valid while the tool remains registered.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when a required pointer is NULL,
 *     the server handle is invalid, tool_json is invalid, or registration
 *     fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Callback invocations may be concurrent. Do not pass Go pointers in
 *   user_data. Deleting or freeing the server unregisters the callback.
 */
int mcpgo_server_add_tool(
	mcpgo_server server,
	char *tool_json,
	mcpgo_json_callback callback,
	void *user_data
);

/*
 * Register a tool from simple metadata plus raw JSON schemas.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   name (char *): Tool name. Passing NULL is treated as an empty string and
 *     will usually fail validation.
 *   description (char *): Human-readable description. NULL is treated as an
 *     empty string.
 *   input_schema_json (char *): Optional input JSON Schema. NULL or empty input
 *     uses an empty object schema.
 *   output_schema_json (char *): Optional output JSON Schema. NULL or empty
 *     input means no structured output schema is declared.
 *   callback (mcpgo_json_callback): Required function that receives a
 *     CallToolRequest JSON object and returns a CallToolResult JSON object.
 *   user_data (void *): Caller-owned pointer passed to callback. It may be
 *     NULL and must remain valid while the tool remains registered.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when callback is NULL, the server
 *     handle is invalid, schema JSON is invalid, or registration fails. Use
 *     mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Empty names are invalid for MCP tools. Callback invocations may be
 *   concurrent, so protect shared user_data state in the caller.
 */
int mcpgo_server_add_tool_schema(
	mcpgo_server server,
	char *name,
	char *description,
	char *input_schema_json,
	char *output_schema_json,
	mcpgo_json_callback callback,
	void *user_data
);

/*
 * Register a simple callback tool with an empty object input schema.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   name (char *): Tool name. Passing NULL is treated as an empty string and
 *     will usually fail validation.
 *   description (char *): Human-readable description. NULL is treated as an
 *     empty string.
 *   callback (mcpgo_json_callback): Required function that receives a
 *     CallToolRequest JSON object and returns a CallToolResult JSON object.
 *   user_data (void *): Caller-owned pointer passed to callback. It may be
 *     NULL and must remain valid while the tool remains registered.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when callback is NULL, the server
 *     handle is invalid, the name is invalid, or registration fails. Use
 *     mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   This form always uses an empty object input schema. Use
 *   mcpgo_server_add_tool_schema() when schema detail is required.
 */
int mcpgo_server_add_tool_simple(
	mcpgo_server server,
	char *name,
	char *description,
	mcpgo_json_callback callback,
	void *user_data
);

/*
 * Register a simple static text tool without a callback.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   name (char *): Tool name. Passing NULL is treated as an empty string and
 *     will usually fail validation.
 *   description (char *): Human-readable description. NULL is treated as an
 *     empty string.
 *   text (char *): Text returned by the tool. NULL is treated as an empty
 *     string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when the server handle is invalid,
 *     the name is invalid, or registration fails. Use mcpgo_last_error() for
 *     details.
 *
 * Boundary cases:
 *   The text value is copied during registration. The caller may release or
 *   modify its buffer after this function returns.
 */
int mcpgo_server_add_tool_static_text(
	mcpgo_server server,
	char *name,
	char *description,
	char *text
);

/*
 * Register a resource backed by a C/C++ callback.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   resource_json (char *): Required UTF-8 JSON MCP Resource definition object.
 *   callback (mcpgo_json_callback): Required function that receives a
 *     ReadResourceRequest JSON object and returns a ReadResourceResult JSON
 *     object or raw JSON contents array.
 *   user_data (void *): Caller-owned pointer passed to callback. It may be
 *     NULL and must remain valid while the resource remains registered.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when a required pointer is NULL,
 *     the server handle is invalid, resource_json is invalid, or registration
 *     fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Callback invocations may be concurrent. Protect shared user_data state in
 *   the caller.
 */
int mcpgo_server_add_resource(
	mcpgo_server server,
	char *resource_json,
	mcpgo_json_callback callback,
	void *user_data
);

/*
 * Register a resource template backed by a C/C++ callback.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   template_json (char *): Required UTF-8 JSON MCP ResourceTemplate definition
 *     object.
 *   callback (mcpgo_json_callback): Required function that receives a
 *     ReadResourceRequest JSON object and returns a ReadResourceResult JSON
 *     object or raw JSON contents array.
 *   user_data (void *): Caller-owned pointer passed to callback. It may be
 *     NULL and must remain valid while the template remains registered.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when a required pointer is NULL,
 *     the server handle is invalid, template_json is invalid, or registration
 *     fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Template callbacks may be invoked concurrently and may receive different
 *   expanded URIs. The caller owns synchronization for user_data.
 */
int mcpgo_server_add_resource_template(
	mcpgo_server server,
	char *template_json,
	mcpgo_json_callback callback,
	void *user_data
);

/*
 * Register a prompt backed by a C/C++ callback.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   prompt_json (char *): Required UTF-8 JSON MCP Prompt definition object.
 *   callback (mcpgo_json_callback): Required function that receives a
 *     GetPromptRequest JSON object and returns a GetPromptResult JSON object.
 *   user_data (void *): Caller-owned pointer passed to callback. It may be
 *     NULL and must remain valid while the prompt remains registered.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when a required pointer is NULL,
 *     the server handle is invalid, prompt_json is invalid, or registration
 *     fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Prompt callbacks may be invoked concurrently. Protect shared user_data
 *   state in the caller.
 */
int mcpgo_server_add_prompt(
	mcpgo_server server,
	char *prompt_json,
	mcpgo_json_callback callback,
	void *user_data
);

/*
 * Remove a globally registered tool by name.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   name (char *): Required tool name to remove.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when name is NULL, the server
 *     handle is invalid, or deletion fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Removing an unknown name is delegated to the underlying server behavior.
 *   After successful deletion, future calls no longer invoke the old callback.
 */
int mcpgo_server_delete_tool(mcpgo_server server, char *name);

/*
 * Remove a globally registered resource by URI.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   uri (char *): Required resource URI to remove.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when uri is NULL, the server
 *     handle is invalid, or deletion fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Removing an unknown URI is delegated to the underlying server behavior.
 */
int mcpgo_server_delete_resource(mcpgo_server server, char *uri);

/*
 * Remove a globally registered prompt by name.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   name (char *): Required prompt name to remove.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when name is NULL, the server
 *     handle is invalid, or deletion fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Removing an unknown name is delegated to the underlying server behavior.
 */
int mcpgo_server_delete_prompt(mcpgo_server server, char *name);

/*
 * Send a notification to every initialized client session.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   method (char *): Required MCP notification method name.
 *   params_json (char *): Optional UTF-8 JSON object. Passing NULL or an empty
 *     string sends no params.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when method is NULL, the server
 *     handle is invalid, params_json is invalid, or delivery fails. Use
 *     mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   params_json must be a JSON object when present because the underlying Go
 *   API accepts map-shaped notification params.
 */
int mcpgo_server_send_notification(
	mcpgo_server server,
	char *method,
	char *params_json
);

/*
 * Run the server on stdin/stdout.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *
 * Returns:
 *   int: MCPGO_OK after the stdio server exits cleanly, or MCPGO_ERROR when the
 *     server handle is invalid or stdio serving fails. Use mcpgo_last_error()
 *     for details.
 *
 * Boundary cases:
 *   This call blocks the current thread until the stdio server exits. Run it on
 *   an application-owned thread if other work must continue concurrently.
 */
int mcpgo_serve_stdio(mcpgo_server server);

/*
 * Start the streamable HTTP transport in a background goroutine.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   config_json (char *): Optional UTF-8 JSON object. Supported fields include
 *     addr, endpointPath, stateless, stateful, disableStreaming,
 *     heartbeatIntervalMs, sessionIdleTtlMs, tlsCertFile, and tlsKeyFile.
 *     Passing NULL or an empty string uses defaults.
 *   out_transport (mcpgo_transport *): Required output location. On success it
 *     receives a non-zero transport handle. On failure it is set to 0.
 *
 * Returns:
 *   int: MCPGO_OK when startup is scheduled, or MCPGO_ERROR when out_transport
 *     is NULL, the server handle is invalid, config_json is invalid, or startup
 *     fails synchronously. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Errors that occur after the background goroutine starts can be read with
 *   mcpgo_transport_last_error().
 */
int mcpgo_streamable_http_start(
	mcpgo_server server,
	char *config_json,
	mcpgo_transport *out_transport
);

/*
 * Start the legacy SSE transport in a background goroutine.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   config_json (char *): Optional UTF-8 JSON object. Supported fields include
 *     addr, baseUrl, basePath, messageEndpoint, sseEndpoint,
 *     appendQueryToMessageEndpoint, useFullUrlForMessageEndpoint, keepAlive,
 *     and keepAliveIntervalMs. Passing NULL or an empty string uses defaults.
 *   out_transport (mcpgo_transport *): Required output location. On success it
 *     receives a non-zero transport handle. On failure it is set to 0.
 *
 * Returns:
 *   int: MCPGO_OK when startup is scheduled, or MCPGO_ERROR when out_transport
 *     is NULL, the server handle is invalid, config_json is invalid, or startup
 *     fails synchronously. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Errors that occur after the background goroutine starts can be read with
 *   mcpgo_transport_last_error().
 */
int mcpgo_sse_start(
	mcpgo_server server,
	char *config_json,
	mcpgo_transport *out_transport
);

/*
 * Gracefully shut down a running transport.
 *
 * Parameters:
 *   transport (mcpgo_transport): Handle returned by a transport start function.
 *   timeout_ms (int): Graceful shutdown timeout in milliseconds. Values less
 *     than or equal to zero mean no explicit timeout.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when the transport handle is
 *     invalid or shutdown fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   The transport handle remains valid after shutdown and can be released later
 *   with mcpgo_transport_free(). Already stopped transports are tolerated.
 */
int mcpgo_transport_shutdown(mcpgo_transport transport, int timeout_ms);

/*
 * Shut down and release a transport handle.
 *
 * Parameters:
 *   transport (mcpgo_transport): Handle returned by a transport start function.
 *   timeout_ms (int): Graceful shutdown timeout in milliseconds. Values less
 *     than or equal to zero mean no explicit timeout.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when the transport handle is
 *     invalid or shutdown/free fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   The handle must not be used again after successful free. Freeing a
 *   transport does not free the owning server.
 */
int mcpgo_transport_free(mcpgo_transport transport, int timeout_ms);

/*
 * Return the last asynchronous error recorded by a transport.
 *
 * Parameters:
 *   transport (mcpgo_transport): Handle returned by a transport start function.
 *   out_error (char **): Required output location. Receives NULL when no
 *     transport error is recorded, otherwise receives a newly allocated UTF-8
 *     diagnostic string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_error is NULL, the
 *     transport handle is invalid, or lookup fails. Use mcpgo_last_error() for
 *     details.
 *
 * Boundary cases:
 *   *out_error is set to NULL before work begins. Non-NULL strings must be
 *   released with mcpgo_free_string().
 */
int mcpgo_transport_last_error(mcpgo_transport transport, char **out_error);

/*
 * Return a server state snapshot as JSON.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   out_json (char **): Required output location. On success it receives a
 *     newly allocated UTF-8 JSON snapshot string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_json is NULL, the server
 *     handle is invalid, or snapshot generation fails. Use mcpgo_last_error()
 *     for details.
 *
 * Boundary cases:
 *   *out_json is set to NULL before work begins. The returned string must be
 *   released with mcpgo_free_string().
 */
int mcpgo_server_snapshot(mcpgo_server server, char **out_json);

/*
 * Return server metrics counters as JSON.
 *
 * Parameters:
 *   server (mcpgo_server): Handle returned by mcpgo_server_new().
 *   out_json (char **): Required output location. On success it receives a
 *     newly allocated UTF-8 JSON metrics string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_json is NULL, the server
 *     handle is invalid, or metrics generation fails. Use mcpgo_last_error()
 *     for details.
 *
 * Boundary cases:
 *   *out_json is set to NULL before work begins. The returned string must be
 *   released with mcpgo_free_string().
 */
int mcpgo_server_metrics(mcpgo_server server, char **out_json);

/*
 * Return one transport state snapshot as JSON.
 *
 * Parameters:
 *   transport (mcpgo_transport): Handle returned by a transport start function.
 *   out_json (char **): Required output location. On success it receives a
 *     newly allocated UTF-8 JSON snapshot string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_json is NULL, the
 *     transport handle is invalid, or snapshot generation fails. Use
 *     mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   *out_json is set to NULL before work begins. The returned string must be
 *   released with mcpgo_free_string().
 */
int mcpgo_transport_snapshot(mcpgo_transport transport, char **out_json);

/*
 * Build a CallToolResult JSON object with one text content item.
 *
 * Parameters:
 *   text (char *): Text to embed in the result. NULL is treated as an empty
 *     string.
 *   out_json (char **): Required output location. On success it receives a
 *     newly allocated UTF-8 JSON CallToolResult string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_json is NULL or result
 *     construction fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   The returned string must be released with mcpgo_free_string(). When used
 *   inside a callback, copy it into caller-owned memory before returning it.
 */
int mcpgo_tool_result_text(char *text, char **out_json);

/*
 * Build a CallToolResult JSON object marked as a tool execution error.
 *
 * Parameters:
 *   text (char *): Error text to embed in the result. NULL is treated as an
 *     empty string.
 *   out_json (char **): Required output location. On success it receives a
 *     newly allocated UTF-8 JSON CallToolResult string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_json is NULL or result
 *     construction fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   This represents a tool-level failure while the MCP protocol request itself
 *   can still succeed. Release the returned string with mcpgo_free_string().
 */
int mcpgo_tool_result_error(char *text, char **out_json);

/*
 * Build a CallToolResult JSON object with structuredContent and text fallback.
 *
 * Parameters:
 *   structured_json (char *): Optional UTF-8 JSON value for structuredContent.
 *     NULL or empty input is accepted.
 *   fallback_text (char *): Text content for clients that do not consume
 *     structured output. NULL is treated as an empty string.
 *   out_json (char **): Required output location. On success it receives a
 *     newly allocated UTF-8 JSON CallToolResult string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_json is NULL,
 *     structured_json is invalid, or result construction fails. Use
 *     mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   The returned string must be released with mcpgo_free_string(). When used
 *   inside a callback, copy it into caller-owned memory before returning it.
 */
int mcpgo_tool_result_structured(
	char *structured_json,
	char *fallback_text,
	char **out_json
);

/*
 * Build a ReadResourceResult JSON object with one text resource content item.
 *
 * Parameters:
 *   uri (char *): Resource URI. NULL is treated as an empty string and may fail
 *     downstream validation.
 *   mime_type (char *): Optional MIME type. NULL is treated as an empty string.
 *   text (char *): Resource text content. NULL is treated as an empty string.
 *   out_json (char **): Required output location. On success it receives a
 *     newly allocated UTF-8 JSON ReadResourceResult string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_json is NULL or result
 *     construction fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Release the returned string with mcpgo_free_string(). When used inside a
 *   callback, copy it into caller-owned memory before returning it.
 */
int mcpgo_resource_result_text(
	char *uri,
	char *mime_type,
	char *text,
	char **out_json
);

/*
 * Build a ReadResourceResult JSON object with one binary resource content item.
 *
 * Parameters:
 *   uri (char *): Resource URI. NULL is treated as an empty string and may fail
 *     downstream validation.
 *   mime_type (char *): Optional MIME type. NULL is treated as an empty string.
 *   base64_blob (char *): Base64-encoded binary content. NULL is treated as an
 *     empty string. The function does not decode or re-encode this value.
 *   out_json (char **): Required output location. On success it receives a
 *     newly allocated UTF-8 JSON ReadResourceResult string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_json is NULL or result
 *     construction fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Invalid base64 may be preserved as text and rejected by clients later.
 *   Release the returned string with mcpgo_free_string().
 */
int mcpgo_resource_result_blob(
	char *uri,
	char *mime_type,
	char *base64_blob,
	char **out_json
);

/*
 * Build a GetPromptResult JSON object with one text message.
 *
 * Parameters:
 *   description (char *): Optional prompt result description. NULL is treated
 *     as an empty string.
 *   role (char *): Message role, normally "user" or "assistant". NULL or empty
 *     input defaults to "user".
 *   text (char *): Message text content. NULL is treated as an empty string.
 *   out_json (char **): Required output location. On success it receives a
 *     newly allocated UTF-8 JSON GetPromptResult string.
 *
 * Returns:
 *   int: MCPGO_OK on success, or MCPGO_ERROR when out_json is NULL or result
 *     construction fails. Use mcpgo_last_error() for details.
 *
 * Boundary cases:
 *   Unexpected role values may be rejected by clients. Release the returned
 *   string with mcpgo_free_string().
 */
int mcpgo_prompt_result_text(
	char *description,
	char *role,
	char *text,
	char **out_json
);

#ifdef __cplusplus
}
#endif

#endif /* MCPGO_CAPI_H */
