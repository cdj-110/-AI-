#include "api/wk_opcda.h"

#include <inttypes.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static const char* environment(const char* name, const char* fallback) {
    const char* value = getenv(name);
    return value == NULL ? fallback : value;
}

static const char* native_type_name(wk_opcda_value_type_t type) {
    switch (type) {
        case WK_OPCDA_VALUE_EMPTY: return "empty";
        case WK_OPCDA_VALUE_BOOL: return "bool";
        case WK_OPCDA_VALUE_INT64: return "signed_integer";
        case WK_OPCDA_VALUE_UINT64: return "unsigned_integer";
        case WK_OPCDA_VALUE_FLOAT64: return "floating_point";
        case WK_OPCDA_VALUE_STRING: return "string";
        case WK_OPCDA_VALUE_DATETIME: return "datetime";
        case WK_OPCDA_VALUE_FLOAT64_ARRAY: return "float64_array";
        case WK_OPCDA_VALUE_STRING_ARRAY: return "string_array";
        default: return "unknown";
    }
}

static void write_bytes(const char* value, size_t size) {
    while (size != 0) {
        const ssize_t written = write(STDOUT_FILENO, value, size);
        if (written <= 0) return;
        value += written;
        size -= (size_t)written;
    }
}

static void write_text(const char* value) {
    write_bytes(value, strlen(value));
}

static void write_uint64(uint64_t value) {
    char digits[20];
    size_t size = 0;
    do {
        digits[size++] = (char)('0' + value % 10);
        value /= 10;
    } while (value != 0);
    while (size != 0) write_bytes(&digits[--size], 1);
}

static void write_int64(int64_t value) {
    if (value < 0) {
        write_text("-");
        write_uint64((uint64_t)(-(value + 1)) + 1);
    } else {
        write_uint64((uint64_t)value);
    }
}

static void write_double(double value) {
    if (!isfinite(value)) {
        write_text(isnan(value) ? "nan" : value < 0 ? "-inf" : "inf");
        return;
    }
    if (value < 0) {
        write_text("-");
        value = -value;
    }
    const uint64_t whole = (uint64_t)value;
    uint64_t fraction = (uint64_t)llround((value - (double)whole) * 1000000000.0);
    write_uint64(whole);
    if (fraction == 0) return;
    if (fraction == 1000000000ULL) {
        write_text(".999999999");
        return;
    }
    char digits[9];
    for (int index = 8; index >= 0; --index) {
        digits[index] = (char)('0' + fraction % 10);
        fraction /= 10;
    }
    size_t size = 9;
    while (size > 1 && digits[size - 1] == '0') --size;
    write_text(".");
    write_bytes(digits, size);
}

static void print_value(const wk_opcda_value_t* value) {
    switch (value->type) {
        case WK_OPCDA_VALUE_BOOL:
        case WK_OPCDA_VALUE_INT64:
        case WK_OPCDA_VALUE_DATETIME:
            write_int64(value->int64_value);
            break;
        case WK_OPCDA_VALUE_UINT64:
            write_uint64(value->uint64_value);
            break;
        case WK_OPCDA_VALUE_FLOAT64:
            write_double(value->float64_value);
            break;
        case WK_OPCDA_VALUE_STRING:
            write_text("\"");
            write_bytes(value->string_value, value->string_length);
            write_text("\"");
            break;
        case WK_OPCDA_VALUE_FLOAT64_ARRAY:
            write_text("[");
            for (size_t index = 0; index < value->array_length; ++index) {
                if (index != 0) write_text(",");
                write_double(value->float64_array_values[index]);
            }
            write_text("]");
            break;
        case WK_OPCDA_VALUE_STRING_ARRAY:
            write_text("[");
            for (size_t index = 0; index < value->array_length; ++index) {
                if (index != 0) write_text(",");
                write_text("\"");
                write_bytes(value->string_array_values[index],
                            value->string_array_lengths[index]);
                write_text("\"");
            }
            write_text("]");
            break;
        default:
            write_text("null");
            break;
    }
}

int main(void) {
    const char* item_ids[] = {
        "Random.ArrayOfReal8", "Random.ArrayOfString", "Random.Real8",
        "Random.Real4", "Random.Int1", "Random.Int8", "Random.String",
    };
    enum { item_count = sizeof(item_ids) / sizeof(item_ids[0]) };
    wk_opcda_config_t config = {0};
    config.abi_version = WK_OPCDA_ABI_VERSION;
    config.host = environment("WK_OPCDA_HOST", "");
    config.prog_id = environment("WK_OPCDA_PROG_ID", "");
    config.clsid = environment("WK_OPCDA_CLSID", "");
    config.domain = environment("WK_OPCDA_DOMAIN", "");
    config.username = environment("WK_OPCDA_USERNAME", "");
    config.password = environment("WK_OPCDA_PASSWORD", "");
    config.workstation = environment("WK_OPCDA_WORKSTATION", "EG100");
    config.security_mode = environment("WK_OPCDA_SECURITY_MODE", "packet_privacy");
    config.authentication = "ntlm";
    config.authentication_level = "packet_privacy";
    config.group_name = "WK_OPCDA_T02_VARIANT";
    config.locale_id = 0x0409;
    config.requested_update_rate_ms = 1000;
    config.connect_timeout_ms = 10000;
    config.request_timeout_ms = 10000;

    wk_opcda_handle_t* handle = NULL;
    wk_opcda_error_t code = wk_opcda_create(&config, &handle);
    if (code != WK_OPCDA_OK || handle == NULL) {
        write_text("create_failed code=");
        write_int64(code);
        write_text(" library_abi=");
        write_uint64(wk_opcda_abi_version());
        write_text(" expected_abi=");
        write_uint64(WK_OPCDA_ABI_VERSION);
        write_text("\n");
        return 1;
    }
    code = wk_opcda_connect(handle);
    if (code != WK_OPCDA_OK) {
        write_text("connect_failed: ");
        write_text(wk_opcda_last_error(handle));
        write_text("\n");
        wk_opcda_destroy(handle);
        return 2;
    }

    wk_opcda_item_definition_t definitions[item_count];
    wk_opcda_item_result_t item_results[item_count];
    uint32_t handles[item_count];
    wk_opcda_value_t values[item_count];
    memset(definitions, 0, sizeof(definitions));
    memset(item_results, 0, sizeof(item_results));
    memset(values, 0, sizeof(values));
    for (size_t index = 0; index < item_count; ++index) {
        definitions[index].client_handle = (uint32_t)(index + 1);
        definitions[index].item_id = item_ids[index];
        definitions[index].access_path = "";
        definitions[index].requested_data_type = "auto";
        definitions[index].active = 1;
        handles[index] = (uint32_t)(index + 1);
    }
    code = wk_opcda_add_items(
        handle, definitions, item_count, item_results);
    if (code != WK_OPCDA_OK) {
        write_text("AddItems_failed: ");
        write_text(wk_opcda_last_error(handle));
        write_text("\n");
        wk_opcda_destroy(handle);
        return 3;
    }
    code = wk_opcda_read(
        handle, WK_OPCDA_READ_DEVICE, handles, item_count, values);
    if (code != WK_OPCDA_OK) {
        write_text("Read_failed: ");
        write_text(wk_opcda_last_error(handle));
        write_text("\n");
        wk_opcda_destroy(handle);
        return 4;
    }
    for (size_t index = 0; index < item_count; ++index) {
        write_text("item=");
        write_text(item_ids[index]);
        write_text(" canonical_vartype=");
        write_uint64(item_results[index].canonical_data_type);
        write_text(" native_type=");
        write_text(native_type_name(values[index].type));
        write_text(" value=");
        print_value(&values[index]);
        write_text(" quality=");
        write_uint64(values[index].quality);
        write_text(" source_ms=");
        write_int64(values[index].source_timestamp_unix_ms);
        write_text(" item_hresult=");
        write_int64(item_results[index].hresult);
        write_text(" read_hresult=");
        write_int64(values[index].hresult);
        write_text("\n");
    }
    wk_opcda_destroy(handle);
    return 0;
}
