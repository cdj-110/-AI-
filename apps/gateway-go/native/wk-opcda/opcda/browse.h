#ifndef WK_OPCDA_OPCDA_BROWSE_H
#define WK_OPCDA_OPCDA_BROWSE_H

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::opcda {

enum class BrowseOrganization : uint32_t {
    Hierarchical = 1,
    Flat = 2,
};

enum class BrowseDirection : uint32_t {
    Up = 1,
    Down = 2,
    To = 3,
};

enum class BrowseType : uint32_t {
    Branch = 1,
    Leaf = 2,
    Flat = 3,
};

std::array<uint8_t, 16> opcBrowseServerAddressSpaceIid();
std::array<uint8_t, 16> enumStringIid();

std::vector<uint8_t> encodeQueryOrganizationRequest(
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid);
bool decodeQueryOrganizationResponse(
    const std::vector<uint8_t>& stub, BrowseOrganization& organization,
    uint32_t& hresult, std::string& error);

bool encodeChangeBrowsePositionRequest(
    BrowseDirection direction, const std::string& position,
    uint16_t comMajor, uint16_t comMinor, const std::array<uint8_t, 16>& cid,
    std::vector<uint8_t>& stub, std::string& error);
bool decodeSimpleBrowseResponse(
    const std::vector<uint8_t>& stub, uint32_t& hresult, std::string& error);

bool encodeBrowseItemIdsRequest(
    BrowseType type, const std::string& filter, uint16_t dataTypeFilter,
    uint32_t accessRightsFilter, uint16_t comMajor, uint16_t comMinor,
    const std::array<uint8_t, 16>& cid, std::vector<uint8_t>& stub,
    std::string& error);
bool decodeBrowseEnumeratorResponse(
    const std::vector<uint8_t>& stub, std::vector<uint8_t>& objectReference,
    uint32_t& hresult, std::string& error);

std::vector<uint8_t> encodeEnumStringNextRequest(
    uint32_t count, uint16_t comMajor, uint16_t comMinor,
    const std::array<uint8_t, 16>& cid);
bool decodeEnumStringNextResponse(
    const std::vector<uint8_t>& stub, std::vector<std::string>& values,
    uint32_t& fetched, uint32_t& hresult, std::string& error);

bool encodeGetItemIdRequest(
    const std::string& name, uint16_t comMajor, uint16_t comMinor,
    const std::array<uint8_t, 16>& cid, std::vector<uint8_t>& stub,
    std::string& error);
bool decodeGetItemIdResponse(
    const std::vector<uint8_t>& stub, std::string& itemId,
    uint32_t& hresult, std::string& error);

}  // namespace wk::opcda::opcda

#endif
