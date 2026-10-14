#ifndef WK_OPCDA_OPCDA_SERVER_H
#define WK_OPCDA_OPCDA_SERVER_H

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::opcda {

struct ServerStatus {
    uint64_t startFiletime = 0;
    uint64_t currentFiletime = 0;
    uint64_t lastUpdateFiletime = 0;
    int64_t currentUnixMilliseconds = 0;
    uint32_t serverState = 0;
    uint32_t groupCount = 0;
    uint32_t bandwidth = 0;
    uint16_t majorVersion = 0;
    uint16_t minorVersion = 0;
    uint16_t buildNumber = 0;
    std::string vendorInfo;
    uint32_t hresult = 0;
};

struct AddGroupResult {
    uint32_t serverGroupHandle = 0;
    uint32_t revisedUpdateRateMs = 0;
    std::vector<uint8_t> itemManagementObjectReference;
    uint32_t hresult = 0;
};

struct InterfaceResult {
    std::vector<uint8_t> objectReference;
    uint32_t hresult = 0;
};

std::array<uint8_t, 16> opcItemManagementIid();
std::array<uint8_t, 16> opcSyncIoIid();

std::vector<uint8_t> encodeGetStatusRequest(
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid);
bool decodeGetStatusResponse(
    const std::vector<uint8_t>& stub, ServerStatus& result, std::string& error);

bool encodeAddGroupRequest(
    const std::string& name, bool active, uint32_t requestedUpdateRateMs,
    uint32_t clientGroupHandle, float percentDeadband, uint32_t localeId,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid,
    std::vector<uint8_t>& stub, std::string& error);
bool decodeAddGroupResponse(
    const std::vector<uint8_t>& stub, AddGroupResult& result, std::string& error);

bool encodeGetGroupByNameRequest(
    const std::string& name, const std::array<uint8_t, 16>& requestedIid,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid,
    std::vector<uint8_t>& stub, std::string& error);
bool decodeGetGroupByNameResponse(
    const std::vector<uint8_t>& stub, InterfaceResult& result, std::string& error);

}  // namespace wk::opcda::opcda

#endif
