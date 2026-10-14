#ifndef WK_OPCDA_RPC_ENDPOINT_MAPPER_H
#define WK_OPCDA_RPC_ENDPOINT_MAPPER_H

#include "rpc/rpc_pdu.h"

#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::rpc {

struct EndpointMapResult {
    uint16_t tcpPort = 0;
    std::vector<uint8_t> tower;
    uint32_t status = 0;
};

std::vector<uint8_t> encodeTcpIpTower(const SyntaxId& target);
bool parseTcpPortFromTower(const std::vector<uint8_t>& tower, uint16_t& port, std::string& error);
std::vector<uint8_t> encodeEndpointMapRequest(const SyntaxId& target, uint32_t maximumTowers = 4);
bool decodeEndpointMapResponse(const std::vector<uint8_t>& stub, EndpointMapResult& result,
                               std::string& error);

SyntaxId remoteScmActivatorSyntax();

}  // namespace wk::opcda::rpc

#endif
