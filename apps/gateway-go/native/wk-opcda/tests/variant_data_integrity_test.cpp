#include "opcda/sync_io.h"

#include <cassert>
#include <cmath>
#include <cstdint>
#include <cstring>
#include <functional>
#include <limits>
#include <string>
#include <vector>

namespace {

using wk::opcda::opcda::SyncReadValue;
using wk::opcda::opcda::SyncValueKind;

void appendU16(std::vector<uint8_t>& output, uint16_t value) {
    output.push_back(static_cast<uint8_t>(value));
    output.push_back(static_cast<uint8_t>(value >> 8));
}

void appendU32(std::vector<uint8_t>& output, uint32_t value) {
    for (int shift = 0; shift < 32; shift += 8) {
        output.push_back(static_cast<uint8_t>(value >> shift));
    }
}

void appendU64(std::vector<uint8_t>& output, uint64_t value) {
    appendU32(output, static_cast<uint32_t>(value));
    appendU32(output, static_cast<uint32_t>(value >> 32));
}

void appendFloat32(std::vector<uint8_t>& output, float value) {
    uint32_t bits = 0;
    std::memcpy(&bits, &value, sizeof(bits));
    appendU32(output, bits);
}

void appendFloat64(std::vector<uint8_t>& output, double value) {
    uint64_t bits = 0;
    std::memcpy(&bits, &value, sizeof(bits));
    appendU64(output, bits);
}

void align(std::vector<uint8_t>& output, std::size_t boundary) {
    while (output.size() % boundary != 0) output.push_back(0);
}

struct VariantCase {
    uint16_t vartype;
    std::function<void(std::vector<uint8_t>&)> appendValue;
};

void appendVariant(std::vector<uint8_t>& output, const VariantCase& value) {
    align(output, 8);
    appendU32(output, 16);
    appendU32(output, 0);
    appendU16(output, value.vartype);
    appendU16(output, 0);
    appendU32(output, 0);
    appendU32(output, (value.vartype & 0x2000U) != 0 ? 0x2000U : value.vartype);
    value.appendValue(output);
}

VariantCase signed8(int8_t value) {
    return {16, [value](std::vector<uint8_t>& output) {
        output.push_back(static_cast<uint8_t>(value));
    }};
}

VariantCase unsigned8(uint8_t value) {
    return {17, [value](std::vector<uint8_t>& output) { output.push_back(value); }};
}

VariantCase signed16(int16_t value) {
    return {2, [value](std::vector<uint8_t>& output) {
        appendU16(output, static_cast<uint16_t>(value));
    }};
}

VariantCase unsigned16(uint16_t value) {
    return {18, [value](std::vector<uint8_t>& output) { appendU16(output, value); }};
}

VariantCase signed32(int32_t value) {
    return {3, [value](std::vector<uint8_t>& output) {
        appendU32(output, static_cast<uint32_t>(value));
    }};
}

VariantCase unsigned32(uint32_t value) {
    return {19, [value](std::vector<uint8_t>& output) { appendU32(output, value); }};
}

VariantCase signed64(int64_t value) {
    return {20, [value](std::vector<uint8_t>& output) {
        align(output, 8);
        appendU64(output, static_cast<uint64_t>(value));
    }};
}

VariantCase unsigned64(uint64_t value) {
    return {21, [value](std::vector<uint8_t>& output) {
        align(output, 8);
        appendU64(output, value);
    }};
}

VariantCase float32(float value) {
    return {4, [value](std::vector<uint8_t>& output) { appendFloat32(output, value); }};
}

VariantCase float64(double value) {
    return {5, [value](std::vector<uint8_t>& output) {
        align(output, 8);
        appendFloat64(output, value);
    }};
}

VariantCase boolean(bool value) {
    return {11, [value](std::vector<uint8_t>& output) {
        appendU16(output, value ? 0xffff : 0);
    }};
}

VariantCase date(double oleAutomationDate) {
    return {7, [oleAutomationDate](std::vector<uint8_t>& output) {
        align(output, 8);
        appendFloat64(output, oleAutomationDate);
    }};
}

VariantCase bstr(std::vector<uint16_t> units) {
    return {8, [units = std::move(units)](std::vector<uint8_t>& output) {
        appendU32(output, 0x00020000);
        appendU32(output, static_cast<uint32_t>(units.size()));
        appendU32(output, static_cast<uint32_t>(units.size() * 2));
        appendU32(output, static_cast<uint32_t>(units.size()));
        for (uint16_t unit : units) appendU16(output, unit);
        align(output, 4);
    }};
}

void appendSafeArrayDescriptor(std::vector<uint8_t>& output, uint16_t vartype,
                               uint16_t features, uint32_t elementBytes,
                               uint32_t safeArrayType, uint32_t elements) {
    appendU32(output, 0x00030000);  // wirePSAFEARRAY
    appendU32(output, 0x00030004);  // wireSAFEARRAY
    appendU32(output, 1);           // conformant rgsabound count
    appendU16(output, 1);           // cDims
    appendU16(output, features);
    appendU32(output, elementBytes);
    appendU32(output, static_cast<uint32_t>(vartype) << 16);  // cLocks + VT
    appendU32(output, safeArrayType);
    appendU32(output, elements);
    appendU32(output, elements == 0 ? 0 : 0x00030008);  // data pointer
    appendU32(output, elements);  // bound cElements
    appendU32(output, 0);         // bound lLbound
    appendU32(output, elements);  // data conformance count
}

VariantCase float64Array(std::vector<double> values) {
    return {static_cast<uint16_t>(0x2000 | 5),
        [values = std::move(values)](std::vector<uint8_t>& output) {
            appendSafeArrayDescriptor(output, 5, 0x0080, 8, 20,
                                      static_cast<uint32_t>(values.size()));
            align(output, 8);
            for (double value : values) appendFloat64(output, value);
        }};
}

VariantCase bstrArray(std::vector<std::vector<uint16_t>> values) {
    return {static_cast<uint16_t>(0x2000 | 8),
        [values = std::move(values)](std::vector<uint8_t>& output) {
            appendSafeArrayDescriptor(output, 8, 0x0180, 4, 8,
                                      static_cast<uint32_t>(values.size()));
            for (std::size_t index = 0; index < values.size(); ++index) {
                appendU32(output, static_cast<uint32_t>(0x00040000 + index * 4));
            }
            for (const auto& units : values) {
                appendU32(output, static_cast<uint32_t>(units.size()));
                appendU32(output, static_cast<uint32_t>(units.size() * 2));
                appendU32(output, static_cast<uint32_t>(units.size()));
                for (uint16_t unit : units) appendU16(output, unit);
                align(output, 4);
            }
        }};
}

std::vector<uint8_t> syncReadResponse(const std::vector<VariantCase>& variants) {
    std::vector<uint8_t> output;
    appendU32(output, 0);  // ORPCTHAT flags
    appendU32(output, 0);  // ORPCTHAT extensions
    appendU32(output, 0x00020000);
    appendU32(output, static_cast<uint32_t>(variants.size()));
    constexpr uint64_t sourceFiletime = 116444736000000000ULL + 123456789ULL * 10000ULL;
    for (std::size_t index = 0; index < variants.size(); ++index) {
        appendU32(output, static_cast<uint32_t>(index + 1));
        appendU64(output, sourceFiletime);
        appendU16(output, 0x00c0);
        appendU16(output, 0);
        appendU32(output, static_cast<uint32_t>(0x00020004 + index * 4));
    }
    for (const VariantCase& variant : variants) appendVariant(output, variant);
    align(output, 4);
    appendU32(output, 0x00030000);
    appendU32(output, static_cast<uint32_t>(variants.size()));
    for (std::size_t index = 0; index < variants.size(); ++index) appendU32(output, 0);
    appendU32(output, 0);
    return output;
}

}  // namespace

extern "C" int wk_run_variant_data_integrity_test() {
    const std::vector<VariantCase> variants = {
        signed8(-128), unsigned8(255),
        signed16(-32768), signed16(0), signed16(32767),
        unsigned16(0), unsigned16(65535),
        signed32(std::numeric_limits<int32_t>::min()),
        signed32(std::numeric_limits<int32_t>::max()),
        unsigned32(std::numeric_limits<uint32_t>::max()),
        signed64(std::numeric_limits<int64_t>::min()),
        signed64(std::numeric_limits<int64_t>::max()),
        unsigned64(std::numeric_limits<uint64_t>::max()),
        float32(123.25f), float64(-9876.5),
        boolean(true), boolean(false),
        date(25570.5),
        bstr({'A', 'B', 'C', '1', '2', '3'}),
        bstr({0x4e2d, 0x6587, 0x6d4b, 0x8bd5}),
        bstr({}),
        float64Array({-9876.5, 0.0, 123.25}),
        bstrArray({{'A', 'B', 'C'}, {0x4e2d, 0x6587}, {}}),
        float64Array({}),
        bstrArray({}),
    };
    std::vector<SyncReadValue> values;
    uint32_t hresult = 0xffffffff;
    std::string error;
    assert(wk::opcda::opcda::decodeSyncReadResponse(
        syncReadResponse(variants), variants.size(), values, hresult, error));
    assert(hresult == 0 && values.size() == variants.size());
    for (const SyncReadValue& value : values) {
        assert(value.quality == 0x00c0);
        assert(value.timestampUnixMilliseconds == 123456789);
        assert(value.hresult == 0);
    }
    assert(values[0].kind == SyncValueKind::SignedInteger && values[0].signedValue == -128);
    assert(values[1].kind == SyncValueKind::UnsignedInteger && values[1].unsignedValue == 255);
    assert(values[2].signedValue == -32768 && values[3].signedValue == 0 && values[4].signedValue == 32767);
    assert(values[5].unsignedValue == 0 && values[6].unsignedValue == 65535);
    assert(values[7].signedValue == std::numeric_limits<int32_t>::min());
    assert(values[8].signedValue == std::numeric_limits<int32_t>::max());
    assert(values[9].unsignedValue == 4294967295ULL);
    assert(values[10].signedValue == std::numeric_limits<int64_t>::min());
    assert(values[11].signedValue == std::numeric_limits<int64_t>::max());
    assert(values[12].unsignedValue == std::numeric_limits<uint64_t>::max());
    assert(values[13].kind == SyncValueKind::FloatingPoint && values[13].floatingValue == 123.25);
    assert(values[14].floatingValue == -9876.5);
    assert(values[15].kind == SyncValueKind::Boolean && values[15].signedValue == 1);
    assert(values[16].kind == SyncValueKind::Boolean && values[16].signedValue == 0);
    assert(values[17].kind == SyncValueKind::DateTime && values[17].signedValue == 129600000);
    assert(values[18].stringValue == "ABC123");
    assert(values[19].stringValue == "\xe4\xb8\xad\xe6\x96\x87\xe6\xb5\x8b\xe8\xaf\x95");
    assert(values[20].kind == SyncValueKind::String && values[20].stringValue.empty());
    assert(values[21].kind == SyncValueKind::FloatingPointArray);
    assert(values[21].floatingArrayValue == std::vector<double>({-9876.5, 0.0, 123.25}));
    assert(values[22].kind == SyncValueKind::StringArray);
    assert(values[22].stringArrayValue == std::vector<std::string>({
        "ABC", "\xe4\xb8\xad\xe6\x96\x87", ""}));
    assert(values[23].kind == SyncValueKind::FloatingPointArray);
    assert(values[23].floatingArrayValue.empty());
    assert(values[24].kind == SyncValueKind::StringArray);
    assert(values[24].stringArrayValue.empty());

    std::vector<SyncReadValue> reordered(3);
    reordered[0].clientHandle = 30;
    reordered[0].signedValue = 300;
    reordered[1].clientHandle = 10;
    reordered[1].signedValue = 100;
    reordered[2].clientHandle = 20;
    reordered[2].signedValue = 200;
    error.clear();
    assert(wk::opcda::opcda::orderSyncReadValuesByClientHandle(
        {10, 20, 30}, reordered, error));
    assert(reordered[0].clientHandle == 10 && reordered[0].signedValue == 100);
    assert(reordered[1].clientHandle == 20 && reordered[1].signedValue == 200);
    assert(reordered[2].clientHandle == 30 && reordered[2].signedValue == 300);

    reordered = std::vector<SyncReadValue>(3);
    reordered[0].clientHandle = 30;
    reordered[0].signedValue = 300;
    reordered[1].clientHandle = 0;
    reordered[1].signedValue = 200;
    reordered[2].clientHandle = 10;
    reordered[2].signedValue = 100;
    error.clear();
    assert(wk::opcda::opcda::orderSyncReadValuesByClientHandle(
        {10, 20, 30}, reordered, error));
    assert(reordered[0].clientHandle == 10 && reordered[0].signedValue == 100);
    assert(reordered[1].clientHandle == 20 && reordered[1].signedValue == 200);
    assert(reordered[2].clientHandle == 30 && reordered[2].signedValue == 300);

    const VariantCase unsupportedArray{static_cast<uint16_t>(0x2000 | 3),
        [](std::vector<uint8_t>&) {}};
    values.clear();
    error.clear();
    assert(!wk::opcda::opcda::decodeSyncReadResponse(
        syncReadResponse({unsupportedArray}), 1, values, hresult, error));
    assert(error.find("unsupported OPC DA VARIANT type 8195") != std::string::npos);
    return 0;
}

#ifndef WK_VARIANT_TEST_NO_MAIN
int main() {
    return wk_run_variant_data_integrity_test();
}
#endif
