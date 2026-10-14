#include "dcom/object_reference.h"

#include <algorithm>
#include <cstddef>
#include <limits>

namespace wk::opcda::dcom {
namespace {

constexpr uint32_t kObjrefSignature = 0x574f454d;
constexpr uint32_t kObjrefStandard = 0x00000001;
constexpr uint16_t kNcacnIpTcp = 0x0007;
constexpr uint16_t kRpcAuthnWinnt = 0x000a;
constexpr std::size_t kMaximumDualStringEntries = 4096;

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

void appendU64(std::vector<uint8_t>& data, uint64_t value) {
    appendU32(data, static_cast<uint32_t>(value));
    appendU32(data, static_cast<uint32_t>(value >> 32));
}

bool appendUtf8(uint16_t value, std::string& output) {
    if (value >= 0xd800 && value <= 0xdfff) return false;
    if (value <= 0x7f) {
        output.push_back(static_cast<char>(value));
    } else if (value <= 0x7ff) {
        output.push_back(static_cast<char>(0xc0 | (value >> 6)));
        output.push_back(static_cast<char>(0x80 | (value & 0x3f)));
    } else {
        output.push_back(static_cast<char>(0xe0 | (value >> 12)));
        output.push_back(static_cast<char>(0x80 | ((value >> 6) & 0x3f)));
        output.push_back(static_cast<char>(0x80 | (value & 0x3f)));
    }
    return true;
}

bool decodeBindings(const uint8_t* array, uint16_t count, uint16_t securityOffset,
                    std::vector<StringBinding>& bindings, std::string& error) {
    bindings.clear();
    if (count < 4 || count > kMaximumDualStringEntries ||
        securityOffset < 2 || securityOffset > count - 2) {
        error = "DUALSTRINGARRAY count or security offset is invalid";
        return false;
    }
    std::size_t index = 0;
    bool terminated = false;
    while (index < securityOffset) {
        const uint16_t towerId = readU16(array + index * 2);
        ++index;
        if (towerId == 0) {
            terminated = true;
            break;
        }
        StringBinding binding{};
        binding.towerId = towerId;
        bool stringTerminated = false;
        while (index < securityOffset) {
            const uint16_t character = readU16(array + index * 2);
            ++index;
            if (character == 0) {
                stringTerminated = true;
                break;
            }
            if (!appendUtf8(character, binding.networkAddress)) {
                error = "STRINGBINDING contains an unsupported UTF-16 surrogate";
                return false;
            }
        }
        if (!stringTerminated || binding.networkAddress.empty()) {
            error = "STRINGBINDING network address is missing its terminator";
            return false;
        }
        bindings.push_back(std::move(binding));
    }
    if (!terminated || index != securityOffset) {
        error = "DUALSTRINGARRAY string-binding terminator does not match wSecurityOffset";
        return false;
    }
    // Security bindings are not selected yet, but validate their outer bounds
    // and the mandatory final array terminator before trusting the structure.
    if (readU16(array + (count - 1) * 2) != 0) {
        error = "DUALSTRINGARRAY security-binding array is not terminated";
        return false;
    }
    return true;
}

bool parsePort(const std::string& text, uint16_t& port) {
    if (text.empty()) return false;
    uint32_t value = 0;
    for (char character : text) {
        if (character < '0' || character > '9') return false;
        value = value * 10 + static_cast<uint32_t>(character - '0');
        if (value > std::numeric_limits<uint16_t>::max()) return false;
    }
    if (value == 0) return false;
    port = static_cast<uint16_t>(value);
    return true;
}

}  // namespace

bool decodeStandardObjectReference(
    const std::vector<uint8_t>& objref, const std::array<uint8_t, 16>& expectedIid,
    StandardObjectReference& result, std::string& error) {
    result = StandardObjectReference{};
    if (objref.size() < 76) {
        error = "OBJREF_STANDARD is shorter than its fixed fields and minimum DUALSTRINGARRAY";
        return false;
    }
    if (readU32(objref.data()) != kObjrefSignature ||
        readU32(objref.data() + 4) != kObjrefStandard) {
        error = "object reference is not OBJREF_STANDARD";
        return false;
    }
    std::copy_n(objref.data() + 8, result.iid.size(), result.iid.begin());
    if (result.iid != expectedIid) {
        error = "OBJREF_STANDARD IID does not match IID_IOPCServer";
        return false;
    }
    result.standardFlags = readU32(objref.data() + 24);
    result.publicReferences = readU32(objref.data() + 28);
    result.oxid = readU64(objref.data() + 32);
    result.oid = readU64(objref.data() + 40);
    std::copy_n(objref.data() + 48, result.ipid.size(), result.ipid.begin());
    const uint16_t count = readU16(objref.data() + 64);
    const uint16_t securityOffset = readU16(objref.data() + 66);
    const std::size_t expectedSize = 68 + static_cast<std::size_t>(count) * 2;
    if (count > kMaximumDualStringEntries || objref.size() != expectedSize) {
        error = "OBJREF_STANDARD DUALSTRINGARRAY length does not match the object reference";
        return false;
    }
    return decodeBindings(objref.data() + 68, count, securityOffset,
                          result.resolverBindings, error);
}

std::vector<uint8_t> encodeStandardObjectReference(
    const std::array<uint8_t, 16>& iid, uint64_t oxid, uint64_t oid,
    const std::array<uint8_t, 16>& ipid, const std::string& host,
    uint16_t resolverPort, uint32_t publicReferences,
    bool advertiseWinnt) {
    std::vector<uint16_t> bindings;
    bindings.push_back(kNcacnIpTcp);
    const std::string address = resolverPort == 0
        ? host
        : host + "[" + std::to_string(resolverPort) + "]";
    for (unsigned char character : address) bindings.push_back(character);
    bindings.push_back(0);
    bindings.push_back(0);  // string-binding terminator
    const uint16_t securityOffset = static_cast<uint16_t>(bindings.size());
    if (advertiseWinnt) {
        bindings.push_back(kRpcAuthnWinnt);
        bindings.push_back(0xffff);
        bindings.push_back(0);  // empty principal name
    } else {
        bindings.push_back(0);  // empty security-binding list
    }
    bindings.push_back(0);  // security-binding array terminator

    std::vector<uint8_t> result;
    result.reserve(68 + bindings.size() * 2);
    appendU32(result, kObjrefSignature);
    appendU32(result, kObjrefStandard);
    result.insert(result.end(), iid.begin(), iid.end());
    appendU32(result, 0);  // STDOBJREF flags
    appendU32(result, publicReferences);
    appendU64(result, oxid);
    appendU64(result, oid);
    result.insert(result.end(), ipid.begin(), ipid.end());
    appendU16(result, static_cast<uint16_t>(bindings.size()));
    appendU16(result, securityOffset);
    for (uint16_t value : bindings) appendU16(result, value);
    return result;
}

std::vector<uint8_t> encodeResolveOxid2Response(
    const std::string& host, uint16_t exporterPort,
    const std::array<uint8_t, 16>& remoteUnknownIpid,
    uint32_t authenticationHint, uint16_t comMajor,
    uint16_t comMinor, uint32_t status, bool advertiseWinnt) {
    std::vector<uint16_t> bindings;
    bindings.push_back(kNcacnIpTcp);
    const std::string address = host + "[" + std::to_string(exporterPort) + "]";
    for (unsigned char character : address) bindings.push_back(character);
    bindings.push_back(0);
    bindings.push_back(0);
    const uint16_t securityOffset = static_cast<uint16_t>(bindings.size());
    if (advertiseWinnt) {
        bindings.push_back(kRpcAuthnWinnt);
        bindings.push_back(0xffff);
        bindings.push_back(0);  // empty principal name
    } else {
        bindings.push_back(0);  // empty security-binding list
    }
    bindings.push_back(0);  // security-binding array terminator

    std::vector<uint8_t> result;
    appendU32(result, 0x00020000);  // non-null DUALSTRINGARRAY pointer
    appendU32(result, static_cast<uint32_t>(bindings.size()));
    appendU16(result, static_cast<uint16_t>(bindings.size()));
    appendU16(result, securityOffset);
    for (uint16_t value : bindings) appendU16(result, value);
    while (result.size() % 4 != 0) result.push_back(0);
    result.insert(result.end(), remoteUnknownIpid.begin(), remoteUnknownIpid.end());
    appendU32(result, authenticationHint);
    appendU16(result, comMajor);
    appendU16(result, comMinor);
    appendU32(result, status);
    return result;
}

std::vector<uint8_t> encodeResolveOxidResponse(
    const std::string& host, uint16_t exporterPort,
    const std::array<uint8_t, 16>& remoteUnknownIpid,
    uint32_t authenticationHint, uint32_t status,
    bool advertiseWinnt) {
    std::vector<uint8_t> result = encodeResolveOxid2Response(
        host, exporterPort, remoteUnknownIpid, authenticationHint, 5, 7,
        status, advertiseWinnt);
    // ResolveOxid predates COMVERSION; remove that field while retaining the
    // final error_status_t.
    result.erase(result.end() - 8, result.end() - 4);
    return result;
}

std::vector<uint8_t> encodeServerAlive2Response(
    const std::string& host, uint16_t comMajor,
    uint16_t comMinor, uint32_t status, bool advertiseWinnt) {
    std::vector<uint16_t> bindings;
    bindings.push_back(kNcacnIpTcp);
    for (unsigned char character : host) bindings.push_back(character);
    bindings.push_back(0);  // end of ncacn_ip_tcp string binding
    bindings.push_back(0);  // end of string-binding array
    const uint16_t securityOffset = static_cast<uint16_t>(bindings.size());
    if (advertiseWinnt) {
        bindings.push_back(kRpcAuthnWinnt);
        bindings.push_back(0xffff);
        bindings.push_back(0);  // empty principal name
    } else {
        bindings.push_back(0);  // RPC_C_AUTHN_NONE
    }
    bindings.push_back(0);  // end of security-binding array

    std::vector<uint8_t> result;
    appendU16(result, comMajor);
    appendU16(result, comMinor);
    appendU32(result, 0x00020000);
    appendU32(result, static_cast<uint32_t>(bindings.size()));
    appendU16(result, static_cast<uint16_t>(bindings.size()));
    appendU16(result, securityOffset);
    for (uint16_t value : bindings) appendU16(result, value);
    while (result.size() % 4 != 0) result.push_back(0);
    appendU32(result, 0);
    appendU32(result, status);
    return result;
}

std::vector<uint8_t> encodeResolveOxid2Request(uint64_t oxid) {
    std::vector<uint8_t> request;
    request.reserve(18);
    appendU64(request, oxid);
    appendU16(request, 1);  // cRequestedProtseqs
    appendU16(request, 0);  // NDR alignment padding
    appendU32(request, 1);  // conformant array maximum count
    appendU16(request, kNcacnIpTcp);
    return request;
}

std::vector<uint8_t> encodeOrpcThis(
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid) {
    std::vector<uint8_t> request;
    request.reserve(32);
    appendU16(request, comMajor);
    appendU16(request, comMinor);
    appendU32(request, 0);  // ORPC flags
    appendU32(request, 0);  // reserved
    request.insert(request.end(), cid.begin(), cid.end());
    appendU32(request, 0);  // extensions
    return request;
}

bool decodeResolveOxid2Response(
    const std::vector<uint8_t>& stub, OxidResolution& result, std::string& error) {
    result = OxidResolution{};
    if (stub.size() < 44) {
        error = "ResolveOxid2 response is truncated";
        return false;
    }
    const uint32_t bindingsPointer = readU32(stub.data());
    if (bindingsPointer == 0) {
        result.status = readU32(stub.data() + stub.size() - 4);
        error = "ResolveOxid2 returned a null object-exporter binding array";
        return false;
    }
    const uint32_t maximumCount = readU32(stub.data() + 4);
    const uint16_t count = readU16(stub.data() + 8);
    const uint16_t securityOffset = readU16(stub.data() + 10);
    if (maximumCount != count || count > kMaximumDualStringEntries) {
        error = "ResolveOxid2 returned inconsistent DUALSTRINGARRAY counts";
        return false;
    }
    const std::size_t afterArray = 12 + static_cast<std::size_t>(count) * 2;
    const std::size_t fixedOffset = (afterArray + 3) & ~std::size_t(3);
    if (fixedOffset + 28 != stub.size()) {
        error = "ResolveOxid2 response length does not match its NDR fields";
        return false;
    }
    if (!decodeBindings(stub.data() + 12, count, securityOffset,
                        result.exporterBindings, error)) {
        return false;
    }
    std::copy_n(stub.data() + fixedOffset, result.remoteUnknownIpid.size(),
                result.remoteUnknownIpid.begin());
    result.authenticationHint = readU32(stub.data() + fixedOffset + 16);
    result.comMajor = readU16(stub.data() + fixedOffset + 20);
    result.comMinor = readU16(stub.data() + fixedOffset + 22);
    result.status = readU32(stub.data() + fixedOffset + 24);
    if (result.status != 0) {
        error = "ResolveOxid2 returned status " + std::to_string(result.status);
        return false;
    }
    return true;
}

bool selectTcpBinding(const std::vector<StringBinding>& bindings,
                      const std::string& fallbackHost, std::string& host,
                      uint16_t& port, std::string& error) {
    host.clear();
    port = 0;
    for (const StringBinding& binding : bindings) {
        if (binding.towerId != kNcacnIpTcp) continue;
        const std::size_t close = binding.networkAddress.rfind(']');
        const std::size_t open = binding.networkAddress.rfind('[');
        if (open == std::string::npos || close != binding.networkAddress.size() - 1 ||
            open >= close || !parsePort(binding.networkAddress.substr(open + 1, close - open - 1), port)) {
            continue;
        }
        host = binding.networkAddress.substr(0, open);
        if (host.empty()) host = fallbackHost;
        if (!host.empty()) return true;
    }
    error = "ResolveOxid2 returned no usable ncacn_ip_tcp binding with an endpoint";
    return false;
}

}  // namespace wk::opcda::dcom
