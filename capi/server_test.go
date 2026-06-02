package capi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddToolJSONAndHandleMessage(t *testing.T) {
	handle, err := NewServer([]byte(`{"name":"capi-test","version":"1.0.0"}`))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, FreeServer(handle))
	}()

	var receivedRequest map[string]any
	err = AddToolJSON(handle, []byte(`{
		"name":"echo",
		"description":"Echo text",
		"inputSchema":{
			"type":"object",
			"properties":{"text":{"type":"string"}},
			"required":["text"]
		}
	}`), func(requestJSON []byte) ([]byte, error) {
		require.NoError(t, json.Unmarshal(requestJSON, &receivedRequest))
		return ToolResultTextJSON("hello from c")
	})
	require.NoError(t, err)

	listResponse, hasResponse, err := HandleMessage(handle, []byte(`{
		"jsonrpc":"2.0",
		"id":1,
		"method":"tools/list",
		"params":{}
	}`))
	require.NoError(t, err)
	require.True(t, hasResponse)
	assert.Contains(t, string(listResponse), `"name":"echo"`)
	assert.Contains(t, string(listResponse), `"inputSchema"`)

	callResponse, hasResponse, err := HandleMessage(handle, []byte(`{
		"jsonrpc":"2.0",
		"id":2,
		"method":"tools/call",
		"params":{"name":"echo","arguments":{"text":"hello"}}
	}`))
	require.NoError(t, err)
	require.True(t, hasResponse)

	var decoded struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(callResponse, &decoded))
	require.Len(t, decoded.Result.Content, 1)
	assert.Equal(t, "text", decoded.Result.Content[0].Type)
	assert.Equal(t, "hello from c", decoded.Result.Content[0].Text)
	assert.Equal(t, "tools/call", receivedRequest["method"])
}

func TestAddResourceJSONAndHandleMessage(t *testing.T) {
	handle, err := NewServer([]byte(`{"name":"capi-test","version":"1.0.0"}`))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, FreeServer(handle))
	}()

	err = AddResourceJSON(handle, []byte(`{
		"uri":"memo://one",
		"name":"memo",
		"description":"A memo",
		"mimeType":"text/plain"
	}`), func(requestJSON []byte) ([]byte, error) {
		assert.Contains(t, string(requestJSON), `"uri":"memo://one"`)
		return ResourceResultTextJSON("memo://one", "text/plain", "resource body")
	})
	require.NoError(t, err)

	response, hasResponse, err := HandleMessage(handle, []byte(`{
		"jsonrpc":"2.0",
		"id":1,
		"method":"resources/read",
		"params":{"uri":"memo://one"}
	}`))
	require.NoError(t, err)
	require.True(t, hasResponse)

	var decoded struct {
		Result struct {
			Contents []struct {
				URI      string `json:"uri"`
				MIMEType string `json:"mimeType"`
				Text     string `json:"text"`
			} `json:"contents"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(response, &decoded))
	require.Len(t, decoded.Result.Contents, 1)
	assert.Equal(t, "memo://one", decoded.Result.Contents[0].URI)
	assert.Equal(t, "text/plain", decoded.Result.Contents[0].MIMEType)
	assert.Equal(t, "resource body", decoded.Result.Contents[0].Text)
}

func TestAddPromptJSONAndHandleMessage(t *testing.T) {
	handle, err := NewServer([]byte(`{"name":"capi-test","version":"1.0.0"}`))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, FreeServer(handle))
	}()

	err = AddPromptJSON(handle, []byte(`{
		"name":"summarize",
		"description":"Summarize input",
		"arguments":[{"name":"topic","required":true}]
	}`), func(requestJSON []byte) ([]byte, error) {
		assert.Contains(t, string(requestJSON), `"name":"summarize"`)
		return PromptResultTextJSON("summary prompt", "user", "summarize this")
	})
	require.NoError(t, err)

	response, hasResponse, err := HandleMessage(handle, []byte(`{
		"jsonrpc":"2.0",
		"id":1,
		"method":"prompts/get",
		"params":{"name":"summarize","arguments":{"topic":"sdk"}}
	}`))
	require.NoError(t, err)
	require.True(t, hasResponse)

	var decoded struct {
		Result struct {
			Description string `json:"description"`
			Messages    []struct {
				Role    string `json:"role"`
				Content struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"messages"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(response, &decoded))
	assert.Equal(t, "summary prompt", decoded.Result.Description)
	require.Len(t, decoded.Result.Messages, 1)
	assert.Equal(t, "user", decoded.Result.Messages[0].Role)
	assert.Equal(t, "text", decoded.Result.Messages[0].Content.Type)
	assert.Equal(t, "summarize this", decoded.Result.Messages[0].Content.Text)
}

func TestDeleteTool(t *testing.T) {
	handle, err := NewServer([]byte(`{"name":"capi-test","version":"1.0.0"}`))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, FreeServer(handle))
	}()

	err = AddToolJSON(handle, []byte(`{"name":"temporary","inputSchema":{"type":"object","properties":{}}}`), func(requestJSON []byte) ([]byte, error) {
		return ToolResultTextJSON("ok")
	})
	require.NoError(t, err)
	require.NoError(t, DeleteTool(handle, "temporary"))

	response, hasResponse, err := HandleMessage(handle, []byte(`{
		"jsonrpc":"2.0",
		"id":1,
		"method":"tools/list",
		"params":{}
	}`))
	require.NoError(t, err)
	require.True(t, hasResponse)
	assert.NotContains(t, string(response), `"name":"temporary"`)
}

func TestToolRegistrationFormsAndObservability(t *testing.T) {
	ClearLogs(0)
	SetLogCallback(nil)

	handle, err := NewServer([]byte(`{"name":"capi-test","version":"1.0.0"}`))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, FreeServer(handle))
	}()

	var callbackLogs []LogEntry
	SetLogCallback(func(entry LogEntry) {
		callbackLogs = append(callbackLogs, entry)
	})
	defer SetLogCallback(nil)

	require.NoError(t, AddSimpleTool(handle, "simple", "Simple callback", func(requestJSON []byte) ([]byte, error) {
		return ToolResultTextJSON("simple result")
	}))
	require.NoError(t, AddToolWithSchemaJSON(
		handle,
		"schema",
		"Schema callback",
		[]byte(`{"type":"object","properties":{"value":{"type":"string"}}}`),
		nil,
		func(requestJSON []byte) ([]byte, error) {
			return ToolResultTextJSON("schema result")
		},
	))
	require.NoError(t, AddStaticTextTool(handle, "static", "Static text", "static result"))

	response, hasResponse, err := HandleMessage(handle, []byte(`{
		"jsonrpc":"2.0",
		"id":1,
		"method":"tools/call",
		"params":{"name":"static","arguments":{}}
	}`))
	require.NoError(t, err)
	require.True(t, hasResponse)
	assert.Contains(t, string(response), "static result")

	snapshotJSON, err := ServerSnapshotJSON(handle)
	require.NoError(t, err)
	var snapshot ServerSnapshot
	require.NoError(t, json.Unmarshal(snapshotJSON, &snapshot))
	assert.Equal(t, uint64(handle), snapshot.Handle)
	assert.Equal(t, "created", snapshot.State)
	assert.Equal(t, 3, snapshot.Registered.Tools)
	assert.Equal(t, uint64(3), snapshot.Metrics.ToolsRegistered)
	assert.Equal(t, uint64(1), snapshot.Metrics.ToolCallsTotal)
	assert.Equal(t, uint64(1), snapshot.Metrics.ResponsesTotal)

	metricsJSON, err := ServerMetricsJSON(handle)
	require.NoError(t, err)
	var metrics ServerMetrics
	require.NoError(t, json.Unmarshal(metricsJSON, &metrics))
	assert.Equal(t, uint64(1), metrics.ToolCallsTotal)

	logsJSON, err := LogsSnapshotJSON(handle)
	require.NoError(t, err)
	assert.Contains(t, string(logsJSON), "tool.registered")
	assert.Contains(t, string(logsJSON), "message.responded")
	require.NotEmpty(t, callbackLogs)
}

func TestTransportSnapshotAndServerShutdown(t *testing.T) {
	ClearLogs(0)

	handle, err := NewServer([]byte(`{"name":"capi-test","version":"1.0.0"}`))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, FreeServer(handle))
	}()

	transport, err := StartStreamableHTTP(handle, []byte(`{"addr":"127.0.0.1:0","endpointPath":"/mcp"}`))
	require.NoError(t, err)

	transportJSON, err := TransportSnapshotJSON(transport)
	require.NoError(t, err)
	var transportSnapshot TransportSnapshot
	require.NoError(t, json.Unmarshal(transportJSON, &transportSnapshot))
	assert.Equal(t, uint64(transport), transportSnapshot.Handle)
	assert.Equal(t, uint64(handle), transportSnapshot.Server)
	assert.Equal(t, "streamable-http", transportSnapshot.Kind)
	assert.Equal(t, "running", transportSnapshot.State)

	require.NoError(t, ShutdownServer(handle, time.Second))

	snapshotJSON, err := ServerSnapshotJSON(handle)
	require.NoError(t, err)
	var snapshot ServerSnapshot
	require.NoError(t, json.Unmarshal(snapshotJSON, &snapshot))
	assert.Equal(t, "stopped", snapshot.State)
	assert.Equal(t, uint64(1), snapshot.Metrics.TransportsStarted)
	assert.GreaterOrEqual(t, snapshot.Metrics.TransportsStopped, uint64(1))

	logsJSON, err := LogsSnapshotJSON(handle)
	require.NoError(t, err)
	assert.Contains(t, string(logsJSON), "transport.started")
	assert.Contains(t, string(logsJSON), "server.stopped")
}
