#ifndef WK_OPCDA_DCOM_PING_H
#define WK_OPCDA_DCOM_PING_H

#include "rpc/ndr.h"
#include <vector>
#include <stdexcept>

namespace wk::opcda::dcom {
// Top-level unique array referents are emitted immediately after their pointer.
inline std::vector<uint8_t> encodeComplexPing(uint64_t setId, uint16_t sequence,
        const std::vector<uint64_t>& add, const std::vector<uint64_t>& remove) {
    if (add.size() > 65535 || remove.size() > 65535)
        throw std::invalid_argument("DCOM ping set is too large");
    rpc::ndr::Writer writer;
    writer.writeU64(setId);
    writer.writeU16(sequence);
    writer.writeU16(static_cast<uint16_t>(add.size()));
    writer.writeU16(static_cast<uint16_t>(remove.size()));
    for (const auto* list : {&add, &remove}) {
        writer.writePointer(!list->empty());
        if (!list->empty()) {
            writer.writeU32(static_cast<uint32_t>(list->size()));
            for (auto oid : *list) writer.writeU64(oid);
        }
    }
    return writer.take();
}

inline std::vector<uint8_t> encodeSimplePing(uint64_t setId) {
    rpc::ndr::Writer writer;
    writer.writeU64(setId);
    return writer.take();
}

inline bool decodePingResponse(const std::vector<uint8_t>& stub, bool complex,
        uint64_t& setId, std::string& error) {
    rpc::ndr::Reader reader(stub);
    uint64_t returnedSet = setId;
    uint16_t backoff = 0;
    uint32_t status = 0;
    if ((complex && (!reader.readU64(returnedSet, error) ||
                     !reader.readU16(backoff, error))) ||
        !reader.readU32(status, error)) return false;
    if (status != 0) {
        error = "DCOM ping returned status=" + std::to_string(status);
        return false;
    }
    if (reader.remaining() != 0 || returnedSet == 0) {
        error = "DCOM ping response has invalid size or zero SETID";
        return false;
    }
    setId = returnedSet;
    return true;
}
} // namespace wk::opcda::dcom
#endif
