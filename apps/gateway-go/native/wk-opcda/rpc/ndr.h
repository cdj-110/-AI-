#ifndef WK_OPCDA_RPC_NDR_H
#define WK_OPCDA_RPC_NDR_H

#include <array>
#include <cstddef>
#include <cstdint>
#include <string>
#include <utility>
#include <vector>

namespace wk::opcda::rpc::ndr {

class Writer {
public:
    void align(std::size_t boundary);
    void writeU8(uint8_t value);
    void writeU16(uint16_t value);
    void writeU32(uint32_t value);
    void writeU64(uint64_t value);
    void writeBytes(const uint8_t* data, std::size_t size);
    void writeUuid(const std::array<uint8_t, 16>& wireUuid);
    void writeConformantVaryingHeader(uint32_t maximumCount, uint32_t offset, uint32_t actualCount);
    uint32_t writePointer(bool present);

    const std::vector<uint8_t>& data() const noexcept { return data_; }
    std::vector<uint8_t> take() { return std::move(data_); }

private:
    std::vector<uint8_t> data_;
    uint32_t nextReferent_ = 0x00020000;
};

class Reader {
public:
    Reader(const uint8_t* data, std::size_t size) : data_(data), size_(size) {}
    explicit Reader(const std::vector<uint8_t>& data) : Reader(data.data(), data.size()) {}

    bool align(std::size_t boundary, std::string& error);
    bool readU8(uint8_t& value, std::string& error);
    bool readU16(uint16_t& value, std::string& error);
    bool readU32(uint32_t& value, std::string& error);
    bool readU64(uint64_t& value, std::string& error);
    bool readBytes(std::size_t count, std::vector<uint8_t>& value, std::string& error);
    bool readUuid(std::array<uint8_t, 16>& value, std::string& error);
    bool readConformantVaryingHeader(uint32_t& maximumCount, uint32_t& offset,
                                    uint32_t& actualCount, std::string& error);

    std::size_t position() const noexcept { return offset_; }
    std::size_t remaining() const noexcept { return size_ - offset_; }

private:
    bool require(std::size_t count, std::string& error) const;
    const uint8_t* data_ = nullptr;
    std::size_t size_ = 0;
    std::size_t offset_ = 0;
};

}  // namespace wk::opcda::rpc::ndr

#endif
