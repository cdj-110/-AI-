#ifndef WK_OPCDA_DCOM_ACTIVATION_H
#define WK_OPCDA_DCOM_ACTIVATION_H

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::dcom {

struct RemoteCreateInstanceResult {
    uint32_t hresult = 0;
    std::vector<uint8_t> interfaceObjectReference;
};

std::array<uint8_t, 16> randomCid();
bool encodeRemoteCreateInstanceRequest(
    const std::string& clsid, const std::array<uint8_t, 16>& requestedIid,
    const std::array<uint8_t, 16>& cid, std::vector<uint8_t>& stub,
    std::string& error);
bool decodeRemoteCreateInstanceResponse(
    const std::vector<uint8_t>& stub, const std::array<uint8_t, 16>& requestedIid,
    RemoteCreateInstanceResult& result, std::string& error);

std::array<uint8_t, 16> opcServerIid();

}  // namespace wk::opcda::dcom

#endif
