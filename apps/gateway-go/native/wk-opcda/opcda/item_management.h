#ifndef WK_OPCDA_OPCDA_ITEM_MANAGEMENT_H
#define WK_OPCDA_OPCDA_ITEM_MANAGEMENT_H

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::opcda {

struct ItemDefinition {
    uint32_t clientHandle = 0;
    std::string itemId;
    std::string accessPath;
    bool active = true;
    uint16_t requestedDataType = 0;
};

struct ItemResult {
    uint32_t clientHandle = 0;
    uint32_t serverHandle = 0;
    uint16_t canonicalDataType = 0;
    uint32_t accessRights = 0;
    uint32_t hresult = 0;
};

bool vartypeFromName(const std::string& name, uint16_t& vartype);

bool encodeAddItemsRequest(
    const std::vector<ItemDefinition>& items,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid,
    std::vector<uint8_t>& stub, std::string& error);

bool decodeAddItemsResponse(
    const std::vector<uint8_t>& stub,
    const std::vector<ItemDefinition>& requestedItems,
    std::vector<ItemResult>& results, uint32_t& hresult, std::string& error);

}  // namespace wk::opcda::opcda

#endif
