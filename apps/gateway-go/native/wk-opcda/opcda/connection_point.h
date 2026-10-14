#ifndef WK_OPCDA_OPCDA_CONNECTION_POINT_H
#define WK_OPCDA_OPCDA_CONNECTION_POINT_H

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::opcda {

std::array<uint8_t, 16> connectionPointContainerIid();
std::array<uint8_t, 16> connectionPointIid();
std::array<uint8_t, 16> unknownIid();
std::array<uint8_t, 16> remoteUnknownIid();
std::array<uint8_t, 16> remoteUnknown2Iid();
std::array<uint8_t, 16> opcDataCallbackIid();
std::array<uint8_t, 16> opcAsyncIo2Iid();

std::vector<uint8_t> encodeFindConnectionPointRequest(
    const std::array<uint8_t, 16>& sinkIid, uint16_t comMajor,
    uint16_t comMinor, const std::array<uint8_t, 16>& cid);

bool decodeInterfacePointerResponse(
    const std::vector<uint8_t>& stub, const char* method,
    std::vector<uint8_t>& objectReference, uint32_t& hresult,
    std::string& error);

std::vector<uint8_t> encodeAdviseRequest(
    const std::vector<uint8_t>& callbackObjectReference,
    uint16_t comMajor, uint16_t comMinor,
    const std::array<uint8_t, 16>& cid);

bool decodeAdviseResponse(const std::vector<uint8_t>& stub,
                          uint32_t& cookie, uint32_t& hresult,
                          std::string& error);

std::vector<uint8_t> encodeUnadviseRequest(
    uint32_t cookie, uint16_t comMajor, uint16_t comMinor,
    const std::array<uint8_t, 16>& cid);

bool decodeHresultResponse(const std::vector<uint8_t>& stub,
                           const char* method, uint32_t& hresult,
                           std::string& error);

}  // namespace wk::opcda::opcda

#endif
