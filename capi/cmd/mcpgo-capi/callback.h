#ifndef MCPGO_CAPI_CALLBACK_H
#define MCPGO_CAPI_CALLBACK_H

#ifdef __cplusplus
extern "C" {
#endif

/*
 * JSON callback used by tools, resources, resource templates, and prompts.
 *
 * Parameters:
 *   request_json (const char *): UTF-8, NUL-terminated JSON request object
 *     owned by mcp-go. It is valid only for the duration of the callback call.
 *   user_data (void *): Caller-owned pointer supplied when the handler was
 *     registered. It may be NULL. If non-NULL, it must remain valid until the
 *     handler is removed or the owning server is freed.
 *
 * Returns:
 *   const char *: UTF-8, NUL-terminated JSON result string owned by the C/C++
 *     application. mcp-go copies the string before returning from the callback
 *     and never frees the returned pointer. Return NULL to report a callback
 *     failure to mcp-go.
 *
 * Boundary cases:
 *   The callback may be invoked concurrently. Protect shared user_data state in
 *   the caller. Do not return a pointer to stack memory. Do not pass Go
 *   pointers through user_data. Expected result shapes are CallToolResult JSON
 *   for tools, ReadResourceResult JSON or a raw contents array for resources,
 *   and GetPromptResult JSON for prompts.
 */
typedef const char *(*mcpgo_json_callback)(const char *request_json, void *user_data);

/*
 * Structured log callback used by mcp-go log delivery.
 *
 * Parameters:
 *   log_json (const char *): UTF-8, NUL-terminated JSON log record owned by
 *     mcp-go. It is valid only for the duration of the callback call.
 *   user_data (void *): Caller-owned pointer supplied to
 *     mcpgo_logs_set_callback(). It may be NULL.
 *
 * Returns:
 *   void: No status is returned. The callback should handle its own failures.
 *
 * Boundary cases:
 *   The callback is process-wide and may be invoked concurrently. It should not
 *   block for long periods because it runs on mcp-go execution paths. Do not
 *   store log_json after the callback returns unless you copy it first.
 */
typedef void (*mcpgo_log_callback)(const char *log_json, void *user_data);

/*
 * Internal cgo callback bridge for JSON callbacks.
 *
 * Parameters:
 *   callback (mcpgo_json_callback): Function pointer to invoke. It must not be
 *     NULL when called by the Go bridge.
 *   request_json (const char *): UTF-8 request JSON passed through to callback.
 *   user_data (void *): Caller-owned state pointer passed through to callback.
 *
 * Returns:
 *   const char *: Whatever callback returns, including NULL on callback
 *     failure.
 *
 * Boundary cases:
 *   This helper is used by the DLL internals so Go can call a C function
 *   pointer. Applications normally must not call it directly or depend on it as
 *   a stable exported ABI symbol.
 */
const char *mcpgo_call_json_callback(
	mcpgo_json_callback callback,
	const char *request_json,
	void *user_data
);

/*
 * Internal cgo callback bridge for log callbacks.
 *
 * Parameters:
 *   callback (mcpgo_log_callback): Function pointer to invoke. It must not be
 *     NULL when called by the Go bridge.
 *   log_json (const char *): UTF-8 log record JSON passed through to callback.
 *   user_data (void *): Caller-owned state pointer passed through to callback.
 *
 * Returns:
 *   void: No status is returned.
 *
 * Boundary cases:
 *   This helper is used by the DLL internals so Go can call a C function
 *   pointer. Applications normally must not call it directly or depend on it as
 *   a stable exported ABI symbol.
 */
void mcpgo_call_log_callback(
	mcpgo_log_callback callback,
	const char *log_json,
	void *user_data
);

#ifdef __cplusplus
}
#endif

#endif /* MCPGO_CAPI_CALLBACK_H */
