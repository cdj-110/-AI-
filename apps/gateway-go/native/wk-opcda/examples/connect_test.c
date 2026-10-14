#include "api/wk_opcda.h"

#include <stdlib.h>
#include <unistd.h>

static const char* environment(const char* name, const char* fallback) {
    const char* value = getenv(name);
    return value == NULL ? fallback : value;
}

static void trace(const char* message) {
    size_t length = 0;
    while (message[length] != '\0') ++length;
    while (length > 0) {
        ssize_t written = write(STDERR_FILENO, message, length);
        if (written <= 0) return;
        message += written;
        length -= (size_t)written;
    }
}

int main(void) {
    trace("[diag] main\n");
    wk_opcda_config_t config = {0};
    config.abi_version = WK_OPCDA_ABI_VERSION;
    config.host = environment("WK_OPCDA_HOST", "");
    config.prog_id = environment("WK_OPCDA_PROG_ID", "");
    config.clsid = environment("WK_OPCDA_CLSID", "");
    config.domain = environment("WK_OPCDA_DOMAIN", "");
    config.username = environment("WK_OPCDA_USERNAME", "");
    config.password = environment("WK_OPCDA_PASSWORD", "");
    config.workstation = environment("WK_OPCDA_WORKSTATION", "EG100");
    config.security_mode = environment("WK_OPCDA_SECURITY_MODE", "modern");
    config.authentication = "ntlm";
    config.authentication_level = "packet_integrity";
    config.group_name = "WK_OPCDA_DIAGNOSTIC";
    config.locale_id = 0x0409;
    config.requested_update_rate_ms = 1000;
    config.connect_timeout_ms = 10000;
    config.request_timeout_ms = 10000;

    wk_opcda_handle_t* handle = NULL;
    trace("[diag] create\n");
    wk_opcda_error_t code = wk_opcda_create(&config, &handle);
    if (code != WK_OPCDA_OK) {
        trace("[diag] create failed\n");
        return 1;
    }

    trace("[diag] connect\n");
    code = wk_opcda_connect(handle);
    if (code != WK_OPCDA_OK) {
        trace("[diag] connect failed: ");
        trace(wk_opcda_last_error(handle));
        trace("\n");
        wk_opcda_destroy(handle);
        return 1;
    }
    trace("[diag] connect complete\n");

    wk_opcda_item_result_t item_result = {0};
    wk_opcda_value_t read_value = {0};
    const char* item_id = environment("WK_OPCDA_ITEM_ID", "");
    const int add_item = item_id[0] != '\0';
    if (add_item) {
        const wk_opcda_item_definition_t item = {
            1, item_id, environment("WK_OPCDA_ACCESS_PATH", ""),
            environment("WK_OPCDA_REQUESTED_DATA_TYPE", "auto"), 1};
        trace("[diag] add_items\n");
        code = wk_opcda_add_items(handle, &item, 1, &item_result);
        if (code != WK_OPCDA_OK || item_result.hresult < 0) {
            trace("[diag] add_items failed: ");
            trace(wk_opcda_last_error(handle));
            trace("\n");
            wk_opcda_destroy(handle);
            return 1;
        }
        trace("[diag] add_items complete\n");

        const uint32_t client_handle = item_result.client_handle;
        trace("[diag] read\n");
        code = wk_opcda_read(
            handle, WK_OPCDA_READ_DEVICE, &client_handle, 1, &read_value);
        if (code != WK_OPCDA_OK || read_value.hresult < 0) {
            trace("[diag] read failed: ");
            trace(wk_opcda_last_error(handle));
            trace("\n");
            wk_opcda_destroy(handle);
            return 1;
        }
        if ((read_value.quality & 0xC0u) != 0xC0u ||
            read_value.source_timestamp_unix_ms <= 0) {
            trace("[diag] read returned invalid quality or timestamp\n");
            wk_opcda_destroy(handle);
            return 1;
        }
        trace("[diag] read complete: quality=GOOD timestamp=valid\n");
    }

    wk_opcda_status_t status = {0};
    if (wk_opcda_get_status(handle, &status) != WK_OPCDA_OK ||
        status.state != WK_OPCDA_STATE_CONNECTED) {
        trace("[diag] status is not connected\n");
        wk_opcda_destroy(handle);
        return 1;
    }
    trace("[diag] status=CONNECTED configured packet-security path complete\n");
    wk_opcda_destroy(handle);
    trace("[diag] done\n");
    return 0;
}
