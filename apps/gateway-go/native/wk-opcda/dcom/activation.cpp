#include "dcom/activation.h"

#include "rpc/rpc_pdu.h"

#include <algorithm>
#include <cstddef>
#include <random>
#include <utility>

namespace wk::opcda::dcom {
namespace {

constexpr uint32_t kObjrefSignature = 0x574f454d;
constexpr uint32_t kObjrefStandard = 0x00000001;
constexpr uint16_t kNcacnIpTcp = 0x0007;

class Writer {
public:
    void align(std::size_t boundary, uint8_t fill = 0) {
        if (boundary <= 1) return;
        data_.insert(data_.end(), (boundary - data_.size() % boundary) % boundary, fill);
    }
    void u16(uint16_t value) {
        align(2);
        data_.push_back(static_cast<uint8_t>(value));
        data_.push_back(static_cast<uint8_t>(value >> 8));
    }
    void u32(uint32_t value) {
        align(4);
        for (int shift = 0; shift < 32; shift += 8) {
            data_.push_back(static_cast<uint8_t>(value >> shift));
        }
    }
    void uuid(const std::array<uint8_t, 16>& value) {
        align(4);
        data_.insert(data_.end(), value.begin(), value.end());
    }
    uint32_t pointer(bool present) {
        const uint32_t value = present ? nextReferent_++ : 0;
        u32(value);
        return value;
    }
    void bytes(const std::vector<uint8_t>& value) {
        data_.insert(data_.end(), value.begin(), value.end());
    }
    void bytes(const uint8_t* value, std::size_t size) {
        data_.insert(data_.end(), value, value + size);
    }
    void patchU32(std::size_t offset, uint32_t value) {
        for (int shift = 0; shift < 32; shift += 8) {
            data_[offset + static_cast<std::size_t>(shift / 8)] =
                static_cast<uint8_t>(value >> shift);
        }
    }
    std::size_t size() const { return data_.size(); }
    const std::vector<uint8_t>& data() const { return data_; }
    std::vector<uint8_t> take() { return std::move(data_); }

private:
    std::vector<uint8_t> data_;
    uint32_t nextReferent_ = 0x00020000;
};

uint32_t readU32(const uint8_t* data) {
    return static_cast<uint32_t>(data[0]) |
           (static_cast<uint32_t>(data[1]) << 8) |
           (static_cast<uint32_t>(data[2]) << 16) |
           (static_cast<uint32_t>(data[3]) << 24);
}

uint16_t readU16(const uint8_t* data) {
    return static_cast<uint16_t>(data[0]) |
           static_cast<uint16_t>(static_cast<uint16_t>(data[1]) << 8);
}

std::array<uint8_t, 16> uuid(const char* text) {
    std::array<uint8_t, 16> value{};
    std::string ignored;
    rpc::parseUuid(text, value, ignored);
    return value;
}

std::vector<uint8_t> serializeConstructed(std::vector<uint8_t> body) {
    Writer value;
    // MS-RPCE type serialization version 1 common and private headers.
    value.bytes(std::vector<uint8_t>{1, 0x10, 8, 0, 0xcc, 0xcc, 0xcc, 0xcc});
    value.u32(static_cast<uint32_t>(body.size()));
    value.u32(0xcccccccc);
    value.bytes(body);
    value.align(8, 0xfa);
    return value.take();
}

std::vector<uint8_t> instantiationProperty(
    const std::array<uint8_t, 16>& classId,
    const std::array<uint8_t, 16>& requestedIid) {
    Writer body;
    body.uuid(classId);
    body.u32(0);  // class context is ignored by the server
    body.u32(0);  // actvflags
    body.u32(0);  // fIsSurrogate
    body.u32(1);  // cIID
    body.u32(0);  // instFlag
    body.pointer(true);
    const std::size_t thisSizeOffset = body.size();
    body.u32(0);
    body.u16(5);
    body.u16(7);
    body.u32(1);  // conformant pIID maximum count (cIID is the actual count)
    body.uuid(requestedIid);
    std::vector<uint8_t> serialized = serializeConstructed(body.take());
    // thisSize is the complete serialized property, including both headers.
    const std::size_t serializedFieldOffset = 16 + thisSizeOffset;
    const uint32_t size = static_cast<uint32_t>(serialized.size());
    for (int shift = 0; shift < 32; shift += 8) {
        serialized[serializedFieldOffset + static_cast<std::size_t>(shift / 8)] =
            static_cast<uint8_t>(size >> shift);
    }
    return serialized;
}

std::vector<uint8_t> scmRequestProperty() {
    Writer body;
    body.pointer(false);  // pdwReserved
    body.pointer(true);   // remoteRequest
    body.u32(0);          // ignored by the server
    body.u16(1);
    body.pointer(true);   // pRequestedProtseqs
    body.u32(1);          // conformant array maximum count
    body.u16(kNcacnIpTcp);
    return serializeConstructed(body.take());
}

std::vector<uint8_t> activationContextProperty() {
    Writer body;
    body.u32(0);          // clientOK
    body.u32(0);          // reserved
    body.u32(0);          // dwReserved1
    body.u32(0);          // dwReserved2
    body.pointer(false);  // clientContext
    body.pointer(false);  // prototypeContext
    return serializeConstructed(body.take());
}

std::vector<uint8_t> locationProperty() {
    Writer body;
    body.pointer(false);  // machineName
    body.u32(0);
    body.u32(0);
    body.u32(0);
    return serializeConstructed(body.take());
}

std::vector<uint8_t> customHeader(
    const std::array<std::array<uint8_t, 16>, 4>& propertyClsids,
    const std::array<uint32_t, 4>& propertySizes, uint32_t totalSize,
    uint32_t headerSize) {
    Writer body;
    body.u32(totalSize);
    body.u32(headerSize);
    body.u32(0);
    body.u32(2);  // MSHCTX_DIFFERENTMACHINE
    body.u32(4);
    body.uuid({});
    body.pointer(true);
    body.pointer(true);
    body.pointer(false);
    body.u32(4);
    for (const auto& clsid : propertyClsids) body.uuid(clsid);
    body.u32(4);
    for (uint32_t size : propertySizes) body.u32(size);
    return serializeConstructed(body.take());
}

std::vector<uint8_t> activationPropertiesBlob(
    const std::array<uint8_t, 16>& classId,
    const std::array<uint8_t, 16>& requestedIid) {
    const std::array<std::array<uint8_t, 16>, 4> propertyClsids = {
        uuid("000001ab-0000-0000-c000-000000000046"),
        uuid("000001a5-0000-0000-c000-000000000046"),
        uuid("000001a4-0000-0000-c000-000000000046"),
        uuid("000001aa-0000-0000-c000-000000000046"),
    };
    const std::array<std::vector<uint8_t>, 4> properties = {
        instantiationProperty(classId, requestedIid),
        activationContextProperty(),
        locationProperty(),
        scmRequestProperty(),
    };
    const std::array<uint32_t, 4> propertySizes = {
        static_cast<uint32_t>(properties[0].size()),
        static_cast<uint32_t>(properties[1].size()),
        static_cast<uint32_t>(properties[2].size()),
        static_cast<uint32_t>(properties[3].size()),
    };

    // Build once to determine the serialized header size, then fill the two
    // self-referential size fields with their final values.
    std::vector<uint8_t> header = customHeader(propertyClsids, propertySizes, 0, 0);
    const uint32_t headerSize = static_cast<uint32_t>(header.size());
    uint32_t totalSize = headerSize;
    for (uint32_t size : propertySizes) totalSize += size;
    header = customHeader(propertyClsids, propertySizes, totalSize, headerSize);

    Writer stream;
    stream.bytes(header);
    for (const auto& property : properties) stream.bytes(property);
    const std::vector<uint8_t> serializedStream = stream.take();

    Writer blob;
    blob.u32(static_cast<uint32_t>(serializedStream.size()));
    blob.u32(0);
    blob.bytes(serializedStream);
    return blob.take();
}

std::vector<uint8_t> activationPropertiesObjref(
    const std::array<uint8_t, 16>& classId,
    const std::array<uint8_t, 16>& requestedIid) {
    const std::vector<uint8_t> blob = activationPropertiesBlob(classId, requestedIid);
    Writer objref;
    objref.u32(kObjrefSignature);
    objref.u32(0x00000004);  // OBJREF_CUSTOM
    objref.uuid(uuid("000001a2-0000-0000-c000-000000000046"));
    objref.uuid(uuid("00000338-0000-0000-c000-000000000046"));
    objref.u32(0);
    objref.u32(static_cast<uint32_t>(blob.size() + 8));
    objref.bytes(blob);
    return objref.take();
}

bool sameUuid(const uint8_t* value, const std::array<uint8_t, 16>& expected) {
    return std::equal(expected.begin(), expected.end(), value);
}

}  // namespace

std::array<uint8_t, 16> randomCid() {
    std::array<uint8_t, 16> value{};
    std::random_device source;
    for (uint8_t& byte : value) byte = static_cast<uint8_t>(source());
    // RFC 4122 variant/version bits are not required for CID, but keeping a
    // conventional UUID shape improves diagnostics and packet inspection.
    value[7] = static_cast<uint8_t>((value[7] & 0x0f) | 0x40);
    value[8] = static_cast<uint8_t>((value[8] & 0x3f) | 0x80);
    return value;
}

std::array<uint8_t, 16> opcServerIid() {
    return uuid("39c13a4d-011e-11d0-9675-0020afd8adb3");
}

bool encodeRemoteCreateInstanceRequest(
    const std::string& clsid, const std::array<uint8_t, 16>& requestedIid,
    const std::array<uint8_t, 16>& cid, std::vector<uint8_t>& stub,
    std::string& error) {
    std::array<uint8_t, 16> classId{};
    if (!rpc::parseUuid(clsid, classId, error)) {
        error = "invalid activation CLSID: " + error;
        return false;
    }
    const std::vector<uint8_t> objref = activationPropertiesObjref(classId, requestedIid);

    Writer request;
    // ORPCTHIS is an embedded value in RemoteCreateInstance, not a pointer.
    request.u16(5);
    request.u16(7);
    request.u32(1);  // ORPCF_NULL is forbidden; Windows uses ORPCF_LOCAL.
    request.u32(0);
    request.uuid(cid);
    request.pointer(false);  // extensions
    request.pointer(false);  // pUnkOuter
    request.pointer(true);   // pActProperties
    request.u32(static_cast<uint32_t>(objref.size()));  // conformant maximum count
    request.u32(static_cast<uint32_t>(objref.size()));  // ulCntData
    request.bytes(objref);
    stub = request.take();
    return true;
}

bool decodeRemoteCreateInstanceResponse(
    const std::vector<uint8_t>& stub, const std::array<uint8_t, 16>& requestedIid,
    RemoteCreateInstanceResult& result, std::string& error) {
    result = RemoteCreateInstanceResult{};
    if (stub.size() < 4) {
        error = "RemoteCreateInstance response is truncated before HRESULT";
        return false;
    }
    result.hresult = readU32(stub.data() + stub.size() - 4);
    if ((result.hresult & 0x80000000U) != 0) {
        error = "RemoteCreateInstance returned HRESULT 0x";
        constexpr char hex[] = "0123456789ABCDEF";
        for (int shift = 28; shift >= 0; shift -= 4) {
            error.push_back(hex[(result.hresult >> shift) & 0x0f]);
        }
        return false;
    }

    // ActivationPropertiesOut contains nested hand-marshaled OBJREF values.
    // Locate the one whose IID is exactly the requested OPC interface and
    // validate the complete OBJREF_STANDARD boundary before retaining it.
    for (std::size_t offset = 0; offset + 68 <= stub.size() - 4; ++offset) {
        if (readU32(stub.data() + offset) != kObjrefSignature ||
            readU32(stub.data() + offset + 4) != kObjrefStandard ||
            !sameUuid(stub.data() + offset + 8, requestedIid)) {
            continue;
        }
        const uint16_t stringEntries = readU16(stub.data() + offset + 64);
        const std::size_t objrefSize = 68 + static_cast<std::size_t>(stringEntries) * 2;
        if (objrefSize > stub.size() - 4 - offset || stringEntries > 4096) {
            error = "RemoteCreateInstance returned a truncated OPC interface OBJREF";
            return false;
        }
        result.interfaceObjectReference.assign(
            stub.begin() + static_cast<std::ptrdiff_t>(offset),
            stub.begin() + static_cast<std::ptrdiff_t>(offset + objrefSize));
        return true;
    }
    error = "RemoteCreateInstance succeeded but returned no IID_IOPCServer OBJREF";
    return false;
}

}  // namespace wk::opcda::dcom
