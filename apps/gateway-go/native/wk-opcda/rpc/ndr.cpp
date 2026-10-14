#include "rpc/ndr.h"

#include <algorithm>
namespace wk::opcda::rpc::ndr {

void Writer::align(std::size_t boundary) {
    if (boundary <= 1) return;
    const std::size_t padding = (boundary - (data_.size() % boundary)) % boundary;
    data_.insert(data_.end(), padding, 0);
}

void Writer::writeU8(uint8_t value) {
    data_.push_back(value);
}

void Writer::writeU16(uint16_t value) {
    align(2);
    data_.push_back(static_cast<uint8_t>(value));
    data_.push_back(static_cast<uint8_t>(value >> 8));
}

void Writer::writeU32(uint32_t value) {
    align(4);
    for (int shift = 0; shift < 32; shift += 8) {
        data_.push_back(static_cast<uint8_t>(value >> shift));
    }
}

void Writer::writeU64(uint64_t value) {
    align(8);
    for (int shift = 0; shift < 64; shift += 8) {
        data_.push_back(static_cast<uint8_t>(value >> shift));
    }
}

void Writer::writeBytes(const uint8_t* data, std::size_t size) {
    data_.insert(data_.end(), data, data + size);
}

void Writer::writeUuid(const std::array<uint8_t, 16>& wireUuid) {
    align(4);
    writeBytes(wireUuid.data(), wireUuid.size());
}

void Writer::writeConformantVaryingHeader(uint32_t maximumCount, uint32_t offset, uint32_t actualCount) {
    writeU32(maximumCount);
    writeU32(offset);
    writeU32(actualCount);
}

uint32_t Writer::writePointer(bool present) {
    const uint32_t referent = present ? nextReferent_++ : 0;
    writeU32(referent);
    return referent;
}

bool Reader::require(std::size_t count, std::string& error) const {
    if (count <= size_ - offset_) return true;
    error = "NDR buffer is truncated at offset " + std::to_string(offset_) +
            ": need " + std::to_string(count) + " bytes, have " + std::to_string(size_ - offset_);
    return false;
}

bool Reader::align(std::size_t boundary, std::string& error) {
    if (boundary <= 1) return true;
    const std::size_t padding = (boundary - (offset_ % boundary)) % boundary;
    if (!require(padding, error)) return false;
    offset_ += padding;
    return true;
}

bool Reader::readU8(uint8_t& value, std::string& error) {
    if (!require(1, error)) return false;
    value = data_[offset_++];
    return true;
}

bool Reader::readU16(uint16_t& value, std::string& error) {
    if (!align(2, error) || !require(2, error)) return false;
    value = static_cast<uint16_t>(data_[offset_]) |
            static_cast<uint16_t>(static_cast<uint16_t>(data_[offset_ + 1]) << 8);
    offset_ += 2;
    return true;
}

bool Reader::readU32(uint32_t& value, std::string& error) {
    if (!align(4, error) || !require(4, error)) return false;
    value = 0;
    for (int shift = 0; shift < 32; shift += 8) {
        value |= static_cast<uint32_t>(data_[offset_++]) << shift;
    }
    return true;
}

bool Reader::readU64(uint64_t& value, std::string& error) {
    if (!align(8, error) || !require(8, error)) return false;
    value = 0;
    for (int shift = 0; shift < 64; shift += 8) {
        value |= static_cast<uint64_t>(data_[offset_++]) << shift;
    }
    return true;
}

bool Reader::readBytes(std::size_t count, std::vector<uint8_t>& value, std::string& error) {
    if (!require(count, error)) return false;
    value.assign(data_ + offset_, data_ + offset_ + count);
    offset_ += count;
    return true;
}

bool Reader::readUuid(std::array<uint8_t, 16>& value, std::string& error) {
    if (!align(4, error) || !require(value.size(), error)) return false;
    std::copy_n(data_ + offset_, value.size(), value.begin());
    offset_ += value.size();
    return true;
}

bool Reader::readConformantVaryingHeader(uint32_t& maximumCount, uint32_t& offset,
                                         uint32_t& actualCount, std::string& error) {
    if (!readU32(maximumCount, error) || !readU32(offset, error) || !readU32(actualCount, error)) {
        return false;
    }
    if (offset > maximumCount || actualCount > maximumCount - offset) {
        error = "invalid NDR conformant varying array bounds";
        return false;
    }
    return true;
}

}  // namespace wk::opcda::rpc::ndr
