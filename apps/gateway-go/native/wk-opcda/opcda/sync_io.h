#ifndef WK_OPCDA_OPCDA_SYNC_IO_H
#define WK_OPCDA_OPCDA_SYNC_IO_H

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::opcda {

enum class SyncValueKind {
    Empty,
    Boolean,
    SignedInteger,
    UnsignedInteger,
    FloatingPoint,
    String,
    DateTime,
    FloatingPointArray,
    StringArray,
};

struct SyncReadValue {
    uint32_t clientHandle = 0;
    uint64_t timestampFiletime = 0;
    int64_t timestampUnixMilliseconds = 0;
    uint16_t quality = 0;
    uint16_t vartype = 0;
    uint32_t hresult = 0;
    SyncValueKind kind = SyncValueKind::Empty;
    int64_t signedValue = 0;
    uint64_t unsignedValue = 0;
    double floatingValue = 0;
    std::string stringValue;
    std::vector<double> floatingArrayValue;
    std::vector<std::string> stringArrayValue;
};

struct SyncWriteValue {
    uint16_t vartype = 0;
    int64_t signedValue = 0;
    uint64_t unsignedValue = 0;
    double floatingValue = 0;
    std::string stringValue;
    int64_t datetimeUnixMilliseconds = 0;
};

bool decodeWireVariant(const std::vector<uint8_t>& stub, std::size_t& offset,
                       SyncReadValue& value, std::string& error);

std::vector<uint8_t> encodeSyncReadRequest(
    uint32_t source, const std::vector<uint32_t>& serverHandles,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid);

bool decodeSyncReadResponse(
    const std::vector<uint8_t>& stub, std::size_t expectedCount,
    std::vector<SyncReadValue>& values, uint32_t& hresult, std::string& error);

bool orderSyncReadValuesByClientHandle(
    const std::vector<uint32_t>& requestedClientHandles,
    std::vector<SyncReadValue>& values, std::string& error);

bool encodeSyncWriteRequest(
    const std::vector<uint32_t>& serverHandles,
    const std::vector<SyncWriteValue>& values,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid,
    std::vector<uint8_t>& stub, std::string& error);

bool decodeSyncWriteResponse(
    const std::vector<uint8_t>& stub, std::size_t expectedCount,
    std::vector<uint32_t>& itemHresults, uint32_t& hresult,
    std::string& error);

}  // namespace wk::opcda::opcda

#endif
