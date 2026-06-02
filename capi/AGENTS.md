# C API Module Instructions

This file captures the current requirements and constraints for the `./capi`
module. Future work on this module should read this file first.

## Scope

- Treat `./capi` as the dedicated home for all C/C++ integration work.
- Other repository directories may change at any time. Do not assume files
  outside `./capi` are stable; inspect current upstream code before depending
  on behavior outside this directory.
- Keep new C API code, docs, examples, and command entrypoints inside `./capi`
  unless the user explicitly asks otherwise.
- The Go package path `github.com/mark3labs/mcp-go/capi` must remain usable.

## Goal

Provide a module that wraps the project public MCP server API and exports it to
C so it can be integrated into C/C++ systems.

The design boundary is:

- Opaque handles for server and transport objects.
- UTF-8 JSON at the C boundary for definitions, requests, responses, snapshots,
  metrics, and logs.
- C callbacks for dynamic tool/resource/prompt behavior.
- Explicit memory ownership through `mcpgo_free_string()`.

## Directory Layout

- `./capi/*.go`: Go wrapper package and tests.
- `./capi/cmd/mcpgo-capi`: cgo `c-shared` export command and public C header.
- `./capi/docs/c-api.md`: integration guide.
- `./capi/examples/echo_server.c`: minimal C example.

## Required Capabilities

1. Multiple tool registration forms:
   - Full MCP Tool JSON plus callback.
   - Name/description plus raw input/output schema JSON plus callback.
   - Simple name/description plus empty object schema plus callback.
   - Static text tool with no callback.

2. Free service startup and shutdown:
   - Blocking stdio support.
   - Background streamable HTTP support.
   - Background SSE support.
   - Stop all transports without freeing the server.
   - Shut down and free individual transports.
   - Freeing a server must stop owned transports.

3. Complete structured logging:
   - In-memory ring buffer.
   - Configurable log capacity.
   - Optional live C log callback.
   - Snapshot and clear APIs.
   - Record lifecycle, registration, message handling, callbacks,
     notifications, transport start/stop, and errors.

4. Observable and monitorable state:
   - Server state snapshot.
   - Metrics-only snapshot.
   - Transport state snapshot.
   - Include lifecycle state, uptime, registered object counts, request and
     response counters, error counters, callback counters, notification
     counters, transport counters, and last transport errors.

## C ABI Requirements

- Exported C functions must return `MCPGO_OK` or `MCPGO_ERROR`.
- Detailed failures must be available through `mcpgo_last_error()`.
- Any string allocated by the library and returned to C must be freed with
  `mcpgo_free_string()`.
- Callback request strings are owned by mcp-go and valid only during callback
  execution.
- Callback result strings are owned by the C/C++ application; mcp-go copies them
  before returning from the callback and does not free them.
- `user_data` must be C/C++-owned state that remains valid while the handler is
  registered. Do not store Go pointers in `user_data`.
- Keep the public header comments detailed enough for C/C++ users to integrate
  without reading Go code.

## Exported API Surface To Preserve

Server lifecycle and raw JSON-RPC:

- `mcpgo_server_new`
- `mcpgo_server_shutdown`
- `mcpgo_server_free`
- `mcpgo_server_handle_message`

Tool/resource/prompt registration:

- `mcpgo_server_add_tool`
- `mcpgo_server_add_tool_schema`
- `mcpgo_server_add_tool_simple`
- `mcpgo_server_add_tool_static_text`
- `mcpgo_server_add_resource`
- `mcpgo_server_add_resource_template`
- `mcpgo_server_add_prompt`
- `mcpgo_server_delete_tool`
- `mcpgo_server_delete_resource`
- `mcpgo_server_delete_prompt`

Transports:

- `mcpgo_serve_stdio`
- `mcpgo_streamable_http_start`
- `mcpgo_sse_start`
- `mcpgo_transport_shutdown`
- `mcpgo_transport_free`
- `mcpgo_transport_last_error`
- `mcpgo_transport_snapshot`

Logging and monitoring:

- `mcpgo_logs_set_capacity`
- `mcpgo_logs_set_callback`
- `mcpgo_logs_snapshot`
- `mcpgo_logs_clear`
- `mcpgo_server_snapshot`
- `mcpgo_server_metrics`

Result helpers:

- `mcpgo_tool_result_text`
- `mcpgo_tool_result_error`
- `mcpgo_tool_result_structured`
- `mcpgo_resource_result_text`
- `mcpgo_resource_result_blob`
- `mcpgo_prompt_result_text`

Memory/error helpers:

- `mcpgo_free_string`
- `mcpgo_last_error`

## Testing

Use the local Go binary if `go` is not on PATH:

```powershell
& "C:\Program Files\Go\bin\go.exe" test ./capi ./capi/cmd/mcpgo-capi
```

Current environment limitation observed during development:

- `go test ./... -race` requires cgo.
- With cgo enabled, this Windows environment lacked `gcc`.
- Some repository-wide tests outside `./capi` had Windows-specific failures
  unrelated to this module.

For C shared library builds, a C compiler is required:

```bash
CGO_ENABLED=1 go build -buildmode=c-shared -o build/libmcpgo_capi.so ./capi/cmd/mcpgo-capi
```

Windows example:

```powershell
$env:CGO_ENABLED = "1"
& "C:\Program Files\Go\bin\go.exe" build -buildmode=c-shared -o build\mcpgo_capi.dll ./capi/cmd/mcpgo-capi
```

## Change Discipline

- Prefer changes limited to `./capi`.
- Keep Go code formatted with `gofmt`.
- Add or update tests for behavior changes in `./capi`.
- Update `./capi/docs/c-api.md` and `./capi/cmd/mcpgo-capi/mcpgo_capi.h` when
  exported API behavior changes.
- Do not remove existing exported C functions unless the user explicitly asks
  for an ABI break.
