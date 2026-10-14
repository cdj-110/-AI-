#ifndef WK_OPCDA_RPC_PDU_H
#define WK_OPCDA_RPC_PDU_H

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::auth { class NtlmSignatureState; }

namespace wk::opcda::rpc {

enum class PacketType : uint8_t {
    Request = 0,
    Response = 2,
    Fault = 3,
    Bind = 11,
    BindAck = 12,
    BindNak = 13,
    AlterContext = 14,
    AlterContextResponse = 15,
    Auth3 = 16,
};

struct SyntaxId {
    std::array<uint8_t, 16> uuid{};
    uint16_t major = 0;
    uint16_t minor = 0;
};

struct CommonHeader {
    PacketType packetType = PacketType::Fault;
    uint8_t flags = 0;
    uint16_t fragmentLength = 0;
    uint16_t authLength = 0;
    uint32_t callId = 0;
};

struct SecurityTrailer {
    uint8_t authenticationType = 0;
    uint8_t authenticationLevel = 0;
    uint8_t paddingLength = 0;
    uint32_t contextId = 0;
    std::vector<uint8_t> token;
};

struct RpcResponse {
    std::vector<uint8_t> stub;
    uint32_t faultStatus = 0;
    bool fault = false;
};

bool parseUuid(const std::string& text, std::array<uint8_t, 16>& wireUuid, std::string& error);
std::vector<uint8_t> encodeBind(uint32_t callId, uint16_t contextId, const SyntaxId& abstractSyntax);
std::vector<uint8_t> encodeAuthenticatedBind(uint32_t callId, uint16_t contextId,
                                             const SyntaxId& abstractSyntax,
                                             uint8_t authenticationLevel,
                                             uint32_t authenticationContextId,
                                             const std::vector<uint8_t>& token);
std::vector<uint8_t> encodeAlterContext(uint32_t callId, uint16_t contextId,
                                        const SyntaxId& abstractSyntax);
std::vector<std::vector<uint8_t>> encodeRequest(uint32_t callId, uint16_t contextId,
                                                uint16_t operationNumber,
                                                const std::vector<uint8_t>& stub,
                                                uint16_t maximumFragment = 4280);
std::vector<std::vector<uint8_t>> encodeObjectRequest(
    uint32_t callId, uint16_t contextId, uint16_t operationNumber,
    const std::array<uint8_t, 16>& objectUuid, const std::vector<uint8_t>& stub,
    uint16_t maximumFragment = 4280);
std::vector<uint8_t> encodeAuth3(uint32_t callId, uint8_t authenticationType,
                                 uint8_t authenticationLevel, uint32_t authenticationContextId,
                                 const std::vector<uint8_t>& token);
bool decodeCommonHeader(const uint8_t* data, std::size_t size, CommonHeader& header, std::string& error);
bool validateBindAck(const std::vector<uint8_t>& fragment, uint32_t expectedCallId,
                     std::string& error, uint8_t expectedResultCount = 1);
bool decodeSecurityTrailer(const std::vector<uint8_t>& fragment, const CommonHeader& header,
                           SecurityTrailer& trailer, std::size_t& bodyEnd, std::string& error);
bool decodeResponseFragments(const std::vector<std::vector<uint8_t>>& fragments,
                             uint32_t expectedCallId, uint16_t expectedContextId,
                             RpcResponse& response, std::string& error);
bool protectPacketIntegrity(std::vector<uint8_t>& fragment, uint32_t authenticationContextId,
                            auth::NtlmSignatureState& security, std::string& error,
                            bool packetPrivacy = false);
bool verifyPacketIntegrity(std::vector<uint8_t>& fragment,
                           uint32_t expectedAuthenticationContextId,
                           auth::NtlmSignatureState& security, std::string& error,
                           bool packetPrivacy = false);
bool protectPacketIntegrityServer(std::vector<uint8_t>& fragment,
                                  uint32_t authenticationContextId,
                                  auth::NtlmSignatureState& security,
                                  std::string& error,
                                  bool packetPrivacy = false,
                                  uint8_t authenticationType = 10);
bool verifyPacketIntegrityClient(std::vector<uint8_t>& fragment,
                                 uint32_t expectedAuthenticationContextId,
                                 auth::NtlmSignatureState& security,
                                 std::string& error,
                                 bool packetPrivacy = false,
                                 uint8_t authenticationType = 10);

SyntaxId endpointMapperSyntax();
SyntaxId objectExporterSyntax();
SyntaxId ndr32Syntax();

}  // namespace wk::opcda::rpc

#endif
