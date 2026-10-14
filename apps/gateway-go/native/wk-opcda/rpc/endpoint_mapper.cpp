#include "rpc/endpoint_mapper.h"

#include "rpc/ndr.h"

#include <algorithm>
#include <array>
#include <utility>

namespace wk::opcda::rpc {
namespace {

constexpr uint8_t kUuidProtocolIdentifier = 0x0d;
constexpr uint8_t kConnectionOrientedRpcIdentifier = 0x0b;
constexpr uint8_t kTcpProtocolIdentifier = 0x07;
constexpr uint8_t kIpProtocolIdentifier = 0x09;
constexpr uint32_t kEndpointNotRegistered = 0x16c9a0d6;

void appendU16LE(std::vector<uint8_t>& output, uint16_t value) {
    output.push_back(static_cast<uint8_t>(value));
    output.push_back(static_cast<uint8_t>(value >> 8));
}

void appendFloor(std::vector<uint8_t>& tower, const std::vector<uint8_t>& left,
                 const std::vector<uint8_t>& right) {
    appendU16LE(tower, static_cast<uint16_t>(left.size()));
    tower.insert(tower.end(), left.begin(), left.end());
    appendU16LE(tower, static_cast<uint16_t>(right.size()));
    tower.insert(tower.end(), right.begin(), right.end());
}

std::vector<uint8_t> syntaxFloorLeft(const SyntaxId& syntax) {
    std::vector<uint8_t> value;
    value.reserve(19);
    value.push_back(kUuidProtocolIdentifier);
    value.insert(value.end(), syntax.uuid.begin(), syntax.uuid.end());
    appendU16LE(value, syntax.major);
    return value;
}

std::vector<uint8_t> versionRight(uint16_t minor) {
    return {static_cast<uint8_t>(minor), static_cast<uint8_t>(minor >> 8)};
}

bool readTower(ndr::Reader& reader, std::vector<uint8_t>& tower, std::string& error) {
    uint32_t maximumCount = 0;
    uint32_t towerLength = 0;
    if (!reader.readU32(maximumCount, error) || !reader.readU32(towerLength, error)) return false;
    if (towerLength > maximumCount || towerLength > reader.remaining()) {
        error = "Endpoint Mapper returned an invalid tower length";
        return false;
    }
    return reader.readBytes(towerLength, tower, error);
}

}  // namespace

std::vector<uint8_t> encodeTcpIpTower(const SyntaxId& target) {
    std::vector<uint8_t> tower;
    tower.reserve(80);
    appendU16LE(tower, 5);
    appendFloor(tower, syntaxFloorLeft(target), versionRight(target.minor));
    const SyntaxId transfer = ndr32Syntax();
    appendFloor(tower, syntaxFloorLeft(transfer), versionRight(transfer.minor));
    appendFloor(tower, {kConnectionOrientedRpcIdentifier}, {0, 0});
    appendFloor(tower, {kTcpProtocolIdentifier}, {0, 0});
    appendFloor(tower, {kIpProtocolIdentifier}, {0, 0, 0, 0});
    return tower;
}

bool parseTcpPortFromTower(const std::vector<uint8_t>& tower, uint16_t& port, std::string& error) {
    port = 0;
    if (tower.size() < 2) {
        error = "Endpoint Mapper tower is truncated before the floor count";
        return false;
    }
    std::size_t offset = 0;
    const auto readU16 = [&tower, &offset](uint16_t& value) {
        if (offset + 2 > tower.size()) return false;
        value = static_cast<uint16_t>(tower[offset]) |
                static_cast<uint16_t>(static_cast<uint16_t>(tower[offset + 1]) << 8);
        offset += 2;
        return true;
    };
    uint16_t floorCount = 0;
    if (!readU16(floorCount) || floorCount == 0 || floorCount > 32) {
        error = "Endpoint Mapper tower has an invalid floor count";
        return false;
    }
    for (uint16_t floor = 0; floor < floorCount; ++floor) {
        uint16_t leftLength = 0;
        if (!readU16(leftLength) || leftLength == 0 || offset + leftLength > tower.size()) {
            error = "Endpoint Mapper tower contains a truncated left-hand floor";
            return false;
        }
        const uint8_t protocol = tower[offset];
        offset += leftLength;
        uint16_t rightLength = 0;
        if (!readU16(rightLength) || offset + rightLength > tower.size()) {
            error = "Endpoint Mapper tower contains a truncated right-hand floor";
            return false;
        }
        if (protocol == kTcpProtocolIdentifier) {
            if (rightLength != 2) {
                error = "Endpoint Mapper TCP floor does not contain a 16-bit port";
                return false;
            }
            port = static_cast<uint16_t>(static_cast<uint16_t>(tower[offset]) << 8) |
                   static_cast<uint16_t>(tower[offset + 1]);
            if (port == 0) {
                error = "Endpoint Mapper returned TCP port zero";
                return false;
            }
            return true;
        }
        offset += rightLength;
    }
    error = "Endpoint Mapper response has no TCP floor";
    return false;
}

std::vector<uint8_t> encodeEndpointMapRequest(const SyntaxId& target, uint32_t maximumTowers) {
    const std::vector<uint8_t> tower = encodeTcpIpTower(target);
    ndr::Writer writer;
    writer.writePointer(false);  // optional object UUID
    writer.writePointer(true);   // map_tower
    // A procedure parameter is a top-level NDR value, so its pointed-to
    // twr_t follows that parameter's referent immediately. Deferring it until
    // after the later entry_handle/max_towers parameters makes Windows parse
    // those fields as the conformant-array header and reject the stub.
    writer.writeU32(static_cast<uint32_t>(tower.size()));
    writer.writeU32(static_cast<uint32_t>(tower.size()));
    writer.writeBytes(tower.data(), tower.size());

    writer.writeU32(0);  // context handle attributes
    const std::array<uint8_t, 16> nilUuid{};
    writer.writeUuid(nilUuid);
    writer.writeU32(std::min<uint32_t>(maximumTowers, 500));
    return writer.take();
}

bool decodeEndpointMapResponse(const std::vector<uint8_t>& stub, EndpointMapResult& result,
                               std::string& error) {
    result = EndpointMapResult{};
    ndr::Reader reader(stub);
    uint32_t contextAttributes = 0;
    std::array<uint8_t, 16> contextUuid{};
    uint32_t numberOfTowers = 0;
    if (!reader.readU32(contextAttributes, error) || !reader.readUuid(contextUuid, error) ||
        !reader.readU32(numberOfTowers, error)) {
        return false;
    }
    if (numberOfTowers > 500) {
        error = "Endpoint Mapper returned more than 500 towers";
        return false;
    }

    uint32_t maximumCount = 0;
    uint32_t arrayOffset = 0;
    uint32_t actualCount = 0;
    if (!reader.readConformantVaryingHeader(maximumCount, arrayOffset, actualCount, error)) return false;
    if (actualCount < numberOfTowers) {
        error = "Endpoint Mapper tower array is shorter than num_towers";
        return false;
    }
    std::vector<uint32_t> referents(actualCount);
    for (uint32_t& referent : referents) {
        if (!reader.readU32(referent, error)) return false;
    }
    std::vector<std::vector<uint8_t>> towers;
    towers.reserve(numberOfTowers);
    for (uint32_t index = 0; index < actualCount; ++index) {
        if (referents[index] == 0) continue;
        std::vector<uint8_t> tower;
        if (!readTower(reader, tower, error)) return false;
        if (index < numberOfTowers) towers.push_back(std::move(tower));
    }
    if (!reader.readU32(result.status, error)) return false;
    if (result.status == kEndpointNotRegistered) {
        error = "Endpoint Mapper did not find the requested RPC interface";
        return false;
    }
    if (result.status != 0) {
        error = "Endpoint Mapper ept_map failed with status 0x";
        constexpr char hex[] = "0123456789ABCDEF";
        for (int shift = 28; shift >= 0; shift -= 4) error.push_back(hex[(result.status >> shift) & 0x0f]);
        return false;
    }
    for (const std::vector<uint8_t>& tower : towers) {
        uint16_t port = 0;
        std::string towerError;
        if (parseTcpPortFromTower(tower, port, towerError)) {
            result.tcpPort = port;
            result.tower = tower;
            return true;
        }
    }
    error = "Endpoint Mapper returned no usable TCP/IP tower";
    return false;
}

SyntaxId remoteScmActivatorSyntax() {
    SyntaxId syntax{};
    std::string ignored;
    parseUuid("000001a0-0000-0000-c000-000000000046", syntax.uuid, ignored);
    syntax.major = 0;
    syntax.minor = 0;
    return syntax;
}

}  // namespace wk::opcda::rpc
