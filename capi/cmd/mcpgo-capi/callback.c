#include "callback.h"

const char *mcpgo_call_json_callback(
	mcpgo_json_callback callback,
	const char *request_json,
	void *user_data
) {
	if (callback == 0) {
		return 0;
	}
	return callback(request_json, user_data);
}

void mcpgo_call_log_callback(
	mcpgo_log_callback callback,
	const char *log_json,
	void *user_data
) {
	if (callback == 0) {
		return;
	}
	callback(log_json, user_data);
}
