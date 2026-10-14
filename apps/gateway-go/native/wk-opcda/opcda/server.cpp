#include "opcda/server.h"

#include "dcom/object_reference.h"
#include "rpc/rpc_pdu.h"

#include <algorithm>
#include <cstddef>
#include <cstring>
#include <limits>

namespace wk::opcda::opcda {
namespace {

constexpr uint64_t kWindowsToUnixEpoch100Nanoseconds = 116444736000000000ULL;
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

uint64_t readU64(const uint8_t* data) {
    return static_cast<uint64_t>(readU32(data)) |
           (static_cast<uint64_t>(readU32(data + 4)) << 32);
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
        error = "ORPC response is truncated before ORPCTHAT and HRESULT";
        return false;
    }
    if ((readU32(stub.data()) & ~1U) != 0) {
        error = "ORPCTHAT contains unsupported flags";
        return false;
    }
    if (readU32(stub.data() + 4) != 0) {
        error = "ORPCTHAT extensions are not supported";
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

bool decodeWideString(const uint8_t* data, std::size_t size,
                      std::string& output, std::string& error) {
    output.clear();
    if (size < 12) {
        error = "NDR wide string is truncated before its varying-array header";
        return false;
    }
    const uint32_t maximum = readU32(data);
    const uint32_t offset = readU32(data + 4);
    const uint32_t actual = readU32(data + 8);
    if (maximum > kMaximumStringUnits || offset > maximum || actual > maximum - offset ||
        12 + static_cast<std::size_t>(actual) * 2 > size) {
        error = "NDR wide string bounds are invalid";
        return false;
    }
    for (uint32_t index = 0; index < actual; ++index) {
        uint32_t codePoint = readU16(data + 12 + static_cast<std::size_t>(index) * 2);
        if (codePoint == 0) break;
        if (codePoint >= 0xd800 && codePoint <= 0xdbff) {
            if (++index >= actual) {
                error = "NDR wide string ends inside a surrogate pair";
                return false;
            }
            const uint16_t low = readU16(data + 12 + static_cast<std::size_t>(index) * 2);
            if (low < 0xdc00 || low > 0xdfff) {
                error = "NDR wide string contains an invalid surrogate pair";
                return false;
            }
            codePoint = 0x10000 + ((codePoint - 0xd800) << 10) + (low - 0xdc00);
        } else if (codePoint >= 0xdc00 && codePoint <= 0xdfff) {
            error = "NDR wide string contains an unpaired low surrogate";
            return false;
        }
        if (!appendUtf8(codePoint, output)) {
            error = "NDR wide string contains an invalid code point";
            return false;
        }
    }
    return true;
}

int64_t filetimeToUnixMilliseconds(uint64_t value) {
    if (value < kWindowsToUnixEpoch100Nanoseconds) return 0;
    const uint64_t milliseconds =
        (value - kWindowsToUnixEpoch100Nanoseconds) / 10000ULL;
    return milliseconds > static_cast<uint64_t>(std::numeric_limits<int64_t>::max())
        ? std::numeric_limits<int64_t>::max()
        : static_cast<int64_t>(milliseconds);
}

}  // namespace

std::array<uint8_t, 16> opcItemManagementIid() {
    std::array<uint8_t, 16> value{};
    std::string ignored;
    rpc::parseUuid("39c13a54-011e-11d0-9675-0020afd8adb3", value, ignored);
    return value;
}

std::array<uint8_t, 16> opcSyncIoIid() {
    std::array<uint8_t, 16> value{};
    std::string ignored;
    rpc::parseUuid("39c13a52-011e-11d0-9675-0020afd8adb3", value, ignored);
    return value;
}

std::vector<uint8_t> encodeGetStatusRequest(
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid) {
    return dcom::encodeOrpcThis(comMajor, comMinor, cid);
}

bool decodeGetStatusResponse(
    const std::vector<uint8_t>& stub, ServerStatus& result, std::string& error) {
    result = ServerStatus{};
    if (!validateOrpcThat(stub, error)) return false;
    result.hresult = readU32(stub.data() + stub.size() - 4);
    if (failed(result.hresult)) {
        error = "IOPCServer::GetStatus returned HRESULT " + hexHresult(result.hresult);
        return false;
    }
    if (stub.size() < 64 || readU32(stub.data() + 8) == 0) {
        error = "IOPCServer::GetStatus returned a null or truncated OPCSERVERSTATUS";
        return false;
    }
    const std::size_t statusOffset = 12;
    result.startFiletime = readU64(stub.data() + statusOffset);
    result.currentFiletime = readU64(stub.data() + statusOffset + 8);
    result.lastUpdateFiletime = readU64(stub.data() + statusOffset + 16);
    result.currentUnixMilliseconds = filetimeToUnixMilliseconds(result.currentFiletime);
    result.serverState = readU32(stub.data() + statusOffset + 24);
    result.groupCount = readU32(stub.data() + statusOffset + 28);
    result.bandwidth = readU32(stub.data() + statusOffset + 32);
    result.majorVersion = readU16(stub.data() + statusOffset + 36);
    result.minorVersion = readU16(stub.data() + statusOffset + 38);
    result.buildNumber = readU16(stub.data() + statusOffset + 40);
    const uint32_t vendorPointer = readU32(stub.data() + statusOffset + 44);
    const std::size_t vendorOffset = statusOffset + 48;
    const std::size_t bodyEnd = stub.size() - 4;
    if (vendorPointer == 0) {
        if (vendorOffset != bodyEnd) {
            error = "IOPCServer::GetStatus contains data after a null vendor string";
            return false;
        }
    } else if (!decodeWideString(stub.data() + vendorOffset,
                                 bodyEnd - vendorOffset,
                                 result.vendorInfo, error)) {
        error = "IOPCServer::GetStatus vendor string invalid: " + error;
        return false;
    }
    return true;
}

bool encodeAddGroupRequest(
    const std::string& name, bool active, uint32_t requestedUpdateRateMs,
    uint32_t clientGroupHandle, float percentDeadband, uint32_t localeId,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid,
    std::vector<uint8_t>& stub, std::string& error) {
    std::vector<uint16_t> wideName;
    if (!utf8ToUtf16(name, wideName) || wideName.size() >= kMaximumStringUnits) {
        error = "OPC group name is not valid UTF-8 or is too long";
        return false;
    }
    wideName.push_back(0);
    stub = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    // LPCWSTR and FLOAT* are [ref] parameters in the OPC DA IDL. Their
    // referents are encoded directly; only the [unique] pTimeBias carries a
    // pointer referent ID on the wire.
    appendU32(stub, static_cast<uint32_t>(wideName.size()));
    appendU32(stub, 0);
    appendU32(stub, static_cast<uint32_t>(wideName.size()));
    for (uint16_t character : wideName) appendU16(stub, character);
    align4(stub);
    appendU32(stub, active ? 1 : 0);
    appendU32(stub, requestedUpdateRateMs);
    appendU32(stub, clientGroupHandle);
    appendU32(stub, 0);  // pTimeBias
    uint32_t deadbandBits = 0;
    static_assert(sizeof(deadbandBits) == sizeof(percentDeadband));
    std::memcpy(&deadbandBits, &percentDeadband, sizeof(deadbandBits));
    appendU32(stub, deadbandBits);
    appendU32(stub, localeId);
    const std::array<uint8_t, 16> iid = opcItemManagementIid();
    stub.insert(stub.end(), iid.begin(), iid.end());
    return true;
}

bool decodeAddGroupResponse(
    const std::vector<uint8_t>& stub, AddGroupResult& result, std::string& error) {
    result = AddGroupResult{};
    if (!validateOrpcThat(stub, error)) return false;
    result.hresult = readU32(stub.data() + stub.size() - 4);
    if (failed(result.hresult)) {
        error = "IOPCServer::AddGroup returned HRESULT " + hexHresult(result.hresult);
        return false;
    }
    if (stub.size() < 32) {
        error = "IOPCServer::AddGroup response is truncated";
        return false;
    }
    result.serverGroupHandle = readU32(stub.data() + 8);
    result.revisedUpdateRateMs = readU32(stub.data() + 12);
    if (readU32(stub.data() + 16) == 0) {
        error = "IOPCServer::AddGroup returned a null IOPCItemMgt pointer";
        return false;
    }
    const uint32_t count = readU32(stub.data() + 20);
    const uint32_t maximum = readU32(stub.data() + 24);
    if (count != maximum || count < 76 || count > 16 * 1024 * 1024) {
        error = "IOPCServer::AddGroup returned invalid IOPCItemMgt MInterfacePointer bounds";
        return false;
    }
    const std::size_t hresultOffset =
        (28 + static_cast<std::size_t>(count) + 3) & ~std::size_t(3);
    if (hresultOffset + 4 != stub.size()) {
        error = "IOPCServer::AddGroup response length does not match its MInterfacePointer";
        return false;
    }
    result.itemManagementObjectReference.assign(
        stub.begin() + 28,
        stub.begin() + static_cast<std::ptrdiff_t>(28 + count));
    return true;
}

bool encodeGetGroupByNameRequest(
    const std::string& name, const std::array<uint8_t, 16>& requestedIid,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid,
    std::vector<uint8_t>& stub, std::string& error) {
    std::vector<uint16_t> wideName;
    if (name.empty() || !utf8ToUtf16(name, wideName) ||
        wideName.size() >= kMaximumStringUnits) {
        error = "OPC group name is empty, invalid UTF-8, or too long";
        return false;
    }
    wideName.push_back(0);
    stub = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    appendU32(stub, static_cast<uint32_t>(wideName.size()));
    appendU32(stub, 0);
    appendU32(stub, static_cast<uint32_t>(wideName.size()));
    for (uint16_t character : wideName) appendU16(stub, character);
    align4(stub);
    stub.insert(stub.end(), requestedIid.begin(), requestedIid.end());
    return true;
}

bool decodeGetGroupByNameResponse(
    const std::vector<uint8_t>& stub, InterfaceResult& result, std::string& error) {
    result = InterfaceResult{};
    if (!validateOrpcThat(stub, error)) return false;
    result.hresult = readU32(stub.data() + stub.size() - 4);
    if (failed(result.hresult)) {
        error = "IOPCServer::GetGroupByName returned HRESULT " +
            hexHresult(result.hresult);
        return false;
    }
    if (stub.size() < 24 || readU32(stub.data() + 8) == 0) {
        error = "IOPCServer::GetGroupByName returned a null or truncated interface pointer";
        return false;
    }
    const uint32_t count = readU32(stub.data() + 12);
    const uint32_t maximum = readU32(stub.data() + 16);
    if (count != maximum || count < 76 || count > 16 * 1024 * 1024) {
        error = "IOPCServer::GetGroupByName returned invalid MInterfacePointer bounds";
        return false;
    }
    const std::size_t hresultOffset =
        (20 + static_cast<std::size_t>(count) + 3) & ~std::size_t(3);
    if (hresultOffset + 4 != stub.size()) {
        error = "IOPCServer::GetGroupByName response length does not match its MInterfacePointer";
        return false;
    }
    result.objectReference.assign(
        stub.begin() + 20,
        stub.begin() + static_cast<std::ptrdiff_t>(20 + count));
    return true;
}

}  // namespace wk::opcda::opcda
