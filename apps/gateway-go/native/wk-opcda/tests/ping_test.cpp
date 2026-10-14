#include "dcom/ping.h"
#include <cassert>

int main() {
    using namespace wk::opcda::dcom;
    // MS-DCOM NDR32: SETID, three ushorts, unique pointer, conformant count,
    // aligned 64-bit OIDs, then the NULL deletion-array pointer.
    const std::vector<uint8_t> golden = {
        0,0,0,0,0,0,0,0, 1,0, 2,0, 0,0, 0,0,
        0,0,2,0, 2,0,0,0,
        8,7,6,5,4,3,2,1, 0x18,0x17,0x16,0x15,0x14,0x13,0x12,0x11,
        0,0,0,0
    };
    assert(encodeComplexPing(0, 1, {0x0102030405060708ULL, 0x1112131415161718ULL}, {}) == golden);
    assert(encodeSimplePing(0x0102030405060708ULL) ==
        (std::vector<uint8_t>{8,7,6,5,4,3,2,1}));
    auto deletion = encodeComplexPing(5, 2, {}, {9});
    assert(deletion.size() == 40 && deletion[16] == 0 && deletion[20] == 0 && deletion[22] == 2);
    uint64_t setId = 0;
    std::string error;
    assert(decodePingResponse({8,7,6,5,4,3,2,1, 1,0,0,0, 0,0,0,0}, true, setId, error));
    assert(setId == 0x0102030405060708ULL);
    assert(decodePingResponse({0,0,0,0}, false, setId, error));
    assert(!decodePingResponse({5,0,0,0}, false, setId, error));
    assert(error.find("status=5") != std::string::npos);
    for (size_t n = 0; n < 16; ++n) {
        std::vector<uint8_t> truncated(n, 0);
        assert(!decodePingResponse(truncated, true, setId, error));
    }
    assert(!decodePingResponse(std::vector<uint8_t>(16, 0), true, setId, error));
    assert(!decodePingResponse(std::vector<uint8_t>(5, 0), false, setId, error));
}
