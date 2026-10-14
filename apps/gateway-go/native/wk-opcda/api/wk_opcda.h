#ifndef WK_OPCDA_H
#define WK_OPCDA_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define WK_OPCDA_ABI_VERSION 3u

typedef struct wk_opcda_handle wk_opcda_handle_t;

typedef enum wk_opcda_error {
    WK_OPCDA_OK = 0,
    WK_OPCDA_ERR_ARGUMENT = 1,
    WK_OPCDA_ERR_NETWORK = 2,
    WK_OPCDA_ERR_RPC = 3,
    WK_OPCDA_ERR_AUTH = 4,
    WK_OPCDA_ERR_DCOM = 5,
    WK_OPCDA_ERR_OPC_SERVER = 6,
    WK_OPCDA_ERR_GROUP = 7,
    WK_OPCDA_ERR_ITEM = 8,
    WK_OPCDA_ERR_READ = 9,
    WK_OPCDA_ERR_WRITE = 10,
    WK_OPCDA_ERR_TIMEOUT = 11,
    WK_TIMEOUT = WK_OPCDA_ERR_TIMEOUT,
    WK_OPCDA_ERR_CANCELLED = 12,
    WK_OPCDA_ERR_NOT_IMPLEMENTED = 13,

    WK_RPC_CONNECT_FAILED = 100,
    WK_RPC_ENDPOINT_FAILED = 101,
    WK_RPC_BIND_FAILED = 102,
    WK_RPC_CONTEXT_FAILED = 103,
    WK_NTLM_NEGOTIATE_FAILED = 120,
    WK_NTLM_CHALLENGE_FAILED = 121,
    WK_NTLM_AUTH_FAILED = 122,
    WK_RPC_AUTH_LEVEL_UNSUPPORTED = 123,
    WK_RPC_PACKET_SIGN_FAILED = 124,
    WK_RPC_PACKET_VERIFY_FAILED = 125,
    WK_DCOM_REMOTE_ACTIVATION_DENIED = 140,
    WK_DCOM_REMOTE_ACTIVATION_FAILED = 141,
    WK_DCOM_QUERY_INTERFACE_FAILED = 142,
    WK_OPC_ADD_GROUP_FAILED = 160,
    WK_OPC_ADD_ITEMS_FAILED = 161,
    WK_OPC_READ_FAILED = 162,
    WK_OPC_WRITE_FAILED = 163,
    WK_OPC_BROWSE_FAILED = 164,
    WK_DISCONNECTED = 180
} wk_opcda_error_t;

typedef enum wk_opcda_security_mode {
    WK_OPCDA_SECURITY_MODERN = 1,
    WK_OPCDA_SECURITY_PACKET_INTEGRITY = 2,
    WK_OPCDA_SECURITY_PACKET_PRIVACY = 3,
    WK_OPCDA_SECURITY_LEGACY_CONNECT = 4
} wk_opcda_security_mode_t;

typedef enum wk_opcda_connection_state {
    WK_OPCDA_STATE_DISCONNECTED = 0,
    WK_OPCDA_STATE_CONNECTING = 1,
    WK_OPCDA_STATE_AUTHENTICATING = 2,
    WK_OPCDA_STATE_ACTIVATING = 3,
    WK_OPCDA_STATE_CONNECTED = 4,
    WK_OPCDA_STATE_FAULT = 5,
    WK_OPCDA_STATE_RECONNECTING = 6
} wk_opcda_connection_state_t;

typedef enum wk_opcda_value_type {
    WK_OPCDA_VALUE_EMPTY = 0,
    WK_OPCDA_VALUE_BOOL = 1,
    WK_OPCDA_VALUE_INT64 = 2,
    WK_OPCDA_VALUE_UINT64 = 3,
    WK_OPCDA_VALUE_FLOAT64 = 4,
    WK_OPCDA_VALUE_STRING = 5,
    WK_OPCDA_VALUE_DATETIME = 6,
    WK_OPCDA_VALUE_FLOAT64_ARRAY = 7,
    WK_OPCDA_VALUE_STRING_ARRAY = 8
} wk_opcda_value_type_t;

typedef enum wk_opcda_read_source {
    WK_OPCDA_READ_CACHE = 1,
    WK_OPCDA_READ_DEVICE = 2
} wk_opcda_read_source_t;

typedef struct wk_opcda_config {
    uint32_t abi_version;
    const char* host;
    const char* prog_id;
    const char* clsid;
    const char* domain;
    const char* username;
    const char* password;
    const char* workstation;
    const char* security_mode;
    /* Deprecated V1.0 compatibility fields. security_mode takes precedence. */
    const char* authentication;
    const char* authentication_level;
    const char* group_name;
    const char* local_bind_ip;
    uint32_t locale_id;
    uint32_t requested_update_rate_ms;
    uint32_t connect_timeout_ms;
    uint32_t request_timeout_ms;
    float deadband;
    uint8_t auto_reconnect;
} wk_opcda_config_t;

typedef struct wk_opcda_item_definition {
    uint32_t client_handle;
    const char* item_id;
    const char* access_path;
    const char* requested_data_type;
    uint8_t active;
} wk_opcda_item_definition_t;

typedef struct wk_opcda_item_result {
    uint32_t client_handle;
    uint32_t server_handle;
    uint16_t canonical_data_type;
    uint32_t access_rights;
    int32_t hresult;
} wk_opcda_item_result_t;

typedef struct wk_opcda_value {
    uint32_t client_handle;
    uint32_t quality;
    int64_t source_timestamp_unix_ms;
    int32_t hresult;
    wk_opcda_value_type_t type;
    int64_t int64_value;
    uint64_t uint64_value;
    double float64_value;
    const char* string_value;
    size_t string_length;
    const double* float64_array_values;
    const char** string_array_values;
    const size_t* string_array_lengths;
    size_t array_length;
} wk_opcda_value_t;

typedef struct wk_opcda_browse_entry {
    const char* name;
    const char* item_id;
    const char* data_type;
    uint32_t access_rights;
    uint8_t is_branch;
} wk_opcda_browse_entry_t;

typedef struct wk_opcda_status {
    wk_opcda_connection_state_t state;
    wk_opcda_security_mode_t security_mode;
    wk_opcda_error_t last_error_code;
    uint16_t rpc_endpoint_port;
} wk_opcda_status_t;

typedef void (*wk_opcda_data_callback_t)(const wk_opcda_value_t* values,
                                         size_t value_count,
                                         void* user_data);

uint32_t wk_opcda_abi_version(void);
wk_opcda_error_t wk_opcda_create(const wk_opcda_config_t* config, wk_opcda_handle_t** out_handle);
wk_opcda_error_t wk_opcda_connect(wk_opcda_handle_t* handle);
wk_opcda_error_t wk_opcda_add_items(wk_opcda_handle_t* handle,
                                    const wk_opcda_item_definition_t* items,
                                    size_t item_count,
                                    wk_opcda_item_result_t* results);
wk_opcda_error_t wk_opcda_remove_items(wk_opcda_handle_t* handle,
                                       const uint32_t* client_handles,
                                       size_t item_count,
                                       int32_t* item_hresults);
wk_opcda_error_t wk_opcda_read(wk_opcda_handle_t* handle,
                               wk_opcda_read_source_t source,
                               const uint32_t* client_handles,
                               size_t item_count,
                               wk_opcda_value_t* values);
wk_opcda_error_t wk_opcda_write(wk_opcda_handle_t* handle,
                                const uint32_t* client_handles,
                                const wk_opcda_value_t* values,
                                size_t item_count,
                                int32_t* item_results);
wk_opcda_error_t wk_opcda_browse(wk_opcda_handle_t* handle,
                                 const char* branch_item_id,
                                 wk_opcda_browse_entry_t* entries,
                                 size_t entry_capacity,
                                 size_t* entry_count);
wk_opcda_error_t wk_opcda_subscribe(wk_opcda_handle_t* handle,
                                    wk_opcda_data_callback_t callback,
                                    void* user_data);
wk_opcda_error_t wk_opcda_unsubscribe(wk_opcda_handle_t* handle);
wk_opcda_error_t wk_opcda_get_status(const wk_opcda_handle_t* handle,
                                     wk_opcda_status_t* status);
wk_opcda_error_t wk_opcda_cancel(wk_opcda_handle_t* handle);
void wk_opcda_disconnect(wk_opcda_handle_t* handle);
void wk_opcda_destroy(wk_opcda_handle_t* handle);
const char* wk_opcda_last_error(const wk_opcda_handle_t* handle);

#ifdef __cplusplus
}
#endif

#endif
