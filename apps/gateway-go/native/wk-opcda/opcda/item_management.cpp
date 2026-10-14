#include "opcda/item_management.h"

#include "dcom/object_reference.h"

#include <algorithm>
#include <cctype>
#include <cstddef>

namespace wk::opcda::opcda {
namespace {

constexpr std::size_t kMaximumItems = 10000;
constexpr std::size_t kMaximumStringUnits = 1024 * 1024;
constexpr std::size_t kMaximumBlobBytes = 16 * 1024 * 1024;

uint16_t readU16(const uint8_t* data) {
    return static_cast<uint16_t>(data[0]) |
           static_cast<uint16_t>(static_cast<uint16_t>(data[1]) << 8);
}

uint32_t readU32(const uint8_t* data) {
    return static_cast<uint32_t>(data[0]) |
           (static_cast<uint32_t>(data[1]) << 8) |
           (static_cast<uint32_t>(data[2]) << 16) |
           (static_cast<uint32_t>(data[3]) << 24);
}

void appendU16(std::vector<uint8_t>& data, uint16_t value) {
    data.push_back(static_cast<uint8_t>(value));
    data.push_back(static_cast<uint8_t>(value >> 8));
}

void appendU32(std::vector<uint8_t>& data, uint32_t value) {
    data.push_back(static_cast<uint8_t>(value));
    data.push_back(static_cast<uint8_t>(value >> 8));
    data.push_back(static_cast<uint8_t>(value >> 16));
    data.push_back(static_cast<uint8_t>(value >> 24));
}

void align4(std::vector<uint8_t>& data) {
    while (data.size() % 4 != 0) data.push_back(0);
}

std::size_t align4(std::size_t value) {
    return (value + 3) & ~std::size_t(3);
}

bool failed(uint32_t hresult) {
    return (hresult & 0x80000000U) != 0;
}

std::string hexHresult(uint32_t value) {
    constexpr char hex[] = "0123456789ABCDEF";
    std::string text = "0x";
    for (int shift = 28; shift >= 0; shift -= 4) {
        text.push_back(hex[(value >> shift) & 0x0f]);
    }
    return text;
}

bool validateOrpcThat(const std::vector<uint8_t>& stub, std::string& error) {
    if (stub.size() < 12) {
        error = "AddItems response is truncated before ORPCTHAT and HRESULT";
        return false;
    }
    if ((readU32(stub.data()) & ~1U) != 0 || readU32(stub.data() + 4) != 0) {
        error = "AddItems response contains unsupported ORPCTHAT data";
        return false;
    }
    return true;
}

bool appendUtf8CodePoint(uint32_t codePoint, std::vector<uint16_t>& output) {
    if (codePoint > 0x10ffff || (codePoint >= 0xd800 && codePoint <= 0xdfff)) return false;
    if (codePoint <= 0xffff) {
        output.push_back(static_cast<uint16_t>(codePoint));
    } else {
        codePoint -= 0x10000;
        output.push_back(static_cast<uint16_t>(0xd800 + (codePoint >> 10)));
        output.push_back(static_cast<uint16_t>(0xdc00 + (codePoint & 0x3ff)));
    }
    return true;
}

bool utf8ToUtf16(const std::string& input, std::vector<uint16_t>& output) {
    output.clear();
    for (std::size_t index = 0; index < input.size();) {
        const uint8_t first = static_cast<uint8_t>(input[index]);
        uint32_t codePoint = 0;
        std::size_t count = 0;
        if (first < 0x80) {
            codePoint = first;
            count = 1;
        } else if ((first & 0xe0) == 0xc0) {
            codePoint = first & 0x1f;
            count = 2;
            if (codePoint == 0) return false;
        } else if ((first & 0xf0) == 0xe0) {
            codePoint = first & 0x0f;
            count = 3;
        } else if ((first & 0xf8) == 0xf0) {
            codePoint = first & 0x07;
            count = 4;
        } else {
            return false;
        }
        if (index + count > input.size()) return false;
        for (std::size_t offset = 1; offset < count; ++offset) {
            const uint8_t next = static_cast<uint8_t>(input[index + offset]);
            if ((next & 0xc0) != 0x80) return false;
            codePoint = (codePoint << 6) | (next & 0x3f);
        }
        if ((count == 2 && codePoint < 0x80) ||
            (count == 3 && codePoint < 0x800) ||
            (count == 4 && codePoint < 0x10000) ||
            !appendUtf8CodePoint(codePoint, output)) {
            return false;
        }
        index += count;
    }
    return true;
}

void appendWideStringReferent(std::vector<uint8_t>& output,
                              const std::vector<uint16_t>& value) {
    align4(output);
    appendU32(output, static_cast<uint32_t>(value.size()));
    appendU32(output, 0);
    appendU32(output, static_cast<uint32_t>(value.size()));
    for (uint16_t character : value) appendU16(output, character);
}

std::string normalizedTypeName(const std::string& name) {
    std::string result;
    result.reserve(name.size());
    for (char character : name) {
        if (character == '_' || character == '-' || std::isspace(static_cast<unsigned char>(character))) {
            continue;
        }
        result.push_back(static_cast<char>(std::tolower(static_cast<unsigned char>(character))));
    }
    return result;
}

}  // namespace

bool vartypeFromName(const std::string& name, uint16_t& vartype) {
    const std::string normalized = normalizedTypeName(name);
    if (normalized.empty() || normalized == "auto" || normalized == "empty" || normalized == "vtempty") {
        vartype = 0;
    } else if (normalized == "int16" || normalized == "short" || normalized == "i2" || normalized == "vti2") {
        vartype = 2;
    } else if (normalized == "int32" || normalized == "long" || normalized == "i4" || normalized == "vti4") {
        vartype = 3;
    } else if (normalized == "float32" || normalized == "single" || normalized == "r4" || normalized == "vtr4") {
        vartype = 4;
    } else if (normalized == "float64" || normalized == "double" || normalized == "r8" || normalized == "vtr8") {
        vartype = 5;
    } else if (normalized == "datetime" || normalized == "date" || normalized == "vtdate") {
        vartype = 7;
    } else if (normalized == "string" || normalized == "bstr" || normalized == "vtbstr") {
        vartype = 8;
    } else if (normalized == "bool" || normalized == "boolean" || normalized == "vtbool") {
        vartype = 11;
    } else if (normalized == "int8" || normalized == "i1" || normalized == "vti1") {
        vartype = 16;
    } else if (normalized == "uint8" || normalized == "ui1" || normalized == "vtui1" || normalized == "byte") {
        vartype = 17;
    } else if (normalized == "uint16" || normalized == "ui2" || normalized == "vtui2") {
        vartype = 18;
    } else if (normalized == "uint32" || normalized == "ui4" || normalized == "vtui4") {
        vartype = 19;
    } else if (normalized == "int64" || normalized == "i8" || normalized == "vti8") {
        vartype = 20;
    } else if (normalized == "uint64" || normalized == "ui8" || normalized == "vtui8") {
        vartype = 21;
    } else {
        return false;
    }
    return true;
}

bool encodeAddItemsRequest(
    const std::vector<ItemDefinition>& items,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid,
    std::vector<uint8_t>& stub, std::string& error) {
    if (items.empty() || items.size() > kMaximumItems) {
        error = "AddItems requires between 1 and 10000 item definitions";
        return false;
    }
    std::vector<std::vector<uint16_t>> accessPaths(items.size());
    std::vector<std::vector<uint16_t>> itemIds(items.size());
    for (std::size_t index = 0; index < items.size(); ++index) {
        if (items[index].itemId.empty()) {
            error = "AddItems item ID must not be empty";
            return false;
        }
        if (!utf8ToUtf16(items[index].accessPath, accessPaths[index]) ||
            !utf8ToUtf16(items[index].itemId, itemIds[index])) {
            error = "AddItems contains an invalid UTF-8 access path or item ID";
            return false;
        }
        accessPaths[index].push_back(0);
        itemIds[index].push_back(0);
        if (accessPaths[index].size() > kMaximumStringUnits ||
            itemIds[index].size() > kMaximumStringUnits) {
            error = "AddItems access path or item ID is too long";
            return false;
        }
    }

    stub = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    appendU32(stub, static_cast<uint32_t>(items.size()));
    appendU32(stub, static_cast<uint32_t>(items.size()));
    uint32_t referent = 0x00020000;
    for (std::size_t index = 0; index < items.size(); ++index) {
        appendU32(stub, referent);
        referent += 4;
        appendU32(stub, referent);
        referent += 4;
        appendU32(stub, items[index].active ? 1 : 0);
        appendU32(stub, items[index].clientHandle);
        appendU32(stub, 0);  // dwBlobSize
        appendU32(stub, 0);  // pBlob
        appendU16(stub, items[index].requestedDataType);
        appendU16(stub, 0);  // wReserved
    }
    for (std::size_t index = 0; index < items.size(); ++index) {
        appendWideStringReferent(stub, accessPaths[index]);
        appendWideStringReferent(stub, itemIds[index]);
    }
    return true;
}

bool decodeAddItemsResponse(
    const std::vector<uint8_t>& stub,
    const std::vector<ItemDefinition>& requestedItems,
    std::vector<ItemResult>& results, uint32_t& hresult, std::string& error) {
    results.clear();
    hresult = 0;
    if (requestedItems.empty() || requestedItems.size() > kMaximumItems) {
        error = "AddItems response has an invalid expected item count";
        return false;
    }
    if (!validateOrpcThat(stub, error)) return false;

    std::size_t offset = 8;
    if (offset + 8 > stub.size() || readU32(stub.data() + offset) == 0) {
        error = "AddItems returned a null or truncated OPCITEMRESULT array";
        return false;
    }
    offset += 4;
    const uint32_t resultCount = readU32(stub.data() + offset);
    offset += 4;
    if (resultCount != requestedItems.size() || resultCount > kMaximumItems ||
        offset + static_cast<std::size_t>(resultCount) * 20 > stub.size()) {
        error = "AddItems OPCITEMRESULT array bounds do not match the request";
        return false;
    }

    struct BlobDescriptor {
        uint32_t size;
        uint32_t pointer;
    };
    std::vector<BlobDescriptor> blobs;
    blobs.reserve(resultCount);
    results.reserve(resultCount);
    for (uint32_t index = 0; index < resultCount; ++index) {
        ItemResult result{};
        result.clientHandle = requestedItems[index].clientHandle;
        result.serverHandle = readU32(stub.data() + offset);
        result.canonicalDataType = readU16(stub.data() + offset + 4);
        result.accessRights = readU32(stub.data() + offset + 8);
        blobs.push_back({readU32(stub.data() + offset + 12),
                         readU32(stub.data() + offset + 16)});
        results.push_back(result);
        offset += 20;
    }
    for (const BlobDescriptor& blob : blobs) {
        if (blob.pointer == 0) {
            if (blob.size != 0) {
                error = "AddItems returned a null blob pointer with a non-zero size";
                return false;
            }
            continue;
        }
        if (offset + 4 > stub.size()) {
            error = "AddItems returned a truncated blob descriptor";
            return false;
        }
        const uint32_t count = readU32(stub.data() + offset);
        offset += 4;
        if (count != blob.size || count > kMaximumBlobBytes ||
            offset + count > stub.size()) {
            error = "AddItems returned invalid blob bounds";
            return false;
        }
        offset = align4(offset + count);
    }

    if (offset + 8 > stub.size() || readU32(stub.data() + offset) == 0) {
        error = "AddItems returned a null or truncated per-item HRESULT array";
        return false;
    }
    offset += 4;
    const uint32_t errorCount = readU32(stub.data() + offset);
    offset += 4;
    if (errorCount != requestedItems.size() || errorCount > kMaximumItems ||
        offset + static_cast<std::size_t>(errorCount) * 4 + 4 != stub.size()) {
        error = "AddItems per-item HRESULT array bounds do not match the request";
        return false;
    }
    for (uint32_t index = 0; index < errorCount; ++index) {
        results[index].hresult = readU32(stub.data() + offset);
        offset += 4;
    }
    hresult = readU32(stub.data() + offset);
    if (failed(hresult)) {
        error = "IOPCItemMgt::AddItems returned HRESULT " + hexHresult(hresult);
        return false;
    }
    return true;
}

}  // namespace wk::opcda::opcda
