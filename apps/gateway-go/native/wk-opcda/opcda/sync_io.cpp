#include "opcda/sync_io.h"

#include "dcom/object_reference.h"

#include <cmath>
#include <cstddef>
#include <cstring>
#include <limits>
#include <unordered_map>

namespace wk::opcda::opcda {
namespace {

constexpr std::size_t kMaximumItems = 10000;
constexpr std::size_t kMaximumStringUnits = 1024 * 1024;
constexpr std::size_t kMaximumArrayDimensions = 32;
constexpr std::size_t kMaximumArrayElements = 1024 * 1024;
constexpr uint64_t kWindowsToUnixEpoch100Nanoseconds = 116444736000000000ULL;
constexpr double kOleDateUnixEpochDays = 25569.0;
constexpr double kMillisecondsPerDay = 86400000.0;

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

void appendU32(std::vector<uint8_t>& data, uint32_t value) {
    data.push_back(static_cast<uint8_t>(value));
    data.push_back(static_cast<uint8_t>(value >> 8));
    data.push_back(static_cast<uint8_t>(value >> 16));
    data.push_back(static_cast<uint8_t>(value >> 24));
}

void appendU16(std::vector<uint8_t>& data, uint16_t value) {
    data.push_back(static_cast<uint8_t>(value));
    data.push_back(static_cast<uint8_t>(value >> 8));
}

void appendU64(std::vector<uint8_t>& data, uint64_t value) {
    appendU32(data, static_cast<uint32_t>(value));
    appendU32(data, static_cast<uint32_t>(value >> 32));
}

void align(std::vector<uint8_t>& data, std::size_t boundary) {
    while (data.size() % boundary != 0) data.push_back(0);
}

std::size_t align4(std::size_t value) {
    return (value + 3) & ~std::size_t(3);
}

std::size_t align8(std::size_t value) {
    return (value + 7) & ~std::size_t(7);
}

bool failed(uint32_t hresult) {
    return (hresult & 0x80000000U) != 0;
}

std::string hexValue(uint32_t value) {
    constexpr char hex[] = "0123456789ABCDEF";
    std::string text = "0x";
    for (int shift = 28; shift >= 0; shift -= 4) {
        text.push_back(hex[(value >> shift) & 0x0f]);
    }
    return text;
}

int64_t filetimeToUnixMilliseconds(uint64_t value) {
    if (value < kWindowsToUnixEpoch100Nanoseconds) return 0;
    const uint64_t milliseconds =
        (value - kWindowsToUnixEpoch100Nanoseconds) / 10000ULL;
    return milliseconds > static_cast<uint64_t>(std::numeric_limits<int64_t>::max())
        ? std::numeric_limits<int64_t>::max()
        : static_cast<int64_t>(milliseconds);
}

bool appendUtf8(uint32_t codePoint, std::string& output) {
    if (codePoint <= 0x7f) {
        output.push_back(static_cast<char>(codePoint));
    } else if (codePoint <= 0x7ff) {
        output.push_back(static_cast<char>(0xc0 | (codePoint >> 6)));
        output.push_back(static_cast<char>(0x80 | (codePoint & 0x3f)));
    } else if (codePoint <= 0xffff) {
        output.push_back(static_cast<char>(0xe0 | (codePoint >> 12)));
        output.push_back(static_cast<char>(0x80 | ((codePoint >> 6) & 0x3f)));
        output.push_back(static_cast<char>(0x80 | (codePoint & 0x3f)));
    } else if (codePoint <= 0x10ffff) {
        output.push_back(static_cast<char>(0xf0 | (codePoint >> 18)));
        output.push_back(static_cast<char>(0x80 | ((codePoint >> 12) & 0x3f)));
        output.push_back(static_cast<char>(0x80 | ((codePoint >> 6) & 0x3f)));
        output.push_back(static_cast<char>(0x80 | (codePoint & 0x3f)));
    } else {
        return false;
    }
    return true;
}

bool appendUtf16CodePoint(uint32_t codePoint, std::vector<uint16_t>& output) {
    if (codePoint > 0x10ffff || (codePoint >= 0xd800 && codePoint <= 0xdfff)) return false;
    if (codePoint <= 0xffff) {
        output.push_back(static_cast<uint16_t>(codePoint));
    } else {
        codePoint -= 0x10000;
        output.push_back(static_cast<uint16_t>(0xd800 + (codePoint >> 10)));
        output.push_back(static_cast<uint16_t>(0xdc00 + (codePoint & 0x3ff)));
    }
    return true;
}

bool utf8ToUtf16(const std::string& input, std::vector<uint16_t>& output) {
    output.clear();
    for (std::size_t index = 0; index < input.size();) {
        const uint8_t first = static_cast<uint8_t>(input[index]);
        uint32_t codePoint = 0;
        std::size_t count = 0;
        if (first < 0x80) {
            codePoint = first;
            count = 1;
        } else if ((first & 0xe0) == 0xc0) {
            codePoint = first & 0x1f;
            count = 2;
            if (codePoint == 0) return false;
        } else if ((first & 0xf0) == 0xe0) {
            codePoint = first & 0x0f;
            count = 3;
        } else if ((first & 0xf8) == 0xf0) {
            codePoint = first & 0x07;
            count = 4;
        } else {
            return false;
        }
        if (index + count > input.size()) return false;
        for (std::size_t offset = 1; offset < count; ++offset) {
            const uint8_t next = static_cast<uint8_t>(input[index + offset]);
            if ((next & 0xc0) != 0x80) return false;
            codePoint = (codePoint << 6) | (next & 0x3f);
        }
        if ((count == 2 && codePoint < 0x80) ||
            (count == 3 && codePoint < 0x800) ||
            (count == 4 && codePoint < 0x10000) ||
            !appendUtf16CodePoint(codePoint, output)) return false;
        index += count;
    }
    return true;
}

bool appendWireVariant(std::vector<uint8_t>& output, const SyncWriteValue& value,
                       uint32_t& referent, std::string& error) {
    align(output, 8);
    appendU32(output, 16);  // wireVARIANT clSize (sizeof VARIANT)
    appendU32(output, 0);   // rpcReserved
    appendU16(output, value.vartype);
    appendU16(output, 0);
    appendU32(output, 0);
    appendU32(output, value.vartype);  // union discriminator
    switch (value.vartype) {
        case 0:  // VT_EMPTY
        case 1:  // VT_NULL
            return true;
        case 16:  // VT_I1
            output.push_back(static_cast<uint8_t>(value.signedValue));
            return true;
        case 17:  // VT_UI1
            output.push_back(static_cast<uint8_t>(value.unsignedValue));
            return true;
        case 2:  // VT_I2
            appendU16(output, static_cast<uint16_t>(value.signedValue));
            return true;
        case 11:  // VT_BOOL
            appendU16(output, value.signedValue == 0 ? 0 : 0xffff);
            return true;
        case 18:  // VT_UI2
            appendU16(output, static_cast<uint16_t>(value.unsignedValue));
            return true;
        case 3:   // VT_I4
        case 22:  // VT_INT
            appendU32(output, static_cast<uint32_t>(value.signedValue));
            return true;
        case 19:  // VT_UI4
        case 23:  // VT_UINT
            appendU32(output, static_cast<uint32_t>(value.unsignedValue));
            return true;
        case 4: {  // VT_R4
            const float converted = static_cast<float>(value.floatingValue);
            uint32_t bits = 0;
            std::memcpy(&bits, &converted, sizeof(bits));
            appendU32(output, bits);
            return true;
        }
        case 6:  // VT_CY
            align(output, 8);
            appendU64(output, static_cast<uint64_t>(value.signedValue));
            return true;
        case 20:  // VT_I8
            align(output, 8);
            appendU64(output, static_cast<uint64_t>(value.signedValue));
            return true;
        case 21:  // VT_UI8
            align(output, 8);
            appendU64(output, value.unsignedValue);
            return true;
        case 5: {  // VT_R8
            align(output, 8);
            uint64_t bits = 0;
            std::memcpy(&bits, &value.floatingValue, sizeof(bits));
            appendU64(output, bits);
            return true;
        }
        case 7: {  // VT_DATE
            align(output, 8);
            const double date = static_cast<double>(value.datetimeUnixMilliseconds) /
                                    kMillisecondsPerDay +
                                kOleDateUnixEpochDays;
            uint64_t bits = 0;
            std::memcpy(&bits, &date, sizeof(bits));
            appendU64(output, bits);
            return true;
        }
        case 8: {  // VT_BSTR
            std::vector<uint16_t> units;
            if (!utf8ToUtf16(value.stringValue, units) || units.size() > kMaximumStringUnits) {
                error = "SyncWrite contains an invalid or oversized UTF-8 string";
                return false;
            }
            appendU32(output, referent);
            referent += 4;
            appendU32(output, static_cast<uint32_t>(units.size()));
            appendU32(output, static_cast<uint32_t>(units.size() * 2));
            appendU32(output, static_cast<uint32_t>(units.size()));
            for (uint16_t unit : units) appendU16(output, unit);
            align(output, 4);
            return true;
        }
        default:
            error = "SyncWrite does not support VARIANT type " +
                    std::to_string(value.vartype);
            return false;
    }
}

bool decodeBstrReferent(const std::vector<uint8_t>& stub, std::size_t& offset,
                        std::string& value, std::string& error) {
    value.clear();
    if (offset + 12 > stub.size()) {
        error = "VARIANT BSTR referent is truncated";
        return false;
    }
    const uint32_t maximum = readU32(stub.data() + offset);
    const uint32_t byteCount = readU32(stub.data() + offset + 4);
    const uint32_t characterCount = readU32(stub.data() + offset + 8);
    offset += 12;
    if (maximum != characterCount || maximum > kMaximumStringUnits ||
        (byteCount & 1U) != 0 || byteCount > characterCount * 2ULL ||
        offset + static_cast<std::size_t>(characterCount) * 2 > stub.size()) {
        error = "VARIANT BSTR bounds are invalid";
        return false;
    }
    const uint32_t valueUnits = byteCount / 2;
    for (uint32_t index = 0; index < valueUnits; ++index) {
        uint32_t codePoint = readU16(
            stub.data() + offset + static_cast<std::size_t>(index) * 2);
        if (codePoint >= 0xd800 && codePoint <= 0xdbff) {
            if (++index >= valueUnits) {
                error = "VARIANT BSTR ends inside a surrogate pair";
                return false;
            }
            const uint16_t low = readU16(
                stub.data() + offset + static_cast<std::size_t>(index) * 2);
            if (low < 0xdc00 || low > 0xdfff) {
                error = "VARIANT BSTR contains an invalid surrogate pair";
                return false;
            }
            codePoint = 0x10000 + ((codePoint - 0xd800) << 10) + (low - 0xdc00);
        } else if (codePoint >= 0xdc00 && codePoint <= 0xdfff) {
            error = "VARIANT BSTR contains an unpaired low surrogate";
            return false;
        }
        if (!appendUtf8(codePoint, value)) {
            error = "VARIANT BSTR contains an invalid Unicode code point";
            return false;
        }
    }
    offset = align4(offset + static_cast<std::size_t>(characterCount) * 2);
    return true;
}

bool decodeBstr(const std::vector<uint8_t>& stub, std::size_t& offset,
                std::string& value, std::string& error) {
    value.clear();
    if (offset + 4 > stub.size()) {
        error = "VARIANT BSTR pointer is truncated";
        return false;
    }
    const uint32_t pointer = readU32(stub.data() + offset);
    offset += 4;
    if (pointer == 0) return true;
    return decodeBstrReferent(stub, offset, value, error);
}

bool decodeSafeArray(const std::vector<uint8_t>& stub, std::size_t& offset,
                     uint16_t elementVartype, SyncReadValue& value,
                     std::string& error) {
    // A wire VARIANT contains wirePSAFEARRAY (SAFEARRAY **), hence the two
    // non-null NDR pointer referents before the conformant SAFEARRAY body.
    if (offset + 8 > stub.size()) {
        error = "SAFEARRAY pointer chain is truncated";
        return false;
    }
    const uint32_t outerPointer = readU32(stub.data() + offset);
    const uint32_t arrayPointer = readU32(stub.data() + offset + 4);
    offset += 8;
    if (outerPointer == 0 || arrayPointer == 0) {
        if (elementVartype == 5) {
            value.kind = SyncValueKind::FloatingPointArray;
            value.floatingArrayValue.clear();
        } else {
            value.kind = SyncValueKind::StringArray;
            value.stringArrayValue.clear();
        }
        return true;
    }
    if (offset + 28 > stub.size()) {
        error = "SAFEARRAY descriptor is truncated";
        return false;
    }
    const uint32_t conformantDimensions = readU32(stub.data() + offset);
    const uint16_t dimensions = readU16(stub.data() + offset + 4);
    const uint16_t features = readU16(stub.data() + offset + 6);
    const uint32_t elementBytes = readU32(stub.data() + offset + 8);
    const uint32_t locksAndVartype = readU32(stub.data() + offset + 12);
    const uint32_t safeArrayType = readU32(stub.data() + offset + 16);
    const uint32_t declaredElements = readU32(stub.data() + offset + 20);
    const uint32_t dataPointer = readU32(stub.data() + offset + 24);
    offset += 28;
    if (dimensions == 0 || dimensions != conformantDimensions ||
        dimensions > kMaximumArrayDimensions) {
        error = "SAFEARRAY dimensions are invalid";
        return false;
    }
    const bool haveVartype = (features & 0x0080U) != 0;
    const uint16_t descriptorVartype = static_cast<uint16_t>(locksAndVartype >> 16);
    if (haveVartype && descriptorVartype != elementVartype) {
        error = "SAFEARRAY descriptor VARTYPE does not match VARIANT VARTYPE";
        return false;
    }
    const uint32_t expectedSafeArrayType = elementVartype == 5 ? 20U : 8U;
    const uint32_t expectedElementBytes = elementVartype == 5 ? 8U : 4U;
    if (safeArrayType != expectedSafeArrayType ||
        elementBytes != expectedElementBytes) {
        error = "SAFEARRAY element descriptor is incompatible with VT_" +
            std::string(elementVartype == 5 ? "R8" : "BSTR");
        return false;
    }
    if (elementVartype == 8 && (features & (0x0100U | 0x0080U)) == 0) {
        error = "VT_BSTR SAFEARRAY is missing FADF_BSTR/FADF_HAVEVARTYPE";
        return false;
    }
    if (declaredElements > kMaximumArrayElements ||
        offset + static_cast<std::size_t>(dimensions) * 8 > stub.size()) {
        error = "SAFEARRAY bounds are invalid or oversized";
        return false;
    }
    uint64_t boundedElements = 1;
    for (uint16_t index = 0; index < dimensions; ++index) {
        const uint32_t dimensionElements = readU32(stub.data() + offset);
        offset += 8;  // lLbound is intentionally preserved only on the wire.
        // A SAFEARRAY may legally be empty. In that case at least one bound
        // has cElements == 0, the descriptor's declared element count is zero,
        // and the data pointer/conformance count are also zero.
        if (dimensionElements == 0) {
            boundedElements = 0;
            continue;
        }
        if (boundedElements != 0 &&
            boundedElements > kMaximumArrayElements / dimensionElements) {
            error = "SAFEARRAY dimension product is invalid or oversized";
            return false;
        }
        boundedElements *= dimensionElements;
    }
    if (boundedElements != declaredElements) {
        error = "SAFEARRAY bounds do not match its declared element count";
        return false;
    }
    if (offset + 4 > stub.size()) {
        error = "SAFEARRAY data conformance count is truncated";
        return false;
    }
    const uint32_t transmittedElements = readU32(stub.data() + offset);
    offset += 4;
    if (transmittedElements != declaredElements ||
        (transmittedElements != 0 && dataPointer == 0)) {
        error = "SAFEARRAY transmitted element count or data pointer is invalid";
        return false;
    }
    if (elementVartype == 5) {
        offset = align8(offset);
        const std::size_t byteCount =
            static_cast<std::size_t>(transmittedElements) * sizeof(uint64_t);
        if (offset + byteCount > stub.size()) {
            error = "VT_ARRAY | VT_R8 data is truncated";
            return false;
        }
        value.kind = SyncValueKind::FloatingPointArray;
        value.floatingArrayValue.resize(transmittedElements);
        for (uint32_t index = 0; index < transmittedElements; ++index) {
            const uint64_t bits = readU64(
                stub.data() + offset + static_cast<std::size_t>(index) * 8);
            std::memcpy(&value.floatingArrayValue[index], &bits, sizeof(bits));
        }
        offset += byteCount;
        return true;
    }

    const std::size_t pointerBytes =
        static_cast<std::size_t>(transmittedElements) * sizeof(uint32_t);
    if (offset + pointerBytes > stub.size()) {
        error = "VT_ARRAY | VT_BSTR pointer table is truncated";
        return false;
    }
    std::vector<uint32_t> stringPointers(transmittedElements);
    for (uint32_t index = 0; index < transmittedElements; ++index) {
        stringPointers[index] = readU32(
            stub.data() + offset + static_cast<std::size_t>(index) * 4);
    }
    offset += pointerBytes;
    value.kind = SyncValueKind::StringArray;
    value.stringArrayValue.resize(transmittedElements);
    for (uint32_t index = 0; index < transmittedElements; ++index) {
        if (stringPointers[index] == 0) continue;
        if (!decodeBstrReferent(stub, offset, value.stringArrayValue[index], error)) {
            error = "VT_ARRAY | VT_BSTR element " + std::to_string(index) +
                ": " + error;
            return false;
        }
    }
    return true;
}

bool decodeVariantInternal(const std::vector<uint8_t>& stub, std::size_t& offset,
                           SyncReadValue& value, std::string& error) {
    offset = align8(offset);
    if (offset + 20 > stub.size()) {
        error = "wire VARIANT is truncated before its union value";
        return false;
    }
    const uint32_t clSize = readU32(stub.data() + offset);
    const uint32_t rpcReserved = readU32(stub.data() + offset + 4);
    const uint16_t vartype = readU16(stub.data() + offset + 8);
    const uint32_t discriminator = readU32(stub.data() + offset + 16);
    const bool discriminatorMatches = discriminator == vartype ||
        ((vartype & 0x2000U) != 0 && discriminator == 0x2000U);
    if (clSize > 16 * 1024 * 1024 || rpcReserved != 0 ||
        !discriminatorMatches) {
        error = "wire VARIANT header or discriminator is invalid at offset " +
            std::to_string(offset) + " (clSize=" + std::to_string(clSize) +
            ", rpcReserved=" + std::to_string(rpcReserved) +
            ", vartype=" + std::to_string(vartype) +
            ", discriminator=" + std::to_string(discriminator) + ")";
        return false;
    }
    value.vartype = vartype;
    offset += 20;
    switch (vartype) {
        case 0:  // VT_EMPTY
        case 1:  // VT_NULL
            value.kind = SyncValueKind::Empty;
            return true;
        case 16:  // VT_I1
            if (offset + 1 > stub.size()) break;
            value.kind = SyncValueKind::SignedInteger;
            value.signedValue = static_cast<int8_t>(stub[offset++]);
            return true;
        case 17:  // VT_UI1
            if (offset + 1 > stub.size()) break;
            value.kind = SyncValueKind::UnsignedInteger;
            value.unsignedValue = stub[offset++];
            return true;
        case 2:  // VT_I2
            if (offset + 2 > stub.size()) break;
            value.kind = SyncValueKind::SignedInteger;
            value.signedValue = static_cast<int16_t>(readU16(stub.data() + offset));
            offset += 2;
            return true;
        case 11:  // VT_BOOL
            if (offset + 2 > stub.size()) break;
            value.kind = SyncValueKind::Boolean;
            value.signedValue = readU16(stub.data() + offset) == 0 ? 0 : 1;
            offset += 2;
            return true;
        case 18:  // VT_UI2
            if (offset + 2 > stub.size()) break;
            value.kind = SyncValueKind::UnsignedInteger;
            value.unsignedValue = readU16(stub.data() + offset);
            offset += 2;
            return true;
        case 3:   // VT_I4
        case 10:  // VT_ERROR
        case 22:  // VT_INT
            if (offset + 4 > stub.size()) break;
            value.kind = SyncValueKind::SignedInteger;
            value.signedValue = static_cast<int32_t>(readU32(stub.data() + offset));
            offset += 4;
            return true;
        case 19:  // VT_UI4
        case 23:  // VT_UINT
            if (offset + 4 > stub.size()) break;
            value.kind = SyncValueKind::UnsignedInteger;
            value.unsignedValue = readU32(stub.data() + offset);
            offset += 4;
            return true;
        case 4: {  // VT_R4
            if (offset + 4 > stub.size()) break;
            uint32_t bits = readU32(stub.data() + offset);
            float decoded = 0;
            std::memcpy(&decoded, &bits, sizeof(decoded));
            value.kind = SyncValueKind::FloatingPoint;
            value.floatingValue = decoded;
            offset += 4;
            return true;
        }
        case 20:  // VT_I8
            offset = align8(offset);
            if (offset + 8 > stub.size()) break;
            value.kind = SyncValueKind::SignedInteger;
            value.signedValue = static_cast<int64_t>(readU64(stub.data() + offset));
            offset += 8;
            return true;
        case 21:  // VT_UI8
            offset = align8(offset);
            if (offset + 8 > stub.size()) break;
            value.kind = SyncValueKind::UnsignedInteger;
            value.unsignedValue = readU64(stub.data() + offset);
            offset += 8;
            return true;
        case 5: {  // VT_R8
            offset = align8(offset);
            if (offset + 8 > stub.size()) break;
            const uint64_t bits = readU64(stub.data() + offset);
            std::memcpy(&value.floatingValue, &bits, sizeof(value.floatingValue));
            value.kind = SyncValueKind::FloatingPoint;
            offset += 8;
            return true;
        }
        case 6: {  // VT_CY
            offset = align8(offset);
            if (offset + 8 > stub.size()) break;
            const int64_t scaled = static_cast<int64_t>(readU64(stub.data() + offset));
            value.kind = SyncValueKind::FloatingPoint;
            value.floatingValue = static_cast<double>(scaled) / 10000.0;
            offset += 8;
            return true;
        }
        case 7: {  // VT_DATE
            offset = align8(offset);
            if (offset + 8 > stub.size()) break;
            const uint64_t bits = readU64(stub.data() + offset);
            double date = 0;
            std::memcpy(&date, &bits, sizeof(date));
            if (!std::isfinite(date)) {
                error = "wire VARIANT contains a non-finite VT_DATE";
                return false;
            }
            value.kind = SyncValueKind::DateTime;
            value.signedValue = static_cast<int64_t>(
                std::llround((date - kOleDateUnixEpochDays) * kMillisecondsPerDay));
            offset += 8;
            return true;
        }
        case 8:  // VT_BSTR
            value.kind = SyncValueKind::String;
            return decodeBstr(stub, offset, value.stringValue, error);
        case 0x2005:  // VT_ARRAY | VT_R8
            return decodeSafeArray(stub, offset, 5, value, error);
        case 0x2008:  // VT_ARRAY | VT_BSTR
            return decodeSafeArray(stub, offset, 8, value, error);
        default:
            error = "unsupported OPC DA VARIANT type " + std::to_string(vartype);
            return false;
    }
    error = "wire VARIANT " + std::to_string(vartype) + " value is truncated";
    return false;
}

}  // namespace

bool decodeWireVariant(const std::vector<uint8_t>& stub, std::size_t& offset,
                       SyncReadValue& value, std::string& error) {
    return decodeVariantInternal(stub, offset, value, error);
}

std::vector<uint8_t> encodeSyncReadRequest(
    uint32_t source, const std::vector<uint32_t>& serverHandles,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid) {
    std::vector<uint8_t> request = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    appendU32(request, source);
    appendU32(request, static_cast<uint32_t>(serverHandles.size()));
    appendU32(request, static_cast<uint32_t>(serverHandles.size()));
    for (uint32_t handle : serverHandles) appendU32(request, handle);
    return request;
}

bool decodeSyncReadResponse(
    const std::vector<uint8_t>& stub, std::size_t expectedCount,
    std::vector<SyncReadValue>& values, uint32_t& hresult, std::string& error) {
    values.clear();
    hresult = 0;
    if (expectedCount == 0 || expectedCount > kMaximumItems || stub.size() < 12) {
        error = "SyncRead response or expected item count is invalid";
        return false;
    }
    if ((readU32(stub.data()) & ~1U) != 0 || readU32(stub.data() + 4) != 0) {
        error = "SyncRead response contains unsupported ORPCTHAT data";
        return false;
    }
    std::size_t offset = 8;
    if (offset + 8 > stub.size() || readU32(stub.data() + offset) == 0) {
        error = "SyncRead returned a null or truncated OPCITEMSTATE array";
        return false;
    }
    offset += 4;
    const uint32_t count = readU32(stub.data() + offset);
    offset += 4;
    if (count != expectedCount || count > kMaximumItems ||
        offset + static_cast<std::size_t>(count) * 20 > stub.size()) {
        error = "SyncRead OPCITEMSTATE array bounds do not match the request";
        return false;
    }
    std::vector<uint32_t> variantPointers;
    variantPointers.reserve(count);
    values.reserve(count);
    for (uint32_t index = 0; index < count; ++index) {
        SyncReadValue value{};
        value.clientHandle = readU32(stub.data() + offset);
        value.timestampFiletime = readU64(stub.data() + offset + 4);
        value.timestampUnixMilliseconds =
            filetimeToUnixMilliseconds(value.timestampFiletime);
        value.quality = readU16(stub.data() + offset + 12);
        variantPointers.push_back(readU32(stub.data() + offset + 16));
        values.push_back(std::move(value));
        offset += 20;
    }
    for (uint32_t index = 0; index < count; ++index) {
        if (variantPointers[index] == 0) {
            error = "SyncRead returned a null wire VARIANT pointer";
            return false;
        }
        if (!decodeWireVariant(stub, offset, values[index], error)) {
            error = "SyncRead item " + std::to_string(index) + ": " + error;
            return false;
        }
    }
    offset = align4(offset);
    if (offset + 8 > stub.size() || readU32(stub.data() + offset) == 0) {
        error = "SyncRead returned a null or truncated per-item HRESULT array";
        return false;
    }
    offset += 4;
    const uint32_t errorCount = readU32(stub.data() + offset);
    offset += 4;
    if (errorCount != expectedCount || errorCount > kMaximumItems ||
        offset + static_cast<std::size_t>(errorCount) * 4 + 4 != stub.size()) {
        error = "SyncRead per-item HRESULT array bounds do not match the request";
        return false;
    }
    for (uint32_t index = 0; index < errorCount; ++index) {
        values[index].hresult = readU32(stub.data() + offset);
        offset += 4;
    }
    hresult = readU32(stub.data() + offset);
    if (failed(hresult)) {
        error = "IOPCSyncIO::Read returned HRESULT " + hexValue(hresult);
        return false;
    }
    return true;
}

bool orderSyncReadValuesByClientHandle(
    const std::vector<uint32_t>& requestedClientHandles,
    std::vector<SyncReadValue>& values, std::string& error) {
    if (requestedClientHandles.size() != values.size()) {
        error = "SyncRead response client handle count does not match the request";
        return false;
    }
    std::unordered_map<uint32_t, std::size_t> requestedIndexes;
    requestedIndexes.reserve(requestedClientHandles.size());
    for (std::size_t index = 0; index < requestedClientHandles.size(); ++index) {
        if (!requestedIndexes.emplace(requestedClientHandles[index], index).second) {
            error = "SyncRead request contains a duplicate client handle";
            return false;
        }
    }
    std::vector<SyncReadValue> ordered(values.size());
    std::vector<bool> received(values.size(), false);
    std::vector<std::size_t> zeroHandleIndexes;
    for (std::size_t responseIndex = 0; responseIndex < values.size();
         ++responseIndex) {
        SyncReadValue& value = values[responseIndex];
        if (value.clientHandle == 0) {
            zeroHandleIndexes.push_back(responseIndex);
            continue;
        }
        const auto found = requestedIndexes.find(value.clientHandle);
        if (found == requestedIndexes.end()) {
            error = "SyncRead response contains an unexpected client handle " +
                std::to_string(value.clientHandle);
            return false;
        }
        if (received[found->second]) {
            error = "SyncRead response contains a duplicate client handle " +
                std::to_string(value.clientHandle);
            return false;
        }
        received[found->second] = true;
        ordered[found->second] = std::move(value);
    }
    // Some classic OPC DA servers return hClient=0 for empty arrays or
    // per-item failures even though the OPCITEMSTATE and HRESULT arrays remain
    // position-correlated with the request. Prefer that position when it is
    // still free, then use the only remaining slot. Normalize hClient so the
    // public API continues to identify the requested point.
    for (const std::size_t responseIndex : zeroHandleIndexes) {
        std::size_t requestedIndex = responseIndex;
        if (requestedIndex >= received.size() || received[requestedIndex]) {
            requestedIndex = received.size();
            for (std::size_t candidate = 0; candidate < received.size(); ++candidate) {
                if (!received[candidate]) {
                    requestedIndex = candidate;
                    break;
                }
            }
        }
        if (requestedIndex >= received.size()) {
            error = "SyncRead response contains more zero client handles than request slots";
            return false;
        }
        values[responseIndex].clientHandle =
            requestedClientHandles[requestedIndex];
        received[requestedIndex] = true;
        ordered[requestedIndex] = std::move(values[responseIndex]);
    }
    for (bool present : received) {
        if (!present) {
            error = "SyncRead response is missing a requested client handle";
            return false;
        }
    }
    values = std::move(ordered);
    return true;
}

bool encodeSyncWriteRequest(
    const std::vector<uint32_t>& serverHandles,
    const std::vector<SyncWriteValue>& values,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid,
    std::vector<uint8_t>& stub, std::string& error) {
    if (serverHandles.empty() || serverHandles.size() > kMaximumItems ||
        serverHandles.size() != values.size()) {
        error = "SyncWrite requires matching handle and value arrays between 1 and 10000 items";
        return false;
    }
    stub = dcom::encodeOrpcThis(comMajor, comMinor, cid);
    appendU32(stub, static_cast<uint32_t>(serverHandles.size()));
    appendU32(stub, static_cast<uint32_t>(serverHandles.size()));
    for (uint32_t handle : serverHandles) appendU32(stub, handle);
    appendU32(stub, static_cast<uint32_t>(values.size()));
    uint32_t referent = 0x00020000;
    for (std::size_t index = 0; index < values.size(); ++index) {
        appendU32(stub, referent);
        referent += 4;
    }
    for (const SyncWriteValue& value : values) {
        if (!appendWireVariant(stub, value, referent, error)) return false;
    }
    return true;
}

bool decodeSyncWriteResponse(
    const std::vector<uint8_t>& stub, std::size_t expectedCount,
    std::vector<uint32_t>& itemHresults, uint32_t& hresult,
    std::string& error) {
    itemHresults.clear();
    hresult = 0;
    if (expectedCount == 0 || expectedCount > kMaximumItems || stub.size() < 20) {
        error = "SyncWrite response or expected item count is invalid";
        return false;
    }
    if ((readU32(stub.data()) & ~1U) != 0 || readU32(stub.data() + 4) != 0) {
        error = "SyncWrite response contains unsupported ORPCTHAT data";
        return false;
    }
    std::size_t offset = 8;
    if (readU32(stub.data() + offset) == 0) {
        error = "SyncWrite returned a null per-item HRESULT array";
        return false;
    }
    offset += 4;
    const uint32_t count = readU32(stub.data() + offset);
    offset += 4;
    if (count != expectedCount || count > kMaximumItems ||
        offset + static_cast<std::size_t>(count) * 4 + 4 != stub.size()) {
        error = "SyncWrite per-item HRESULT array bounds do not match the request";
        return false;
    }
    itemHresults.reserve(count);
    for (uint32_t index = 0; index < count; ++index) {
        itemHresults.push_back(readU32(stub.data() + offset));
        offset += 4;
    }
    hresult = readU32(stub.data() + offset);
    if (failed(hresult)) {
        error = "IOPCSyncIO::Write returned HRESULT " + hexValue(hresult);
        return false;
    }
    return true;
}

}  // namespace wk::opcda::opcda
