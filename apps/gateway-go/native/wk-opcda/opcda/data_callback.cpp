#include "opcda/data_callback.h"

#include <cstddef>
#include <limits>

namespace wk::opcda::opcda {
namespace {

constexpr std::size_t kOrpcThisSize = 32;
constexpr uint32_t kMaximumItems = 10000;

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

std::size_t align4(std::size_t value) { return (value + 3) & ~std::size_t(3); }
std::size_t align8(std::size_t value) { return (value + 7) & ~std::size_t(7); }

int64_t filetimeToUnixMilliseconds(uint64_t value) {
    constexpr uint64_t epoch = 116444736000000000ULL;
    if (value < epoch) return 0;
    const uint64_t milliseconds = (value - epoch) / 10000ULL;
    return milliseconds > static_cast<uint64_t>(std::numeric_limits<int64_t>::max())
        ? std::numeric_limits<int64_t>::max()
        : static_cast<int64_t>(milliseconds);
}

bool readArrayCount(const std::vector<uint8_t>& stub, std::size_t& offset,
                    uint32_t expected, const char* name, std::string& error) {
    offset = align4(offset);
    if (offset + 4 > stub.size()) {
        error = std::string("IOPCDataCallback ") + name + " array is truncated";
        return false;
    }
    const uint32_t count = readU32(stub.data() + offset);
    offset += 4;
    if (count != expected || count > kMaximumItems) {
        error = std::string("IOPCDataCallback ") + name +
            " array count does not match dwCount";
        return false;
    }
    return true;
}

}  // namespace

bool decodeDataChangeRequest(const std::vector<uint8_t>& stub,
                             DataChange& change, std::string& error) {
    change = DataChange{};
    // ORPCTHIS (including the null extensions pointer) followed by five
    // scalar values. The callback arrays are [ref, size_is(dwCount)] in the
    // OPC DA IDL, so NDR places each conformant array directly in the stream;
    // there are no top-level referent IDs to skip here.
    if (stub.size() < kOrpcThisSize + 20) {
        error = "IOPCDataCallback::OnDataChange request is truncated";
        return false;
    }
    if (readU32(stub.data() + 28) != 0) {
        error = "IOPCDataCallback ORPCTHIS extensions are not supported";
        return false;
    }
    std::size_t offset = kOrpcThisSize;
    change.transactionId = readU32(stub.data() + offset); offset += 4;
    change.clientGroupHandle = readU32(stub.data() + offset); offset += 4;
    change.masterQuality = readU32(stub.data() + offset); offset += 4;
    change.masterError = readU32(stub.data() + offset); offset += 4;
    const uint32_t count = readU32(stub.data() + offset); offset += 4;
    if (count == 0 || count > kMaximumItems) {
        error = "IOPCDataCallback dwCount is outside the supported range";
        return false;
    }
    if (!readArrayCount(stub, offset, count, "client handle", error) ||
        offset + static_cast<std::size_t>(count) * 4 > stub.size()) return false;
    change.values.resize(count);
    for (uint32_t index = 0; index < count; ++index) {
        change.values[index].clientHandle = readU32(stub.data() + offset);
        offset += 4;
    }

    if (!readArrayCount(stub, offset, count, "VARIANT", error) ||
        offset + static_cast<std::size_t>(count) * 4 > stub.size()) return false;
    // VARIANT is itself a [unique] pointer to wireVARIANT. Therefore an array
    // of VARIANTs first carries one referent ID per element, followed by the
    // deferred wireVARIANT values. Matrikon's Windows automation marshaler
    // uses this standard MS-OAUT representation.
    for (uint32_t index = 0; index < count; ++index) {
        if (readU32(stub.data() + offset) == 0) {
            error = "IOPCDataCallback returned a null VARIANT pointer";
            return false;
        }
        offset += 4;
    }
    for (uint32_t index = 0; index < count; ++index) {
        if (!decodeWireVariant(stub, offset, change.values[index], error)) {
            error = "IOPCDataCallback item " + std::to_string(index) + ": " + error;
            return false;
        }
    }

    if (!readArrayCount(stub, offset, count, "quality", error) ||
        offset + static_cast<std::size_t>(count) * 2 > stub.size()) return false;
    for (uint32_t index = 0; index < count; ++index) {
        change.values[index].quality = readU16(stub.data() + offset);
        offset += 2;
    }
    offset = align4(offset);

    if (!readArrayCount(stub, offset, count, "timestamp", error)) return false;
    // FILETIME is an NDR structure of two DWORD fields and therefore has
    // four-byte alignment. Treating it as an eight-byte scalar only happened
    // to work for odd-sized quality arrays and shifted even callback batches.
    offset = align4(offset);
    if (offset + static_cast<std::size_t>(count) * 8 > stub.size()) return false;
    for (uint32_t index = 0; index < count; ++index) {
        const uint64_t filetime = readU64(stub.data() + offset);
        change.values[index].timestampFiletime = filetime;
        change.values[index].timestampUnixMilliseconds = filetimeToUnixMilliseconds(filetime);
        offset += 8;
    }

    if (!readArrayCount(stub, offset, count, "HRESULT", error) ||
        offset + static_cast<std::size_t>(count) * 4 != stub.size()) return false;
    for (uint32_t index = 0; index < count; ++index) {
        change.values[index].hresult = readU32(stub.data() + offset);
        offset += 4;
    }
    return true;
}

}  // namespace wk::opcda::opcda
