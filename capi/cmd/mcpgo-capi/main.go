//go:build cgo

package main

/*
#include <stdint.h>
#include <stdlib.h>
#include "callback.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
	"unsafe"

	"github.com/mark3labs/mcp-go/capi"
)

var lastError = struct {
	sync.Mutex
	message string
}{}

func main() {}

//export mcpgo_free_string
func mcpgo_free_string(value *C.char) {
	C.free(unsafe.Pointer(value))
}

//export mcpgo_last_error
func mcpgo_last_error() *C.char {
	lastError.Lock()
	message := lastError.message
	lastError.Unlock()
	return C.CString(message)
}

//export mcpgo_server_new
func mcpgo_server_new(configJSON *C.char, outServer *C.uint64_t) C.int {
	if outServer == nil {
		return fail(errors.New("out_server is NULL"))
	}
	*outServer = 0

	handle, err := capi.NewServer(cBytes(configJSON))
	if err != nil {
		return fail(err)
	}
	*outServer = C.uint64_t(handle)
	return ok()
}

//export mcpgo_server_free
func mcpgo_server_free(server C.uint64_t) C.int {
	return finish(capi.FreeServer(capi.ServerHandle(server)))
}

//export mcpgo_server_shutdown
func mcpgo_server_shutdown(server C.uint64_t, timeoutMS C.int) C.int {
	return finish(capi.ShutdownServer(
		capi.ServerHandle(server),
		durationFromMS(timeoutMS),
	))
}

//export mcpgo_server_handle_message
func mcpgo_server_handle_message(server C.uint64_t, messageJSON *C.char, outResponseJSON **C.char) C.int {
	if messageJSON == nil {
		return fail(errors.New("message_json is NULL"))
	}
	if outResponseJSON == nil {
		return fail(errors.New("out_response_json is NULL"))
	}
	*outResponseJSON = nil

	responseJSON, hasResponse, err := capi.HandleMessage(capi.ServerHandle(server), cBytes(messageJSON))
	if err != nil {
		return fail(err)
	}
	if !hasResponse {
		return ok()
	}
	return finish(writeCString(outResponseJSON, responseJSON))
}

//export mcpgo_server_add_tool
func mcpgo_server_add_tool(
	server C.uint64_t,
	toolJSON *C.char,
	callback C.mcpgo_json_callback,
	userData unsafe.Pointer,
) C.int {
	if toolJSON == nil {
		return fail(errors.New("tool_json is NULL"))
	}
	goCallback, err := makeJSONCallback(callback, userData)
	if err != nil {
		return fail(err)
	}
	return finish(capi.AddToolJSON(capi.ServerHandle(server), cBytes(toolJSON), goCallback))
}

//export mcpgo_server_add_tool_schema
func mcpgo_server_add_tool_schema(
	server C.uint64_t,
	name *C.char,
	description *C.char,
	inputSchemaJSON *C.char,
	outputSchemaJSON *C.char,
	callback C.mcpgo_json_callback,
	userData unsafe.Pointer,
) C.int {
	goCallback, err := makeJSONCallback(callback, userData)
	if err != nil {
		return fail(err)
	}
	return finish(capi.AddToolWithSchemaJSON(
		capi.ServerHandle(server),
		goString(name),
		goString(description),
		cBytes(inputSchemaJSON),
		cBytes(outputSchemaJSON),
		goCallback,
	))
}

//export mcpgo_server_add_tool_simple
func mcpgo_server_add_tool_simple(
	server C.uint64_t,
	name *C.char,
	description *C.char,
	callback C.mcpgo_json_callback,
	userData unsafe.Pointer,
) C.int {
	goCallback, err := makeJSONCallback(callback, userData)
	if err != nil {
		return fail(err)
	}
	return finish(capi.AddSimpleTool(
		capi.ServerHandle(server),
		goString(name),
		goString(description),
		goCallback,
	))
}

//export mcpgo_server_add_tool_static_text
func mcpgo_server_add_tool_static_text(
	server C.uint64_t,
	name *C.char,
	description *C.char,
	text *C.char,
) C.int {
	return finish(capi.AddStaticTextTool(
		capi.ServerHandle(server),
		goString(name),
		goString(description),
		goString(text),
	))
}

//export mcpgo_server_add_resource
func mcpgo_server_add_resource(
	server C.uint64_t,
	resourceJSON *C.char,
	callback C.mcpgo_json_callback,
	userData unsafe.Pointer,
) C.int {
	if resourceJSON == nil {
		return fail(errors.New("resource_json is NULL"))
	}
	goCallback, err := makeJSONCallback(callback, userData)
	if err != nil {
		return fail(err)
	}
	return finish(capi.AddResourceJSON(capi.ServerHandle(server), cBytes(resourceJSON), goCallback))
}

//export mcpgo_server_add_resource_template
func mcpgo_server_add_resource_template(
	server C.uint64_t,
	templateJSON *C.char,
	callback C.mcpgo_json_callback,
	userData unsafe.Pointer,
) C.int {
	if templateJSON == nil {
		return fail(errors.New("template_json is NULL"))
	}
	goCallback, err := makeJSONCallback(callback, userData)
	if err != nil {
		return fail(err)
	}
	return finish(capi.AddResourceTemplateJSON(capi.ServerHandle(server), cBytes(templateJSON), goCallback))
}

//export mcpgo_server_add_prompt
func mcpgo_server_add_prompt(
	server C.uint64_t,
	promptJSON *C.char,
	callback C.mcpgo_json_callback,
	userData unsafe.Pointer,
) C.int {
	if promptJSON == nil {
		return fail(errors.New("prompt_json is NULL"))
	}
	goCallback, err := makeJSONCallback(callback, userData)
	if err != nil {
		return fail(err)
	}
	return finish(capi.AddPromptJSON(capi.ServerHandle(server), cBytes(promptJSON), goCallback))
}

//export mcpgo_server_delete_tool
func mcpgo_server_delete_tool(server C.uint64_t, name *C.char) C.int {
	if name == nil {
		return fail(errors.New("name is NULL"))
	}
	return finish(capi.DeleteTool(capi.ServerHandle(server), goString(name)))
}

//export mcpgo_server_delete_resource
func mcpgo_server_delete_resource(server C.uint64_t, uri *C.char) C.int {
	if uri == nil {
		return fail(errors.New("uri is NULL"))
	}
	return finish(capi.DeleteResource(capi.ServerHandle(server), goString(uri)))
}

//export mcpgo_server_delete_prompt
func mcpgo_server_delete_prompt(server C.uint64_t, name *C.char) C.int {
	if name == nil {
		return fail(errors.New("name is NULL"))
	}
	return finish(capi.DeletePrompt(capi.ServerHandle(server), goString(name)))
}

//export mcpgo_server_send_notification
func mcpgo_server_send_notification(server C.uint64_t, method *C.char, paramsJSON *C.char) C.int {
	if method == nil {
		return fail(errors.New("method is NULL"))
	}
	return finish(capi.SendNotificationToAllClients(
		capi.ServerHandle(server),
		goString(method),
		cBytes(paramsJSON),
	))
}

//export mcpgo_serve_stdio
func mcpgo_serve_stdio(server C.uint64_t) C.int {
	return finish(capi.ServeStdio(capi.ServerHandle(server)))
}

//export mcpgo_streamable_http_start
func mcpgo_streamable_http_start(server C.uint64_t, configJSON *C.char, outTransport *C.uint64_t) C.int {
	if outTransport == nil {
		return fail(errors.New("out_transport is NULL"))
	}
	*outTransport = 0

	handle, err := capi.StartStreamableHTTP(capi.ServerHandle(server), cBytes(configJSON))
	if err != nil {
		return fail(err)
	}
	*outTransport = C.uint64_t(handle)
	return ok()
}

//export mcpgo_sse_start
func mcpgo_sse_start(server C.uint64_t, configJSON *C.char, outTransport *C.uint64_t) C.int {
	if outTransport == nil {
		return fail(errors.New("out_transport is NULL"))
	}
	*outTransport = 0

	handle, err := capi.StartSSE(capi.ServerHandle(server), cBytes(configJSON))
	if err != nil {
		return fail(err)
	}
	*outTransport = C.uint64_t(handle)
	return ok()
}

//export mcpgo_transport_shutdown
func mcpgo_transport_shutdown(transport C.uint64_t, timeoutMS C.int) C.int {
	return finish(capi.ShutdownTransport(
		capi.TransportHandle(transport),
		durationFromMS(timeoutMS),
	))
}

//export mcpgo_transport_free
func mcpgo_transport_free(transport C.uint64_t, timeoutMS C.int) C.int {
	return finish(capi.FreeTransport(
		capi.TransportHandle(transport),
		durationFromMS(timeoutMS),
	))
}

//export mcpgo_transport_last_error
func mcpgo_transport_last_error(transport C.uint64_t, outError **C.char) C.int {
	if outError == nil {
		return fail(errors.New("out_error is NULL"))
	}
	*outError = nil

	message, err := capi.TransportLastError(capi.TransportHandle(transport))
	if err != nil {
		return fail(err)
	}
	if message == "" {
		return ok()
	}
	return finish(writeCString(outError, []byte(message)))
}

//export mcpgo_server_snapshot
func mcpgo_server_snapshot(server C.uint64_t, outJSON **C.char) C.int {
	if outJSON == nil {
		return fail(errors.New("out_json is NULL"))
	}
	*outJSON = nil

	snapshotJSON, err := capi.ServerSnapshotJSON(capi.ServerHandle(server))
	if err != nil {
		return fail(err)
	}
	return finish(writeCString(outJSON, snapshotJSON))
}

//export mcpgo_server_metrics
func mcpgo_server_metrics(server C.uint64_t, outJSON **C.char) C.int {
	if outJSON == nil {
		return fail(errors.New("out_json is NULL"))
	}
	*outJSON = nil

	metricsJSON, err := capi.ServerMetricsJSON(capi.ServerHandle(server))
	if err != nil {
		return fail(err)
	}
	return finish(writeCString(outJSON, metricsJSON))
}

//export mcpgo_transport_snapshot
func mcpgo_transport_snapshot(transport C.uint64_t, outJSON **C.char) C.int {
	if outJSON == nil {
		return fail(errors.New("out_json is NULL"))
	}
	*outJSON = nil

	snapshotJSON, err := capi.TransportSnapshotJSON(capi.TransportHandle(transport))
	if err != nil {
		return fail(err)
	}
	return finish(writeCString(outJSON, snapshotJSON))
}

//export mcpgo_logs_snapshot
func mcpgo_logs_snapshot(server C.uint64_t, outJSON **C.char) C.int {
	if outJSON == nil {
		return fail(errors.New("out_json is NULL"))
	}
	*outJSON = nil

	logsJSON, err := capi.LogsSnapshotJSON(capi.ServerHandle(server))
	if err != nil {
		return fail(err)
	}
	return finish(writeCString(outJSON, logsJSON))
}

//export mcpgo_logs_clear
func mcpgo_logs_clear(server C.uint64_t) C.int {
	capi.ClearLogs(capi.ServerHandle(server))
	return ok()
}

//export mcpgo_logs_set_capacity
func mcpgo_logs_set_capacity(capacity C.int) C.int {
	capi.SetLogCapacity(int(capacity))
	return ok()
}

//export mcpgo_logs_set_callback
func mcpgo_logs_set_callback(callback C.mcpgo_log_callback, userData unsafe.Pointer) C.int {
	if callback == nil {
		capi.SetLogCallback(nil)
		return ok()
	}
	capi.SetLogCallback(func(entry capi.LogEntry) {
		data, err := json.Marshal(entry)
		if err != nil {
			return
		}
		cLogJSON := C.CString(string(data))
		defer C.free(unsafe.Pointer(cLogJSON))
		C.mcpgo_call_log_callback(callback, cLogJSON, userData)
	})
	return ok()
}

//export mcpgo_tool_result_text
func mcpgo_tool_result_text(text *C.char, outJSON **C.char) C.int {
	if outJSON == nil {
		return fail(errors.New("out_json is NULL"))
	}
	*outJSON = nil

	resultJSON, err := capi.ToolResultTextJSON(goString(text))
	if err != nil {
		return fail(err)
	}
	return finish(writeCString(outJSON, resultJSON))
}

//export mcpgo_tool_result_error
func mcpgo_tool_result_error(text *C.char, outJSON **C.char) C.int {
	if outJSON == nil {
		return fail(errors.New("out_json is NULL"))
	}
	*outJSON = nil

	resultJSON, err := capi.ToolResultErrorJSON(goString(text))
	if err != nil {
		return fail(err)
	}
	return finish(writeCString(outJSON, resultJSON))
}

//export mcpgo_tool_result_structured
func mcpgo_tool_result_structured(structuredJSON *C.char, fallbackText *C.char, outJSON **C.char) C.int {
	if outJSON == nil {
		return fail(errors.New("out_json is NULL"))
	}
	*outJSON = nil

	resultJSON, err := capi.ToolResultStructuredJSON(cBytes(structuredJSON), goString(fallbackText))
	if err != nil {
		return fail(err)
	}
	return finish(writeCString(outJSON, resultJSON))
}

//export mcpgo_resource_result_text
func mcpgo_resource_result_text(uri *C.char, mimeType *C.char, text *C.char, outJSON **C.char) C.int {
	if outJSON == nil {
		return fail(errors.New("out_json is NULL"))
	}
	*outJSON = nil

	resultJSON, err := capi.ResourceResultTextJSON(goString(uri), goString(mimeType), goString(text))
	if err != nil {
		return fail(err)
	}
	return finish(writeCString(outJSON, resultJSON))
}

//export mcpgo_resource_result_blob
func mcpgo_resource_result_blob(uri *C.char, mimeType *C.char, base64Blob *C.char, outJSON **C.char) C.int {
	if outJSON == nil {
		return fail(errors.New("out_json is NULL"))
	}
	*outJSON = nil

	resultJSON, err := capi.ResourceResultBlobJSON(goString(uri), goString(mimeType), goString(base64Blob))
	if err != nil {
		return fail(err)
	}
	return finish(writeCString(outJSON, resultJSON))
}

//export mcpgo_prompt_result_text
func mcpgo_prompt_result_text(description *C.char, role *C.char, text *C.char, outJSON **C.char) C.int {
	if outJSON == nil {
		return fail(errors.New("out_json is NULL"))
	}
	*outJSON = nil

	resultJSON, err := capi.PromptResultTextJSON(goString(description), goString(role), goString(text))
	if err != nil {
		return fail(err)
	}
	return finish(writeCString(outJSON, resultJSON))
}

func makeJSONCallback(callback C.mcpgo_json_callback, userData unsafe.Pointer) (capi.JSONCallback, error) {
	if callback == nil {
		return nil, errors.New("callback is NULL")
	}
	return func(requestJSON []byte) ([]byte, error) {
		cRequestJSON := C.CString(string(requestJSON))
		defer C.free(unsafe.Pointer(cRequestJSON))

		cResultJSON := C.mcpgo_call_json_callback(callback, cRequestJSON, userData)
		if cResultJSON == nil {
			return nil, errors.New("callback returned NULL")
		}
		return []byte(C.GoString(cResultJSON)), nil
	}, nil
}

func cBytes(value *C.char) []byte {
	if value == nil {
		return nil
	}
	return []byte(goString(value))
}

func goString(value *C.char) string {
	if value == nil {
		return ""
	}
	return C.GoString(value)
}

func writeCString(out **C.char, value []byte) error {
	if out == nil {
		return errors.New("output string pointer is NULL")
	}
	if value == nil {
		*out = nil
		return nil
	}
	*out = C.CString(string(value))
	return nil
}

func durationFromMS(timeoutMS C.int) time.Duration {
	if timeoutMS <= 0 {
		return 0
	}
	return time.Duration(int(timeoutMS)) * time.Millisecond
}

func ok() C.int {
	lastError.Lock()
	lastError.message = ""
	lastError.Unlock()
	return C.int(capi.StatusOK)
}

func fail(err error) C.int {
	if err == nil {
		return ok()
	}
	capi.RecordLog("error", 0, 0, "capi.error", err.Error(), nil)
	lastError.Lock()
	lastError.message = err.Error()
	lastError.Unlock()
	return C.int(capi.StatusError)
}

func finish(err error) C.int {
	if err != nil {
		return fail(err)
	}
	return ok()
}
