#ifndef WK_OPCDA_AUTH_NTLM_H
#define WK_OPCDA_AUTH_NTLM_H

#include "auth/crypto.h"

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::auth {

constexpr uint32_t kNtlmNegotiateSign = 0x00000010;
constexpr uint32_t kNtlmNegotiateSeal = 0x00000020;
constexpr uint32_t kNtlmNegotiateExtendedSessionSecurity = 0x00080000;
constexpr uint32_t kNtlmNegotiateKeyExchange = 0x40000000;

struct NtlmCredentials {
    std::string username;
    std::string password;
    std::string domain;
    std::string workstation;
};

struct NtlmChallenge {
    uint32_t flags = 0;
    std::array<uint8_t, 8> serverChallenge{};
    std::vector<uint8_t> targetInfo;
    std::vector<uint8_t> rawMessage;
    uint64_t timestamp = 0;
    bool hasTimestamp = false;
    bool micRequired = false;
    bool hasNbComputerName = false;
    bool hasNbDomainName = false;
};

struct NtlmClientNonce {
    std::array<uint8_t, 8> clientChallenge{};
    Digest16 exportedSessionKey{};
    uint64_t timestamp = 0;
};

struct NtlmAuthenticateResult {
    std::vector<uint8_t> message;
    uint32_t negotiatedFlags = 0;
    Digest16 exportedSessionKey{};
    Digest16 clientSigningKey{};
    Digest16 serverSigningKey{};
    Digest16 clientSealingKey{};
    Digest16 serverSealingKey{};
    bool keyExchange = false;
};

std::vector<uint8_t> encodeNtlmNegotiate(bool requestIntegrity = true,
                                         bool packetPrivacy = false);
bool parseNtlmChallenge(const std::vector<uint8_t>& token, bool requireIntegrity,
                        NtlmChallenge& challenge, std::string& error);
NtlmClientNonce randomNtlmClientNonce();
bool encodeNtlmAuthenticate(const NtlmCredentials& credentials,
                            const std::vector<uint8_t>& negotiateMessage,
                            const NtlmChallenge& challenge,
                            const NtlmClientNonce& nonce,
                            NtlmAuthenticateResult& result,
                            std::string& error);
bool encodeNtlmChallenge(const std::vector<uint8_t>& negotiateMessage,
                         const std::string& computerName,
                         const std::string& domainName,
                         NtlmChallenge& challenge,
                         std::string& error);
bool acceptNtlmAuthenticate(const NtlmCredentials& credentials,
                            const std::vector<uint8_t>& negotiateMessage,
                            const NtlmChallenge& challenge,
                            const std::vector<uint8_t>& authenticateMessage,
                            NtlmAuthenticateResult& result,
                            std::string& error);

class NtlmSignatureState {
public:
    bool initialize(const NtlmAuthenticateResult& result);
    std::array<uint8_t, 16> signClient(const uint8_t* message, std::size_t size);
    std::array<uint8_t, 16> sealClient(const uint8_t* message, std::size_t size,
                                       uint8_t* data, std::size_t dataSize);
    bool verifyServer(const uint8_t* message, std::size_t size,
                      const std::vector<uint8_t>& signature);
    bool unsealAndVerifyServer(uint8_t* data, std::size_t dataSize,
                               const uint8_t* message, std::size_t size,
                               const std::vector<uint8_t>& signature);
    bool verifyClient(const uint8_t* message, std::size_t size,
                      const std::vector<uint8_t>& signature);
    bool unsealAndVerifyClient(uint8_t* data, std::size_t dataSize,
                               const uint8_t* message, std::size_t size,
                               const std::vector<uint8_t>& signature);
    std::array<uint8_t, 16> signServer(const uint8_t* message, std::size_t size);
    std::array<uint8_t, 16> sealServer(const uint8_t* message, std::size_t size,
                                       uint8_t* data, std::size_t dataSize);

    // Used by deterministic protocol tests to emulate the server half of a
    // second context. Production code only calls signClient/verifyServer.
    std::array<uint8_t, 16> signServerForTest(const uint8_t* message, std::size_t size);
    std::array<uint8_t, 16> sealServerForTest(const uint8_t* message, std::size_t size,
                                              uint8_t* data, std::size_t dataSize);

private:
    std::array<uint8_t, 16> signature(const Digest16& signingKey, Rc4& sealing,
                                      uint32_t& sequence, const uint8_t* message,
                                      std::size_t size);
    std::array<uint8_t, 16> seal(const Digest16& signingKey, Rc4& sealing,
                                 uint32_t& sequence, const uint8_t* message,
                                 std::size_t size, uint8_t* data,
                                 std::size_t dataSize);
    Digest16 clientSigningKey_{};
    Digest16 serverSigningKey_{};
    Rc4 clientSealing_;
    Rc4 serverSealing_;
    uint32_t clientSequence_ = 0;
    uint32_t serverSequence_ = 0;
    bool keyExchange_ = false;
    bool initialized_ = false;
};

}  // namespace wk::opcda::auth

#endif
