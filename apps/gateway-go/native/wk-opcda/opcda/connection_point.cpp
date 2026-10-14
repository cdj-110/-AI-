#include "opcda/connection_point.h"

#include "dcom/object_reference.h"
#include "rpc/rpc_pdu.h"

#include <cstddef>

namespace wk::opcda::opcda {
namespace {

void appendU32(std::vector<uint8_t>& data, uint32_t value) {
    data.push_back(static_cast<uint8_t>(value));
    data.push_back(static_cast<uint8_t>(value >> 8));
    data.push_back(static_cast<uint8_t>(value >> 16));
    data.push_back(static_cast<uint8_t>(value >> 24));
}

uint32_t readU32(const uint8_t* data) {
    return static_cast<uint32_t>(data[0]) |
        (static_cast<uint32_t>(data[1]) << 8) |
        (static_cast<uint32_t>(data[2]) << 16) |
        (static_cast<uint32_t>(data[3]) << 24);
}

bool failed(uint32_t value) { return (value & 0x80000000U) != 0; }

std::string hexHresult(uint32_t value) {
    constexpr char hex[] = "0123456789ABCDEF";
    std::string text = "0x";
    for (int shift = 28; shift >= 0; shift -= 4) text.push_back(hex[(value >> shift) & 15]);
    return text;
}

std::array<uint8_t, 16> iid(const char* text) {
    std::array<uint8_t, 16> value{};
    std::string ignored;
    rpc::parseUuid(text, value, ignored);
    return value;
}

}  // namespace

std::array<uint8_t, 16> connectionPointContainerIid() {
    return iid("b196b284-bab4-101a-b69c-00aa00341d07");
}

std::array<uint8_t, 16> connectionPointIid() {
    return iid("b196b286-bab4-101a-b69c-00aa00341d07");
}

std::array<uint8_t, 16> unknownIid() {
    return iid("00000000-0000-0000-c000-000000000046");
}

std::array<uint8_t, 16> remoteUnknownIid() {
    return iid("00000131-0000-0000-c000-000000000046");
}

std::array<uint8_t, 16> remoteUnknown2Iid() {
    return iid("00000143-0000-0000-c000-000000000046");
}

std::array<uint8_t, 16> opcDataCallbackIid() {
    return iid("39c13a70-011e-11d0-9675-0020afd8adb3");
}

std::array<uint8_t, 16> opcAsyncIo2Iid() {
    return iid("39c13a71-011e-11d0-9675-0020afd8adb3");
}

std::vector<uint8_t> encodeFindConnectionPointRequest(
    const std::array<uint8_t, 16>& sinkIid, uint16_t comMajor,
    uint16_t comMinor, const std::array<uint8_t, 16>& cid) {
    std::vector<uint8_t> result = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    result.insert(result.end(), sinkIid.begin(), sinkIid.end());
    return result;
}

bool decodeInterfacePointerResponse(
    const std::vector<uint8_t>& stub, const char* method,
    std::vector<uint8_t>& objectReference, uint32_t& hresult,
    std::string& error) {
    objectReference.clear();
    hresult = 0;
    if (stub.size() < 24 || (readU32(stub.data()) & ~1U) != 0 ||
        readU32(stub.data() + 4) != 0) {
        error = std::string(method) + " returned truncated or unsupported ORPCTHAT data";
        return false;
    }
    hresult = readU32(stub.data() + stub.size() - 4);
    if (failed(hresult)) {
        error = std::string(method) + " returned HRESULT " + hexHresult(hresult);
        return false;
    }
    if (readU32(stub.data() + 8) == 0) {
        error = std::string(method) + " returned a null interface pointer";
        return false;
    }
    const uint32_t count = readU32(stub.data() + 12);
    const uint32_t maximum = readU32(stub.data() + 16);
    const std::size_t hresultOffset = (20 + static_cast<std::size_t>(count) + 3) & ~std::size_t(3);
    if (count != maximum || count < 76 || count > 16 * 1024 * 1024 ||
        hresultOffset + 4 != stub.size()) {
        error = std::string(method) + " returned invalid MInterfacePointer bounds";
        return false;
    }
    objectReference.assign(stub.begin() + 20,
                           stub.begin() + static_cast<std::ptrdiff_t>(20 + count));
    return true;
}

std::vector<uint8_t> encodeAdviseRequest(
    const std::vector<uint8_t>& callbackObjectReference,
    uint16_t comMajor, uint16_t comMinor,
    const std::array<uint8_t, 16>& cid) {
    std::vector<uint8_t> result = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    appendU32(result, 0x00020000);  // unique IUnknown pointer
    appendU32(result, static_cast<uint32_t>(callbackObjectReference.size()));
    appendU32(result, static_cast<uint32_t>(callbackObjectReference.size()));
    result.insert(result.end(), callbackObjectReference.begin(), callbackObjectReference.end());
    while (result.size() % 4 != 0) result.push_back(0);
    return result;
}

bool decodeAdviseResponse(const std::vector<uint8_t>& stub,
                          uint32_t& cookie, uint32_t& hresult,
                          std::string& error) {
    cookie = 0;
    if (stub.size() != 16 || (readU32(stub.data()) & ~1U) != 0 ||
        readU32(stub.data() + 4) != 0) {
        error = "IConnectionPoint::Advise returned malformed ORPCTHAT data";
        return false;
    }
    cookie = readU32(stub.data() + 8);
    hresult = readU32(stub.data() + 12);
    if (failed(hresult) || cookie == 0) {
        error = "IConnectionPoint::Advise returned HRESULT " + hexHresult(hresult) +
            (cookie == 0 ? " and a zero cookie" : "");
        return false;
    }
    return true;
}

std::vector<uint8_t> encodeUnadviseRequest(
    uint32_t cookie, uint16_t comMajor, uint16_t comMinor,
    const std::array<uint8_t, 16>& cid) {
    std::vector<uint8_t> result = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    appendU32(result, cookie);
    return result;
}

bool decodeHresultResponse(const std::vector<uint8_t>& stub,
                           const char* method, uint32_t& hresult,
                           std::string& error) {
    if (stub.size() != 12 || (readU32(stub.data()) & ~1U) != 0 ||
        readU32(stub.data() + 4) != 0) {
        error = std::string(method) + " returned malformed ORPCTHAT data";
        return false;
    }
    hresult = readU32(stub.data() + 8);
    if (failed(hresult)) {
        error = std::string(method) + " returned HRESULT " + hexHresult(hresult);
        return false;
    }
    return true;
}

}  // namespace wk::opcda::opcda
