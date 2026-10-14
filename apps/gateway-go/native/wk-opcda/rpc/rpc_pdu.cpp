#include "rpc/rpc_pdu.h"

#include "auth/ntlm.h"

#include <algorithm>
#include <cctype>
#include <sstream>
#include <utility>

namespace wk::opcda::rpc {
namespace {

constexpr uint8_t kFirstFragment = 0x01;
constexpr uint8_t kLastFragment = 0x02;
constexpr std::size_t kCommonHeaderSize = 16;
constexpr std::size_t kRequestHeaderSize = 24;
constexpr std::size_t kResponseHeaderSize = 24;
constexpr std::size_t kFaultHeaderSize = 32;
constexpr uint8_t kNtlmAuthenticationType = 10;
constexpr uint8_t kPacketIntegrityLevel = 5;
constexpr uint8_t kPacketPrivacyLevel = 6;
constexpr uint8_t kSupportHeaderSign = 0x04;
constexpr uint8_t kObjectUuid = 0x80;

void appendU16(std::vector<uint8_t>& output, uint16_t value) {
    output.push_back(static_cast<uint8_t>(value));
    output.push_back(static_cast<uint8_t>(value >> 8));
}

void appendU32(std::vector<uint8_t>& output, uint32_t value) {
    output.push_back(static_cast<uint8_t>(value));
    output.push_back(static_cast<uint8_t>(value >> 8));
    output.push_back(static_cast<uint8_t>(value >> 16));
    output.push_back(static_cast<uint8_t>(value >> 24));
}

uint16_t readU16(const uint8_t* data) {
    return static_cast<uint16_t>(data[0]) | static_cast<uint16_t>(data[1] << 8);
}

uint32_t readU32(const uint8_t* data) {
    return static_cast<uint32_t>(data[0]) |
           (static_cast<uint32_t>(data[1]) << 8) |
           (static_cast<uint32_t>(data[2]) << 16) |
           (static_cast<uint32_t>(data[3]) << 24);
}

std::vector<uint8_t> encodePresentationContext(uint32_t callId, uint16_t contextId,
                                               const SyntaxId& abstractSyntax, PacketType packetType) {
    const SyntaxId transferSyntax = ndr32Syntax();
    std::vector<uint8_t> output;
    output.reserve(72);
    output.insert(output.end(), {5, 0, static_cast<uint8_t>(packetType),
                                 static_cast<uint8_t>(kFirstFragment | kLastFragment)});
    output.insert(output.end(), {0x10, 0x00, 0x00, 0x00});
    appendU16(output, 0);
    appendU16(output, 0);
    appendU32(output, callId);
    appendU16(output, 4280);
    appendU16(output, 4280);
    appendU32(output, 0);
    output.insert(output.end(), {1, 0, 0, 0});
    appendU16(output, contextId);
    output.insert(output.end(), {1, 0});
    output.insert(output.end(), abstractSyntax.uuid.begin(), abstractSyntax.uuid.end());
    appendU16(output, abstractSyntax.major);
    appendU16(output, abstractSyntax.minor);
    output.insert(output.end(), transferSyntax.uuid.begin(), transferSyntax.uuid.end());
    appendU16(output, transferSyntax.major);
    appendU16(output, transferSyntax.minor);
    const uint16_t length = static_cast<uint16_t>(output.size());
    output[8] = static_cast<uint8_t>(length);
    output[9] = static_cast<uint8_t>(length >> 8);
    return output;
}

void setLengths(std::vector<uint8_t>& output, uint16_t authLength) {
    const uint16_t length = static_cast<uint16_t>(output.size());
    output[8] = static_cast<uint8_t>(length);
    output[9] = static_cast<uint8_t>(length >> 8);
    output[10] = static_cast<uint8_t>(authLength);
    output[11] = static_cast<uint8_t>(authLength >> 8);
}

void appendSecurityTrailer(std::vector<uint8_t>& output, uint8_t authenticationLevel,
                           uint32_t authenticationContextId,
                           const std::vector<uint8_t>& token, bool alignTrailer,
                           uint8_t authenticationType = kNtlmAuthenticationType) {
    uint8_t padding = 0;
    if (alignTrailer) {
        padding = static_cast<uint8_t>((16 - ((output.size() - kCommonHeaderSize) % 16)) % 16);
        output.insert(output.end(), padding, 0);
    }
    output.insert(output.end(), {authenticationType, authenticationLevel, padding, 0});
    appendU32(output, authenticationContextId);
    output.insert(output.end(), token.begin(), token.end());
    setLengths(output, static_cast<uint16_t>(token.size()));
}

int hexNibble(char value) {
    if (value >= '0' && value <= '9') return value - '0';
    value = static_cast<char>(std::tolower(static_cast<unsigned char>(value)));
    if (value >= 'a' && value <= 'f') return value - 'a' + 10;
    return -1;
}

bool parseHexByte(const std::string& compact, std::size_t offset, uint8_t& value) {
    const int high = hexNibble(compact[offset]);
    const int low = hexNibble(compact[offset + 1]);
    if (high < 0 || low < 0) return false;
    value = static_cast<uint8_t>((high << 4) | low);
    return true;
}

}  // namespace

bool parseUuid(const std::string& text, std::array<uint8_t, 16>& wireUuid, std::string& error) {
    std::string compact;
    compact.reserve(32);
    for (char value : text) {
        if (value == '-' || value == '{' || value == '}') continue;
        compact.push_back(value);
    }
    if (compact.size() != 32) {
        error = "UUID must contain 32 hexadecimal digits: " + text;
        return false;
    }
    std::array<uint8_t, 16> canonical{};
    for (std::size_t index = 0; index < canonical.size(); ++index) {
        if (!parseHexByte(compact, index * 2, canonical[index])) {
            error = "UUID contains a non-hexadecimal character: " + text;
            return false;
        }
    }
    // DCE/RPC UUIDs use NDR little-endian representation for the first three
    // fields; the remaining eight bytes stay in canonical order.
    wireUuid = {
        canonical[3], canonical[2], canonical[1], canonical[0],
        canonical[5], canonical[4], canonical[7], canonical[6],
        canonical[8], canonical[9], canonical[10], canonical[11],
        canonical[12], canonical[13], canonical[14], canonical[15],
    };
    return true;
}

std::vector<uint8_t> encodeBind(uint32_t callId, uint16_t contextId, const SyntaxId& abstractSyntax) {
    return encodePresentationContext(callId, contextId, abstractSyntax, PacketType::Bind);
}

std::vector<uint8_t> encodeAuthenticatedBind(uint32_t callId, uint16_t contextId,
                                             const SyntaxId& abstractSyntax,
                                             uint8_t authenticationLevel,
                                             uint32_t authenticationContextId,
                                             const std::vector<uint8_t>& token) {
    std::vector<uint8_t> output = encodePresentationContext(callId, contextId, abstractSyntax, PacketType::Bind);
    output[3] |= kSupportHeaderSign;
    appendSecurityTrailer(output, authenticationLevel, authenticationContextId, token, true);
    return output;
}

std::vector<uint8_t> encodeAlterContext(uint32_t callId, uint16_t contextId,
                                        const SyntaxId& abstractSyntax) {
    return encodePresentationContext(callId, contextId, abstractSyntax, PacketType::AlterContext);
}

std::vector<std::vector<uint8_t>> encodeRequest(uint32_t callId, uint16_t contextId,
                                                uint16_t operationNumber,
                                                const std::vector<uint8_t>& stub,
                                                uint16_t maximumFragment) {
    if (maximumFragment <= kRequestHeaderSize) maximumFragment = static_cast<uint16_t>(kRequestHeaderSize + 1);
    const std::size_t capacity = maximumFragment - kRequestHeaderSize;
    std::vector<std::vector<uint8_t>> fragments;
    std::size_t offset = 0;
    do {
        const std::size_t count = std::min(capacity, stub.size() - offset);
        std::vector<uint8_t> output;
        output.reserve(kRequestHeaderSize + count);
        uint8_t flags = 0;
        if (offset == 0) flags |= kFirstFragment;
        if (offset + count == stub.size()) flags |= kLastFragment;
        output.insert(output.end(), {5, 0, static_cast<uint8_t>(PacketType::Request), flags,
                                     0x10, 0, 0, 0});
        appendU16(output, static_cast<uint16_t>(kRequestHeaderSize + count));
        appendU16(output, 0);
        appendU32(output, callId);
        appendU32(output, static_cast<uint32_t>(stub.size() - offset));
        appendU16(output, contextId);
        appendU16(output, operationNumber);
        output.insert(output.end(), stub.begin() + static_cast<std::ptrdiff_t>(offset),
                      stub.begin() + static_cast<std::ptrdiff_t>(offset + count));
        fragments.push_back(std::move(output));
        offset += count;
    } while (offset < stub.size());
    return fragments;
}

std::vector<std::vector<uint8_t>> encodeObjectRequest(
    uint32_t callId, uint16_t contextId, uint16_t operationNumber,
    const std::array<uint8_t, 16>& objectUuid, const std::vector<uint8_t>& stub,
    uint16_t maximumFragment) {
    constexpr std::size_t objectRequestHeaderSize = kRequestHeaderSize + 16;
    if (maximumFragment <= objectRequestHeaderSize) {
        maximumFragment = static_cast<uint16_t>(objectRequestHeaderSize + 1);
    }
    const std::size_t capacity = maximumFragment - objectRequestHeaderSize;
    std::vector<std::vector<uint8_t>> fragments;
    std::size_t offset = 0;
    do {
        const std::size_t count = std::min(capacity, stub.size() - offset);
        std::vector<uint8_t> output;
        output.reserve(objectRequestHeaderSize + count);
        uint8_t flags = kObjectUuid;
        if (offset == 0) flags |= kFirstFragment;
        if (offset + count == stub.size()) flags |= kLastFragment;
        output.insert(output.end(), {5, 0, static_cast<uint8_t>(PacketType::Request), flags,
                                     0x10, 0, 0, 0});
        appendU16(output, static_cast<uint16_t>(objectRequestHeaderSize + count));
        appendU16(output, 0);
        appendU32(output, callId);
        appendU32(output, static_cast<uint32_t>(stub.size() - offset));
        appendU16(output, contextId);
        appendU16(output, operationNumber);
        output.insert(output.end(), objectUuid.begin(), objectUuid.end());
        output.insert(output.end(), stub.begin() + static_cast<std::ptrdiff_t>(offset),
                      stub.begin() + static_cast<std::ptrdiff_t>(offset + count));
        fragments.push_back(std::move(output));
        offset += count;
    } while (offset < stub.size());
    return fragments;
}

std::vector<uint8_t> encodeAuth3(uint32_t callId, uint8_t authenticationType,
                                 uint8_t authenticationLevel, uint32_t authenticationContextId,
                                 const std::vector<uint8_t>& token) {
    std::vector<uint8_t> output = {
        5, 0, static_cast<uint8_t>(PacketType::Auth3), static_cast<uint8_t>(kFirstFragment | kLastFragment),
        0x10, 0, 0, 0, 0, 0, 0, 0,
    };
    appendU32(output, callId);
    appendU32(output, 0);  // four-byte auth3 body/padding
    output.insert(output.end(), {authenticationType, authenticationLevel, 0, 0});
    appendU32(output, authenticationContextId);
    output.insert(output.end(), token.begin(), token.end());
    const uint16_t length = static_cast<uint16_t>(output.size());
    output[8] = static_cast<uint8_t>(length);
    output[9] = static_cast<uint8_t>(length >> 8);
    const uint16_t authLength = static_cast<uint16_t>(token.size());
    output[10] = static_cast<uint8_t>(authLength);
    output[11] = static_cast<uint8_t>(authLength >> 8);
    return output;
}

bool decodeCommonHeader(const uint8_t* data, std::size_t size, CommonHeader& header, std::string& error) {
    if (size < kCommonHeaderSize) {
        error = "DCE/RPC fragment is shorter than the 16-byte common header";
        return false;
    }
    if (data[0] != 5 || data[1] != 0) {
        error = "unsupported DCE/RPC version " + std::to_string(data[0]) + "." + std::to_string(data[1]);
        return false;
    }
    if (data[4] != 0x10) {
        error = "only little-endian NDR data representation is supported";
        return false;
    }
    header.packetType = static_cast<PacketType>(data[2]);
    header.flags = data[3];
    header.fragmentLength = readU16(data + 8);
    header.authLength = readU16(data + 10);
    header.callId = readU32(data + 12);
    if (header.fragmentLength < kCommonHeaderSize || header.fragmentLength > size) {
        error = "invalid DCE/RPC fragment length " + std::to_string(header.fragmentLength);
        return false;
    }
    return true;
}

bool validateBindAck(const std::vector<uint8_t>& fragment, uint32_t expectedCallId,
                     std::string& error, uint8_t expectedResultCount) {
    CommonHeader header{};
    if (!decodeCommonHeader(fragment.data(), fragment.size(), header, error)) {
        return false;
    }
    if (header.packetType == PacketType::BindNak) {
        error = "DCE/RPC server rejected the presentation context";
        return false;
    }
    if (header.packetType != PacketType::BindAck) {
        error = "expected BindAck but received packet type " + std::to_string(static_cast<int>(header.packetType));
        return false;
    }
    if (header.callId != expectedCallId) {
        error = "BindAck call ID does not match the request";
        return false;
    }
    if ((header.flags & (kFirstFragment | kLastFragment)) != (kFirstFragment | kLastFragment)) {
        error = "fragmented BindAck is not supported";
        return false;
    }
    if (fragment.size() < 30) {
        error = "BindAck does not contain a secondary address and result list";
        return false;
    }
    const uint16_t secondaryAddressLength = readU16(fragment.data() + 24);
    std::size_t offset = 26 + secondaryAddressLength;
    offset = (offset + 3) & ~std::size_t(3);
    if (offset + 8 > header.fragmentLength) {
        error = "BindAck result list is truncated";
        return false;
    }
    const uint8_t resultCount = fragment[offset];
    offset += 4;
    if (resultCount != expectedResultCount ||
        offset + static_cast<std::size_t>(resultCount) * 24 > header.fragmentLength) {
        error = "BindAck presentation result count does not match the request";
        return false;
    }
    for (uint8_t index = 0; index < resultCount; ++index, offset += 24) {
        const uint16_t result = readU16(fragment.data() + offset);
        const uint16_t reason = readU16(fragment.data() + offset + 2);
        if (result != 0) {
            std::ostringstream stream;
            stream << "DCE/RPC presentation context " << static_cast<int>(index)
                   << " rejected: result=" << result << " reason=" << reason;
            error = stream.str();
            return false;
        }
    }
    return true;
}

bool decodeSecurityTrailer(const std::vector<uint8_t>& fragment, const CommonHeader& header,
                           SecurityTrailer& trailer, std::size_t& bodyEnd, std::string& error) {
    bodyEnd = header.fragmentLength;
    trailer = SecurityTrailer{};
    if (header.authLength == 0) return true;
    const std::size_t trailerSize = 8;
    if (header.fragmentLength < kCommonHeaderSize + trailerSize + header.authLength) {
        error = "DCE/RPC authentication trailer is truncated";
        return false;
    }
    const std::size_t trailerOffset = header.fragmentLength - header.authLength - trailerSize;
    trailer.authenticationType = fragment[trailerOffset];
    trailer.authenticationLevel = fragment[trailerOffset + 1];
    trailer.paddingLength = fragment[trailerOffset + 2];
    trailer.contextId = readU32(fragment.data() + trailerOffset + 4);
    if (trailerOffset < trailer.paddingLength + kCommonHeaderSize) {
        error = "DCE/RPC authentication padding exceeds the PDU body";
        return false;
    }
    bodyEnd = trailerOffset - trailer.paddingLength;
    trailer.token.assign(fragment.begin() + static_cast<std::ptrdiff_t>(trailerOffset + trailerSize),
                         fragment.begin() + static_cast<std::ptrdiff_t>(header.fragmentLength));
    return true;
}

bool decodeResponseFragments(const std::vector<std::vector<uint8_t>>& fragments,
                             uint32_t expectedCallId, uint16_t expectedContextId,
                             RpcResponse& response, std::string& error) {
    response = RpcResponse{};
    if (fragments.empty()) {
        error = "DCE/RPC response contains no fragments";
        return false;
    }
    bool sawLast = false;
    for (std::size_t index = 0; index < fragments.size(); ++index) {
        const std::vector<uint8_t>& fragment = fragments[index];
        CommonHeader header{};
        if (!decodeCommonHeader(fragment.data(), fragment.size(), header, error)) return false;
        if (header.callId != expectedCallId) {
            error = "DCE/RPC response call ID does not match the request";
            return false;
        }
        const bool first = (header.flags & kFirstFragment) != 0;
        const bool last = (header.flags & kLastFragment) != 0;
        if (first != (index == 0) || sawLast) {
            error = "DCE/RPC response fragment flags are out of sequence";
            return false;
        }
        SecurityTrailer trailer{};
        std::size_t bodyEnd = 0;
        if (!decodeSecurityTrailer(fragment, header, trailer, bodyEnd, error)) return false;
        if (header.packetType == PacketType::Fault) {
            if (fragment.size() < kFaultHeaderSize || bodyEnd < kFaultHeaderSize) {
                error = "DCE/RPC Fault PDU is truncated";
                return false;
            }
            const uint16_t context = readU16(fragment.data() + 20);
            if (context != expectedContextId) {
                error = "DCE/RPC Fault context ID does not match the request";
                return false;
            }
            response.fault = true;
            response.faultStatus = readU32(fragment.data() + 24);
            return true;
        }
        if (header.packetType != PacketType::Response) {
            error = "expected Response or Fault but received packet type " +
                    std::to_string(static_cast<int>(header.packetType));
            return false;
        }
        if (fragment.size() < kResponseHeaderSize || bodyEnd < kResponseHeaderSize) {
            error = "DCE/RPC Response PDU is truncated";
            return false;
        }
        const uint16_t context = readU16(fragment.data() + 20);
        if (context != expectedContextId) {
            error = "DCE/RPC Response context ID does not match the request";
            return false;
        }
        response.stub.insert(response.stub.end(),
                             fragment.begin() + static_cast<std::ptrdiff_t>(kResponseHeaderSize),
                             fragment.begin() + static_cast<std::ptrdiff_t>(bodyEnd));
        sawLast = last;
    }
    if (!sawLast) {
        error = "DCE/RPC response did not contain a last fragment";
        return false;
    }
    return true;
}

bool protectPacketIntegrity(std::vector<uint8_t>& fragment, uint32_t authenticationContextId,
                            auth::NtlmSignatureState& security, std::string& error,
                            bool packetPrivacy) {
    CommonHeader header{};
    if (!decodeCommonHeader(fragment.data(), fragment.size(), header, error)) return false;
    if (header.authLength != 0 || header.fragmentLength != fragment.size()) {
        error = "RPC fragment already has an authentication verifier";
        return false;
    }
    const std::vector<uint8_t> emptySignature(16, 0);
    appendSecurityTrailer(fragment, packetPrivacy ? kPacketPrivacyLevel : kPacketIntegrityLevel,
                          authenticationContextId, emptySignature, true);
    const std::size_t signedSize = fragment.size() - emptySignature.size();
    const bool objectRequest = header.packetType == PacketType::Request &&
                               (header.flags & kObjectUuid) != 0;
    const std::size_t bodyOffset = header.packetType == PacketType::Request
        ? kRequestHeaderSize + (objectRequest ? 16 : 0)
        : header.packetType == PacketType::Response ? kResponseHeaderSize
        : header.packetType == PacketType::Fault ? kFaultHeaderSize : signedSize;
    if (packetPrivacy && bodyOffset > signedSize - 8) {
        error = "RPC packet body is too short for Packet Privacy";
        return false;
    }
    const std::size_t sealedSize = packetPrivacy ? signedSize - 8 - bodyOffset : 0;
    const auto signature = packetPrivacy
        ? security.sealClient(fragment.data(), signedSize, fragment.data() + bodyOffset, sealedSize)
        : security.signClient(fragment.data(), signedSize);
    std::copy(signature.begin(), signature.end(), fragment.begin() + static_cast<std::ptrdiff_t>(signedSize));
    return true;
}

bool verifyPacketIntegrity(std::vector<uint8_t>& fragment,
                           uint32_t expectedAuthenticationContextId,
                           auth::NtlmSignatureState& security, std::string& error,
                           bool packetPrivacy) {
    CommonHeader header{};
    if (!decodeCommonHeader(fragment.data(), fragment.size(), header, error)) return false;
    SecurityTrailer trailer{};
    std::size_t bodyEnd = 0;
    if (!decodeSecurityTrailer(fragment, header, trailer, bodyEnd, error)) return false;
    if (trailer.authenticationType != kNtlmAuthenticationType ||
        trailer.authenticationLevel != (packetPrivacy ? kPacketPrivacyLevel : kPacketIntegrityLevel) ||
        trailer.contextId != expectedAuthenticationContextId || trailer.token.size() != 16) {
        error = packetPrivacy
            ? "RPC response does not carry the expected NTLM Packet Privacy verifier"
            : "RPC response does not carry the expected NTLM Packet Integrity verifier";
        return false;
    }
    const std::size_t signedSize = header.fragmentLength - header.authLength;
    const std::size_t bodyOffset = header.packetType == PacketType::Response
        ? kResponseHeaderSize : header.packetType == PacketType::Fault
        ? kFaultHeaderSize : signedSize;
    if (packetPrivacy && bodyOffset > bodyEnd) {
        error = "RPC response body is too short for Packet Privacy";
        return false;
    }
    const bool valid = packetPrivacy
        ? security.unsealAndVerifyServer(fragment.data() + bodyOffset,
                                         bodyEnd + trailer.paddingLength - bodyOffset,
                                         fragment.data(), signedSize, trailer.token)
        : security.verifyServer(fragment.data(), signedSize, trailer.token);
    if (!valid) {
        error = packetPrivacy
            ? "NTLM Packet Privacy unsealing or signature verification failed"
            : "NTLM Packet Integrity signature verification failed";
        return false;
    }
    return true;
}

bool protectPacketIntegrityServer(std::vector<uint8_t>& fragment,
                                  uint32_t authenticationContextId,
                                  auth::NtlmSignatureState& security,
                                  std::string& error,
                                  bool packetPrivacy,
                                  uint8_t authenticationType) {
    CommonHeader header{};
    if (!decodeCommonHeader(fragment.data(), fragment.size(), header, error)) return false;
    if (header.authLength != 0 || header.fragmentLength != fragment.size()) {
        error = "RPC server fragment already has an authentication verifier";
        return false;
    }
    const std::vector<uint8_t> emptySignature(16, 0);
    appendSecurityTrailer(fragment,
                          packetPrivacy ? kPacketPrivacyLevel : kPacketIntegrityLevel,
                          authenticationContextId, emptySignature, true,
                          authenticationType);
    const std::size_t signedSize = fragment.size() - emptySignature.size();
    const std::size_t bodyOffset = header.packetType == PacketType::Response
        ? kResponseHeaderSize : header.packetType == PacketType::Fault
        ? kFaultHeaderSize : signedSize;
    if (packetPrivacy && bodyOffset > signedSize - 8) {
        error = "RPC server response body is too short for Packet Privacy";
        return false;
    }
    const std::size_t sealedSize = packetPrivacy ? signedSize - 8 - bodyOffset : 0;
    const auto signature = packetPrivacy
        ? security.sealServer(fragment.data(), signedSize,
                              fragment.data() + bodyOffset, sealedSize)
        : security.signServer(fragment.data(), signedSize);
    std::copy(signature.begin(), signature.end(),
              fragment.begin() + static_cast<std::ptrdiff_t>(signedSize));
    return true;
}

bool verifyPacketIntegrityClient(std::vector<uint8_t>& fragment,
                                 uint32_t expectedAuthenticationContextId,
                                 auth::NtlmSignatureState& security,
                                 std::string& error,
                                 bool packetPrivacy,
                                 uint8_t authenticationType) {
    CommonHeader header{};
    if (!decodeCommonHeader(fragment.data(), fragment.size(), header, error)) return false;
    SecurityTrailer trailer{};
    std::size_t bodyEnd = 0;
    if (!decodeSecurityTrailer(fragment, header, trailer, bodyEnd, error)) return false;
    if (trailer.authenticationType != authenticationType ||
        trailer.authenticationLevel !=
            (packetPrivacy ? kPacketPrivacyLevel : kPacketIntegrityLevel) ||
        trailer.contextId != expectedAuthenticationContextId ||
        trailer.token.size() != 16) {
        error = (packetPrivacy
            ? "RPC request does not carry the expected NTLM Packet Privacy verifier"
            : "RPC request does not carry the expected NTLM Packet Integrity verifier") +
            std::string(" (auth type=") +
            std::to_string(trailer.authenticationType) +
            ", expected=" + std::to_string(authenticationType) + ")";
        return false;
    }
    const std::size_t signedSize = header.fragmentLength - header.authLength;
    const bool objectRequest = header.packetType == PacketType::Request &&
                               (header.flags & kObjectUuid) != 0;
    const std::size_t bodyOffset = header.packetType == PacketType::Request
        ? kRequestHeaderSize + (objectRequest ? 16 : 0) : signedSize;
    if (packetPrivacy && bodyOffset > bodyEnd) {
        error = "RPC request body is too short for Packet Privacy";
        return false;
    }
    const bool valid = packetPrivacy
        ? security.unsealAndVerifyClient(
            fragment.data() + bodyOffset,
            bodyEnd + trailer.paddingLength - bodyOffset,
            fragment.data(), signedSize, trailer.token)
        : security.verifyClient(fragment.data(), signedSize, trailer.token);
    if (!valid) {
        error = packetPrivacy
            ? "NTLM client Packet Privacy unsealing or signature verification failed"
            : "NTLM client Packet Integrity signature verification failed";
        return false;
    }
    return true;
}

SyntaxId endpointMapperSyntax() {
    SyntaxId syntax{};
    std::string ignored;
    parseUuid("e1af8308-5d1f-11c9-91a4-08002b14a0fa", syntax.uuid, ignored);
    syntax.major = 3;
    syntax.minor = 0;
    return syntax;
}

SyntaxId objectExporterSyntax() {
    SyntaxId syntax{};
    std::string ignored;
    parseUuid("99fcfec4-5260-101b-bbcb-00aa0021347a", syntax.uuid, ignored);
    syntax.major = 0;
    syntax.minor = 0;
    return syntax;
}

SyntaxId ndr32Syntax() {
    SyntaxId syntax{};
    std::string ignored;
    parseUuid("8a885d04-1ceb-11c9-9fe8-08002b104860", syntax.uuid, ignored);
    syntax.major = 2;
    syntax.minor = 0;
    return syntax;
}

}  // namespace wk::opcda::rpc
