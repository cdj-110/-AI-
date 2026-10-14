#include "api/wk_opcda.h"

#include <cstdlib>
#include <iostream>
#include <string>

namespace {

const char* environment(const char* name, const char* fallback = "") {
    const char* value = std::getenv(name);
    return value == nullptr ? fallback : value;
}

}  // namespace

int main() {
    wk_opcda_config_t config{};
    config.abi_version = WK_OPCDA_ABI_VERSION;
    config.host = environment("WK_OPCDA_HOST");
    config.prog_id = environment("WK_OPCDA_PROG_ID");
    config.clsid = environment("WK_OPCDA_CLSID");
    config.domain = environment("WK_OPCDA_DOMAIN");
    config.username = environment("WK_OPCDA_USERNAME");
    config.password = environment("WK_OPCDA_PASSWORD");
    config.workstation = environment("WK_OPCDA_WORKSTATION", "EG100");
    config.security_mode = environment("WK_OPCDA_SECURITY_MODE", "modern");
    config.authentication = "ntlm";
    config.authentication_level = "packet_integrity";
    config.group_name = "WK_OPCDA_DIAGNOSTIC";
    config.locale_id = 0x0409;
    config.requested_update_rate_ms = 1000;
    config.connect_timeout_ms = 10000;
    config.request_timeout_ms = 10000;
    config.auto_reconnect = 0;

    wk_opcda_handle_t* handle = nullptr;
    wk_opcda_error_t code = wk_opcda_create(&config, &handle);
    if (code == WK_OPCDA_OK) code = wk_opcda_connect(handle);
    wk_opcda_item_result_t itemResult{};
    wk_opcda_value_t readValue{};
    const char* itemId = environment("WK_OPCDA_ITEM_ID");
    const bool addItem = itemId[0] != '\0';
    if (code == WK_OPCDA_OK && addItem) {
        const wk_opcda_item_definition_t item{
            1, itemId, environment("WK_OPCDA_ACCESS_PATH"),
            environment("WK_OPCDA_REQUESTED_DATA_TYPE", "auto"), 1};
        code = wk_opcda_add_items(handle, &item, 1, &itemResult);
        std::cout << "item=" << itemId
                  << " client_handle=" << itemResult.client_handle
                  << " server_handle=" << itemResult.server_handle
                  << " canonical_type=" << itemResult.canonical_data_type
                  << " access_rights=" << itemResult.access_rights
                  << " hresult=" << itemResult.hresult << '\n';
    }
    if (code == WK_OPCDA_OK && addItem && itemResult.hresult >= 0) {
        const uint32_t clientHandle = itemResult.client_handle;
        code = wk_opcda_read(
            handle, WK_OPCDA_READ_DEVICE, &clientHandle, 1, &readValue);
        std::cout << "read client_handle=" << readValue.client_handle
                  << " type=" << static_cast<int>(readValue.type)
                  << " quality=" << readValue.quality
                  << " timestamp_ms=" << readValue.source_timestamp_unix_ms
                  << " hresult=" << readValue.hresult;
        switch (readValue.type) {
            case WK_OPCDA_VALUE_BOOL:
            case WK_OPCDA_VALUE_INT64:
            case WK_OPCDA_VALUE_DATETIME:
                std::cout << " value=" << readValue.int64_value;
                break;
            case WK_OPCDA_VALUE_UINT64:
                std::cout << " value=" << readValue.uint64_value;
                break;
            case WK_OPCDA_VALUE_FLOAT64:
                std::cout << " value=" << readValue.float64_value;
                break;
            case WK_OPCDA_VALUE_STRING:
                std::cout << " value="
                          << std::string(readValue.string_value, readValue.string_length);
                break;
            case WK_OPCDA_VALUE_EMPTY:
                break;
        }
        std::cout << '\n';
    }
    wk_opcda_status_t status{};
    if (handle != nullptr) wk_opcda_get_status(handle, &status);
    std::cout << "code=" << static_cast<int>(code)
              << " state=" << static_cast<int>(status.state)
              << " security=" << static_cast<int>(status.security_mode)
              << " endpoint=" << status.rpc_endpoint_port << '\n';
    if (handle != nullptr) {
        const char* detail = wk_opcda_last_error(handle);
        if (detail != nullptr) std::cout << detail << '\n';
    }
    const bool connected = code == WK_OPCDA_OK &&
                           status.state == WK_OPCDA_STATE_CONNECTED &&
                           (!addItem || (itemResult.hresult >= 0 &&
                                        readValue.hresult >= 0));
    wk_opcda_destroy(handle);
    return connected ? 0 : 1;
}
