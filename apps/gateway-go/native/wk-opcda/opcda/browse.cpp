#include "opcda/browse.h"

#include "dcom/object_reference.h"
#include "rpc/rpc_pdu.h"

#include <algorithm>
#include <cstddef>
#include <limits>

namespace wk::opcda::opcda {
namespace {

constexpr uint32_t kObjrefSignature = 0x574f454d;
constexpr uint32_t kObjrefStandard = 0x00000001;
constexpr std::size_t kMaximumStringUnits = 1024 * 1024;

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

bool failed(uint32_t value) { return (value & 0x80000000U) != 0; }

std::string hexHresult(uint32_t value) {
    constexpr char hex[] = "0123456789ABCDEF";
    std::string result = "0x";
    for (int shift = 28; shift >= 0; shift -= 4) {
        result.push_back(hex[(value >> shift) & 0x0f]);
    }
    return result;
}

bool validateOrpcThat(const std::vector<uint8_t>& stub, std::string& error) {
    if (stub.size() < 12) {
        error = "browse response is truncated before ORPCTHAT and HRESULT";
        return false;
    }
    if ((readU32(stub.data()) & ~1U) != 0 || readU32(stub.data() + 4) != 0) {
        error = "browse response contains unsupported ORPCTHAT data";
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

bool appendUtf8(uint32_t codePoint, std::string& output) {
    if (codePoint <= 0x7f) {
        output.push_back(static_cast<char>(codePoint));
    } else if (codePoint <= 0x7ff) {
        output.push_back(static_cast<char>(0xc0 | (codePoint >> 6)));
        output.push_back(static_cast<char>(0x80 | (codePoint & 0x3f)));
    } else if (codePoint <= 0xffff) {
        output.push_back(static_cast<char>(0xe0 | (codePoint >> 12)));
        output.push_back(static_cast<char>(0x80 | ((codePoint >> 6) & 0x3f)));
        output.push_back(static_cast<char>(0x80 | (codePoint & 0x3f)));
    } else if (codePoint <= 0x10ffff) {
        output.push_back(static_cast<char>(0xf0 | (codePoint >> 18)));
        output.push_back(static_cast<char>(0x80 | ((codePoint >> 12) & 0x3f)));
        output.push_back(static_cast<char>(0x80 | ((codePoint >> 6) & 0x3f)));
        output.push_back(static_cast<char>(0x80 | (codePoint & 0x3f)));
    } else {
        return false;
    }
    return true;
}

bool decodeWideStringAt(const std::vector<uint8_t>& stub, std::size_t offset,
                        std::string& output, std::size_t& end) {
    output.clear();
    if (offset + 12 > stub.size()) return false;
    const uint32_t maximum = readU32(stub.data() + offset);
    const uint32_t first = readU32(stub.data() + offset + 4);
    const uint32_t actual = readU32(stub.data() + offset + 8);
    if (maximum == 0 || maximum > kMaximumStringUnits || first > maximum ||
        actual > maximum - first || actual == 0 ||
        offset + 12 + static_cast<std::size_t>(actual) * 2 > stub.size()) {
        return false;
    }
    bool terminated = false;
    for (uint32_t index = 0; index < actual; ++index) {
        uint32_t codePoint = readU16(
            stub.data() + offset + 12 + static_cast<std::size_t>(index) * 2);
        if (codePoint == 0) {
            terminated = true;
            break;
        }
        if (codePoint >= 0xd800 && codePoint <= 0xdbff) {
            if (++index >= actual) return false;
            const uint16_t low = readU16(
                stub.data() + offset + 12 + static_cast<std::size_t>(index) * 2);
            if (low < 0xdc00 || low > 0xdfff) return false;
            codePoint = 0x10000 + ((codePoint - 0xd800) << 10) + (low - 0xdc00);
        } else if (codePoint >= 0xdc00 && codePoint <= 0xdfff) {
            return false;
        }
        if (!appendUtf8(codePoint, output)) return false;
    }
    if (!terminated || output.empty()) return false;
    end = offset + 12 + static_cast<std::size_t>(actual) * 2;
    end = (end + 3) & ~std::size_t(3);
    return true;
}

bool findWideStrings(const std::vector<uint8_t>& stub,
                     std::vector<std::string>& values) {
    values.clear();
    const std::size_t bodyEnd = stub.size() < 4 ? 0 : stub.size() - 4;
    for (std::size_t offset = 8; offset + 12 <= bodyEnd; offset += 4) {
        std::string value;
        std::size_t end = 0;
        if (!decodeWideStringAt(stub, offset, value, end) || end > bodyEnd) continue;
        values.push_back(std::move(value));
        offset = end >= 4 ? end - 4 : offset;
    }
    return !values.empty();
}

bool appendWideString(std::vector<uint8_t>& stub, const std::string& value,
                      std::string& error) {
    std::vector<uint16_t> wide;
    if (!utf8ToUtf16(value, wide) || wide.size() >= kMaximumStringUnits) {
        error = "browse string is invalid UTF-8 or too long";
        return false;
    }
    wide.push_back(0);
    appendU32(stub, static_cast<uint32_t>(wide.size()));
    appendU32(stub, 0);
    appendU32(stub, static_cast<uint32_t>(wide.size()));
    for (uint16_t character : wide) appendU16(stub, character);
    align4(stub);
    return true;
}

bool findObjectReference(const std::vector<uint8_t>& stub,
                         const std::array<uint8_t, 16>& iid,
                         std::vector<uint8_t>& objectReference) {
    objectReference.clear();
    if (stub.size() < 72) return false;
    for (std::size_t offset = 8; offset + 68 <= stub.size() - 4; ++offset) {
        if (readU32(stub.data() + offset) != kObjrefSignature ||
            readU32(stub.data() + offset + 4) != kObjrefStandard ||
            !std::equal(iid.begin(), iid.end(), stub.data() + offset + 8)) {
            continue;
        }
        const uint16_t entries = readU16(stub.data() + offset + 64);
        if (entries > 4096) return false;
        const std::size_t size = 68 + static_cast<std::size_t>(entries) * 2;
        if (offset + size > stub.size() - 4) return false;
        objectReference.assign(
            stub.begin() + static_cast<std::ptrdiff_t>(offset),
            stub.begin() + static_cast<std::ptrdiff_t>(offset + size));
        return true;
    }
    return false;
}

}  // namespace

std::array<uint8_t, 16> opcBrowseServerAddressSpaceIid() {
    std::array<uint8_t, 16> value{};
    std::string ignored;
    rpc::parseUuid("39c13a4f-011e-11d0-9675-0020afd8adb3", value, ignored);
    return value;
}

std::array<uint8_t, 16> enumStringIid() {
    std::array<uint8_t, 16> value{};
    std::string ignored;
    rpc::parseUuid("00000101-0000-0000-c000-000000000046", value, ignored);
    return value;
}

std::vector<uint8_t> encodeQueryOrganizationRequest(
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid) {
    return dcom::encodeOrpcThis(comMajor, comMinor, cid);
}

bool decodeQueryOrganizationResponse(
    const std::vector<uint8_t>& stub, BrowseOrganization& organization,
    uint32_t& hresult, std::string& error) {
    if (!validateOrpcThat(stub, error) || stub.size() != 16) return false;
    organization = static_cast<BrowseOrganization>(readU32(stub.data() + 8));
    hresult = readU32(stub.data() + 12);
    if (failed(hresult)) {
        error = "QueryOrganization returned HRESULT " + hexHresult(hresult);
        return false;
    }
    if (organization != BrowseOrganization::Hierarchical &&
        organization != BrowseOrganization::Flat) {
        error = "QueryOrganization returned an unknown namespace type";
        return false;
    }
    return true;
}

bool encodeChangeBrowsePositionRequest(
    BrowseDirection direction, const std::string& position,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid,
    std::vector<uint8_t>& stub, std::string& error) {
    stub = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    appendU32(stub, static_cast<uint32_t>(direction));
    return appendWideString(stub, position, error);
}

bool decodeSimpleBrowseResponse(
    const std::vector<uint8_t>& stub, uint32_t& hresult, std::string& error) {
    if (!validateOrpcThat(stub, error) || stub.size() != 12) return false;
    hresult = readU32(stub.data() + 8);
    if (failed(hresult)) {
        error = "browse method returned HRESULT " + hexHresult(hresult);
        return false;
    }
    return true;
}

bool encodeBrowseItemIdsRequest(
    BrowseType type, const std::string& filter, uint16_t dataTypeFilter,
    uint32_t accessRightsFilter, uint16_t comMajor, uint16_t comMinor,
    const std::array<uint8_t, 16>& cid, std::vector<uint8_t>& stub,
    std::string& error) {
    stub = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    appendU32(stub, static_cast<uint32_t>(type));
    if (!appendWideString(stub, filter, error)) return false;
    appendU16(stub, dataTypeFilter);
    appendU16(stub, 0);
    appendU32(stub, accessRightsFilter);
    return true;
}

bool decodeBrowseEnumeratorResponse(
    const std::vector<uint8_t>& stub, std::vector<uint8_t>& objectReference,
    uint32_t& hresult, std::string& error) {
    if (!validateOrpcThat(stub, error)) return false;
    hresult = readU32(stub.data() + stub.size() - 4);
    if (failed(hresult)) {
        error = "BrowseOPCItemIDs returned HRESULT " + hexHresult(hresult);
        return false;
    }
    if (!findObjectReference(stub, enumStringIid(), objectReference)) {
        error = "BrowseOPCItemIDs returned no IEnumString object reference";
        return false;
    }
    return true;
}

std::vector<uint8_t> encodeEnumStringNextRequest(
    uint32_t count, uint16_t comMajor, uint16_t comMinor,
    const std::array<uint8_t, 16>& cid) {
    std::vector<uint8_t> stub = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    appendU32(stub, count);
    return stub;
}

bool decodeEnumStringNextResponse(
    const std::vector<uint8_t>& stub, std::vector<std::string>& values,
    uint32_t& fetched, uint32_t& hresult, std::string& error) {
    values.clear();
    fetched = 0;
    if (!validateOrpcThat(stub, error)) return false;
    hresult = readU32(stub.data() + stub.size() - 4);
    if (failed(hresult)) {
        error = "IEnumString::Next returned HRESULT " + hexHresult(hresult);
        return false;
    }
    findWideStrings(stub, values);
    fetched = static_cast<uint32_t>(values.size());
    if (fetched == 0 && hresult == 0) {
        error = "IEnumString::Next returned S_OK without a string";
        return false;
    }
    return true;
}

bool encodeGetItemIdRequest(
    const std::string& name, uint16_t comMajor, uint16_t comMinor,
    const std::array<uint8_t, 16>& cid, std::vector<uint8_t>& stub,
    std::string& error) {
    stub = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    return appendWideString(stub, name, error);
}

bool decodeGetItemIdResponse(
    const std::vector<uint8_t>& stub, std::string& itemId,
    uint32_t& hresult, std::string& error) {
    itemId.clear();
    if (!validateOrpcThat(stub, error)) return false;
    hresult = readU32(stub.data() + stub.size() - 4);
    if (failed(hresult)) {
        error = "GetItemID returned HRESULT " + hexHresult(hresult);
        return false;
    }
    std::vector<std::string> values;
    if (!findWideStrings(stub, values)) {
        error = "GetItemID returned no item ID string";
        return false;
    }
    itemId = std::move(values.front());
    return true;
}

}  // namespace wk::opcda::opcda
