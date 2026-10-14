#include "api/wk_opcda.h"
#include "auth/crypto.h"
#include "auth/ntlm.h"
#include "dcom/activation.h"
#include "dcom/object_reference.h"
#include "opcda/item_management.h"
#include "opcda/connection_point.h"
#include "opcda/data_callback.h"
#include "opcda/server.h"
#include "opcda/sync_io.h"
#include "rpc/endpoint_mapper.h"
#include "rpc/ndr.h"
#include "rpc/rpc_pdu.h"

#include <algorithm>
#include <cassert>
#include <cstdint>
#include <vector>

namespace {

void appendU16(std::vector<uint8_t>& value, uint16_t number) {
    value.push_back(static_cast<uint8_t>(number));
    value.push_back(static_cast<uint8_t>(number >> 8));
}

void appendU32(std::vector<uint8_t>& value, uint32_t number) {
    for (int shift = 0; shift < 32; shift += 8) value.push_back(static_cast<uint8_t>(number >> shift));
}

void appendU64(std::vector<uint8_t>& value, uint64_t number) {
    appendU32(value, static_cast<uint32_t>(number));
    appendU32(value, static_cast<uint32_t>(number >> 32));
}

uint16_t readU16(const uint8_t* value) {
    return static_cast<uint16_t>(value[0]) |
           static_cast<uint16_t>(static_cast<uint16_t>(value[1]) << 8);
}

uint32_t readU32(const uint8_t* value) {
    return static_cast<uint32_t>(value[0]) |
           (static_cast<uint32_t>(value[1]) << 8) |
           (static_cast<uint32_t>(value[2]) << 16) |
           (static_cast<uint32_t>(value[3]) << 24);
}

std::vector<uint16_t> tcpBinding(const char* address) {
    std::vector<uint16_t> value = {7};
    while (*address != '\0') value.push_back(static_cast<uint8_t>(*address++));
    value.push_back(0);  // address terminator
    value.push_back(0);  // string-binding array terminator
    value.push_back(0);  // empty security binding
    value.push_back(0);  // security-binding array terminator
    return value;
}

std::vector<uint8_t> bytes(const char* text) {
    return std::vector<uint8_t>(text, text + std::char_traits<char>::length(text));
}

std::array<uint8_t, 16> digest(const char* hex) {
    std::array<uint8_t, 16> output{};
    const auto nibble = [](char value) -> uint8_t { return value <= '9' ? static_cast<uint8_t>(value - '0') : static_cast<uint8_t>(value - 'a' + 10); };
    for (std::size_t index = 0; index < output.size(); ++index) output[index] = static_cast<uint8_t>((nibble(hex[index * 2]) << 4) | nibble(hex[index * 2 + 1]));
    return output;
}

std::vector<uint8_t> responseFragment(uint32_t callId, uint16_t contextId,
                                      uint8_t flags, const std::vector<uint8_t>& stub) {
    std::vector<uint8_t> value = {5, 0, 2, flags, 0x10, 0, 0, 0};
    appendU16(value, static_cast<uint16_t>(24 + stub.size()));
    appendU16(value, 0);
    appendU32(value, callId);
    appendU32(value, static_cast<uint32_t>(stub.size()));
    appendU16(value, contextId);
    value.push_back(0);
    value.push_back(0);
    value.insert(value.end(), stub.begin(), stub.end());
    return value;
}

std::vector<uint8_t> faultFragment(uint32_t callId, uint16_t contextId, uint32_t status) {
    std::vector<uint8_t> value = {5, 0, 3, 0x03, 0x10, 0, 0, 0};
    appendU16(value, 32);
    appendU16(value, 0);
    appendU32(value, callId);
    appendU32(value, 0);
    appendU16(value, contextId);
    value.push_back(0);
    value.push_back(0);
    appendU32(value, status);
    appendU32(value, 0);
    return value;
}

}  // namespace

int main() {
    static_assert(WK_OPCDA_ABI_VERSION == 3u);
    static_assert(WK_OPCDA_SECURITY_MODERN != WK_OPCDA_SECURITY_LEGACY_CONNECT);
    static_assert(WK_RPC_PACKET_VERIFY_FAILED != WK_RPC_PACKET_SIGN_FAILED);
    using namespace wk::opcda::rpc;
    using namespace wk::opcda::auth;
    using namespace wk::opcda::dcom;
    using namespace wk::opcda::opcda;
    std::string ntlmError;

    assert(md4(nullptr, 0) == digest("31d6cfe0d16ae931b73c59d7e0c089c0"));
    const auto abc = bytes("abc");
    assert(md5(abc) == digest("900150983cd24fb0d6963f7d28e17f72"));
    const auto key = bytes("key");
    const auto quick = bytes("The quick brown fox jumps over the lazy dog");
    assert(hmacMd5(key, quick) == digest("80070713463e7749b90c2dc24911e275"));
    Rc4 rc4(reinterpret_cast<const uint8_t*>("Key"), 3);
    auto cipher = rc4.processCopy(reinterpret_cast<const uint8_t*>("Plaintext"), 9);
    assert((cipher == std::vector<uint8_t>{0xbb,0xf3,0x16,0xe8,0xd9,0x40,0xaf,0x0a,0xd3}));

    const auto negotiate = encodeNtlmNegotiate(true, false);
    const auto privacyNegotiate = encodeNtlmNegotiate(true, true);
    assert((readU32(privacyNegotiate.data() + 12) & kNtlmNegotiateSeal) != 0);
    std::vector<uint8_t> challengeToken = {'N','T','L','M','S','S','P',0};
    appendU32(challengeToken, 2);
    appendU16(challengeToken, 0); appendU16(challengeToken, 0); appendU32(challengeToken, 48);
    appendU32(challengeToken, static_cast<uint32_t>(negotiate[12]) |
                              (static_cast<uint32_t>(negotiate[13]) << 8) |
                              (static_cast<uint32_t>(negotiate[14]) << 16) |
                              (static_cast<uint32_t>(negotiate[15]) << 24));
    challengeToken.insert(challengeToken.end(), {0x01,0x23,0x45,0x67,0x89,0xab,0xcd,0xef});
    challengeToken.insert(challengeToken.end(), 8, 0);
    const std::vector<uint8_t> targetInfo = {
        0x02,0x00,0x0c,0x00,0x44,0x00,0x6f,0x00,0x6d,0x00,0x61,0x00,0x69,0x00,0x6e,0x00,
        0x01,0x00,0x0c,0x00,0x53,0x00,0x65,0x00,0x72,0x00,0x76,0x00,0x65,0x00,0x72,0x00,
        0x00,0x00,0x00,0x00,
    };
    appendU16(challengeToken, static_cast<uint16_t>(targetInfo.size()));
    appendU16(challengeToken, static_cast<uint16_t>(targetInfo.size())); appendU32(challengeToken, 48);
    challengeToken.insert(challengeToken.end(), targetInfo.begin(), targetInfo.end());
    NtlmChallenge parsedChallenge{};
    assert(parseNtlmChallenge(challengeToken, true, parsedChallenge, ntlmError));
    NtlmClientNonce nonce{}; nonce.clientChallenge.fill(0xaa); nonce.exportedSessionKey.fill(0x55); nonce.timestamp = 0;
    NtlmAuthenticateResult authenticated{};
    assert(encodeNtlmAuthenticate({"User","Password","Domain","COMPUTER"}, negotiate,
                                  parsedChallenge, nonce, authenticated, ntlmError));
    const uint16_t ntLength = static_cast<uint16_t>(authenticated.message[20]) |
                              static_cast<uint16_t>(authenticated.message[21] << 8);
    const uint32_t ntOffset = static_cast<uint32_t>(authenticated.message[24]) |
                              (static_cast<uint32_t>(authenticated.message[25]) << 8) |
                              (static_cast<uint32_t>(authenticated.message[26]) << 16) |
                              (static_cast<uint32_t>(authenticated.message[27]) << 24);
    assert(ntLength > 16 && ntOffset + ntLength <= authenticated.message.size());
    const auto expectedProof = digest("68cd0ab851e51c96aabc927bebef6a1c");
    assert(std::equal(expectedProof.begin(), expectedProof.end(),
                      authenticated.message.begin() + ntOffset));
    NtlmSignatureState peer, verifier;
    assert(peer.initialize(authenticated) && verifier.initialize(authenticated));
    const auto signedMessage = bytes("signed rpc body");
    const auto serverSignature = peer.signServerForTest(signedMessage.data(), signedMessage.size());
    assert(verifier.verifyServer(signedMessage.data(), signedMessage.size(),
                                 std::vector<uint8_t>(serverSignature.begin(), serverSignature.end())));
    NtlmSignatureState tamperVerifier; assert(tamperVerifier.initialize(authenticated));
    auto tampered = signedMessage; tampered[0] ^= 1;
    assert(!tamperVerifier.verifyServer(tampered.data(), tampered.size(),
                                        std::vector<uint8_t>(serverSignature.begin(), serverSignature.end())));
    NtlmSignatureState privacyPeer, privacyVerifier;
    assert(privacyPeer.initialize(authenticated) && privacyVerifier.initialize(authenticated));
    std::vector<uint8_t> sealedMessage = bytes("rpc header and confidential stub");
    const std::vector<uint8_t> plainMessage = sealedMessage;
    constexpr std::size_t sealedOffset = 15;
    const auto privacySignature = privacyPeer.sealServerForTest(
        sealedMessage.data(), sealedMessage.size(),
        sealedMessage.data() + sealedOffset, sealedMessage.size() - sealedOffset);
    assert(!std::equal(sealedMessage.begin() + sealedOffset, sealedMessage.end(),
                       plainMessage.begin() + sealedOffset));
    assert(privacyVerifier.unsealAndVerifyServer(
        sealedMessage.data() + sealedOffset, sealedMessage.size() - sealedOffset,
        sealedMessage.data(), sealedMessage.size(),
        std::vector<uint8_t>(privacySignature.begin(), privacySignature.end())));
    assert(sealedMessage == plainMessage);
    const NtlmCredentials callbackCredentials{
        "User", "Password", "Domain", "EG100"};
    NtlmChallenge callbackChallenge{};
    assert(encodeNtlmChallenge(
        privacyNegotiate, "EG100", callbackCredentials.domain,
        callbackChallenge, ntlmError));
    assert((callbackChallenge.flags & 0x00020000u) != 0);  // TARGET_TYPE_SERVER
    auto identifyNegotiate = privacyNegotiate;
    identifyNegotiate[12 + 2] |= 0x10;  // NTLMSSP_NEGOTIATE_IDENTIFY
    NtlmChallenge identifyChallenge{};
    assert(encodeNtlmChallenge(
        identifyNegotiate, "EG100", callbackCredentials.domain,
        identifyChallenge, ntlmError));
    assert((identifyChallenge.flags & 0x00100000u) != 0);
    const uint16_t callbackTargetLength =
        readU16(callbackChallenge.rawMessage.data() + 12);
    const uint32_t callbackTargetOffset =
        readU32(callbackChallenge.rawMessage.data() + 16);
    assert(callbackTargetLength == 10);
    assert(callbackTargetOffset + callbackTargetLength <=
           callbackChallenge.rawMessage.size());
    const uint8_t expectedCallbackTarget[] = {
        'E', 0, 'G', 0, '1', 0, '0', 0, '0', 0};
    assert(std::equal(
        std::begin(expectedCallbackTarget), std::end(expectedCallbackTarget),
        callbackChallenge.rawMessage.begin() + callbackTargetOffset));
    auto versionNegotiate = privacyNegotiate;
    versionNegotiate[12 + 3] |= 0x02;  // NTLMSSP_NEGOTIATE_VERSION
    versionNegotiate.insert(versionNegotiate.end(),
                            {10, 0, 0, 0, 0, 0, 0, 0x0f});
    NtlmChallenge versionChallenge{};
    assert(encodeNtlmChallenge(
        versionNegotiate, "EG100", callbackCredentials.domain,
        versionChallenge, ntlmError));
    assert((versionChallenge.flags & 0x02000000u) != 0);
    assert(versionChallenge.rawMessage.size() >= 56);
    assert(versionChallenge.rawMessage[48] == 6 &&
           versionChallenge.rawMessage[49] == 1 &&
           versionChallenge.rawMessage[55] == 0x0f);
    NtlmAuthenticateResult callbackClientResult{};
    assert(encodeNtlmAuthenticate(
        callbackCredentials, privacyNegotiate, callbackChallenge,
        randomNtlmClientNonce(), callbackClientResult, ntlmError));
    NtlmAuthenticateResult callbackServerResult{};
    assert(acceptNtlmAuthenticate(
        callbackCredentials, privacyNegotiate, callbackChallenge,
        callbackClientResult.message, callbackServerResult, ntlmError));
    assert(callbackClientResult.exportedSessionKey ==
           callbackServerResult.exportedSessionKey);
    NtlmSignatureState callbackClientSecurity;
    NtlmSignatureState callbackServerSecurity;
    assert(callbackClientSecurity.initialize(callbackClientResult));
    assert(callbackServerSecurity.initialize(callbackServerResult));
    const auto callbackSigned = callbackClientSecurity.signClient(
        signedMessage.data(), signedMessage.size());
    assert(callbackServerSecurity.verifyClient(
        signedMessage.data(), signedMessage.size(),
        std::vector<uint8_t>(callbackSigned.begin(), callbackSigned.end())));
    const auto callbackReply = callbackServerSecurity.signServer(
        signedMessage.data(), signedMessage.size());
    assert(callbackClientSecurity.verifyServer(
        signedMessage.data(), signedMessage.size(),
        std::vector<uint8_t>(callbackReply.begin(), callbackReply.end())));
    const auto bind = encodeBind(7, 0, endpointMapperSyntax());
    assert(bind.size() == 72);
    assert(bind[0] == 5 && bind[2] == static_cast<uint8_t>(PacketType::Bind));
    assert(bind[8] == 72 && bind[9] == 0);
    assert(bind[12] == 7);

    CommonHeader header{};
    std::string error;
    assert(decodeCommonHeader(bind.data(), bind.size(), header, error));
    assert(header.callId == 7);
    assert(header.fragmentLength == bind.size());

    const auto alter = encodeAlterContext(8, 1, endpointMapperSyntax());
    assert(alter[2] == static_cast<uint8_t>(PacketType::AlterContext));
    const auto auth3 = encodeAuth3(9, 10, 5, 1, {0xaa, 0xbb, 0xcc});
    assert(decodeCommonHeader(auth3.data(), auth3.size(), header, error));
    assert(header.packetType == PacketType::Auth3 && header.authLength == 3);
    SecurityTrailer security{};
    std::size_t bodyEnd = 0;
    assert(decodeSecurityTrailer(auth3, header, security, bodyEnd, error));
    assert(security.authenticationType == 10 && security.authenticationLevel == 5);
    assert(security.contextId == 1 && security.token.size() == 3 && bodyEnd == 20);
    const auto privacyAuth3 = encodeAuth3(10, 10, 6, 1, {0x11, 0x22});
    assert(decodeCommonHeader(privacyAuth3.data(), privacyAuth3.size(), header, error));
    assert(decodeSecurityTrailer(privacyAuth3, header, security, bodyEnd, error));
    assert(security.authenticationLevel == 6);

    NtlmSignatureState requestPrivacy;
    assert(requestPrivacy.initialize(authenticated));
    auto privateRequest = encodeRequest(11, 0, 3, bytes("private stub"))[0];
    const std::vector<uint8_t> privatePlaintext(privateRequest.begin() + 24,
                                                privateRequest.end());
    assert(protectPacketIntegrity(privateRequest, 1, requestPrivacy, error, true));
    assert(decodeCommonHeader(privateRequest.data(), privateRequest.size(), header, error));
    assert(decodeSecurityTrailer(privateRequest, header, security, bodyEnd, error));
    assert(security.authenticationLevel == 6 && security.token.size() == 16);
    assert(!std::equal(privateRequest.begin() + 24,
                       privateRequest.begin() + 24 + privatePlaintext.size(),
                       privatePlaintext.begin()));

    std::array<uint8_t, 16> objectUuid{};
    objectUuid.fill(0x5a);
    const auto objectRequest = encodeObjectRequest(12, 0, 0xffff, objectUuid, {});
    assert(objectRequest.size() == 1 && objectRequest[0].size() == 40);
    assert((objectRequest[0][3] & 0x80) != 0);
    assert(std::equal(objectUuid.begin(), objectUuid.end(), objectRequest[0].begin() + 24));

    std::array<uint8_t, 16> uuid{};
    assert(parseUuid("e1af8308-5d1f-11c9-91a4-08002b14a0fa", uuid, error));
    assert(uuid[0] == 0x08 && uuid[1] == 0x83 && uuid[2] == 0xaf && uuid[3] == 0xe1);

    ndr::Writer ndrWriter;
    ndrWriter.writeU8(0x7f);
    ndrWriter.writeU32(0x11223344);
    ndrWriter.writeU64(0x0102030405060708ULL);
    ndrWriter.writeConformantVaryingHeader(8, 2, 3);
    ndr::Reader ndrReader(ndrWriter.data());
    uint8_t u8 = 0;
    uint32_t u32 = 0;
    uint64_t u64 = 0;
    uint32_t maximum = 0, offset = 0, actual = 0;
    assert(ndrReader.readU8(u8, error) && u8 == 0x7f);
    assert(ndrReader.readU32(u32, error) && u32 == 0x11223344);
    assert(ndrReader.readU64(u64, error) && u64 == 0x0102030405060708ULL);
    assert(ndrReader.readConformantVaryingHeader(maximum, offset, actual, error));
    assert(maximum == 8 && offset == 2 && actual == 3);

    std::vector<uint8_t> largeStub(9000, 0x5a);
    const auto requestFragments = encodeRequest(9, 3, 7, largeStub, 1024);
    assert(requestFragments.size() > 1);
    assert((requestFragments.front()[3] & 0x01) != 0);
    assert((requestFragments.front()[3] & 0x02) == 0);
    assert((requestFragments.back()[3] & 0x02) != 0);

    const std::vector<uint8_t> firstStub = {1, 2, 3};
    const std::vector<uint8_t> secondStub = {4, 5};
    RpcResponse rpcResponse{};
    const std::vector<std::vector<uint8_t>> responseFragments = {
        responseFragment(10, 4, 0x01, firstStub),
        responseFragment(10, 4, 0x02, secondStub),
    };
    assert(decodeResponseFragments(responseFragments, 10, 4, rpcResponse, error));
    assert((rpcResponse.stub == std::vector<uint8_t>{1, 2, 3, 4, 5}));
    assert(decodeResponseFragments({faultFragment(11, 4, 0x1c010002)}, 11, 4, rpcResponse, error));
    assert(rpcResponse.fault && rpcResponse.faultStatus == 0x1c010002);

    std::vector<uint8_t> tower = encodeTcpIpTower(remoteScmActivatorSyntax());
    assert(tower.size() > 65);
    tower[64] = 0xc0;
    tower[65] = 0x01;
    uint16_t port = 0;
    assert(parseTcpPortFromTower(tower, port, error));
    assert(port == 49153);

    ndr::Writer endpointResponse;
    endpointResponse.writeU32(0);
    endpointResponse.writeUuid({});
    endpointResponse.writeU32(1);
    endpointResponse.writeConformantVaryingHeader(4, 0, 1);
    endpointResponse.writePointer(true);
    endpointResponse.writeU32(static_cast<uint32_t>(tower.size()));
    endpointResponse.writeU32(static_cast<uint32_t>(tower.size()));
    endpointResponse.writeBytes(tower.data(), tower.size());
    endpointResponse.writeU32(0);
    EndpointMapResult endpoint{};
    assert(decodeEndpointMapResponse(endpointResponse.data(), endpoint, error));
    assert(endpoint.tcpPort == 49153);

    std::array<uint8_t, 16> cid{};
    cid.fill(0x5a);
    const std::array<uint8_t, 16> requestedOpcServer = opcServerIid();
    std::vector<uint8_t> activationRequest;
    assert(encodeRemoteCreateInstanceRequest(
        "F8582CF2-88FB-11D0-B850-00C0F0104305", requestedOpcServer, cid,
        activationRequest, error));
    assert(activationRequest.size() == 464);
    assert(activationRequest[0] == 5 && activationRequest[1] == 0);
    assert(activationRequest[2] == 7 && activationRequest[3] == 0);
    assert(activationRequest[4] == 1);  // ORPCF_LOCAL
    // OBJREF_CUSTOM and its activation property blob match the Windows layout.
    assert(activationRequest[40] == 0xa0 && activationRequest[41] == 1);
    assert(activationRequest[48] == 0x4d && activationRequest[49] == 0x45 &&
           activationRequest[50] == 0x4f && activationRequest[51] == 0x57);
    assert(std::search(activationRequest.begin(), activationRequest.end(),
                       requestedOpcServer.begin(), requestedOpcServer.end()) != activationRequest.end());
    assert(!encodeRemoteCreateInstanceRequest(
        "not-a-clsid", requestedOpcServer, cid, activationRequest, error));

    std::vector<uint8_t> activationResponse(12, 0);
    appendU32(activationResponse, 0x574f454d);
    appendU32(activationResponse, 1);
    activationResponse.insert(activationResponse.end(), requestedOpcServer.begin(), requestedOpcServer.end());
    activationResponse.insert(activationResponse.end(), 40, 0);  // STDOBJREF
    appendU16(activationResponse, 2);  // DUALSTRINGARRAY entries
    appendU16(activationResponse, 2);
    appendU16(activationResponse, 0);
    appendU16(activationResponse, 0);
    appendU32(activationResponse, 0);  // method HRESULT
    RemoteCreateInstanceResult activated{};
    assert(decodeRemoteCreateInstanceResponse(
        activationResponse, requestedOpcServer, activated, error));
    assert(activated.hresult == 0 && !activated.interfaceObjectReference.empty());
    activationResponse.back() = 0x80;
    assert(!decodeRemoteCreateInstanceResponse(
        activationResponse, requestedOpcServer, activated, error));

    const std::vector<uint16_t> resolverArray = tcpBinding("192.168.1.141");
    const uint16_t resolverSecurityOffset =
        static_cast<uint16_t>(resolverArray.size() - 2);
    std::vector<uint8_t> standardObjref;
    appendU32(standardObjref, 0x574f454d);
    appendU32(standardObjref, 1);
    standardObjref.insert(standardObjref.end(), requestedOpcServer.begin(), requestedOpcServer.end());
    appendU32(standardObjref, 0);
    appendU32(standardObjref, 5);
    appendU64(standardObjref, 0x1122334455667788ULL);
    appendU64(standardObjref, 0x8877665544332211ULL);
    standardObjref.insert(standardObjref.end(), objectUuid.begin(), objectUuid.end());
    appendU16(standardObjref, static_cast<uint16_t>(resolverArray.size()));
    appendU16(standardObjref, resolverSecurityOffset);
    for (uint16_t entry : resolverArray) appendU16(standardObjref, entry);
    StandardObjectReference standard{};
    assert(decodeStandardObjectReference(
        standardObjref, requestedOpcServer, standard, error));
    assert(standard.oxid == 0x1122334455667788ULL &&
           standard.oid == 0x8877665544332211ULL && standard.ipid == objectUuid);
    assert(standard.resolverBindings.size() == 1 &&
           standard.resolverBindings[0].networkAddress == "192.168.1.141");
    standardObjref[0] = 0;
    assert(!decodeStandardObjectReference(
        standardObjref, requestedOpcServer, standard, error));

    const auto resolveRequest = encodeResolveOxid2Request(0x1122334455667788ULL);
    assert(resolveRequest.size() == 18 && resolveRequest[0] == 0x88 &&
           resolveRequest[8] == 1 && resolveRequest[16] == 7);
    const auto orpcThis = encodeOrpcThis(5, 7, objectUuid);
    assert(orpcThis.size() == 32 && orpcThis[0] == 5 && orpcThis[2] == 7 &&
           std::equal(objectUuid.begin(), objectUuid.end(), orpcThis.begin() + 12));
    const std::vector<uint16_t> exporterArray = tcpBinding("192.168.1.141[50123]");
    const uint16_t exporterSecurityOffset =
        static_cast<uint16_t>(exporterArray.size() - 2);
    std::vector<uint8_t> resolveStub;
    appendU32(resolveStub, 0x00020000);
    appendU32(resolveStub, static_cast<uint32_t>(exporterArray.size()));
    appendU16(resolveStub, static_cast<uint16_t>(exporterArray.size()));
    appendU16(resolveStub, exporterSecurityOffset);
    for (uint16_t entry : exporterArray) appendU16(resolveStub, entry);
    while (resolveStub.size() % 4 != 0) resolveStub.push_back(0xab);
    resolveStub.insert(resolveStub.end(), objectUuid.begin(), objectUuid.end());
    appendU32(resolveStub, 5);
    appendU16(resolveStub, 5);
    appendU16(resolveStub, 7);
    appendU32(resolveStub, 0);
    OxidResolution resolved{};
    assert(decodeResolveOxid2Response(resolveStub, resolved, error));
    assert(resolved.authenticationHint == 5 && resolved.comMajor == 5 &&
           resolved.comMinor == 7 && resolved.remoteUnknownIpid == objectUuid);
    std::string exporterHost;
    uint16_t exporterPort = 0;
    assert(selectTcpBinding(
        resolved.exporterBindings, "fallback", exporterHost, exporterPort, error));
    assert(exporterHost == "192.168.1.141" && exporterPort == 50123);
    assert(!selectTcpBinding(
        standard.resolverBindings, "fallback", exporterHost, exporterPort, error));

    const auto getStatusRequest = encodeGetStatusRequest(5, 7, objectUuid);
    assert(getStatusRequest == orpcThis);
    constexpr uint64_t windowsEpoch = 116444736000000000ULL;
    std::vector<uint8_t> getStatusStub;
    appendU32(getStatusStub, 0);  // ORPCTHAT flags
    appendU32(getStatusStub, 0);  // extensions
    appendU32(getStatusStub, 0x00020000);
    appendU64(getStatusStub, windowsEpoch + 10000);
    appendU64(getStatusStub, windowsEpoch + 123450000);
    appendU64(getStatusStub, windowsEpoch + 120000000);
    appendU32(getStatusStub, 1);
    appendU32(getStatusStub, 2);
    appendU32(getStatusStub, 3);
    appendU16(getStatusStub, 4);
    appendU16(getStatusStub, 5);
    appendU16(getStatusStub, 6);
    appendU16(getStatusStub, 0);
    appendU32(getStatusStub, 0x00020004);
    const std::vector<uint16_t> vendor = {'M','a','t','r','i','k','o','n',0};
    appendU32(getStatusStub, static_cast<uint32_t>(vendor.size()));
    appendU32(getStatusStub, 0);
    appendU32(getStatusStub, static_cast<uint32_t>(vendor.size()));
    for (uint16_t character : vendor) appendU16(getStatusStub, character);
    while (getStatusStub.size() % 4 != 0) getStatusStub.push_back(0);
    appendU32(getStatusStub, 0);
    ServerStatus serverStatus{};
    assert(decodeGetStatusResponse(getStatusStub, serverStatus, error));
    assert(serverStatus.serverState == 1 && serverStatus.groupCount == 2 &&
           serverStatus.currentUnixMilliseconds == 12345 &&
           serverStatus.vendorInfo == "Matrikon" && serverStatus.buildNumber == 6);
    getStatusStub.back() = 0x80;
    assert(!decodeGetStatusResponse(getStatusStub, serverStatus, error));

    const float deadband = 0.0f;
    std::vector<uint8_t> addGroupStub;
    assert(encodeAddGroupRequest(
        "G", true, 1000, 1, deadband, 0x0409, 5, 7, objectUuid,
        addGroupStub, error));
    assert(addGroupStub.size() == 88);
    assert(addGroupStub[32] == 2);
    assert(addGroupStub[36] == 0);
    assert(addGroupStub[40] == 2);
    assert(addGroupStub[44] == 'G');
    assert(addGroupStub[48] == 1);
    const auto itemManagementIid = opcItemManagementIid();
    assert(std::search(addGroupStub.begin(), addGroupStub.end(),
                       itemManagementIid.begin(), itemManagementIid.end()) !=
           addGroupStub.end());

    std::vector<uint8_t> itemManagementObjref = standardObjref;
    itemManagementObjref[0] = 0x4d;
    std::copy(itemManagementIid.begin(), itemManagementIid.end(),
              itemManagementObjref.begin() + 8);
    std::vector<uint8_t> addGroupResponse;
    appendU32(addGroupResponse, 0);
    appendU32(addGroupResponse, 0);
    appendU32(addGroupResponse, 42);
    appendU32(addGroupResponse, 500);
    appendU32(addGroupResponse, 0x00020000);
    appendU32(addGroupResponse, static_cast<uint32_t>(itemManagementObjref.size()));
    appendU32(addGroupResponse, static_cast<uint32_t>(itemManagementObjref.size()));
    addGroupResponse.insert(addGroupResponse.end(), itemManagementObjref.begin(),
                            itemManagementObjref.end());
    while (addGroupResponse.size() % 4 != 0) addGroupResponse.push_back(0);
    appendU32(addGroupResponse, 0);
    AddGroupResult addedGroup{};
    assert(decodeAddGroupResponse(addGroupResponse, addedGroup, error));
    assert(addedGroup.serverGroupHandle == 42 &&
           addedGroup.revisedUpdateRateMs == 500 &&
           addedGroup.itemManagementObjectReference == itemManagementObjref);

    std::vector<uint8_t> getGroupRequest;
    assert(encodeGetGroupByNameRequest(
        "G", opcSyncIoIid(), 5, 7, objectUuid, getGroupRequest, error));
    assert(getGroupRequest.size() == 64);
    assert(readU32(getGroupRequest.data() + 32) == 2);
    assert(readU32(getGroupRequest.data() + 40) == 2);
    std::vector<uint8_t> syncObjref = standardObjref;
    syncObjref[0] = 0x4d;
    const auto syncIid = opcSyncIoIid();
    std::copy(syncIid.begin(), syncIid.end(), syncObjref.begin() + 8);
    std::vector<uint8_t> getGroupResponse;
    appendU32(getGroupResponse, 0);
    appendU32(getGroupResponse, 0);
    appendU32(getGroupResponse, 0x00020000);
    appendU32(getGroupResponse, static_cast<uint32_t>(syncObjref.size()));
    appendU32(getGroupResponse, static_cast<uint32_t>(syncObjref.size()));
    getGroupResponse.insert(getGroupResponse.end(), syncObjref.begin(), syncObjref.end());
    while (getGroupResponse.size() % 4 != 0) getGroupResponse.push_back(0);
    appendU32(getGroupResponse, 0);
    InterfaceResult syncInterface{};
    assert(decodeGetGroupByNameResponse(getGroupResponse, syncInterface, error));
    assert(syncInterface.objectReference == syncObjref);

    uint16_t vartype = 0xffff;
    assert(vartypeFromName("auto", vartype) && vartype == 0);
    assert(vartypeFromName("float64", vartype) && vartype == 5);
    assert(vartypeFromName("VT_UI4", vartype) && vartype == 19);
    assert(!vartypeFromName("not-a-vartype", vartype));

    const std::vector<ItemDefinition> itemDefinitions{{
        7, "Random.Real8", "", true, 0}};
    std::vector<uint8_t> addItemsRequest;
    assert(encodeAddItemsRequest(
        itemDefinitions, 5, 7, objectUuid, addItemsRequest, error));
    assert(addItemsRequest.size() == 122);
    assert(readU32(addItemsRequest.data() + 32) == 1);
    assert(readU32(addItemsRequest.data() + 36) == 1);
    assert(readU32(addItemsRequest.data() + 48) == 1);
    assert(readU32(addItemsRequest.data() + 52) == 7);
    assert(readU32(addItemsRequest.data() + 56) == 0);
    assert(readU16(addItemsRequest.data() + 64) == 0);
    assert(readU32(addItemsRequest.data() + 68) == 1);
    assert(readU32(addItemsRequest.data() + 84) == 13);

    std::vector<uint8_t> addItemsResponse;
    appendU32(addItemsResponse, 0);
    appendU32(addItemsResponse, 0);
    appendU32(addItemsResponse, 0x00020000);
    appendU32(addItemsResponse, 1);
    appendU32(addItemsResponse, 42);
    appendU16(addItemsResponse, 5);
    appendU16(addItemsResponse, 0);
    appendU32(addItemsResponse, 3);
    appendU32(addItemsResponse, 0);
    appendU32(addItemsResponse, 0);
    appendU32(addItemsResponse, 0x00020004);
    appendU32(addItemsResponse, 1);
    appendU32(addItemsResponse, 0);
    appendU32(addItemsResponse, 0);
    std::vector<ItemResult> itemResults;
    uint32_t addItemsHresult = 0xffffffff;
    assert(decodeAddItemsResponse(
        addItemsResponse, itemDefinitions, itemResults, addItemsHresult, error));
    assert(addItemsHresult == 0 && itemResults.size() == 1);
    assert(itemResults[0].clientHandle == 7 &&
           itemResults[0].serverHandle == 42 &&
           itemResults[0].canonicalDataType == 5 &&
           itemResults[0].accessRights == 3 &&
           itemResults[0].hresult == 0);

    const auto syncReadRequest = encodeSyncReadRequest(
        2, {42}, 5, 7, objectUuid);
    assert(syncReadRequest.size() == 48);
    assert(readU32(syncReadRequest.data() + 32) == 2);
    assert(readU32(syncReadRequest.data() + 36) == 1);
    assert(readU32(syncReadRequest.data() + 40) == 1);
    assert(readU32(syncReadRequest.data() + 44) == 42);

    std::vector<uint8_t> syncReadResponse;
    appendU32(syncReadResponse, 0);
    appendU32(syncReadResponse, 0);
    appendU32(syncReadResponse, 0x00020000);
    appendU32(syncReadResponse, 1);
    appendU32(syncReadResponse, 7);
    const uint64_t itemFiletime = 116444736000000000ULL + 12345ULL * 10000ULL;
    appendU64(syncReadResponse, itemFiletime);
    appendU16(syncReadResponse, 0x00c0);
    appendU16(syncReadResponse, 0);
    appendU32(syncReadResponse, 0x00020004);
    appendU32(syncReadResponse, 0);  // align wireVARIANT to eight bytes
    appendU32(syncReadResponse, 16);
    appendU32(syncReadResponse, 0);
    appendU16(syncReadResponse, 5);
    appendU16(syncReadResponse, 0);
    appendU16(syncReadResponse, 0);
    appendU16(syncReadResponse, 0);
    appendU32(syncReadResponse, 5);
    appendU32(syncReadResponse, 0);  // align VT_R8 to eight bytes
    appendU64(syncReadResponse, 0x4029000000000000ULL);
    appendU32(syncReadResponse, 0x00020008);
    appendU32(syncReadResponse, 1);
    appendU32(syncReadResponse, 0);
    appendU32(syncReadResponse, 0);
    std::vector<SyncReadValue> syncValues;
    uint32_t syncReadHresult = 0xffffffff;
    assert(decodeSyncReadResponse(
        syncReadResponse, 1, syncValues, syncReadHresult, error));
    assert(syncReadHresult == 0 && syncValues.size() == 1);
    assert(syncValues[0].clientHandle == 7 &&
           syncValues[0].timestampUnixMilliseconds == 12345 &&
           syncValues[0].quality == 0x00c0 &&
           syncValues[0].vartype == 5 &&
           syncValues[0].kind == SyncValueKind::FloatingPoint &&
           syncValues[0].floatingValue == 12.5 &&
           syncValues[0].hresult == 0);

    std::vector<SyncWriteValue> writeValues(3);
    writeValues[0].vartype = 2;
    writeValues[0].signedValue = -32768;
    writeValues[1].vartype = 19;
    writeValues[1].unsignedValue = 4294967295ULL;
    writeValues[2].vartype = 8;
    writeValues[2].stringValue = "\xe4\xb8\xad\xe6\x96\x87\xe6\xb5\x8b\xe8\xaf\x95";
    std::vector<uint8_t> syncWriteRequest;
    error.clear();
    assert(encodeSyncWriteRequest(
        {42, 43, 44}, writeValues, 5, 7, objectUuid,
        syncWriteRequest, error));
    assert(readU32(syncWriteRequest.data() + 32) == 3);
    assert(readU32(syncWriteRequest.data() + 36) == 3);
    assert(readU32(syncWriteRequest.data() + 40) == 42);
    assert(readU32(syncWriteRequest.data() + 48) == 44);
    assert(readU32(syncWriteRequest.data() + 52) == 3);
    std::size_t writeVariantOffset = 68;
    std::vector<SyncReadValue> encodedWriteValues(3);
    for (SyncReadValue& value : encodedWriteValues) {
        assert(decodeWireVariant(
            syncWriteRequest, writeVariantOffset, value, error));
    }
    assert(encodedWriteValues[0].vartype == 2 &&
           encodedWriteValues[0].signedValue == -32768);
    assert(encodedWriteValues[1].vartype == 19 &&
           encodedWriteValues[1].unsignedValue == 4294967295ULL);
    assert(encodedWriteValues[2].vartype == 8 &&
           encodedWriteValues[2].stringValue ==
               "\xe4\xb8\xad\xe6\x96\x87\xe6\xb5\x8b\xe8\xaf\x95");

    std::vector<uint8_t> syncWriteResponse;
    appendU32(syncWriteResponse, 0);
    appendU32(syncWriteResponse, 0);
    appendU32(syncWriteResponse, 0x00020000);
    appendU32(syncWriteResponse, 3);
    appendU32(syncWriteResponse, 0);
    appendU32(syncWriteResponse, 0x80070057);
    appendU32(syncWriteResponse, 0);
    appendU32(syncWriteResponse, 0);
    std::vector<uint32_t> writeResults;
    uint32_t syncWriteHresult = 0xffffffff;
    assert(decodeSyncWriteResponse(
        syncWriteResponse, 3, writeResults, syncWriteHresult, error));
    assert(syncWriteHresult == 0 && writeResults.size() == 3 &&
           writeResults[0] == 0 && writeResults[1] == 0x80070057 &&
           writeResults[2] == 0);

    const auto callbackIid = unknownIid();
    const auto callbackObjref = encodeStandardObjectReference(
        callbackIid, 0x1122334455667788ULL, 0x8877665544332211ULL,
        objectUuid, "192.168.1.214", 45678);
    StandardObjectReference callbackReference{};
    assert(decodeStandardObjectReference(
        callbackObjref, callbackIid, callbackReference, error));
    assert(callbackReference.oxid == 0x1122334455667788ULL &&
           callbackReference.oid == 0x8877665544332211ULL);
    std::string callbackHost;
    uint16_t callbackPort = 0;
    assert(selectTcpBinding(callbackReference.resolverBindings, {}, callbackHost,
                            callbackPort, error));
    assert(callbackHost == "192.168.1.214" && callbackPort == 45678);

    const auto resolverObjref = encodeStandardObjectReference(
        callbackIid, 0x1122334455667788ULL, 0x8877665544332211ULL,
        objectUuid, "192.168.1.214", 0);
    StandardObjectReference resolverReference{};
    assert(decodeStandardObjectReference(
        resolverObjref, callbackIid, resolverReference, error));
    assert(resolverReference.resolverBindings.size() == 1 &&
           resolverReference.resolverBindings[0].networkAddress == "192.168.1.214");
    const uint16_t callbackResolverEntries = readU16(resolverObjref.data() + 64);
    const uint16_t callbackResolverSecurityOffset = readU16(resolverObjref.data() + 66);
    assert(callbackResolverEntries == callbackResolverSecurityOffset + 4);
    const auto unauthenticatedCallbackObjref = encodeStandardObjectReference(
        callbackIid, 0x1122334455667788ULL, 0x8877665544332211ULL,
        objectUuid, "192.168.1.214", 45678, 1, false);
    StandardObjectReference unauthenticatedCallbackReference{};
    assert(decodeStandardObjectReference(
        unauthenticatedCallbackObjref, callbackIid,
        unauthenticatedCallbackReference, error));
    const uint16_t unauthenticatedEntries =
        readU16(unauthenticatedCallbackObjref.data() + 64);
    const uint16_t unauthenticatedSecurityOffset =
        readU16(unauthenticatedCallbackObjref.data() + 66);
    assert(unauthenticatedEntries == unauthenticatedSecurityOffset + 2);

    const auto legacyResolve = encodeResolveOxidResponse(
        "192.168.1.214", 45678, objectUuid, 6);
    const auto modernResolve = encodeResolveOxid2Response(
        "192.168.1.214", 45678, objectUuid, 6);
    assert(legacyResolve.size() + 4 == modernResolve.size());
    const auto callbackCompatibilityResolve = encodeResolveOxid2Response(
        "192.168.1.214", 45678, objectUuid, 1, 5, 7, 0, false);
    OxidResolution callbackCompatibility{};
    assert(decodeResolveOxid2Response(
        callbackCompatibilityResolve, callbackCompatibility, error));
    assert(callbackCompatibility.authenticationHint == 1);
    const uint16_t compatibilityEntries =
        readU16(callbackCompatibilityResolve.data() + 8);
    const uint16_t compatibilitySecurityOffset =
        readU16(callbackCompatibilityResolve.data() + 10);
    assert(compatibilityEntries == compatibilitySecurityOffset + 2);
    const auto alive2 = encodeServerAlive2Response("192.168.1.214");
    assert(alive2.size() >= 28 && readU16(alive2.data()) == 5 &&
           readU16(alive2.data() + 2) == 7);
    const uint16_t aliveEntries = readU16(alive2.data() + 12);
    const uint16_t aliveSecurityOffset = readU16(alive2.data() + 14);
    assert(aliveEntries == aliveSecurityOffset + 4);
    const auto unauthenticatedAlive2 = encodeServerAlive2Response(
        "192.168.1.214", 5, 7, 0, false);
    const uint16_t unauthenticatedAliveEntries =
        readU16(unauthenticatedAlive2.data() + 12);
    const uint16_t unauthenticatedAliveSecurityOffset =
        readU16(unauthenticatedAlive2.data() + 14);
    assert(unauthenticatedAliveEntries ==
           unauthenticatedAliveSecurityOffset + 2);
    assert(readU16(alive2.data() + 16 + aliveSecurityOffset * 2) == 10);
    assert(readU16(alive2.data() + 16 + (aliveSecurityOffset - 1) * 2) == 0);

    const auto findRequest = encodeFindConnectionPointRequest(
        callbackIid, 5, 7, objectUuid);
    assert(findRequest.size() == 48 &&
           std::equal(callbackIid.begin(), callbackIid.end(), findRequest.begin() + 32));
    const auto adviseRequest = encodeAdviseRequest(callbackObjref, 5, 7, objectUuid);
    assert(readU32(adviseRequest.data() + 32) != 0);
    assert(readU32(adviseRequest.data() + 36) == callbackObjref.size());

    std::vector<uint8_t> dataChangeStub(32, 0);
    appendU32(dataChangeStub, 0);
    appendU32(dataChangeStub, 1);
    appendU32(dataChangeStub, 0xc0);
    appendU32(dataChangeStub, 0);
    appendU32(dataChangeStub, 1);
    appendU32(dataChangeStub, 1);
    appendU32(dataChangeStub, 7);
    appendU32(dataChangeStub, 1);
    appendU32(dataChangeStub, 0x20000);  // VARIANT referent ID
    appendU32(dataChangeStub, 0);  // VARIANT eight-byte alignment
    appendU32(dataChangeStub, 16);
    appendU32(dataChangeStub, 0);
    appendU16(dataChangeStub, 5);
    appendU16(dataChangeStub, 0);
    appendU16(dataChangeStub, 0);
    appendU16(dataChangeStub, 0);
    appendU32(dataChangeStub, 5);
    appendU32(dataChangeStub, 0);  // R8 eight-byte alignment
    appendU64(dataChangeStub, 0x4029000000000000ULL);
    appendU32(dataChangeStub, 1);
    appendU16(dataChangeStub, 0xc0);
    appendU16(dataChangeStub, 0);
    appendU32(dataChangeStub, 1);
    appendU64(dataChangeStub, itemFiletime);
    appendU32(dataChangeStub, 1);
    appendU32(dataChangeStub, 0);
    DataChange change{};
    assert(decodeDataChangeRequest(dataChangeStub, change, error));
    assert(change.values.size() == 1 && change.values[0].clientHandle == 7 &&
           change.values[0].quality == 0xc0 &&
           change.values[0].timestampUnixMilliseconds == 12345 &&
           change.values[0].floatingValue == 12.5);
    return 0;
}
