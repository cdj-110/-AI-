#ifndef WK_OPCDA_OPCDA_DATA_CALLBACK_H
#define WK_OPCDA_OPCDA_DATA_CALLBACK_H

#include "opcda/sync_io.h"

#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::opcda {

struct DataChange {
    uint32_t transactionId = 0;
    uint32_t clientGroupHandle = 0;
    uint32_t masterQuality = 0;
    uint32_t masterError = 0;
    std::vector<SyncReadValue> values;
};

bool decodeDataChangeRequest(const std::vector<uint8_t>& stub,
                             DataChange& change, std::string& error);

}  // namespace wk::opcda::opcda

#endif
