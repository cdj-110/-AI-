#ifndef WK_OPCDA_DCOM_OBJECT_REFERENCE_H
#define WK_OPCDA_DCOM_OBJECT_REFERENCE_H

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::dcom {

struct StringBinding {
    uint16_t towerId = 0;
    std::string networkAddress;
};

struct StandardObjectReference {
    std::array<uint8_t, 16> iid{};
    uint32_t standardFlags = 0;
    uint32_t publicReferences = 0;
    uint64_t oxid = 0;
    uint64_t oid = 0;
    std::array<uint8_t, 16> ipid{};
    std::vector<StringBinding> resolverBindings;
};

struct OxidResolution {
    std::vector<StringBinding> exporterBindings;
    std::array<uint8_t, 16> remoteUnknownIpid{};
    uint32_t authenticationHint = 0;
    uint16_t comMajor = 0;
    uint16_t comMinor = 0;
    uint32_t status = 0;
};

bool decodeStandardObjectReference(
    const std::vector<uint8_t>& objref, const std::array<uint8_t, 16>& expectedIid,
    StandardObjectReference& result, std::string& error);

std::vector<uint8_t> encodeStandardObjectReference(
    const std::array<uint8_t, 16>& iid, uint64_t oxid, uint64_t oid,
    const std::array<uint8_t, 16>& ipid, const std::string& host,
    uint16_t resolverPort, uint32_t publicReferences = 1,
    bool advertiseWinnt = true);

std::vector<uint8_t> encodeResolveOxid2Response(
    const std::string& host, uint16_t exporterPort,
    const std::array<uint8_t, 16>& remoteUnknownIpid,
    uint32_t authenticationHint, uint16_t comMajor = 5,
    uint16_t comMinor = 7, uint32_t status = 0,
    bool advertiseWinnt = true);

std::vector<uint8_t> encodeResolveOxidResponse(
    const std::string& host, uint16_t exporterPort,
    const std::array<uint8_t, 16>& remoteUnknownIpid,
    uint32_t authenticationHint, uint32_t status = 0,
    bool advertiseWinnt = true);

std::vector<uint8_t> encodeServerAlive2Response(
    const std::string& host, uint16_t comMajor = 5,
    uint16_t comMinor = 7, uint32_t status = 0,
    bool advertiseWinnt = true);

std::vector<uint8_t> encodeResolveOxid2Request(uint64_t oxid);
std::vector<uint8_t> encodeOrpcThis(
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid);
bool decodeResolveOxid2Response(
    const std::vector<uint8_t>& stub, OxidResolution& result, std::string& error);

bool selectTcpBinding(const std::vector<StringBinding>& bindings,
                      const std::string& fallbackHost, std::string& host,
                      uint16_t& port, std::string& error);

}  // namespace wk::opcda::dcom

#endif
