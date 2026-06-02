# MCP Go C API Integration

This module exposes a small C ABI around the public MCP Go server API. It is
intended for C and C++ systems that want to embed mcp-go without binding Go
structs directly.

The ABI uses:

- Opaque `uint64_t` handles for servers and transports.
- UTF-8 JSON strings for MCP definitions, requests, and results.
- C callbacks for tools, resources, resource templates, and prompts.
- `mcpgo_free_string()` for every string allocated by the shared library.
- Structured in-memory logs, optional log callbacks, and JSON snapshots for
  monitoring.

## Build The Shared Library

The C ABI uses cgo, so a C compiler is required.

Windows with MinGW-w64:

```powershell
$env:CGO_ENABLED = "1"
& "C:\Program Files\Go\bin\go.exe" build -buildmode=c-shared -o build\mcpgo_capi.dll ./capi/cmd/mcpgo-capi
```

Linux:

```bash
CGO_ENABLED=1 go build -buildmode=c-shared -o build/libmcpgo_capi.so ./capi/cmd/mcpgo-capi
```

macOS:

```bash
CGO_ENABLED=1 go build -buildmode=c-shared -o build/libmcpgo_capi.dylib ./capi/cmd/mcpgo-capi
```

Use [`capi/cmd/mcpgo-capi/mcpgo_capi.h`](../cmd/mcpgo-capi/mcpgo_capi.h) as the public
header. Go also generates a header next to the shared library; the source header
contains the documented contract and is designed to match the generated ABI.

## Package A Release

After building the DLL, create a distributable package with:

```powershell
python capi\scripts\publish_capi.py --version 0.1.0
```

The package includes the DLL, documented public C headers, this guide, the C
example, a manifest, and SHA-256 checksums. By default, the package directory is
created at `bin/mcpgo-capi-<version>` and the zip archive is created next to it
under `bin`. The script validates that every published C function and callback
declaration has `Parameters`, `Returns`, and `Boundary cases` documentation
before creating the package.

## Server Configuration

Create a server with `mcpgo_server_new(config_json, &server)`. `config_json` may
be `NULL` or empty for defaults, or a JSON object:

```json
{
  "name": "C Echo Server",
  "version": "1.0.0",
  "instructions": "Optional client instructions",
  "capabilities": {
    "tools": { "listChanged": true },
    "resources": { "subscribe": false, "listChanged": true },
    "prompts": { "listChanged": true },
    "logging": true,
    "roots": true,
    "tasks": { "list": true, "cancel": true, "toolCallTasks": true },
    "completions": true
  },
  "inputSchemaValidation": true,
  "outputSchemaValidation": true,
  "strictInputSchemaDefault": true,
  "maxConcurrentTasks": 8
}
```

If `tools`, `resources`, or `prompts` capabilities are omitted, registering the
first object still enables the relevant capability using the same behavior as
the Go API.

## Register Public API Objects

### Tool Registration Forms

Tools can be registered in several forms:

- `mcpgo_server_add_tool`: full MCP `Tool` JSON plus callback.
- `mcpgo_server_add_tool_schema`: name, description, raw input/output schema
  JSON plus callback.
- `mcpgo_server_add_tool_simple`: name, description, empty object schema plus
  callback.
- `mcpgo_server_add_tool_static_text`: name, description, empty object schema,
  no callback, fixed text result.

Definitions are passed as MCP JSON objects:

```c
static const char *tool =
    "{"
    "\"name\":\"echo\","
    "\"description\":\"Return a static response\","
    "\"inputSchema\":{\"type\":\"object\",\"properties\":{}}"
    "}";

mcpgo_server_add_tool(server, (char *)tool, echo_callback, NULL);
```

Schema form:

```c
mcpgo_server_add_tool_schema(
    server,
    "lookup",
    "Look up a record",
    "{\"type\":\"object\",\"properties\":{\"id\":{\"type\":\"string\"}},\"required\":[\"id\"]}",
    NULL,
    lookup_callback,
    NULL
);
```

Simple form:

```c
mcpgo_server_add_tool_simple(server, "ping", "Return pong", ping_callback, NULL);
```

Static text form:

```c
mcpgo_server_add_tool_static_text(server, "health", "Return health", "ok");
```

Callback result expectations:

- `mcpgo_server_add_tool`: return `CallToolResult` JSON.
- `mcpgo_server_add_resource`: return `ReadResourceResult` JSON or a raw
  resource contents array.
- `mcpgo_server_add_resource_template`: same result shape as resources.
- `mcpgo_server_add_prompt`: return `GetPromptResult` JSON.

The callback input is the corresponding MCP request object encoded as JSON.
Callbacks may be invoked concurrently, especially when stdio worker pools or
HTTP transports process multiple tool calls. Protect shared state in C/C++.
The `user_data` pointer is stored by Go and passed back on every callback; it
must point to C/C++-owned state that remains valid until the handler/server is
freed.

## Memory Ownership

Strings returned through `char **out` parameters are allocated by mcp-go:

```c
char *response = NULL;
if (mcpgo_server_handle_message(server, request, &response) == MCPGO_OK) {
    if (response != NULL) {
        puts(response);
        mcpgo_free_string(response);
    }
}
```

Callback return strings are owned by the application. mcp-go copies them before
the callback returns and does not free them. Return a static string, a
thread-local buffer, or a pointer to memory whose lifetime is managed by your
application. Do not return a pointer to a stack buffer.

Result helper functions such as `mcpgo_tool_result_text()` also allocate output
strings that must be released with `mcpgo_free_string()`. If you use a helper
inside a callback, copy the helper output into application-owned static or
thread-local storage, free the helper output, and return the application-owned
copy. Do not directly return a helper-allocated pointer from a callback unless
your application has another way to free it after mcp-go has copied it.

## Error Handling

Every exported function returns `MCPGO_OK` or `MCPGO_ERROR`. On failure, read the
diagnostic message with `mcpgo_last_error()` and free it:

```c
char *err = mcpgo_last_error();
fprintf(stderr, "mcp-go error: %s\n", err);
mcpgo_free_string(err);
```

## Direct JSON-RPC Integration

For systems that already own networking or event loops, call
`mcpgo_server_handle_message()` with raw JSON-RPC:

```c
char *response = NULL;
mcpgo_server_handle_message(
    server,
    "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"params\":{}}",
    &response
);
```

If the input is an MCP notification, no response is required and
`out_response_json` is set to `NULL`.

## Built-In Transports

Stdio blocks the current thread:

```c
mcpgo_serve_stdio(server);
```

Streamable HTTP starts in the background:

```c
mcpgo_transport transport = 0;
mcpgo_streamable_http_start(
    server,
    "{\"addr\":\":8080\",\"endpointPath\":\"/mcp\"}",
    &transport
);
```

Legacy SSE starts in the background:

```c
mcpgo_transport transport = 0;
mcpgo_sse_start(
    server,
    "{\"addr\":\":8080\",\"sseEndpoint\":\"/sse\",\"messageEndpoint\":\"/message\"}",
    &transport
);
```

Shut down and release transports:

```c
mcpgo_server_shutdown(server, 5000);      /* stop every transport, keep handle */
mcpgo_transport_shutdown(transport, 5000);
mcpgo_transport_free(transport, 5000);
```

`mcpgo_server_shutdown()` stops all transports but keeps the server handle valid,
so you can start HTTP/SSE again later. `mcpgo_server_free()` shuts down any
remaining transports and releases the handle.

If a background transport fails after startup returns, read it with:

```c
char *transport_error = NULL;
mcpgo_transport_last_error(transport, &transport_error);
if (transport_error != NULL) {
    puts(transport_error);
    mcpgo_free_string(transport_error);
}
```

## Logging

The C API records structured logs for server lifecycle, registration changes,
message handling, callbacks, notifications, transport start/stop, and errors.
The default in-memory ring stores 1024 records:

```c
mcpgo_logs_set_capacity(4096);
```

Read logs as JSON:

```c
char *logs = NULL;
mcpgo_logs_snapshot(server, &logs);  /* server == 0 returns all logs */
puts(logs);
mcpgo_free_string(logs);
```

Clear logs:

```c
mcpgo_logs_clear(server); /* server == 0 clears all logs */
```

Register a live log callback:

```c
static void log_sink(const char *log_json, void *user_data) {
    (void)user_data;
    puts(log_json);
}

mcpgo_logs_set_callback(log_sink, NULL);
```

The callback receives one JSON object per log record, including fields such as
`sequence`, `timestamp`, `level`, `event`, `message`, `server`, `transport`, and
event-specific `fields`.

## State And Monitoring

The service state is observable through JSON snapshots.

Server snapshot:

```c
char *snapshot = NULL;
mcpgo_server_snapshot(server, &snapshot);
puts(snapshot);
mcpgo_free_string(snapshot);
```

The server snapshot includes:

- lifecycle state: `created`, `running`, `stopped`, or `freed`
- uptime and creation timestamp
- registered counts for tools, resources, resource templates, and prompts
- request, response, error, callback, transport, registration, and notification
  counters
- active transport snapshots

Metrics-only snapshot:

```c
char *metrics = NULL;
mcpgo_server_metrics(server, &metrics);
puts(metrics);
mcpgo_free_string(metrics);
```

Transport snapshot:

```c
char *transport_state = NULL;
mcpgo_transport_snapshot(transport, &transport_state);
puts(transport_state);
mcpgo_free_string(transport_state);
```

Transport snapshots expose kind, lifecycle state, start/stop timestamps,
shutdown count, and the last transport error.

## Minimal C Example

See [`capi/examples/echo_server.c`](../examples/echo_server.c).

Build after creating the shared library:

Linux:

```bash
cc capi/examples/echo_server.c -Icapi/cmd/mcpgo-capi -Lbuild -lmcpgo_capi -o build/echo_server
LD_LIBRARY_PATH=build ./build/echo_server
```

Windows with MinGW-w64:

```powershell
gcc capi\examples\echo_server.c -Icapi\cmd\mcpgo-capi -Lbuild -lmcpgo_capi -o build\echo_server.exe
$env:PATH = "$PWD\build;$env:PATH"
.\build\echo_server.exe
```
