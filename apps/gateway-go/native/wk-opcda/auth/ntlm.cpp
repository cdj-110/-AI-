#include "auth/ntlm.h"

#include <algorithm>
#include <chrono>
#include <cstring>
#include <random>

namespace wk::opcda::auth {
namespace {

constexpr uint32_t kUnicode = 0x00000001;
constexpr uint32_t kRequestTarget = 0x00000004;
constexpr uint32_t kNtlm = 0x00000200;
constexpr uint32_t kAlwaysSign = 0x00008000;
constexpr uint32_t kTargetInfo = 0x00800000;
constexpr uint32_t kTargetTypeServer = 0x00020000;
constexpr uint32_t kNegotiateIdentify = 0x00100000;
constexpr uint32_t kNegotiateVersion = 0x02000000;
constexpr uint32_t kNegotiate128 = 0x20000000;
constexpr uint32_t kNegotiate56 = 0x80000000;
constexpr uint16_t kMsvAvEol = 0;
constexpr uint16_t kMsvAvNbComputerName = 1;
constexpr uint16_t kMsvAvNbDomainName = 2;
constexpr uint16_t kMsvAvDnsComputerName = 3;
constexpr uint16_t kMsvAvDnsDomainName = 4;
constexpr uint16_t kMsvAvFlags = 6;
constexpr uint16_t kMsvAvTimestamp = 7;
constexpr uint32_t kMicProvided = 0x00000002;

uint16_t readU16(const uint8_t* data) {
    return static_cast<uint16_t>(data[0]) | (static_cast<uint16_t>(data[1]) << 8);
}
uint32_t readU32(const uint8_t* data) {
    return static_cast<uint32_t>(data[0]) | (static_cast<uint32_t>(data[1]) << 8) |
           (static_cast<uint32_t>(data[2]) << 16) | (static_cast<uint32_t>(data[3]) << 24);
}
uint64_t readU64(const uint8_t* data) {
    uint64_t value = 0; for (int index = 0; index < 8; ++index) value |= static_cast<uint64_t>(data[index]) << (index * 8); return value;
}
void appendU16(std::vector<uint8_t>& output, uint16_t value) {
    output.push_back(static_cast<uint8_t>(value)); output.push_back(static_cast<uint8_t>(value >> 8));
}
void appendU32(std::vector<uint8_t>& output, uint32_t value) {
    for (int index = 0; index < 4; ++index) output.push_back(static_cast<uint8_t>(value >> (index * 8)));
}
void appendU64(std::vector<uint8_t>& output, uint64_t value) {
    for (int index = 0; index < 8; ++index) output.push_back(static_cast<uint8_t>(value >> (index * 8)));
}
void writeU16(std::vector<uint8_t>& output, std::size_t offset, uint16_t value) {
    output[offset] = static_cast<uint8_t>(value); output[offset + 1] = static_cast<uint8_t>(value >> 8);
}
void writeU32(std::vector<uint8_t>& output, std::size_t offset, uint32_t value) {
    for (int index = 0; index < 4; ++index) output[offset + static_cast<std::size_t>(index)] = static_cast<uint8_t>(value >> (index * 8));
}

bool hasSignature(const std::vector<uint8_t>& value, uint32_t type) {
    static constexpr uint8_t signature[8] = {'N','T','L','M','S','S','P',0};
    return value.size() >= 12 && std::equal(std::begin(signature), std::end(signature), value.begin()) && readU32(value.data() + 8) == type;
}

std::vector<uint8_t> utf16Le(const std::string& value, bool uppercaseAscii = false) {
    std::vector<uint8_t> output;
    for (std::size_t index = 0; index < value.size();) {
        uint32_t code = static_cast<uint8_t>(value[index++]);
        if ((code & 0x80) != 0) {
            int extra = (code & 0xe0) == 0xc0 ? 1 : (code & 0xf0) == 0xe0 ? 2 : (code & 0xf8) == 0xf0 ? 3 : -1;
            if (extra < 0 || index + static_cast<std::size_t>(extra) > value.size()) code = 0xfffd;
            else {
                code &= static_cast<uint32_t>((1 << (6 - extra)) - 1);
                for (int part = 0; part < extra; ++part) {
                    const uint8_t next = static_cast<uint8_t>(value[index++]);
                    if ((next & 0xc0) != 0x80) { code = 0xfffd; break; }
                    code = (code << 6) | (next & 0x3f);
                }
            }
        }
        if (uppercaseAscii && code >= 'a' && code <= 'z') code -= 'a' - 'A';
        const auto appendUnit = [&output](uint16_t unit) { output.push_back(static_cast<uint8_t>(unit)); output.push_back(static_cast<uint8_t>(unit >> 8)); };
        if (code <= 0xffff) appendUnit(static_cast<uint16_t>(code));
        else { code -= 0x10000; appendUnit(static_cast<uint16_t>(0xd800 + (code >> 10))); appendUnit(static_cast<uint16_t>(0xdc00 + (code & 0x3ff))); }
    }
    return output;
}

void writeSecurityBuffer(std::vector<uint8_t>& message, std::size_t fieldOffset,
                         const std::vector<uint8_t>& value, std::vector<uint8_t>& payload,
                         std::size_t payloadBase) {
    writeU16(message, fieldOffset, static_cast<uint16_t>(value.size()));
    writeU16(message, fieldOffset + 2, static_cast<uint16_t>(value.size()));
    writeU32(message, fieldOffset + 4, static_cast<uint32_t>(payloadBase + payload.size()));
    payload.insert(payload.end(), value.begin(), value.end());
}

Digest16 hashWithMagic(const Digest16& key, const char* magic) {
    std::vector<uint8_t> value(key.begin(), key.end());
    value.insert(value.end(), magic, magic + std::strlen(magic) + 1);
    return md5(value);
}

uint64_t currentFileTime() {
    constexpr uint64_t epochDeltaSeconds = 11644473600ULL;
    const auto now = std::chrono::system_clock::now().time_since_epoch();
    const uint64_t ticks = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(now).count() / 100);
    return ticks + epochDeltaSeconds * 10000000ULL;
}

template <std::size_t N>
void randomBytes(std::array<uint8_t, N>& output) {
    std::random_device source;
    for (uint8_t& value : output) value = static_cast<uint8_t>(source());
}

bool constantTimeEqual(const uint8_t* left, const uint8_t* right, std::size_t size) {
    uint8_t difference = 0; for (std::size_t index = 0; index < size; ++index) difference |= left[index] ^ right[index]; return difference == 0;
}

void uppercaseAsciiUtf16(std::vector<uint8_t>& value) {
    for (std::size_t index = 0; index + 1 < value.size(); index += 2) {
        if (value[index + 1] == 0 && value[index] >= 'a' && value[index] <= 'z') {
            value[index] = static_cast<uint8_t>(value[index] - 'a' + 'A');
        }
    }
}

}  // namespace

std::vector<uint8_t> encodeNtlmNegotiate(bool requestIntegrity, bool packetPrivacy) {
    uint32_t flags = kUnicode | kRequestTarget | kNtlm | kAlwaysSign |
                     kNtlmNegotiateExtendedSessionSecurity | kTargetInfo |
                     kNegotiate128 | kNegotiate56;
    if (requestIntegrity || packetPrivacy) flags |= kNtlmNegotiateSign | kNtlmNegotiateKeyExchange;
    if (packetPrivacy) flags |= kNtlmNegotiateSeal;
    std::vector<uint8_t> output = {'N','T','L','M','S','S','P',0};
    appendU32(output, 1); appendU32(output, flags);
    appendU16(output, 0); appendU16(output, 0); appendU32(output, 32);
    appendU16(output, 0); appendU16(output, 0); appendU32(output, 32);
    return output;
}

bool parseNtlmChallenge(const std::vector<uint8_t>& token, bool requireIntegrity,
                        NtlmChallenge& challenge, std::string& error) {
    challenge = NtlmChallenge{};
    if (!hasSignature(token, 2) || token.size() < 48) { error = "NTLM CHALLENGE_MESSAGE is malformed"; return false; }
    challenge.flags = readU32(token.data() + 20);
    std::copy(token.begin() + 24, token.begin() + 32, challenge.serverChallenge.begin());
    const uint16_t targetInfoLength = readU16(token.data() + 40);
    const uint32_t targetInfoOffset = readU32(token.data() + 44);
    if (targetInfoOffset > token.size() || targetInfoLength > token.size() - targetInfoOffset) {
        error = "NTLM CHALLENGE_MESSAGE TargetInfo is out of bounds"; return false;
    }
    challenge.targetInfo.assign(token.begin() + targetInfoOffset, token.begin() + targetInfoOffset + targetInfoLength);
    challenge.rawMessage = token;
    std::size_t offset = 0; bool terminated = false;
    while (offset + 4 <= challenge.targetInfo.size()) {
        const uint16_t id = readU16(challenge.targetInfo.data() + offset);
        const uint16_t length = readU16(challenge.targetInfo.data() + offset + 2); offset += 4;
        if (length > challenge.targetInfo.size() - offset) { error = "NTLM TargetInfo AV_PAIR is truncated"; return false; }
        if (id == kMsvAvEol) { if (length != 0) { error = "NTLM TargetInfo EOL has data"; return false; } terminated = true; break; }
        if (id == kMsvAvNbComputerName) challenge.hasNbComputerName = true;
        if (id == kMsvAvNbDomainName) challenge.hasNbDomainName = true;
        if (id == kMsvAvTimestamp && length == 8) { challenge.timestamp = readU64(challenge.targetInfo.data() + offset); challenge.hasTimestamp = true; }
        if (id == kMsvAvFlags && length == 4) challenge.micRequired = (readU32(challenge.targetInfo.data() + offset) & kMicProvided) != 0;
        offset += length;
    }
    if (!terminated) { error = "NTLM TargetInfo has no MsvAvEOL terminator"; return false; }
    const uint32_t required = kUnicode | kNtlm | kNtlmNegotiateExtendedSessionSecurity;
    if ((challenge.flags & required) != required) { error = "NTLM server did not negotiate Unicode, NTLM and extended session security"; return false; }
    if (requireIntegrity && (challenge.flags & kNtlmNegotiateSign) == 0) { error = "NTLM server did not negotiate message signing"; return false; }
    if (requireIntegrity && (!challenge.hasNbComputerName || !challenge.hasNbDomainName)) {
        error = "NTLMv2 integrity requires server TargetInfo computer and domain names"; return false;
    }
    return true;
}

NtlmClientNonce randomNtlmClientNonce() {
    NtlmClientNonce nonce{}; randomBytes(nonce.clientChallenge); randomBytes(nonce.exportedSessionKey); nonce.timestamp = currentFileTime(); return nonce;
}

bool encodeNtlmAuthenticate(const NtlmCredentials& credentials,
                            const std::vector<uint8_t>& negotiateMessage,
                            const NtlmChallenge& challenge,
                            const NtlmClientNonce& nonce,
                            NtlmAuthenticateResult& result,
                            std::string& error) {
    result = NtlmAuthenticateResult{};
    if (credentials.username.empty() || credentials.password.empty()) { error = "NTLM username and password are required"; return false; }
    const std::vector<uint8_t> password = utf16Le(credentials.password);
    const Digest16 ntHash = md4(password);
    std::vector<uint8_t> identity = utf16Le(credentials.username, true);
    const std::vector<uint8_t> domain = utf16Le(credentials.domain);
    identity.insert(identity.end(), domain.begin(), domain.end());
    const Digest16 responseKey = hmacMd5(ntHash.data(), ntHash.size(), identity.data(), identity.size());

    std::vector<uint8_t> temp = {1,1,0,0,0,0,0,0};
    appendU64(temp, challenge.hasTimestamp ? challenge.timestamp : nonce.timestamp);
    temp.insert(temp.end(), nonce.clientChallenge.begin(), nonce.clientChallenge.end());
    appendU32(temp, 0); temp.insert(temp.end(), challenge.targetInfo.begin(), challenge.targetInfo.end()); appendU32(temp, 0);
    std::vector<uint8_t> proofInput(challenge.serverChallenge.begin(), challenge.serverChallenge.end());
    proofInput.insert(proofInput.end(), temp.begin(), temp.end());
    const Digest16 proof = hmacMd5(responseKey.data(), responseKey.size(), proofInput.data(), proofInput.size());
    std::vector<uint8_t> ntResponse(proof.begin(), proof.end()); ntResponse.insert(ntResponse.end(), temp.begin(), temp.end());
    std::vector<uint8_t> lmResponse;
    if (challenge.hasTimestamp) lmResponse.assign(24, 0);
    else {
        std::vector<uint8_t> lmInput(challenge.serverChallenge.begin(), challenge.serverChallenge.end());
        lmInput.insert(lmInput.end(), nonce.clientChallenge.begin(), nonce.clientChallenge.end());
        const Digest16 lmProof = hmacMd5(responseKey.data(), responseKey.size(), lmInput.data(), lmInput.size());
        lmResponse.assign(lmProof.begin(), lmProof.end()); lmResponse.insert(lmResponse.end(), nonce.clientChallenge.begin(), nonce.clientChallenge.end());
    }
    const Digest16 sessionBaseKey = hmacMd5(responseKey.data(), responseKey.size(), proof.data(), proof.size());
    result.negotiatedFlags = challenge.flags & readU32(negotiateMessage.data() + 12);
    result.keyExchange = (result.negotiatedFlags & kNtlmNegotiateKeyExchange) != 0 &&
                         (result.negotiatedFlags & (kNtlmNegotiateSign | kNtlmNegotiateSeal)) != 0;
    std::vector<uint8_t> encryptedSessionKey;
    if (result.keyExchange) {
        result.exportedSessionKey = nonce.exportedSessionKey;
        Rc4 exchange(sessionBaseKey); encryptedSessionKey = exchange.processCopy(nonce.exportedSessionKey.data(), nonce.exportedSessionKey.size());
    } else result.exportedSessionKey = sessionBaseKey;

    result.clientSigningKey = hashWithMagic(result.exportedSessionKey, "session key to client-to-server signing key magic constant");
    result.serverSigningKey = hashWithMagic(result.exportedSessionKey, "session key to server-to-client signing key magic constant");
    result.clientSealingKey = hashWithMagic(result.exportedSessionKey, "session key to client-to-server sealing key magic constant");
    result.serverSealingKey = hashWithMagic(result.exportedSessionKey, "session key to server-to-client sealing key magic constant");

    const std::vector<uint8_t> user = utf16Le(credentials.username);
    const std::vector<uint8_t> workstation = utf16Le(credentials.workstation);
    const bool includeMic = challenge.micRequired;
    const std::size_t fixedSize = includeMic ? 80 : 64;
    result.message.assign(fixedSize, 0);
    const uint8_t signature[8] = {'N','T','L','M','S','S','P',0}; std::copy(std::begin(signature), std::end(signature), result.message.begin());
    writeU32(result.message, 8, 3); writeU32(result.message, 60, result.negotiatedFlags);
    std::vector<uint8_t> payload;
    writeSecurityBuffer(result.message, 12, lmResponse, payload, fixedSize);
    writeSecurityBuffer(result.message, 20, ntResponse, payload, fixedSize);
    writeSecurityBuffer(result.message, 28, domain, payload, fixedSize);
    writeSecurityBuffer(result.message, 36, user, payload, fixedSize);
    writeSecurityBuffer(result.message, 44, workstation, payload, fixedSize);
    writeSecurityBuffer(result.message, 52, encryptedSessionKey, payload, fixedSize);
    result.message.insert(result.message.end(), payload.begin(), payload.end());
    if (includeMic) {
        std::vector<uint8_t> micInput = negotiateMessage; micInput.insert(micInput.end(), challenge.rawMessage.begin(), challenge.rawMessage.end()); micInput.insert(micInput.end(), result.message.begin(), result.message.end());
        const Digest16 mic = hmacMd5(result.exportedSessionKey.data(), result.exportedSessionKey.size(), micInput.data(), micInput.size());
        std::copy(mic.begin(), mic.end(), result.message.begin() + 64);
    }
    return true;
}

bool encodeNtlmChallenge(const std::vector<uint8_t>& negotiateMessage,
                         const std::string& computerName,
                         const std::string& domainName,
                         NtlmChallenge& challenge,
                         std::string& error) {
    challenge = NtlmChallenge{};
    if (!hasSignature(negotiateMessage, 1) || negotiateMessage.size() < 16) {
        error = "NTLM NEGOTIATE_MESSAGE is malformed";
        return false;
    }
    const uint32_t supported = kUnicode | kRequestTarget | kNtlm | kAlwaysSign |
        kNtlmNegotiateExtendedSessionSecurity | kTargetInfo | kNegotiate128 |
        kNegotiate56 | kNtlmNegotiateSign | kNtlmNegotiateSeal |
        kNtlmNegotiateKeyExchange | kNegotiateIdentify | kNegotiateVersion;
    challenge.flags = readU32(negotiateMessage.data() + 12) & supported;
    challenge.flags |= kUnicode | kRequestTarget | kNtlm | kAlwaysSign |
        kNtlmNegotiateExtendedSessionSecurity | kTargetInfo |
        kTargetTypeServer;
    randomBytes(challenge.serverChallenge);
    challenge.timestamp = currentFileTime();
    challenge.hasTimestamp = true;
    challenge.hasNbComputerName = true;
    challenge.hasNbDomainName = true;

    const std::vector<uint8_t> computer = utf16Le(
        computerName.empty() ? "EG100" : computerName);
    const std::vector<uint8_t> domain = utf16Le(
        domainName.empty() ? "WORKGROUP" : domainName);
    appendU16(challenge.targetInfo, kMsvAvNbDomainName);
    appendU16(challenge.targetInfo, static_cast<uint16_t>(domain.size()));
    challenge.targetInfo.insert(challenge.targetInfo.end(), domain.begin(), domain.end());
    appendU16(challenge.targetInfo, kMsvAvNbComputerName);
    appendU16(challenge.targetInfo, static_cast<uint16_t>(computer.size()));
    challenge.targetInfo.insert(challenge.targetInfo.end(), computer.begin(), computer.end());
    // Windows RPC emits both NetBIOS and DNS identity AV pairs.  Some COM
    // clients hand the token to SSPI with target-name validation enabled and
    // abort before AUTH3 when the DNS pairs are absent.
    appendU16(challenge.targetInfo, kMsvAvDnsDomainName);
    appendU16(challenge.targetInfo, static_cast<uint16_t>(domain.size()));
    challenge.targetInfo.insert(challenge.targetInfo.end(), domain.begin(), domain.end());
    appendU16(challenge.targetInfo, kMsvAvDnsComputerName);
    appendU16(challenge.targetInfo, static_cast<uint16_t>(computer.size()));
    challenge.targetInfo.insert(challenge.targetInfo.end(), computer.begin(), computer.end());
    appendU16(challenge.targetInfo, kMsvAvTimestamp);
    appendU16(challenge.targetInfo, 8);
    appendU64(challenge.targetInfo, challenge.timestamp);
    appendU16(challenge.targetInfo, kMsvAvEol);
    appendU16(challenge.targetInfo, 0);

    const std::vector<uint8_t> targetName = computer;
    const bool includeVersion =
        (challenge.flags & kNegotiateVersion) != 0;
    const std::size_t fixedSize = includeVersion ? 56 : 48;
    challenge.rawMessage.assign(fixedSize, 0);
    const uint8_t signature[8] = {'N','T','L','M','S','S','P',0};
    std::copy(std::begin(signature), std::end(signature), challenge.rawMessage.begin());
    writeU32(challenge.rawMessage, 8, 2);
    writeU32(challenge.rawMessage, 20, challenge.flags);
    std::copy(challenge.serverChallenge.begin(), challenge.serverChallenge.end(),
              challenge.rawMessage.begin() + 24);
    if (includeVersion) {
        // Windows 6.1 / build 7601, NTLM revision 15.  Version is capability
        // metadata only; using the Windows RPC value maximizes SSPI
        // interoperability with legacy OPC DA servers.
        const uint8_t version[8] = {6, 1, 0xb1, 0x1d, 0, 0, 0, 0x0f};
        std::copy(std::begin(version), std::end(version),
                  challenge.rawMessage.begin() + 48);
    }
    std::vector<uint8_t> payload;
    writeSecurityBuffer(challenge.rawMessage, 12, targetName, payload, fixedSize);
    writeSecurityBuffer(challenge.rawMessage, 40, challenge.targetInfo, payload, fixedSize);
    challenge.rawMessage.insert(challenge.rawMessage.end(), payload.begin(), payload.end());
    return true;
}

bool acceptNtlmAuthenticate(const NtlmCredentials& credentials,
                            const std::vector<uint8_t>& negotiateMessage,
                            const NtlmChallenge& challenge,
                            const std::vector<uint8_t>& authenticateMessage,
                            NtlmAuthenticateResult& result,
                            std::string& error) {
    result = NtlmAuthenticateResult{};
    if (!hasSignature(authenticateMessage, 3) || authenticateMessage.size() < 64 ||
        credentials.username.empty() || credentials.password.empty()) {
        error = "NTLM AUTHENTICATE_MESSAGE or callback credentials are invalid";
        return false;
    }
    const auto readBuffer = [&](std::size_t field, std::vector<uint8_t>& value) {
        const uint16_t length = readU16(authenticateMessage.data() + field);
        const uint32_t offset = readU32(authenticateMessage.data() + field + 4);
        if (offset > authenticateMessage.size() ||
            length > authenticateMessage.size() - offset) return false;
        value.assign(authenticateMessage.begin() + offset,
                     authenticateMessage.begin() + offset + length);
        return true;
    };
    std::vector<uint8_t> ntResponse;
    std::vector<uint8_t> encryptedSessionKey;
    std::vector<uint8_t> authenticateDomain;
    std::vector<uint8_t> authenticateUser;
    const bool ntResponseValid = readBuffer(20, ntResponse);
    const bool sessionKeyValid = readBuffer(52, encryptedSessionKey);
    const bool domainValid = readBuffer(28, authenticateDomain);
    const bool userValid = readBuffer(36, authenticateUser);
    if (!ntResponseValid || !sessionKeyValid || !domainValid || !userValid ||
        ntResponse.size() < 16 + 28 || authenticateUser.empty()) {
        const uint16_t ntLength = readU16(authenticateMessage.data() + 20);
        const uint32_t ntOffset = readU32(authenticateMessage.data() + 24);
        const uint16_t keyLength = readU16(authenticateMessage.data() + 52);
        const uint32_t keyOffset = readU32(authenticateMessage.data() + 56);
        error = "NTLMv2 response buffers are malformed: message=" +
            std::to_string(authenticateMessage.size()) +
            ", ntLength=" + std::to_string(ntLength) +
            ", ntOffset=" + std::to_string(ntOffset) +
            ", keyLength=" + std::to_string(keyLength) +
            ", keyOffset=" + std::to_string(keyOffset) +
            ", flags=" +
            std::to_string(readU32(authenticateMessage.data() + 60));
        return false;
    }
    std::vector<uint8_t> configuredUser = utf16Le(credentials.username);
    uppercaseAsciiUtf16(configuredUser);
    uppercaseAsciiUtf16(authenticateUser);
    if (authenticateUser != configuredUser) {
        error = "NTLM callback username does not match the configured OPC account";
        return false;
    }
    const std::vector<uint8_t> password = utf16Le(credentials.password);
    const Digest16 ntHash = md4(password);
    std::vector<uint8_t> identity = std::move(authenticateUser);
    if (authenticateDomain.empty()) {
        authenticateDomain = utf16Le(credentials.domain);
    }
    identity.insert(identity.end(), authenticateDomain.begin(),
                    authenticateDomain.end());
    const Digest16 responseKey = hmacMd5(
        ntHash.data(), ntHash.size(), identity.data(), identity.size());
    std::vector<uint8_t> proofInput(
        challenge.serverChallenge.begin(), challenge.serverChallenge.end());
    proofInput.insert(proofInput.end(), ntResponse.begin() + 16, ntResponse.end());
    const Digest16 expectedProof = hmacMd5(
        responseKey.data(), responseKey.size(), proofInput.data(), proofInput.size());
    if (!constantTimeEqual(expectedProof.data(), ntResponse.data(), expectedProof.size())) {
        error = "NTLMv2 callback authentication proof is invalid";
        return false;
    }
    const Digest16 sessionBaseKey = hmacMd5(
        responseKey.data(), responseKey.size(), ntResponse.data(), 16);
    result.negotiatedFlags = challenge.flags & readU32(authenticateMessage.data() + 60);
    result.keyExchange = (result.negotiatedFlags & kNtlmNegotiateKeyExchange) != 0 &&
        (result.negotiatedFlags & (kNtlmNegotiateSign | kNtlmNegotiateSeal)) != 0;
    if (result.keyExchange) {
        if (encryptedSessionKey.size() != 16) {
            error = "NTLM callback encrypted session key is missing";
            return false;
        }
        Rc4 exchange(sessionBaseKey);
        const auto exported = exchange.processCopy(
            encryptedSessionKey.data(), encryptedSessionKey.size());
        std::copy(exported.begin(), exported.end(), result.exportedSessionKey.begin());
    } else {
        result.exportedSessionKey = sessionBaseKey;
    }
    result.clientSigningKey = hashWithMagic(result.exportedSessionKey,
        "session key to client-to-server signing key magic constant");
    result.serverSigningKey = hashWithMagic(result.exportedSessionKey,
        "session key to server-to-client signing key magic constant");
    result.clientSealingKey = hashWithMagic(result.exportedSessionKey,
        "session key to client-to-server sealing key magic constant");
    result.serverSealingKey = hashWithMagic(result.exportedSessionKey,
        "session key to server-to-client sealing key magic constant");
    return true;
}

bool NtlmSignatureState::initialize(const NtlmAuthenticateResult& result) {
    clientSigningKey_ = result.clientSigningKey;
    serverSigningKey_ = result.serverSigningKey;
    keyExchange_ = result.keyExchange;
    clientSequence_ = 0;
    serverSequence_ = 0;
    initialized_ = clientSealing_.initialize(
        result.clientSealingKey.data(), result.clientSealingKey.size()) &&
        serverSealing_.initialize(
            result.serverSealingKey.data(), result.serverSealingKey.size());
    return initialized_;
}

std::array<uint8_t, 16> NtlmSignatureState::signature(
    const Digest16& signingKey, Rc4& sealing, uint32_t& sequence,
    const uint8_t* message, std::size_t size) {
    std::vector<uint8_t> input;
    appendU32(input, sequence);
    if (size != 0) input.insert(input.end(), message, message + size);
    Digest16 checksum = hmacMd5(
        signingKey.data(), signingKey.size(), input.data(), input.size());
    if (keyExchange_) sealing.process(checksum.data(), 8);
    std::array<uint8_t, 16> output{};
    output[0] = 1;
    std::copy(checksum.begin(), checksum.begin() + 8, output.begin() + 4);
    for (int index = 0; index < 4; ++index) {
        output[12 + index] = static_cast<uint8_t>(sequence >> (index * 8));
    }
    ++sequence;
    return output;
}

std::array<uint8_t, 16> NtlmSignatureState::seal(
    const Digest16& signingKey, Rc4& sealing, uint32_t& sequence,
    const uint8_t* message, std::size_t size, uint8_t* data,
    std::size_t dataSize) {
    std::vector<uint8_t> input;
    appendU32(input, sequence);
    if (size != 0) input.insert(input.end(), message, message + size);
    Digest16 checksum = hmacMd5(
        signingKey.data(), signingKey.size(), input.data(), input.size());
    if (dataSize != 0) sealing.process(data, dataSize);
    if (keyExchange_) sealing.process(checksum.data(), 8);
    std::array<uint8_t, 16> output{};
    output[0] = 1;
    std::copy(checksum.begin(), checksum.begin() + 8, output.begin() + 4);
    for (int index = 0; index < 4; ++index) {
        output[12 + index] = static_cast<uint8_t>(sequence >> (index * 8));
    }
    ++sequence;
    return output;
}

std::array<uint8_t, 16> NtlmSignatureState::signClient(
    const uint8_t* message, std::size_t size) {
    return initialized_ ? signature(
        clientSigningKey_, clientSealing_, clientSequence_, message, size)
        : std::array<uint8_t, 16>{};
}

std::array<uint8_t, 16> NtlmSignatureState::sealClient(
    const uint8_t* message, std::size_t size, uint8_t* data,
    std::size_t dataSize) {
    return initialized_ ? seal(
        clientSigningKey_, clientSealing_, clientSequence_, message, size,
        data, dataSize) : std::array<uint8_t, 16>{};
}

std::array<uint8_t, 16> NtlmSignatureState::signServerForTest(
    const uint8_t* message, std::size_t size) {
    return signServer(message, size);
}

std::array<uint8_t, 16> NtlmSignatureState::sealServerForTest(
    const uint8_t* message, std::size_t size, uint8_t* data,
    std::size_t dataSize) {
    return sealServer(message, size, data, dataSize);
}

std::array<uint8_t, 16> NtlmSignatureState::signServer(
    const uint8_t* message, std::size_t size) {
    return initialized_ ? signature(
        serverSigningKey_, serverSealing_, serverSequence_, message, size)
        : std::array<uint8_t, 16>{};
}

std::array<uint8_t, 16> NtlmSignatureState::sealServer(
    const uint8_t* message, std::size_t size, uint8_t* data,
    std::size_t dataSize) {
    return initialized_ ? seal(
        serverSigningKey_, serverSealing_, serverSequence_, message, size,
        data, dataSize) : std::array<uint8_t, 16>{};
}

bool NtlmSignatureState::verifyServer(
    const uint8_t* message, std::size_t size,
    const std::vector<uint8_t>& received) {
    if (!initialized_ || received.size() != 16) return false;
    const auto expected = signature(
        serverSigningKey_, serverSealing_, serverSequence_, message, size);
    return constantTimeEqual(expected.data(), received.data(), expected.size());
}

bool NtlmSignatureState::unsealAndVerifyServer(
    uint8_t* data, std::size_t dataSize, const uint8_t* message,
    std::size_t size, const std::vector<uint8_t>& received) {
    if (!initialized_ || received.size() != 16) return false;
    if (dataSize != 0) serverSealing_.process(data, dataSize);
    const auto expected = signature(
        serverSigningKey_, serverSealing_, serverSequence_, message, size);
    return constantTimeEqual(expected.data(), received.data(), expected.size());
}

bool NtlmSignatureState::verifyClient(
    const uint8_t* message, std::size_t size,
    const std::vector<uint8_t>& received) {
    if (!initialized_ || received.size() != 16) return false;
    const auto expected = signature(
        clientSigningKey_, clientSealing_, clientSequence_, message, size);
    return constantTimeEqual(expected.data(), received.data(), expected.size());
}

bool NtlmSignatureState::unsealAndVerifyClient(
    uint8_t* data, std::size_t dataSize, const uint8_t* message,
    std::size_t size, const std::vector<uint8_t>& received) {
    if (!initialized_ || received.size() != 16) return false;
    if (dataSize != 0) clientSealing_.process(data, dataSize);
    const auto expected = signature(
        clientSigningKey_, clientSealing_, clientSequence_, message, size);
    return constantTimeEqual(expected.data(), received.data(), expected.size());
}

}  // namespace wk::opcda::auth
