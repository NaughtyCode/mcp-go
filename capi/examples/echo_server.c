#include <stdio.h>

#include "mcpgo_capi.h"

static void print_last_error(void) {
	char *err = mcpgo_last_error();
	if (err != NULL) {
		fprintf(stderr, "mcp-go error: %s\n", err);
		mcpgo_free_string(err);
	}
}

static const char *echo_tool(const char *request_json, void *user_data) {
	(void)request_json;
	(void)user_data;

	return "{\"content\":[{\"type\":\"text\",\"text\":\"hello from C\"}]}";
}

int main(void) {
	mcpgo_server server = 0;
	char *response = NULL;

	if (mcpgo_server_new("{\"name\":\"c-echo\",\"version\":\"1.0.0\"}", &server) != MCPGO_OK) {
		print_last_error();
		return 1;
	}

	if (mcpgo_server_add_tool(
		    server,
		    "{\"name\":\"echo\",\"description\":\"Return a static response\","
		    "\"inputSchema\":{\"type\":\"object\",\"properties\":{}}}",
		    echo_tool,
		    NULL) != MCPGO_OK) {
		print_last_error();
		mcpgo_server_free(server);
		return 1;
	}

	if (mcpgo_server_handle_message(
		    server,
		    "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\","
		    "\"params\":{\"name\":\"echo\",\"arguments\":{}}}",
		    &response) != MCPGO_OK) {
		print_last_error();
		mcpgo_server_free(server);
		return 1;
	}

	if (response != NULL) {
		puts(response);
		mcpgo_free_string(response);
	}

	if (mcpgo_server_free(server) != MCPGO_OK) {
		print_last_error();
		return 1;
	}
	return 0;
}
