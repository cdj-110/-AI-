#include "wk_opcda.h"

#include "auth/ntlm.h"
#include "dcom/activation.h"
#include "dcom/object_reference.h"
#include "dcom/ping.h"
#include "opcda/browse.h"
#include "opcda/item_management.h"
#include "opcda/connection_point.h"
#include "opcda/data_callback.h"
#include "opcda/server.h"
#include "opcda/sync_io.h"
#include "rpc/endpoint_mapper.h"
#include "rpc/rpc_pdu.h"
#include "rpc/rpc_transport.h"
#include "rpc/callback_server.h"

#include <chrono>
#include <condition_variable>
#include <thread>
#include <algorithm>
#include <iterator>
#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <cstdint>
#include <memory>
#include <mutex>
#include <new>
#include <limits>
#include <string>
#include <unordered_map>
#include <utility>
#include <vector>

struct wk_opcda_handle {
    mutable std::mutex mutex;
    std::condition_variable pingWake;
    std::thread pingThread;
    bool pingStopping = false;
    uint64_t pingSetId = 0;
    uint16_t pingSequence = 1;
    std::vector<uint64_t> pingOids;
    std::string host;
    std::string progId;
    std::string clsid;
    std::string domain;
    std::string username;
    std::string password;
    std::string workstation;
    std::string securityMode;
    std::string authentication;
    std::string authenticationLevel;
    std::string groupName;
    std::string localBindIp;
    uint32_t localeId = 0;
    uint32_t requestedUpdateRateMs = 0;
    uint32_t connectTimeoutMs = 0;
    uint32_t requestTimeoutMs = 0;
    float deadband = 0;
    bool autoReconnect = true;
    std::string lastError;
    mutable std::string lastErrorSnapshot;
    wk_opcda_error_t lastErrorCode = WK_OPCDA_OK;
    wk_opcda_connection_state_t state = WK_OPCDA_STATE_DISCONNECTED;
    wk_opcda_security_mode_t securityModeCode = WK_OPCDA_SECURITY_MODERN;
    wk::opcda::rpc::TcpTransport endpointMapper;
    wk::opcda::rpc::TcpTransport activationTransport;
    wk::opcda::rpc::TcpTransport resolverTransport;
    wk::opcda::rpc::TcpTransport opcServerTransport;
    wk::opcda::rpc::TcpTransport itemManagementTransport;
    wk::opcda::rpc::TcpTransport syncIoTransport;
    wk::opcda::rpc::TcpTransport browseTransport;
    wk::opcda::rpc::TcpTransport connectionPointContainerTransport;
    wk::opcda::rpc::TcpTransport connectionPointTransport;
    uint16_t activationPort = 0;
    uint16_t opcServerPort = 0;
    wk::opcda::auth::NtlmSignatureState activationSecurity;
    wk::opcda::auth::NtlmSignatureState resolverSecurity;
    wk::opcda::auth::NtlmSignatureState opcServerSecurity;
    wk::opcda::auth::NtlmSignatureState itemManagementSecurity;
    wk::opcda::auth::NtlmSignatureState syncIoSecurity;
    wk::opcda::auth::NtlmSignatureState browseSecurity;
    wk::opcda::auth::NtlmSignatureState connectionPointContainerSecurity;
    wk::opcda::auth::NtlmSignatureState connectionPointSecurity;
    bool packetIntegrityEstablished = false;
    std::array<uint8_t, 16> activationCid{};
    std::vector<uint8_t> opcServerObjectReference;
    wk::opcda::dcom::StandardObjectReference opcServerReference;
    wk::opcda::dcom::OxidResolution oxidResolution;
    wk::opcda::opcda::ServerStatus opcServerStatus;
    wk::opcda::opcda::AddGroupResult group;
    wk::opcda::dcom::StandardObjectReference itemManagementReference;
    wk::opcda::dcom::StandardObjectReference syncIoReference;
    wk::opcda::dcom::StandardObjectReference browseReference;
    wk::opcda::dcom::StandardObjectReference connectionPointContainerReference;
    wk::opcda::dcom::StandardObjectReference connectionPointReference;
    uint32_t itemManagementCallId = 3;
    uint32_t syncIoCallId = 3;
    uint32_t activationObjectCallId = 4;
    uint32_t resolverObjectCallId = 4;
    uint32_t browseCallId = 3;
    uint32_t connectionPointContainerCallId = 3;
    uint32_t connectionPointCallId = 3;
    uint32_t subscriptionCookie = 0;
    wk_opcda_data_callback_t subscriptionCallback = nullptr;
    void* subscriptionUserData = nullptr;
    std::unique_ptr<wk::opcda::rpc::CallbackServer> callbackServer;
    std::unordered_map<uint32_t, uint32_t> itemServerHandles;
    std::unordered_map<uint32_t, uint16_t> itemCanonicalTypes;
    std::vector<std::string> readStrings;
    std::vector<std::vector<double>> readFloatArrays;
    std::vector<std::vector<std::string>> readStringArrays;
    std::vector<std::vector<const char*>> readStringArrayPointers;
    std::vector<std::vector<size_t>> readStringArrayLengths;
    std::string browseHost;
    uint16_t browsePort = 0;
    std::vector<std::string> browseNames;
    std::vector<std::string> browseItemIds;
    std::vector<std::string> browseDataTypes;
};

namespace {

struct CachedEndpoint {
    uint16_t port = 0;
    std::chrono::steady_clock::time_point expiresAt{};
};

std::mutex endpointCacheMutex;
std::unordered_map<std::string, CachedEndpoint> endpointCache;

std::string endpointCacheKey(const wk_opcda_handle_t* handle) {
    return handle->host + std::string(1, '\0') + handle->localBindIp;
}

bool loadCachedEndpoint(const wk_opcda_handle_t* handle, uint16_t& port) {
    std::lock_guard<std::mutex> guard(endpointCacheMutex);
    const auto found = endpointCache.find(endpointCacheKey(handle));
    if (found == endpointCache.end()) return false;
    if (found->second.expiresAt <= std::chrono::steady_clock::now()) {
        endpointCache.erase(found);
        return false;
    }
    port = found->second.port;
    return port != 0;
}

void storeCachedEndpoint(const wk_opcda_handle_t* handle, uint16_t port) {
    std::lock_guard<std::mutex> guard(endpointCacheMutex);
    endpointCache[endpointCacheKey(handle)] = CachedEndpoint{
        port, std::chrono::steady_clock::now() + std::chrono::minutes(5)};
}

void invalidateCachedEndpoint(const wk_opcda_handle_t* handle) {
    std::lock_guard<std::mutex> guard(endpointCacheMutex);
    endpointCache.erase(endpointCacheKey(handle));
}

std::string copyString(const char* value) {
    return value == nullptr ? std::string{} : std::string(value);
}

bool buildSyncWriteValue(const wk_opcda_value_t& input, uint16_t canonicalType,
                         wk::opcda::opcda::SyncWriteValue& output,
                         std::string& error) {
    constexpr uint16_t arrayFlag = 0x2000;
    if ((canonicalType & arrayFlag) != 0) {
        error = "ARRAY writes are not supported";
        return false;
    }
    uint16_t vartype = canonicalType;
    if (vartype == 0 || vartype == 12) {  // VT_EMPTY / VT_VARIANT: infer the input type.
        switch (input.type) {
            case WK_OPCDA_VALUE_BOOL: vartype = 11; break;
            case WK_OPCDA_VALUE_INT64: vartype = 20; break;
            case WK_OPCDA_VALUE_UINT64: vartype = 21; break;
            case WK_OPCDA_VALUE_FLOAT64: vartype = 5; break;
            case WK_OPCDA_VALUE_STRING: vartype = 8; break;
            case WK_OPCDA_VALUE_DATETIME: vartype = 7; break;
            default:
                error = "cannot infer a VARIANT type for the write value";
                return false;
        }
    }
    output = wk::opcda::opcda::SyncWriteValue{};
    output.vartype = vartype;
    auto signedInput = [&]() -> int64_t {
        if (input.type == WK_OPCDA_VALUE_BOOL) return input.int64_value == 0 ? 0 : 1;
        return input.int64_value;
    };
    switch (vartype) {
        case 11:  // VT_BOOL
            if (input.type != WK_OPCDA_VALUE_BOOL && input.type != WK_OPCDA_VALUE_INT64 &&
                input.type != WK_OPCDA_VALUE_UINT64) {
                error = "BOOL write requires a boolean or integer value";
                return false;
            }
            output.signedValue = input.type == WK_OPCDA_VALUE_UINT64
                ? (input.uint64_value == 0 ? 0 : 1)
                : (signedInput() == 0 ? 0 : 1);
            return true;
        case 16:  // VT_I1
        case 2:   // VT_I2
        case 3:   // VT_I4
        case 20:  // VT_I8
        case 22: {  // VT_INT
            if (input.type != WK_OPCDA_VALUE_INT64) {
                error = "signed integer write requires a signed integer value";
                return false;
            }
            const int bits = vartype == 16 ? 8 : vartype == 2 ? 16 :
                (vartype == 3 || vartype == 22) ? 32 : 64;
            if (bits < 64) {
                const int64_t minimum = -(int64_t{1} << (bits - 1));
                const int64_t maximum = (int64_t{1} << (bits - 1)) - 1;
                if (input.int64_value < minimum || input.int64_value > maximum) {
                    error = "signed integer write value is outside the canonical VARIANT range";
                    return false;
                }
            }
            output.signedValue = input.int64_value;
            return true;
        }
        case 17:  // VT_UI1
        case 18:  // VT_UI2
        case 19:  // VT_UI4
        case 21:  // VT_UI8
        case 23: {  // VT_UINT
            if (input.type != WK_OPCDA_VALUE_UINT64) {
                error = "unsigned integer write requires an unsigned integer value";
                return false;
            }
            const int bits = vartype == 17 ? 8 : vartype == 18 ? 16 :
                (vartype == 19 || vartype == 23) ? 32 : 64;
            const uint64_t maximum = bits == 64
                ? std::numeric_limits<uint64_t>::max()
                : (uint64_t{1} << bits) - 1;
            if (input.uint64_value > maximum) {
                error = "unsigned integer write value is outside the canonical VARIANT range";
                return false;
            }
            output.unsignedValue = input.uint64_value;
            return true;
        }
        case 4:  // VT_R4
        case 5:  // VT_R8
            if (input.type != WK_OPCDA_VALUE_FLOAT64 || !std::isfinite(input.float64_value) ||
                (vartype == 4 && std::abs(input.float64_value) > std::numeric_limits<float>::max())) {
                error = "floating-point write value is invalid or outside the canonical VARIANT range";
                return false;
            }
            output.floatingValue = input.float64_value;
            return true;
        case 6: {  // VT_CY, signed integer scaled by 10,000.
            if (input.type != WK_OPCDA_VALUE_FLOAT64 || !std::isfinite(input.float64_value)) {
                error = "currency write requires a finite numeric value";
                return false;
            }
            const long double scaled = static_cast<long double>(input.float64_value) * 10000.0L;
            if (scaled < static_cast<long double>(std::numeric_limits<int64_t>::min()) ||
                scaled > static_cast<long double>(std::numeric_limits<int64_t>::max())) {
                error = "currency write value is outside the VT_CY range";
                return false;
            }
            output.signedValue = static_cast<int64_t>(std::llround(scaled));
            return true;
        }
        case 7:  // VT_DATE
            if (input.type != WK_OPCDA_VALUE_DATETIME) {
                error = "datetime write requires a parsed date/time value";
                return false;
            }
            output.datetimeUnixMilliseconds = input.int64_value;
            return true;
        case 8:  // VT_BSTR
            if (input.type != WK_OPCDA_VALUE_STRING ||
                (input.string_value == nullptr && input.string_length != 0)) {
                error = "string write requires a valid UTF-8 string";
                return false;
            }
            output.stringValue.assign(input.string_value == nullptr ? "" : input.string_value,
                                      input.string_length);
            return true;
        default:
            error = "unsupported canonical VARIANT type " + std::to_string(vartype);
            return false;
    }
}

wk_opcda_error_t fail(wk_opcda_handle_t* handle, wk_opcda_error_t code, std::string message) {
    if (handle != nullptr) {
        if (handle->state == WK_OPCDA_STATE_CONNECTED && code != WK_OPCDA_ERR_CANCELLED) {
            std::fprintf(stderr, "[OPCDA][SESSION] host=%s code=%d cause=%s\n",
                handle->host.c_str(), static_cast<int>(code), message.c_str());
        }
        handle->lastError = std::move(message);
        handle->lastErrorCode = code;
        handle->state = code == WK_OPCDA_ERR_CANCELLED ? WK_OPCDA_STATE_DISCONNECTED : WK_OPCDA_STATE_FAULT;
    }
    return code;
}

bool applySecurityMode(wk_opcda_handle_t* handle) {
    if (handle->securityMode.empty()) {
        handle->securityMode = handle->authenticationLevel == "connect" ? "legacy_connect" : "modern";
    }
    if (handle->securityMode == "modern") {
        handle->securityModeCode = WK_OPCDA_SECURITY_MODERN;
        handle->authenticationLevel = "packet_integrity";
        return true;
    }
    if (handle->securityMode == "packet_integrity") {
        handle->securityModeCode = WK_OPCDA_SECURITY_PACKET_INTEGRITY;
        handle->authenticationLevel = "packet_integrity";
        return true;
    }
    if (handle->securityMode == "packet_privacy") {
        handle->securityModeCode = WK_OPCDA_SECURITY_PACKET_PRIVACY;
        handle->authenticationLevel = "packet_privacy";
        return true;
    }
    if (handle->securityMode == "legacy_connect") {
        handle->securityModeCode = WK_OPCDA_SECURITY_LEGACY_CONNECT;
        handle->authenticationLevel = "connect";
        return true;
    }
    return false;
}

bool receiveRpcResponse(wk::opcda::rpc::TcpTransport& transport,
                        std::chrono::milliseconds timeout, uint32_t callId,
                        uint16_t contextId, wk::opcda::rpc::RpcResponse& response,
                        std::string& error) {
    std::vector<std::vector<uint8_t>> fragments;
    std::size_t totalBytes = 0;
    for (std::size_t index = 0; index < 4096; ++index) {
        std::vector<uint8_t> fragment;
        if (!transport.receiveFragment(fragment, timeout, error)) return false;
        wk::opcda::rpc::CommonHeader header{};
        if (!wk::opcda::rpc::decodeCommonHeader(fragment.data(), fragment.size(), header, error)) return false;
        if (header.callId != callId) {
            error = "received an RPC fragment for an unexpected call ID";
            return false;
        }
        totalBytes += fragment.size();
        if (totalBytes > 16 * 1024 * 1024) {
            error = "RPC response exceeds the 16 MiB safety limit";
            return false;
        }
        const bool last = (header.flags & 0x02) != 0;
        fragments.push_back(std::move(fragment));
        if (last) return wk::opcda::rpc::decodeResponseFragments(fragments, callId, contextId, response, error);
    }
    error = "RPC response exceeds the 4096-fragment safety limit";
    return false;
}

bool invokeProtectedRpc(
    wk::opcda::rpc::TcpTransport& transport,
    wk::opcda::auth::NtlmSignatureState& security, bool integrity,
    bool packetPrivacy,
    uint32_t authenticationContextId, uint32_t callId, uint16_t contextId,
    uint16_t operationNumber, const std::vector<uint8_t>& stub,
    std::chrono::milliseconds timeout, wk::opcda::rpc::RpcResponse& response,
    std::string& error,
    const std::array<uint8_t, 16>* objectUuid = nullptr) {
    std::vector<std::vector<uint8_t>> requests = objectUuid == nullptr
        ? wk::opcda::rpc::encodeRequest(callId, contextId, operationNumber, stub)
        : wk::opcda::rpc::encodeObjectRequest(
              callId, contextId, operationNumber, *objectUuid, stub);
    for (std::vector<uint8_t>& fragment : requests) {
        if (integrity && !wk::opcda::rpc::protectPacketIntegrity(
                             fragment, authenticationContextId, security, error,
                             packetPrivacy)) {
            error = "sign protected RPC request failed: " + error;
            return false;
        }
        if (!transport.sendAll(fragment.data(), fragment.size(),
                               timeout, error)) {
            error = "send protected RPC request failed: " + error;
            return false;
        }
    }
    std::vector<std::vector<uint8_t>> responseFragments;
    std::size_t totalBytes = 0;
    for (std::size_t index = 0; index < 4096; ++index) {
        std::vector<uint8_t> fragment;
        if (!transport.receiveFragment(fragment, timeout, error)) {
            error = "receive protected RPC response failed: " + error;
            return false;
        }
        wk::opcda::rpc::CommonHeader header{};
        if (!wk::opcda::rpc::decodeCommonHeader(
                fragment.data(), fragment.size(), header, error) ||
            header.callId != callId) {
            error = "protected RPC response has an invalid header: " + error;
            return false;
        }
        totalBytes += fragment.size();
        if (totalBytes > 16 * 1024 * 1024) {
            error = "protected RPC response exceeds the 16 MiB safety limit";
            return false;
        }
        if (integrity && !wk::opcda::rpc::verifyPacketIntegrity(
                             fragment, authenticationContextId, security, error,
                             packetPrivacy)) {
            std::string detail = error +
                " (packet_type=" + std::to_string(static_cast<int>(header.packetType));
            if (header.packetType == wk::opcda::rpc::PacketType::Fault &&
                fragment.size() >= 28) {
                const uint32_t status = static_cast<uint32_t>(fragment[24]) |
                    (static_cast<uint32_t>(fragment[25]) << 8) |
                    (static_cast<uint32_t>(fragment[26]) << 16) |
                    (static_cast<uint32_t>(fragment[27]) << 24);
                detail += ", fault_status=" + std::to_string(status);
            }
            error = "verify protected RPC response failed: " + detail + ")";
            return false;
        }
        const bool last = (header.flags & 0x02) != 0;
        responseFragments.push_back(std::move(fragment));
        if (last) {
            return wk::opcda::rpc::decodeResponseFragments(
                responseFragments, callId, contextId, response, error);
        }
    }
    error = "protected RPC response exceeds the 4096-fragment safety limit";
    return false;
}

// Caller holds handle->mutex; resolver security sequence and call IDs are shared
// with ResolveOxid/RemQueryInterface and must never be used concurrently.
bool maintainObjectLifetime(wk_opcda_handle_t* handle, bool release,
                            std::string& error) {
    std::vector<uint64_t> current;
    if (!release) {
        for (const auto* ref : {&handle->opcServerReference,
                &handle->itemManagementReference, &handle->syncIoReference,
                &handle->browseReference, &handle->connectionPointContainerReference,
                &handle->connectionPointReference}) {
            // SORF_NOPING exempts a reference from distributed garbage collection.
            if (ref->oid != 0 && (ref->standardFlags & 0x1000) == 0)
                current.push_back(ref->oid);
        }
        std::sort(current.begin(), current.end());
        current.erase(std::unique(current.begin(), current.end()), current.end());
    }
    if (current.empty() && handle->pingSetId == 0) return true;
    std::vector<uint64_t> add, remove;
    std::set_difference(current.begin(), current.end(), handle->pingOids.begin(),
        handle->pingOids.end(), std::back_inserter(add));
    std::set_difference(handle->pingOids.begin(), handle->pingOids.end(),
        current.begin(), current.end(), std::back_inserter(remove));
    const bool complex = handle->pingSetId == 0 || !add.empty() || !remove.empty();
    const auto request = complex
        ? wk::opcda::dcom::encodeComplexPing(handle->pingSetId, handle->pingSequence, add, remove)
        : wk::opcda::dcom::encodeSimplePing(handle->pingSetId);
    wk::opcda::rpc::RpcResponse response{};
    const auto timeout = std::chrono::milliseconds(release ? 1000 :
        std::min<uint32_t>(handle->requestTimeoutMs == 0 ? 10000 : handle->requestTimeoutMs, 5000));
    if (!invokeProtectedRpc(handle->resolverTransport, handle->resolverSecurity,
            handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT,
            handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY,
            1, handle->resolverObjectCallId++, 0, complex ? 2 : 1,
            request, timeout, response, error)) return false;
    if (response.fault) {
        error = "DCOM ping RPC Fault status=" + std::to_string(response.faultStatus);
        return false;
    }
    if (!wk::opcda::dcom::decodePingResponse(response.stub, complex, handle->pingSetId, error))
        return false;
    if (complex) ++handle->pingSequence;
    handle->pingOids = std::move(current);
    if (release) handle->pingSetId = 0;
    return true;
}

void stopObjectLifetimeWorker(wk_opcda_handle_t* handle) {
    {
        std::lock_guard<std::mutex> guard(handle->mutex);
        handle->pingStopping = true;
        handle->pingWake.notify_all();
    }
    // Never join while holding the mutex needed by the worker.
    if (handle->pingThread.joinable()) handle->pingThread.join();
}

bool startObjectLifetimeWorker(wk_opcda_handle_t* handle, std::string& error) {
    if (!maintainObjectLifetime(handle, false, error)) return false;
    handle->pingStopping = false;
    try {
        handle->pingThread = std::thread([handle] {
            std::unique_lock<std::mutex> lock(handle->mutex);
            while (!handle->pingWake.wait_for(lock, std::chrono::seconds(60),
                    [handle] { return handle->pingStopping; })) {
                if (handle->state != WK_OPCDA_STATE_CONNECTED) break;
                std::string pingError;
                if (!maintainObjectLifetime(handle, false, pingError)) {
                    fail(handle, WK_DISCONNECTED, "[DCOM] object lifetime keepalive failed: " + pingError);
                    std::fprintf(stderr, "[OPCDA][KEEPALIVE] host=%s failed: %s\n",
                        handle->host.c_str(), pingError.c_str());
                    break;
                }
                std::fprintf(stderr, "[OPCDA][KEEPALIVE] host=%s OK objects=%zu\n",
                    handle->host.c_str(), handle->pingOids.size());
            }
        });
    } catch (const std::exception& ex) {
        error = std::string("cannot start DCOM keepalive: ") + ex.what();
        return false;
    }
    std::fprintf(stderr, "[OPCDA][KEEPALIVE] host=%s registered objects=%zu interval=60s\n",
        handle->host.c_str(), handle->pingOids.size());
    return true;
}

wk_opcda_error_t authenticateRpcEndpoint(
    wk_opcda_handle_t* handle, wk::opcda::rpc::TcpTransport& transport,
    wk::opcda::auth::NtlmSignatureState& security,
    const wk::opcda::rpc::SyntaxId& abstractSyntax, uint16_t diagnosticOperation,
    bool requireSignedResponse, std::chrono::milliseconds requestTimeout,
    std::string& error, const std::array<uint8_t, 16>* diagnosticObjectUuid = nullptr,
    const std::vector<uint8_t>* diagnosticStub = nullptr) {
    constexpr uint8_t ntlmAuthenticationType = 10;
    constexpr uint32_t authenticationContextId = 1;
    constexpr uint32_t bindCallId = 1;
    constexpr uint16_t presentationContextId = 0;
    const bool integrity = handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;
    const bool packetPrivacy =
        handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY;
    const uint8_t authenticationLevel = packetPrivacy ? 6 : integrity ? 5 : 2;
    const std::vector<uint8_t> negotiate =
        wk::opcda::auth::encodeNtlmNegotiate(integrity, packetPrivacy);
    const std::vector<uint8_t> bind = wk::opcda::rpc::encodeAuthenticatedBind(
        bindCallId, presentationContextId, abstractSyntax,
        authenticationLevel, authenticationContextId, negotiate);
    if (!transport.sendAll(bind.data(), bind.size(), requestTimeout, error)) {
        error = "[AUTH] send NTLM Negotiate Bind failed: " + error;
        return WK_NTLM_NEGOTIATE_FAILED;
    }
    std::vector<uint8_t> bindAck;
    if (!transport.receiveFragment(bindAck, requestTimeout, error)) {
        error = "[AUTH] receive NTLM Challenge BindAck failed: " + error;
        return WK_NTLM_CHALLENGE_FAILED;
    }
    if (!wk::opcda::rpc::validateBindAck(bindAck, bindCallId, error)) {
        error = "[AUTH] authenticated BindAck invalid: " + error;
        return WK_RPC_BIND_FAILED;
    }
    wk::opcda::rpc::CommonHeader bindAckHeader{};
    if (!wk::opcda::rpc::decodeCommonHeader(bindAck.data(), bindAck.size(), bindAckHeader, error)) {
        return WK_NTLM_CHALLENGE_FAILED;
    }
    wk::opcda::rpc::SecurityTrailer bindAckSecurity{};
    std::size_t bindAckBodyEnd = 0;
    if (!wk::opcda::rpc::decodeSecurityTrailer(bindAck, bindAckHeader, bindAckSecurity,
                                                bindAckBodyEnd, error) ||
        bindAckSecurity.authenticationType != ntlmAuthenticationType ||
        bindAckSecurity.authenticationLevel != authenticationLevel ||
        bindAckSecurity.contextId != authenticationContextId ||
        bindAckSecurity.token.empty()) {
        error = "[AUTH] BindAck did not contain the expected NTLM Challenge token";
        return WK_NTLM_CHALLENGE_FAILED;
    }
    wk::opcda::auth::NtlmChallenge challenge{};
    if (!wk::opcda::auth::parseNtlmChallenge(bindAckSecurity.token, integrity, challenge, error)) {
        error = "[AUTH] NTLM Challenge validation failed: " + error;
        return WK_NTLM_CHALLENGE_FAILED;
    }
    if (std::getenv("WK_OPCDA_AUTH_DIAGNOSTICS") != nullptr) {
        std::fprintf(stderr,
                     "[AUTH-DIAG] bindAckFlags=%u authType=%u authLevel=%u "
                     "challengeFlags=%u challengeBytes=%zu targetInfoBytes=%zu "
                     "bindAck=",
                     static_cast<unsigned>(bindAckHeader.flags),
                     static_cast<unsigned>(bindAckSecurity.authenticationType),
                     static_cast<unsigned>(bindAckSecurity.authenticationLevel),
                     static_cast<unsigned>(challenge.flags),
                     challenge.rawMessage.size(), challenge.targetInfo.size());
        for (const uint8_t byte : bindAck) {
            std::fprintf(stderr, "%02x", static_cast<unsigned>(byte));
        }
        std::fprintf(stderr, " challenge=");
        for (const uint8_t byte : challenge.rawMessage) {
            std::fprintf(stderr, "%02x", static_cast<unsigned>(byte));
        }
        std::fprintf(stderr, "\n");
    }
    wk::opcda::auth::NtlmAuthenticateResult authenticated{};
    const wk::opcda::auth::NtlmCredentials credentials{
        handle->username, handle->password, handle->domain, handle->workstation};
    if (!wk::opcda::auth::encodeNtlmAuthenticate(
            credentials, negotiate, challenge, wk::opcda::auth::randomNtlmClientNonce(),
            authenticated, error)) {
        error = "[AUTH] build NTLM Authenticate failed: " + error;
        return WK_NTLM_AUTH_FAILED;
    }
    if (packetPrivacy &&
        (authenticated.negotiatedFlags & wk::opcda::auth::kNtlmNegotiateSeal) == 0) {
        error = "[AUTH] NTLM server did not negotiate message sealing for Packet Privacy";
        return WK_RPC_AUTH_LEVEL_UNSUPPORTED;
    }
    if (integrity && !security.initialize(authenticated)) {
        error = "[AUTH] initialize NTLM signing state failed";
        return WK_RPC_PACKET_SIGN_FAILED;
    }
    const std::vector<uint8_t> auth3 = wk::opcda::rpc::encodeAuth3(
        bindCallId, ntlmAuthenticationType, authenticationLevel,
        authenticationContextId, authenticated.message);
    if (!transport.sendAll(auth3.data(), auth3.size(), requestTimeout, error)) {
        error = "[AUTH] send NTLM Authenticate Auth3 failed: " + error;
        return WK_NTLM_AUTH_FAILED;
    }

    // The caller selects either a side-effect-free ServerAlive2 request or an
    // invalid operation used only to confirm that the activation association
    // accepts the client's protected request.
    constexpr uint32_t diagnosticCallId = 2;
    const std::vector<uint8_t> emptyDiagnosticStub;
    const std::vector<uint8_t>& protectedDiagnosticStub = diagnosticStub == nullptr
        ? emptyDiagnosticStub : *diagnosticStub;
    std::vector<std::vector<uint8_t>> diagnostic = diagnosticObjectUuid == nullptr
        ? wk::opcda::rpc::encodeRequest(
              diagnosticCallId, presentationContextId, diagnosticOperation,
              protectedDiagnosticStub)
        : wk::opcda::rpc::encodeObjectRequest(
              diagnosticCallId, presentationContextId, diagnosticOperation,
              *diagnosticObjectUuid, protectedDiagnosticStub);
    for (std::vector<uint8_t>& fragment : diagnostic) {
        if (integrity && !wk::opcda::rpc::protectPacketIntegrity(
                             fragment, authenticationContextId,
                             security, error, packetPrivacy)) {
            error = "[AUTH] sign RPC diagnostic request failed: " + error;
            return WK_RPC_PACKET_SIGN_FAILED;
        }
        if (!transport.sendAll(fragment.data(), fragment.size(), requestTimeout, error)) {
            error = "[AUTH] send authenticated RPC diagnostic request failed: " + error;
            return WK_NTLM_AUTH_FAILED;
        }
    }
    std::vector<std::vector<uint8_t>> responseFragments;
    for (std::size_t index = 0; index < 64; ++index) {
        std::vector<uint8_t> fragment;
        if (!transport.receiveFragment(fragment, requestTimeout, error)) {
            error = "[AUTH] authenticated RPC diagnostic response failed: " + error;
            return WK_NTLM_AUTH_FAILED;
        }
        wk::opcda::rpc::CommonHeader header{};
        if (!wk::opcda::rpc::decodeCommonHeader(fragment.data(), fragment.size(), header, error) ||
            header.callId != diagnosticCallId) {
            error = "[AUTH] authenticated RPC diagnostic response is malformed: " + error;
            return WK_NTLM_AUTH_FAILED;
        }
        if (integrity) {
            wk::opcda::rpc::SecurityTrailer verifier{};
            std::size_t bodyEnd = 0;
            if (!wk::opcda::rpc::decodeSecurityTrailer(fragment, header, verifier, bodyEnd, error)) {
                error = "[AUTH] malformed RPC authentication trailer: " + error;
                return WK_NTLM_AUTH_FAILED;
            }
            if (verifier.token.empty()) {
                uint32_t status = 0;
                if (header.packetType == wk::opcda::rpc::PacketType::Fault && fragment.size() >= 28) {
                    status = static_cast<uint32_t>(fragment[24]) |
                        (static_cast<uint32_t>(fragment[25]) << 8) |
                        (static_cast<uint32_t>(fragment[26]) << 16) |
                        (static_cast<uint32_t>(fragment[27]) << 24);
                }
                // nca_op_rng_error is generated after the RPC runtime has
                // accepted and verified the protected request. Windows emits
                // this dispatch Fault without an authentication verifier.
                constexpr uint32_t operationOutOfRange = 0x1c010002;
                constexpr uint32_t operationOutOfRangeHresult = 0x800706d1;
                if (!requireSignedResponse &&
                    (status == operationOutOfRange ||
                     status == operationOutOfRangeHresult)) {
                    const bool last = (header.flags & 0x02) != 0;
                    responseFragments.push_back(std::move(fragment));
                    if (last) break;
                    continue;
                }
                std::string rejection = "[AUTH] server returned an unsigned RPC packet after NTLM Authenticate"
                    " (packet_type=" + std::to_string(static_cast<int>(header.packetType));
                if (status != 0) {
                    rejection += ", fault_status=" + std::to_string(status);
                }
                error = rejection + ")";
                return WK_NTLM_AUTH_FAILED;
            }
            if (!wk::opcda::rpc::verifyPacketIntegrity(
                    fragment, authenticationContextId, security, error,
                    packetPrivacy)) {
                error = "[AUTH] " + error;
                return WK_RPC_PACKET_VERIFY_FAILED;
            }
        }
        const bool last = (header.flags & 0x02) != 0;
        responseFragments.push_back(std::move(fragment));
        if (last) break;
    }
    wk::opcda::rpc::RpcResponse diagnosticResponse{};
    if (!wk::opcda::rpc::decodeResponseFragments(
            responseFragments, diagnosticCallId, presentationContextId,
            diagnosticResponse, error)) {
        error = "[AUTH] decode authenticated RPC diagnostic response failed: " + error;
        return integrity ? WK_RPC_PACKET_VERIFY_FAILED : WK_NTLM_AUTH_FAILED;
    }
    return WK_OPCDA_OK;
}

wk_opcda_error_t authenticateConnectedActivation(wk_opcda_handle_t* handle,
                                                 std::chrono::milliseconds requestTimeout) {
    std::string error;
    constexpr uint16_t invalidOperation = 0xffff;
    const wk_opcda_error_t authenticated = authenticateRpcEndpoint(
        handle, handle->activationTransport, handle->activationSecurity,
        wk::opcda::rpc::remoteScmActivatorSyntax(), invalidOperation, false,
        requestTimeout, error);
    if (authenticated != WK_OPCDA_OK) {
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, authenticated, std::move(error));
    }
    if (!handle->resolverTransport.connect(handle->host, handle->activationPort,
                                           requestTimeout, error, handle->localBindIp)) {
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, WK_RPC_CONNECT_FAILED,
                    "[AUTH] connect object-resolver association failed: " + error);
    }
    constexpr uint16_t serverAlive2Operation = 5;
    const wk_opcda_error_t resolverAuthenticated = authenticateRpcEndpoint(
        handle, handle->resolverTransport, handle->resolverSecurity,
        wk::opcda::rpc::objectExporterSyntax(), serverAlive2Operation, true,
        requestTimeout, error);
    if (resolverAuthenticated != WK_OPCDA_OK) {
        handle->resolverTransport.close();
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, resolverAuthenticated,
                    "[AUTH] object-resolver association failed: " + error);
    }
    handle->packetIntegrityEstablished =
        handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;

    handle->state = WK_OPCDA_STATE_ACTIVATING;
    handle->activationCid = wk::opcda::dcom::randomCid();
    std::vector<uint8_t> activationStub;
    std::string activationError;
    if (!wk::opcda::dcom::encodeRemoteCreateInstanceRequest(
            handle->clsid, wk::opcda::dcom::opcServerIid(), handle->activationCid,
            activationStub, activationError)) {
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, WK_DCOM_REMOTE_ACTIVATION_FAILED,
                    "[DCOM] build RemoteCreateInstance request failed: " + activationError);
    }
    constexpr uint32_t activationCallId = 3;
    constexpr uint16_t activationContextId = 0;
    constexpr uint16_t remoteCreateInstanceOperation = 4;
    std::vector<std::vector<uint8_t>> activationRequests = wk::opcda::rpc::encodeRequest(
        activationCallId, activationContextId, remoteCreateInstanceOperation, activationStub);
    const bool integrity = handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;
    const bool packetPrivacy =
        handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY;
    for (std::vector<uint8_t>& fragment : activationRequests) {
        if (integrity && !wk::opcda::rpc::protectPacketIntegrity(
                             fragment, 1, handle->activationSecurity, activationError,
                             packetPrivacy)) {
            handle->activationTransport.close();
            handle->endpointMapper.close();
            return fail(handle, WK_RPC_PACKET_SIGN_FAILED,
                        "[DCOM] sign RemoteCreateInstance request failed: " + activationError);
        }
        if (!handle->activationTransport.sendAll(fragment.data(), fragment.size(),
                                                  requestTimeout, activationError)) {
            handle->activationTransport.close();
            handle->endpointMapper.close();
            return fail(handle, WK_DCOM_REMOTE_ACTIVATION_FAILED,
                        "[DCOM] send RemoteCreateInstance request failed: " + activationError);
        }
    }

    std::vector<std::vector<uint8_t>> activationResponses;
    std::size_t responseBytes = 0;
    for (std::size_t index = 0; index < 4096; ++index) {
        std::vector<uint8_t> fragment;
        if (!handle->activationTransport.receiveFragment(fragment, requestTimeout,
                                                          activationError)) {
            handle->activationTransport.close();
            handle->endpointMapper.close();
            return fail(handle, WK_DCOM_REMOTE_ACTIVATION_FAILED,
                        "[DCOM] receive RemoteCreateInstance response failed: " + activationError);
        }
        wk::opcda::rpc::CommonHeader header{};
        if (!wk::opcda::rpc::decodeCommonHeader(fragment.data(), fragment.size(),
                                                 header, activationError) ||
            header.callId != activationCallId) {
            handle->activationTransport.close();
            handle->endpointMapper.close();
            return fail(handle, WK_DCOM_REMOTE_ACTIVATION_FAILED,
                        "[DCOM] malformed RemoteCreateInstance response: " + activationError);
        }
        responseBytes += fragment.size();
        if (responseBytes > 16 * 1024 * 1024) {
            handle->activationTransport.close();
            handle->endpointMapper.close();
            return fail(handle, WK_DCOM_REMOTE_ACTIVATION_FAILED,
                        "[DCOM] RemoteCreateInstance response exceeds 16 MiB");
        }
        if (integrity && !wk::opcda::rpc::verifyPacketIntegrity(
                             fragment, 1, handle->activationSecurity, activationError,
                             packetPrivacy)) {
            std::string detail = activationError +
                " (packet_type=" + std::to_string(static_cast<int>(header.packetType));
            if (header.packetType == wk::opcda::rpc::PacketType::Fault && fragment.size() >= 28) {
                const uint32_t status = static_cast<uint32_t>(fragment[24]) |
                    (static_cast<uint32_t>(fragment[25]) << 8) |
                    (static_cast<uint32_t>(fragment[26]) << 16) |
                    (static_cast<uint32_t>(fragment[27]) << 24);
                detail += ", fault_status=" + std::to_string(status);
            }
            detail += ")";
            handle->activationTransport.close();
            handle->endpointMapper.close();
            return fail(handle, WK_RPC_PACKET_VERIFY_FAILED,
                        "[DCOM] RemoteCreateInstance response integrity failed: " + detail);
        }
        const bool last = (header.flags & 0x02) != 0;
        activationResponses.push_back(std::move(fragment));
        if (last) break;
    }
    wk::opcda::rpc::RpcResponse activationResponse{};
    if (!wk::opcda::rpc::decodeResponseFragments(
            activationResponses, activationCallId, activationContextId,
            activationResponse, activationError)) {
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, WK_DCOM_REMOTE_ACTIVATION_FAILED,
                    "[DCOM] decode RemoteCreateInstance RPC response failed: " + activationError);
    }
    if (activationResponse.fault) {
        handle->activationTransport.close();
        handle->endpointMapper.close();
        const wk_opcda_error_t code = activationResponse.faultStatus == 5
            ? WK_DCOM_REMOTE_ACTIVATION_DENIED : WK_DCOM_REMOTE_ACTIVATION_FAILED;
        return fail(handle, code,
                    "[DCOM] RemoteCreateInstance RPC Fault status=" +
                        std::to_string(activationResponse.faultStatus));
    }
    wk::opcda::dcom::RemoteCreateInstanceResult created{};
    if (!wk::opcda::dcom::decodeRemoteCreateInstanceResponse(
            activationResponse.stub, wk::opcda::dcom::opcServerIid(),
            created, activationError)) {
        handle->activationTransport.close();
        handle->endpointMapper.close();
        const wk_opcda_error_t code = created.hresult == 0x80070005U
            ? WK_DCOM_REMOTE_ACTIVATION_DENIED : WK_DCOM_REMOTE_ACTIVATION_FAILED;
        return fail(handle, code, "[DCOM] " + activationError);
    }
    handle->opcServerObjectReference = std::move(created.interfaceObjectReference);
    if (!wk::opcda::dcom::decodeStandardObjectReference(
            handle->opcServerObjectReference, wk::opcda::dcom::opcServerIid(),
            handle->opcServerReference, activationError)) {
        handle->resolverTransport.close();
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
                    "[DCOM] decode IID_IOPCServer OBJREF failed: " + activationError);
    }

    constexpr uint32_t resolveOxidCallId = 3;
    constexpr uint16_t objectResolverContextId = 0;
    constexpr uint16_t resolveOxid2Operation = 4;
    wk::opcda::rpc::RpcResponse resolveResponse{};
    const std::vector<uint8_t> resolveRequest =
        wk::opcda::dcom::encodeResolveOxid2Request(handle->opcServerReference.oxid);
    if (!invokeProtectedRpc(
            handle->resolverTransport, handle->resolverSecurity, integrity,
            packetPrivacy, 1,
            resolveOxidCallId, objectResolverContextId, resolveOxid2Operation,
            resolveRequest, requestTimeout, resolveResponse, activationError)) {
        handle->resolverTransport.close();
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
                    "[DCOM] ResolveOxid2 exchange failed: " + activationError);
    }
    if (resolveResponse.fault) {
        handle->resolverTransport.close();
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
                    "[DCOM] ResolveOxid2 RPC Fault status=" +
                        std::to_string(resolveResponse.faultStatus));
    }
    if (!wk::opcda::dcom::decodeResolveOxid2Response(
            resolveResponse.stub, handle->oxidResolution, activationError)) {
        handle->resolverTransport.close();
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
                    "[DCOM] decode ResolveOxid2 response failed: " + activationError);
    }
    std::string opcServerHost;
    if (!wk::opcda::dcom::selectTcpBinding(
            handle->oxidResolution.exporterBindings, handle->host, opcServerHost,
            handle->opcServerPort, activationError)) {
        handle->resolverTransport.close();
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
                    "[DCOM] select IOPCServer endpoint failed: " + activationError);
    }
    if (!handle->opcServerTransport.connect(
            opcServerHost, handle->opcServerPort, requestTimeout,
            activationError, handle->localBindIp)) {
        const std::string bindingError = activationError;
        bool fallbackConnected = false;
        if (opcServerHost != handle->host) {
            activationError.clear();
            fallbackConnected = handle->opcServerTransport.connect(
                handle->host, handle->opcServerPort, requestTimeout,
                activationError, handle->localBindIp);
        }
        if (!fallbackConnected) {
            const std::string configuredError = opcServerHost == handle->host
                ? bindingError : activationError;
            handle->resolverTransport.close();
            handle->activationTransport.close();
            handle->endpointMapper.close();
            return fail(handle, WK_RPC_CONNECT_FAILED,
                        "[DCOM] connect IOPCServer endpoint failed: binding address: " +
                            bindingError + "; configured address: " + configuredError);
        }
    }
    const wk::opcda::rpc::SyntaxId opcServerSyntax{
        wk::opcda::dcom::opcServerIid(), 0, 0};
    const std::vector<uint8_t> opcServerDiagnostic = wk::opcda::dcom::encodeOrpcThis(
        handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
        wk::opcda::dcom::randomCid());
    constexpr uint16_t invalidOpcServerOperation = 0xffff;
    const wk_opcda_error_t opcAuthenticated = authenticateRpcEndpoint(
        handle, handle->opcServerTransport, handle->opcServerSecurity,
        opcServerSyntax, invalidOpcServerOperation, false, requestTimeout,
        activationError, &handle->opcServerReference.ipid, &opcServerDiagnostic);
    if (opcAuthenticated != WK_OPCDA_OK) {
        handle->opcServerTransport.close();
        handle->resolverTransport.close();
        handle->activationTransport.close();
        handle->endpointMapper.close();
        return fail(handle, opcAuthenticated,
                    "[DCOM] IID_IOPCServer association failed: " + activationError);
    }

    constexpr uint16_t opcServerContextId = 0;
    constexpr uint16_t getStatusOperation = 6;
    constexpr uint32_t getStatusCallId = 3;
    const std::vector<uint8_t> getStatusRequest =
        wk::opcda::opcda::encodeGetStatusRequest(
            handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
            wk::opcda::dcom::randomCid());
    wk::opcda::rpc::RpcResponse getStatusResponse{};
    if (!invokeProtectedRpc(
            handle->opcServerTransport, handle->opcServerSecurity, integrity,
            packetPrivacy, 1,
            getStatusCallId, opcServerContextId, getStatusOperation,
            getStatusRequest, requestTimeout, getStatusResponse, activationError,
            &handle->opcServerReference.ipid)) {
        return fail(handle, WK_OPCDA_ERR_OPC_SERVER,
                    "[OPC] IOPCServer::GetStatus exchange failed: " + activationError);
    }
    if (getStatusResponse.fault) {
        return fail(handle, WK_OPCDA_ERR_OPC_SERVER,
                    "[OPC] IOPCServer::GetStatus RPC Fault status=" +
                        std::to_string(getStatusResponse.faultStatus));
    }
    if (!wk::opcda::opcda::decodeGetStatusResponse(
            getStatusResponse.stub, handle->opcServerStatus, activationError)) {
        return fail(handle, WK_OPCDA_ERR_OPC_SERVER,
                    "[OPC] decode IOPCServer::GetStatus response failed: " +
                        activationError);
    }

    std::vector<uint8_t> addGroupRequest;
    const std::string groupName = handle->groupName.empty()
        ? "WK_OPCDA_GROUP" : handle->groupName;
    const uint32_t requestedRate = handle->requestedUpdateRateMs == 0
        ? 1000 : handle->requestedUpdateRateMs;
    constexpr uint32_t clientGroupHandle = 1;
    if (!wk::opcda::opcda::encodeAddGroupRequest(
            groupName, true, requestedRate, clientGroupHandle, handle->deadband,
            handle->localeId, handle->oxidResolution.comMajor,
            handle->oxidResolution.comMinor, wk::opcda::dcom::randomCid(),
            addGroupRequest, activationError)) {
        return fail(handle, WK_OPC_ADD_GROUP_FAILED,
                    "[OPC] encode IOPCServer::AddGroup request failed: " +
                        activationError);
    }
    constexpr uint16_t addGroupOperation = 3;
    constexpr uint32_t addGroupCallId = 4;
    wk::opcda::rpc::RpcResponse addGroupResponse{};
    if (!invokeProtectedRpc(
            handle->opcServerTransport, handle->opcServerSecurity, integrity,
            packetPrivacy, 1,
            addGroupCallId, opcServerContextId, addGroupOperation,
            addGroupRequest, requestTimeout, addGroupResponse, activationError,
            &handle->opcServerReference.ipid)) {
        return fail(handle, WK_OPC_ADD_GROUP_FAILED,
                    "[OPC] IOPCServer::AddGroup exchange failed: " + activationError);
    }
    if (addGroupResponse.fault) {
        return fail(handle, WK_OPC_ADD_GROUP_FAILED,
                    "[OPC] IOPCServer::AddGroup RPC Fault status=" +
                        std::to_string(addGroupResponse.faultStatus));
    }
    if (!wk::opcda::opcda::decodeAddGroupResponse(
            addGroupResponse.stub, handle->group, activationError)) {
        return fail(handle, WK_OPC_ADD_GROUP_FAILED,
                    "[OPC] decode IOPCServer::AddGroup response failed: " +
                        activationError);
    }
    if (!wk::opcda::dcom::decodeStandardObjectReference(
            handle->group.itemManagementObjectReference,
            wk::opcda::opcda::opcItemManagementIid(),
            handle->itemManagementReference, activationError)) {
        return fail(handle, WK_OPC_ADD_GROUP_FAILED,
                    "[OPC] decode IOPCItemMgt object reference failed: " +
                        activationError);
    }

    std::vector<uint8_t> getSyncIoRequest;
    if (!wk::opcda::opcda::encodeGetGroupByNameRequest(
            groupName, wk::opcda::opcda::opcSyncIoIid(),
            handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
            wk::opcda::dcom::randomCid(), getSyncIoRequest, activationError)) {
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
                    "[OPC] encode GetGroupByName(IID_IOPCSyncIO) failed: " +
                        activationError);
    }
    constexpr uint16_t getGroupByNameOperation = 5;
    constexpr uint32_t getGroupByNameCallId = 5;
    wk::opcda::rpc::RpcResponse getSyncIoResponse{};
    if (!invokeProtectedRpc(
            handle->opcServerTransport, handle->opcServerSecurity, integrity,
            packetPrivacy, 1,
            getGroupByNameCallId, opcServerContextId, getGroupByNameOperation,
            getSyncIoRequest, requestTimeout, getSyncIoResponse, activationError,
            &handle->opcServerReference.ipid)) {
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
                    "[OPC] GetGroupByName(IID_IOPCSyncIO) exchange failed: " +
                        activationError);
    }
    if (getSyncIoResponse.fault) {
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
                    "[OPC] GetGroupByName(IID_IOPCSyncIO) RPC Fault status=" +
                        std::to_string(getSyncIoResponse.faultStatus));
    }
    wk::opcda::opcda::InterfaceResult syncIoInterface{};
    if (!wk::opcda::opcda::decodeGetGroupByNameResponse(
            getSyncIoResponse.stub, syncIoInterface, activationError) ||
        !wk::opcda::dcom::decodeStandardObjectReference(
            syncIoInterface.objectReference, wk::opcda::opcda::opcSyncIoIid(),
            handle->syncIoReference, activationError)) {
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
                    "[OPC] decode IID_IOPCSyncIO object reference failed: " +
                        activationError);
    }
    if (!handle->itemManagementTransport.connect(
            handle->host, handle->opcServerPort, requestTimeout,
            activationError, handle->localBindIp)) {
        return fail(handle, WK_RPC_CONNECT_FAILED,
                    "[OPC] connect IOPCItemMgt endpoint failed: " + activationError);
    }
    const wk::opcda::rpc::SyntaxId itemManagementSyntax{
        wk::opcda::opcda::opcItemManagementIid(), 0, 0};
    const std::vector<uint8_t> itemManagementDiagnostic =
        wk::opcda::dcom::encodeOrpcThis(
            handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
            wk::opcda::dcom::randomCid());
    constexpr uint16_t invalidItemManagementOperation = 0xffff;
    const wk_opcda_error_t itemManagementAuthenticated = authenticateRpcEndpoint(
        handle, handle->itemManagementTransport, handle->itemManagementSecurity,
        itemManagementSyntax, invalidItemManagementOperation, false, requestTimeout,
        activationError, &handle->itemManagementReference.ipid,
        &itemManagementDiagnostic);
    if (itemManagementAuthenticated != WK_OPCDA_OK) {
        handle->itemManagementTransport.close();
        return fail(handle, itemManagementAuthenticated,
                    "[OPC] IID_IOPCItemMgt association failed: " + activationError);
    }
    if (!handle->syncIoTransport.connect(
            handle->host, handle->opcServerPort, requestTimeout,
            activationError, handle->localBindIp)) {
        return fail(handle, WK_RPC_CONNECT_FAILED,
                    "[OPC] connect IOPCSyncIO endpoint failed: " + activationError);
    }
    const wk::opcda::rpc::SyntaxId syncIoSyntax{
        wk::opcda::opcda::opcSyncIoIid(), 0, 0};
    const std::vector<uint8_t> syncIoDiagnostic =
        wk::opcda::dcom::encodeOrpcThis(
            handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
            wk::opcda::dcom::randomCid());
    constexpr uint16_t invalidSyncIoOperation = 0xffff;
    const wk_opcda_error_t syncIoAuthenticated = authenticateRpcEndpoint(
        handle, handle->syncIoTransport, handle->syncIoSecurity,
        syncIoSyntax, invalidSyncIoOperation, false, requestTimeout,
        activationError, &handle->syncIoReference.ipid, &syncIoDiagnostic);
    if (syncIoAuthenticated != WK_OPCDA_OK) {
        handle->syncIoTransport.close();
        return fail(handle, syncIoAuthenticated,
                    "[OPC] IID_IOPCSyncIO association failed: " + activationError);
    }
    handle->endpointMapper.close();
    handle->state = WK_OPCDA_STATE_CONNECTED;
    if (!startObjectLifetimeWorker(handle, activationError)) {
        return fail(handle, WK_DISCONNECTED, "[DCOM] register object lifetime failed: " + activationError);
    }
    handle->lastErrorCode = WK_OPCDA_OK;
    handle->lastError = handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY
        ? "[OPC] GetStatus, AddGroup, IOPCItemMgt, and IOPCSyncIO succeeded with Packet Privacy"
        : handle->packetIntegrityEstablished
        ? "[OPC] GetStatus, AddGroup, IOPCItemMgt, and IOPCSyncIO succeeded with Packet Integrity"
        : "[OPC] GetStatus, AddGroup, IOPCItemMgt, and IOPCSyncIO succeeded using Legacy Connect";
    return WK_OPCDA_OK;
}

wk_opcda_error_t ensureBrowseConnected(wk_opcda_handle_t* handle,
                                       std::string& error) {
    if (handle->browseTransport.connected() && handle->browsePort != 0) {
        return WK_OPCDA_OK;
    }
    if (!handle->activationTransport.connected() ||
        !handle->resolverTransport.connected()) {
        error = "the authenticated DCOM activation association is unavailable";
        return WK_DISCONNECTED;
    }

    const bool integrity =
        handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;
    const bool packetPrivacy =
        handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY;
    const std::chrono::milliseconds timeout(
        handle->requestTimeoutMs == 0 ? 10000 : handle->requestTimeoutMs);

    std::vector<uint8_t> activationStub;
    if (!wk::opcda::dcom::encodeRemoteCreateInstanceRequest(
            handle->clsid,
            wk::opcda::opcda::opcBrowseServerAddressSpaceIid(),
            wk::opcda::dcom::randomCid(), activationStub, error)) {
        error = "encode browse RemoteCreateInstance failed: " + error;
        return WK_DCOM_REMOTE_ACTIVATION_FAILED;
    }
    const uint32_t activationCallId = handle->activationObjectCallId++;
    if (handle->activationObjectCallId < 4) handle->activationObjectCallId = 4;
    constexpr uint16_t contextId = 0;
    constexpr uint16_t remoteCreateInstanceOperation = 4;
    wk::opcda::rpc::RpcResponse activationResponse{};
    if (!invokeProtectedRpc(
            handle->activationTransport, handle->activationSecurity, integrity,
            packetPrivacy, 1, activationCallId, contextId,
            remoteCreateInstanceOperation, activationStub, timeout,
            activationResponse, error)) {
        error = "browse RemoteCreateInstance exchange failed: " + error;
        return WK_DCOM_REMOTE_ACTIVATION_FAILED;
    }
    if (activationResponse.fault) {
        error = "browse RemoteCreateInstance RPC Fault status=" +
            std::to_string(activationResponse.faultStatus);
        return WK_DCOM_REMOTE_ACTIVATION_FAILED;
    }
    wk::opcda::dcom::RemoteCreateInstanceResult created{};
    if (!wk::opcda::dcom::decodeRemoteCreateInstanceResponse(
            activationResponse.stub,
            wk::opcda::opcda::opcBrowseServerAddressSpaceIid(), created,
            error) ||
        !wk::opcda::dcom::decodeStandardObjectReference(
            created.interfaceObjectReference,
            wk::opcda::opcda::opcBrowseServerAddressSpaceIid(),
            handle->browseReference, error)) {
        error = "decode IOPCBrowseServerAddressSpace activation failed: " + error;
        return WK_DCOM_QUERY_INTERFACE_FAILED;
    }

    wk::opcda::dcom::OxidResolution browseResolution = handle->oxidResolution;
    if (handle->browseReference.oxid != handle->opcServerReference.oxid) {
        const uint32_t resolveCallId = handle->resolverObjectCallId++;
        if (handle->resolverObjectCallId < 4) handle->resolverObjectCallId = 4;
        constexpr uint16_t resolveOxid2Operation = 4;
        wk::opcda::rpc::RpcResponse resolveResponse{};
        const std::vector<uint8_t> request =
            wk::opcda::dcom::encodeResolveOxid2Request(
                handle->browseReference.oxid);
        if (!invokeProtectedRpc(
                handle->resolverTransport, handle->resolverSecurity, integrity,
                packetPrivacy, 1, resolveCallId, contextId,
                resolveOxid2Operation, request, timeout, resolveResponse,
                error) ||
            resolveResponse.fault ||
            !wk::opcda::dcom::decodeResolveOxid2Response(
                resolveResponse.stub, browseResolution, error)) {
            error = "resolve browse interface OXID failed: " + error;
            return WK_DCOM_QUERY_INTERFACE_FAILED;
        }
    }
    if (!wk::opcda::dcom::selectTcpBinding(
            browseResolution.exporterBindings, handle->host,
            handle->browseHost, handle->browsePort, error)) {
        error = "select browse interface endpoint failed: " + error;
        return WK_DCOM_QUERY_INTERFACE_FAILED;
    }
    if (!handle->browseTransport.connect(
            handle->browseHost, handle->browsePort, timeout, error,
            handle->localBindIp)) {
        const std::string advertisedError = error;
        if (handle->browseHost == handle->host ||
            !handle->browseTransport.connect(
                handle->host, handle->browsePort, timeout, error,
                handle->localBindIp)) {
            error = "connect browse interface endpoint failed: advertised=" +
                advertisedError + "; configured=" + error;
            return WK_RPC_CONNECT_FAILED;
        }
        handle->browseHost = handle->host;
    }
    const wk::opcda::rpc::SyntaxId syntax{
        wk::opcda::opcda::opcBrowseServerAddressSpaceIid(), 0, 0};
    const std::vector<uint8_t> diagnostic =
        wk::opcda::dcom::encodeOrpcThis(
            browseResolution.comMajor, browseResolution.comMinor,
            wk::opcda::dcom::randomCid());
    constexpr uint16_t invalidOperation = 0xffff;
    const wk_opcda_error_t authenticated = authenticateRpcEndpoint(
        handle, handle->browseTransport, handle->browseSecurity, syntax,
        invalidOperation, false, timeout, error,
        &handle->browseReference.ipid, &diagnostic);
    if (authenticated != WK_OPCDA_OK) {
        handle->browseTransport.close();
        error = "IOPCBrowseServerAddressSpace association failed: " + error;
        return authenticated;
    }
    handle->browseCallId = 3;
    return WK_OPCDA_OK;
}

bool browseInvoke(wk_opcda_handle_t* handle, uint16_t operation,
                  const std::vector<uint8_t>& request,
                  wk::opcda::rpc::RpcResponse& response,
                  std::string& error) {
    const bool integrity =
        handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;
    const bool packetPrivacy =
        handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY;
    const std::chrono::milliseconds timeout(
        handle->requestTimeoutMs == 0 ? 10000 : handle->requestTimeoutMs);
    const uint32_t callId = handle->browseCallId++;
    if (handle->browseCallId < 3) handle->browseCallId = 3;
    return invokeProtectedRpc(
        handle->browseTransport, handle->browseSecurity, integrity,
        packetPrivacy, 1, callId, 0, operation, request, timeout, response,
        error, &handle->browseReference.ipid);
}

wk_opcda_error_t enumerateBrowseType(
    wk_opcda_handle_t* handle, wk::opcda::opcda::BrowseType type,
    bool branch, std::size_t capacity,
    std::vector<std::pair<std::string, std::string>>& output,
    std::string& error) {
    if (output.size() >= capacity) return WK_OPCDA_OK;
    std::vector<uint8_t> request;
    if (!wk::opcda::opcda::encodeBrowseItemIdsRequest(
            type, "", 0, 0, handle->oxidResolution.comMajor,
            handle->oxidResolution.comMinor, wk::opcda::dcom::randomCid(),
            request, error)) {
        error = "encode BrowseOPCItemIDs failed: " + error;
        return WK_OPC_BROWSE_FAILED;
    }
    wk::opcda::rpc::RpcResponse response{};
    constexpr uint16_t browseItemIdsOperation = 5;
    if (!browseInvoke(handle, browseItemIdsOperation, request, response, error)) {
        error = "BrowseOPCItemIDs exchange failed: " + error;
        return WK_OPC_BROWSE_FAILED;
    }
    if (response.fault) {
        error = "BrowseOPCItemIDs RPC Fault status=" +
            std::to_string(response.faultStatus);
        return WK_OPC_BROWSE_FAILED;
    }
    std::vector<uint8_t> enumObjectReference;
    uint32_t browseHresult = 0;
    if (!wk::opcda::opcda::decodeBrowseEnumeratorResponse(
            response.stub, enumObjectReference, browseHresult, error)) {
        return WK_OPC_BROWSE_FAILED;
    }
    wk::opcda::dcom::StandardObjectReference enumReference{};
    if (!wk::opcda::dcom::decodeStandardObjectReference(
            enumObjectReference, wk::opcda::opcda::enumStringIid(),
            enumReference, error)) {
        error = "decode IEnumString object reference failed: " + error;
        return WK_OPC_BROWSE_FAILED;
    }

    wk::opcda::rpc::TcpTransport enumTransport;
    wk::opcda::auth::NtlmSignatureState enumSecurity;
    const std::chrono::milliseconds timeout(
        handle->requestTimeoutMs == 0 ? 10000 : handle->requestTimeoutMs);
    if (!enumTransport.connect(handle->browseHost, handle->browsePort, timeout,
                               error, handle->localBindIp)) {
        error = "connect IEnumString endpoint failed: " + error;
        return WK_RPC_CONNECT_FAILED;
    }
    const wk::opcda::rpc::SyntaxId enumSyntax{
        wk::opcda::opcda::enumStringIid(), 0, 0};
    const std::vector<uint8_t> enumDiagnostic =
        wk::opcda::dcom::encodeOrpcThis(
            handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
            wk::opcda::dcom::randomCid());
    constexpr uint16_t invalidOperation = 0xffff;
    const wk_opcda_error_t enumAuthenticated = authenticateRpcEndpoint(
        handle, enumTransport, enumSecurity, enumSyntax, invalidOperation,
        false, timeout, error, &enumReference.ipid, &enumDiagnostic);
    if (enumAuthenticated != WK_OPCDA_OK) {
        error = "IEnumString association failed: " + error;
        return enumAuthenticated;
    }

    const bool integrity =
        handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;
    const bool packetPrivacy =
        handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY;
    uint32_t enumCallId = 3;
    constexpr uint16_t enumNextOperation = 3;
    constexpr uint16_t getItemIdOperation = 6;
    while (output.size() < capacity) {
        const std::vector<uint8_t> nextRequest =
            wk::opcda::opcda::encodeEnumStringNextRequest(
                1, handle->oxidResolution.comMajor,
                handle->oxidResolution.comMinor,
                wk::opcda::dcom::randomCid());
        wk::opcda::rpc::RpcResponse nextResponse{};
        if (!invokeProtectedRpc(
                enumTransport, enumSecurity, integrity, packetPrivacy, 1,
                enumCallId++, 0, enumNextOperation, nextRequest, timeout,
                nextResponse, error, &enumReference.ipid)) {
            error = "IEnumString::Next exchange failed: " + error;
            return WK_OPC_BROWSE_FAILED;
        }
        if (nextResponse.fault) {
            error = "IEnumString::Next RPC Fault status=" +
                std::to_string(nextResponse.faultStatus);
            return WK_OPC_BROWSE_FAILED;
        }
        std::vector<std::string> names;
        uint32_t fetched = 0;
        uint32_t nextHresult = 0;
        if (!wk::opcda::opcda::decodeEnumStringNextResponse(
                nextResponse.stub, names, fetched, nextHresult, error)) {
            return WK_OPC_BROWSE_FAILED;
        }
        if (fetched == 0 || names.empty()) break;
        for (const std::string& name : names) {
            std::vector<uint8_t> itemIdRequest;
            if (!wk::opcda::opcda::encodeGetItemIdRequest(
                    name, handle->oxidResolution.comMajor,
                    handle->oxidResolution.comMinor,
                    wk::opcda::dcom::randomCid(), itemIdRequest, error)) {
                return WK_OPC_BROWSE_FAILED;
            }
            wk::opcda::rpc::RpcResponse itemIdResponse{};
            if (!browseInvoke(handle, getItemIdOperation, itemIdRequest,
                              itemIdResponse, error) ||
                itemIdResponse.fault) {
                error = "GetItemID exchange failed: " + error;
                return WK_OPC_BROWSE_FAILED;
            }
            std::string itemId;
            uint32_t itemIdHresult = 0;
            if (!wk::opcda::opcda::decodeGetItemIdResponse(
                    itemIdResponse.stub, itemId, itemIdHresult, error)) {
                return WK_OPC_BROWSE_FAILED;
            }
            output.emplace_back(name, itemId);
            if (output.size() >= capacity) break;
        }
        if (nextHresult == 1) break;  // S_FALSE: the final element was returned.
    }
    (void)branch;
    return WK_OPCDA_OK;
}

void deliverDataChange(wk_opcda_handle_t* handle,
                       const wk::opcda::opcda::DataChange& change) {
    wk_opcda_data_callback_t callback = handle->subscriptionCallback;
    if (callback == nullptr || change.values.empty()) return;
    std::vector<wk_opcda_value_t> output(change.values.size());
    std::vector<std::string> strings(change.values.size());
    std::vector<std::vector<double>> floatArrays(change.values.size());
    std::vector<std::vector<std::string>> stringArrays(change.values.size());
    std::vector<std::vector<const char*>> stringArrayPointers(change.values.size());
    std::vector<std::vector<size_t>> stringArrayLengths(change.values.size());
    for (std::size_t index = 0; index < change.values.size(); ++index) {
        const auto& decoded = change.values[index];
        auto& value = output[index];
        value.client_handle = decoded.clientHandle;
        value.quality = decoded.quality;
        value.source_timestamp_unix_ms = decoded.timestampUnixMilliseconds;
        value.hresult = static_cast<int32_t>(decoded.hresult);
        switch (decoded.kind) {
            case wk::opcda::opcda::SyncValueKind::Empty:
                value.type = WK_OPCDA_VALUE_EMPTY;
                break;
            case wk::opcda::opcda::SyncValueKind::Boolean:
                value.type = WK_OPCDA_VALUE_BOOL;
                value.int64_value = decoded.signedValue;
                break;
            case wk::opcda::opcda::SyncValueKind::SignedInteger:
                value.type = WK_OPCDA_VALUE_INT64;
                value.int64_value = decoded.signedValue;
                break;
            case wk::opcda::opcda::SyncValueKind::UnsignedInteger:
                value.type = WK_OPCDA_VALUE_UINT64;
                value.uint64_value = decoded.unsignedValue;
                break;
            case wk::opcda::opcda::SyncValueKind::FloatingPoint:
                value.type = WK_OPCDA_VALUE_FLOAT64;
                value.float64_value = decoded.floatingValue;
                break;
            case wk::opcda::opcda::SyncValueKind::String:
                value.type = WK_OPCDA_VALUE_STRING;
                strings[index] = decoded.stringValue;
                value.string_value = strings[index].c_str();
                value.string_length = strings[index].size();
                break;
            case wk::opcda::opcda::SyncValueKind::DateTime:
                value.type = WK_OPCDA_VALUE_DATETIME;
                value.int64_value = decoded.signedValue;
                break;
            case wk::opcda::opcda::SyncValueKind::FloatingPointArray:
                value.type = WK_OPCDA_VALUE_FLOAT64_ARRAY;
                floatArrays[index] = decoded.floatingArrayValue;
                value.float64_array_values = floatArrays[index].data();
                value.array_length = floatArrays[index].size();
                break;
            case wk::opcda::opcda::SyncValueKind::StringArray:
                value.type = WK_OPCDA_VALUE_STRING_ARRAY;
                stringArrays[index] = decoded.stringArrayValue;
                stringArrayPointers[index].resize(stringArrays[index].size());
                stringArrayLengths[index].resize(stringArrays[index].size());
                for (std::size_t stringIndex = 0;
                     stringIndex < stringArrays[index].size(); ++stringIndex) {
                    stringArrayPointers[index][stringIndex] =
                        stringArrays[index][stringIndex].c_str();
                    stringArrayLengths[index][stringIndex] =
                        stringArrays[index][stringIndex].size();
                }
                value.string_array_values = stringArrayPointers[index].data();
                value.string_array_lengths = stringArrayLengths[index].data();
                value.array_length = stringArrays[index].size();
                break;
        }
    }
    callback(output.data(), output.size(), handle->subscriptionUserData);
}

}  // namespace

extern "C" {

uint32_t wk_opcda_abi_version(void) {
    return WK_OPCDA_ABI_VERSION;
}

wk_opcda_error_t wk_opcda_create(const wk_opcda_config_t* config, wk_opcda_handle_t** outHandle) {
    if (config == nullptr || outHandle == nullptr || config->abi_version != WK_OPCDA_ABI_VERSION) {
        return WK_OPCDA_ERR_ARGUMENT;
    }
    *outHandle = nullptr;
    std::unique_ptr<wk_opcda_handle_t> handle(new (std::nothrow) wk_opcda_handle_t());
    if (!handle) {
        return WK_OPCDA_ERR_ARGUMENT;
    }
    handle->host = copyString(config->host);
    handle->progId = copyString(config->prog_id);
    handle->clsid = copyString(config->clsid);
    handle->domain = copyString(config->domain);
    handle->username = copyString(config->username);
    handle->password = copyString(config->password);
    handle->workstation = copyString(config->workstation);
    handle->securityMode = copyString(config->security_mode);
    handle->authentication = copyString(config->authentication);
    handle->authenticationLevel = copyString(config->authentication_level);
    handle->groupName = copyString(config->group_name);
    handle->localBindIp = copyString(config->local_bind_ip);
    handle->localeId = config->locale_id;
    handle->requestedUpdateRateMs = config->requested_update_rate_ms;
    handle->connectTimeoutMs = config->connect_timeout_ms;
    handle->requestTimeoutMs = config->request_timeout_ms;
    handle->deadband = config->deadband;
    handle->autoReconnect = config->auto_reconnect != 0;
    if (handle->host.empty()) {
        return fail(handle.get(), WK_OPCDA_ERR_ARGUMENT, "OPC DA server host is empty");
    }
    if (handle->clsid.empty() && handle->progId.empty()) {
        return fail(handle.get(), WK_OPCDA_ERR_ARGUMENT, "OPC DA CLSID or ProgID is required");
    }
    if (!applySecurityMode(handle.get())) {
        return fail(handle.get(), WK_RPC_AUTH_LEVEL_UNSUPPORTED,
                    "[AUTH] unsupported OPC DA security mode: " + handle->securityMode);
    }
    *outHandle = handle.release();
    return WK_OPCDA_OK;
}

wk_opcda_error_t wk_opcda_connect(wk_opcda_handle_t* handle) {
    if (handle == nullptr) {
        return WK_OPCDA_ERR_ARGUMENT;
    }
    wk_opcda_disconnect(handle);
    std::lock_guard<std::mutex> guard(handle->mutex);
    handle->lastError.clear();
    handle->lastErrorCode = WK_OPCDA_OK;
    handle->state = WK_OPCDA_STATE_CONNECTING;
    const auto connectTimeout = std::chrono::milliseconds(handle->connectTimeoutMs == 0 ? 10000 : handle->connectTimeoutMs);
    const auto requestTimeout = std::chrono::milliseconds(handle->requestTimeoutMs == 0 ? 10000 : handle->requestTimeoutMs);
    std::string error;
    if (loadCachedEndpoint(handle, handle->activationPort)) {
        if (handle->activationTransport.connect(handle->host, handle->activationPort, connectTimeout,
                                                error, handle->localBindIp)) {
            handle->state = WK_OPCDA_STATE_AUTHENTICATING;
            return authenticateConnectedActivation(handle, requestTimeout);
        }
        invalidateCachedEndpoint(handle);
        handle->activationPort = 0;
        error.clear();
    }
    if (!handle->endpointMapper.connect(handle->host, 135, connectTimeout, error, handle->localBindIp)) {
        return fail(handle, WK_RPC_CONNECT_FAILED, "[NET] connect endpoint mapper " + handle->host + ":135 failed: " + error);
    }
    constexpr uint32_t bindCallId = 1;
    constexpr uint16_t endpointMapperContext = 0;
    const std::vector<uint8_t> bind = wk::opcda::rpc::encodeBind(bindCallId, endpointMapperContext,
                                                                 wk::opcda::rpc::endpointMapperSyntax());
    if (!handle->endpointMapper.sendAll(bind.data(), bind.size(), requestTimeout, error)) {
        handle->endpointMapper.close();
        return fail(handle, WK_RPC_BIND_FAILED, "[RPC] endpoint mapper Bind send failed: " + error);
    }
    std::vector<uint8_t> response;
    if (!handle->endpointMapper.receiveFragment(response, requestTimeout, error)) {
        handle->endpointMapper.close();
        return fail(handle, WK_RPC_BIND_FAILED, "[RPC] endpoint mapper BindAck receive failed: " + error);
    }
    if (!wk::opcda::rpc::validateBindAck(response, bindCallId, error)) {
        handle->endpointMapper.close();
        return fail(handle, WK_RPC_BIND_FAILED, "[RPC] endpoint mapper BindAck invalid: " + error);
    }

    constexpr uint32_t mapCallId = 2;
    constexpr uint16_t mapOperationNumber = 3;
    const std::vector<uint8_t> mapStub =
        wk::opcda::rpc::encodeEndpointMapRequest(wk::opcda::rpc::remoteScmActivatorSyntax());
    const std::vector<std::vector<uint8_t>> requests =
        wk::opcda::rpc::encodeRequest(mapCallId, endpointMapperContext, mapOperationNumber, mapStub);
    for (const std::vector<uint8_t>& request : requests) {
        if (!handle->endpointMapper.sendAll(request.data(), request.size(), requestTimeout, error)) {
            handle->endpointMapper.close();
            return fail(handle, WK_RPC_ENDPOINT_FAILED, "[RPC] endpoint mapper ept_map send failed: " + error);
        }
    }
    wk::opcda::rpc::RpcResponse mapResponse;
    if (!receiveRpcResponse(handle->endpointMapper, requestTimeout, mapCallId,
                            endpointMapperContext, mapResponse, error)) {
        handle->endpointMapper.close();
        return fail(handle, WK_RPC_ENDPOINT_FAILED, "[RPC] endpoint mapper ept_map response failed: " + error);
    }
    if (mapResponse.fault) {
        handle->endpointMapper.close();
        return fail(handle, WK_RPC_ENDPOINT_FAILED,
                    "[RPC] endpoint mapper ept_map returned Fault status=" +
                        std::to_string(mapResponse.faultStatus));
    }
    wk::opcda::rpc::EndpointMapResult mapped;
    if (!wk::opcda::rpc::decodeEndpointMapResponse(mapResponse.stub, mapped, error)) {
        // Windows commonly hosts IRemoteSCMActivator on the endpoint mapper's
        // well-known TCP/135 endpoint without publishing a separate dynamic
        // EPM entry. Keep dynamic endpoint discovery when it is available,
        // but use a fresh association on 135 for this standards-compatible
        // deployment shape.
        constexpr uint32_t endpointNotRegistered = 0x16c9a0d6;
        if (mapped.status != endpointNotRegistered) {
            handle->endpointMapper.close();
            return fail(handle, WK_RPC_ENDPOINT_FAILED,
                        "[RPC] endpoint mapper ept_map decode failed: " + error);
        }
        handle->activationPort = 135;
    } else {
        handle->activationPort = mapped.tcpPort;
    }
    storeCachedEndpoint(handle, handle->activationPort);
    if (!handle->activationTransport.connect(handle->host, handle->activationPort, connectTimeout,
                                             error, handle->localBindIp)) {
        invalidateCachedEndpoint(handle);
        handle->endpointMapper.close();
        return fail(handle, WK_RPC_CONNECT_FAILED,
                    "[RPC] connect DCOM activation endpoint " + handle->host + ":" +
                        std::to_string(handle->activationPort) + " failed: " + error);
    }
    handle->state = WK_OPCDA_STATE_AUTHENTICATING;
    return authenticateConnectedActivation(handle, requestTimeout);
}

wk_opcda_error_t wk_opcda_add_items(wk_opcda_handle_t* handle,
                                    const wk_opcda_item_definition_t* items, size_t itemCount,
                                    wk_opcda_item_result_t* outputResults) {
    if (handle == nullptr || items == nullptr || outputResults == nullptr ||
        itemCount == 0 || itemCount > 10000) {
        return WK_OPCDA_ERR_ARGUMENT;
    }
    std::lock_guard<std::mutex> guard(handle->mutex);
    if (handle->state != WK_OPCDA_STATE_CONNECTED) {
        return fail(handle, WK_DISCONNECTED,
                    "[OPC] AddItems requires a connected IOPCItemMgt association");
    }

    std::vector<wk::opcda::opcda::ItemDefinition> definitions;
    definitions.reserve(itemCount);
    for (std::size_t index = 0; index < itemCount; ++index) {
        outputResults[index] = wk_opcda_item_result_t{};
        if (items[index].item_id == nullptr || items[index].item_id[0] == '\0') {
            return fail(handle, WK_OPCDA_ERR_ARGUMENT,
                        "[OPC] AddItems item_id must not be empty");
        }
        uint16_t requestedType = 0;
        if (!wk::opcda::opcda::vartypeFromName(
                copyString(items[index].requested_data_type), requestedType)) {
            return fail(handle, WK_OPCDA_ERR_ARGUMENT,
                        "[OPC] unsupported AddItems requested_data_type for item " +
                            copyString(items[index].item_id));
        }
        definitions.push_back(wk::opcda::opcda::ItemDefinition{
            items[index].client_handle,
            copyString(items[index].item_id),
            copyString(items[index].access_path),
            items[index].active != 0,
            requestedType});
    }

    std::vector<uint8_t> request;
    std::string error;
    if (!wk::opcda::opcda::encodeAddItemsRequest(
            definitions, handle->oxidResolution.comMajor,
            handle->oxidResolution.comMinor, wk::opcda::dcom::randomCid(),
            request, error)) {
        return fail(handle, WK_OPC_ADD_ITEMS_FAILED,
                    "[OPC] encode IOPCItemMgt::AddItems request failed: " + error);
    }
    const bool integrity =
        handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;
    const bool packetPrivacy =
        handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY;
    const std::chrono::milliseconds timeout(
        handle->requestTimeoutMs == 0 ? 10000 : handle->requestTimeoutMs);
    const uint32_t callId = handle->itemManagementCallId++;
    if (handle->itemManagementCallId < 3) handle->itemManagementCallId = 3;
    constexpr uint16_t contextId = 0;
    constexpr uint16_t addItemsOperation = 3;
    wk::opcda::rpc::RpcResponse response{};
    if (!invokeProtectedRpc(
            handle->itemManagementTransport, handle->itemManagementSecurity,
            integrity, packetPrivacy, 1, callId, contextId, addItemsOperation, request, timeout,
            response, error, &handle->itemManagementReference.ipid)) {
        return fail(handle, WK_OPC_ADD_ITEMS_FAILED,
                    "[OPC] IOPCItemMgt::AddItems exchange failed: " + error);
    }
    if (response.fault) {
        return fail(handle, WK_OPC_ADD_ITEMS_FAILED,
                    "[OPC] IOPCItemMgt::AddItems RPC Fault status=" +
                        std::to_string(response.faultStatus));
    }
    std::vector<wk::opcda::opcda::ItemResult> decoded;
    uint32_t hresult = 0;
    if (!wk::opcda::opcda::decodeAddItemsResponse(
            response.stub, definitions, decoded, hresult, error)) {
        return fail(handle, WK_OPC_ADD_ITEMS_FAILED,
                    "[OPC] decode IOPCItemMgt::AddItems response failed: " + error);
    }
    std::size_t succeeded = 0;
    for (std::size_t index = 0; index < decoded.size(); ++index) {
        outputResults[index].client_handle = decoded[index].clientHandle;
        outputResults[index].server_handle = decoded[index].serverHandle;
        outputResults[index].canonical_data_type = decoded[index].canonicalDataType;
        outputResults[index].access_rights = decoded[index].accessRights;
        outputResults[index].hresult = static_cast<int32_t>(decoded[index].hresult);
        if ((decoded[index].hresult & 0x80000000U) == 0) {
            handle->itemServerHandles[decoded[index].clientHandle] =
                decoded[index].serverHandle;
            handle->itemCanonicalTypes[decoded[index].clientHandle] =
                decoded[index].canonicalDataType;
            ++succeeded;
        }
    }
    handle->lastErrorCode = WK_OPCDA_OK;
    handle->lastError = "[OPC] AddItems completed: " +
        std::to_string(succeeded) + "/" + std::to_string(itemCount) +
        " items accepted (method HRESULT=" + std::to_string(hresult) +
        "); SyncRead is the next stage";
    return WK_OPCDA_OK;
}

wk_opcda_error_t wk_opcda_remove_items(wk_opcda_handle_t* handle,
                                       const uint32_t*, size_t, int32_t*) {
    if (handle == nullptr) return WK_OPCDA_ERR_ARGUMENT;
    std::lock_guard<std::mutex> guard(handle->mutex);
    return fail(handle, WK_OPCDA_ERR_NOT_IMPLEMENTED, "[OPC] RemoveItems is not implemented");
}

wk_opcda_error_t wk_opcda_read(wk_opcda_handle_t* handle,
                               wk_opcda_read_source_t source,
                               const uint32_t* clientHandles, size_t itemCount,
                               wk_opcda_value_t* outputValues) {
    if (handle == nullptr || clientHandles == nullptr || outputValues == nullptr ||
        itemCount == 0 || itemCount > 10000 ||
        (source != WK_OPCDA_READ_CACHE && source != WK_OPCDA_READ_DEVICE)) {
        return WK_OPCDA_ERR_ARGUMENT;
    }
    std::lock_guard<std::mutex> guard(handle->mutex);
    if (handle->state != WK_OPCDA_STATE_CONNECTED) {
        return fail(handle, WK_DISCONNECTED,
                    "[OPC] SyncRead requires a connected IOPCSyncIO association");
    }
    std::vector<uint32_t> serverHandles;
    serverHandles.reserve(itemCount);
    for (std::size_t index = 0; index < itemCount; ++index) {
        outputValues[index] = wk_opcda_value_t{};
        const auto found = handle->itemServerHandles.find(clientHandles[index]);
        if (found == handle->itemServerHandles.end()) {
            return fail(handle, WK_OPCDA_ERR_ITEM,
                        "[OPC] SyncRead client handle was not accepted by AddItems: " +
                            std::to_string(clientHandles[index]));
        }
        serverHandles.push_back(found->second);
    }
    const std::vector<uint8_t> request = wk::opcda::opcda::encodeSyncReadRequest(
        static_cast<uint32_t>(source), serverHandles,
        handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
        wk::opcda::dcom::randomCid());
    const bool integrity =
        handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;
    const bool packetPrivacy =
        handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY;
    const std::chrono::milliseconds timeout(
        handle->requestTimeoutMs == 0 ? 10000 : handle->requestTimeoutMs);
    const uint32_t callId = handle->syncIoCallId++;
    if (handle->syncIoCallId < 3) handle->syncIoCallId = 3;
    constexpr uint16_t contextId = 0;
    constexpr uint16_t readOperation = 3;
    std::string error;
    wk::opcda::rpc::RpcResponse response{};
    if (!invokeProtectedRpc(
            handle->syncIoTransport, handle->syncIoSecurity, integrity,
            packetPrivacy, 1,
            callId, contextId, readOperation, request, timeout, response, error,
            &handle->syncIoReference.ipid)) {
        return fail(handle, WK_OPC_READ_FAILED,
                    "[OPC] IOPCSyncIO::Read exchange failed: " + error);
    }
    if (response.fault) {
        return fail(handle, WK_OPC_READ_FAILED,
                    "[OPC] IOPCSyncIO::Read RPC Fault status=" +
                        std::to_string(response.faultStatus));
    }
    std::vector<wk::opcda::opcda::SyncReadValue> decoded;
    uint32_t hresult = 0;
    if (!wk::opcda::opcda::decodeSyncReadResponse(
            response.stub, itemCount, decoded, hresult, error)) {
        // The authenticated response has already been consumed. A VARIANT
        // decode failure does not invalidate the RPC association; Go can split
        // this batch to identify the bad item without replacing the root error.
        handle->lastErrorCode = WK_OPC_READ_FAILED;
        handle->lastError = "[OPC] decode IOPCSyncIO::Read response failed: " + error;
        std::fprintf(stderr, "[OPCDA][DECODE] host=%s items=%zu cause=%s\n",
            handle->host.c_str(), itemCount, error.c_str());
        return WK_OPC_READ_FAILED;
    }
    if (!wk::opcda::opcda::orderSyncReadValuesByClientHandle(
            std::vector<uint32_t>(clientHandles, clientHandles + itemCount),
            decoded, error)) {
        return fail(handle, WK_OPC_READ_FAILED, "[OPC] " + error);
    }
    handle->readStrings.assign(itemCount, std::string{});
    handle->readFloatArrays.assign(itemCount, std::vector<double>{});
    handle->readStringArrays.assign(itemCount, std::vector<std::string>{});
    handle->readStringArrayPointers.assign(itemCount, std::vector<const char*>{});
    handle->readStringArrayLengths.assign(itemCount, std::vector<size_t>{});
    for (std::size_t index = 0; index < decoded.size(); ++index) {
        outputValues[index].client_handle = decoded[index].clientHandle;
        outputValues[index].quality = decoded[index].quality;
        outputValues[index].source_timestamp_unix_ms =
            decoded[index].timestampUnixMilliseconds;
        outputValues[index].hresult = static_cast<int32_t>(decoded[index].hresult);
        switch (decoded[index].kind) {
            case wk::opcda::opcda::SyncValueKind::Empty:
                outputValues[index].type = WK_OPCDA_VALUE_EMPTY;
                break;
            case wk::opcda::opcda::SyncValueKind::Boolean:
                outputValues[index].type = WK_OPCDA_VALUE_BOOL;
                outputValues[index].int64_value = decoded[index].signedValue;
                break;
            case wk::opcda::opcda::SyncValueKind::SignedInteger:
                outputValues[index].type = WK_OPCDA_VALUE_INT64;
                outputValues[index].int64_value = decoded[index].signedValue;
                break;
            case wk::opcda::opcda::SyncValueKind::UnsignedInteger:
                outputValues[index].type = WK_OPCDA_VALUE_UINT64;
                outputValues[index].uint64_value = decoded[index].unsignedValue;
                break;
            case wk::opcda::opcda::SyncValueKind::FloatingPoint:
                outputValues[index].type = WK_OPCDA_VALUE_FLOAT64;
                outputValues[index].float64_value = decoded[index].floatingValue;
                break;
            case wk::opcda::opcda::SyncValueKind::String:
                outputValues[index].type = WK_OPCDA_VALUE_STRING;
                handle->readStrings[index] = decoded[index].stringValue;
                break;
            case wk::opcda::opcda::SyncValueKind::DateTime:
                outputValues[index].type = WK_OPCDA_VALUE_DATETIME;
                outputValues[index].int64_value = decoded[index].signedValue;
                break;
            case wk::opcda::opcda::SyncValueKind::FloatingPointArray:
                outputValues[index].type = WK_OPCDA_VALUE_FLOAT64_ARRAY;
                handle->readFloatArrays[index] = decoded[index].floatingArrayValue;
                break;
            case wk::opcda::opcda::SyncValueKind::StringArray:
                outputValues[index].type = WK_OPCDA_VALUE_STRING_ARRAY;
                handle->readStringArrays[index] = decoded[index].stringArrayValue;
                break;
        }
    }
    for (std::size_t index = 0; index < decoded.size(); ++index) {
        if (outputValues[index].type != WK_OPCDA_VALUE_STRING) continue;
        outputValues[index].string_value = handle->readStrings[index].c_str();
        outputValues[index].string_length = handle->readStrings[index].size();
    }
    for (std::size_t index = 0; index < decoded.size(); ++index) {
        if (outputValues[index].type == WK_OPCDA_VALUE_FLOAT64_ARRAY) {
            outputValues[index].float64_array_values =
                handle->readFloatArrays[index].data();
            outputValues[index].array_length =
                handle->readFloatArrays[index].size();
            continue;
        }
        if (outputValues[index].type != WK_OPCDA_VALUE_STRING_ARRAY) continue;
        auto& pointers = handle->readStringArrayPointers[index];
        auto& lengths = handle->readStringArrayLengths[index];
        pointers.resize(handle->readStringArrays[index].size());
        lengths.resize(handle->readStringArrays[index].size());
        for (std::size_t stringIndex = 0;
             stringIndex < handle->readStringArrays[index].size(); ++stringIndex) {
            pointers[stringIndex] =
                handle->readStringArrays[index][stringIndex].c_str();
            lengths[stringIndex] =
                handle->readStringArrays[index][stringIndex].size();
        }
        outputValues[index].string_array_values = pointers.data();
        outputValues[index].string_array_lengths = lengths.data();
        outputValues[index].array_length = pointers.size();
    }
    handle->lastErrorCode = WK_OPCDA_OK;
    handle->lastError = "[OPC] SyncRead completed: " +
        std::to_string(itemCount) + " items (method HRESULT=" +
        std::to_string(hresult) + ")";
    return WK_OPCDA_OK;
}

wk_opcda_error_t wk_opcda_write(wk_opcda_handle_t* handle,
                                const uint32_t* clientHandles,
                                const wk_opcda_value_t* values, size_t itemCount,
                                int32_t* itemResults) {
    if (handle == nullptr || clientHandles == nullptr || values == nullptr ||
        itemResults == nullptr || itemCount == 0 || itemCount > 10000) {
        return WK_OPCDA_ERR_ARGUMENT;
    }
    std::lock_guard<std::mutex> guard(handle->mutex);
    if (handle->state != WK_OPCDA_STATE_CONNECTED) {
        return fail(handle, WK_DISCONNECTED,
                    "[OPC] SyncWrite requires a connected IOPCSyncIO association");
    }
    std::vector<uint32_t> serverHandles;
    std::vector<wk::opcda::opcda::SyncWriteValue> writeValues;
    serverHandles.reserve(itemCount);
    writeValues.reserve(itemCount);
    std::string error;
    for (std::size_t index = 0; index < itemCount; ++index) {
        itemResults[index] = 0;
        const auto server = handle->itemServerHandles.find(clientHandles[index]);
        const auto canonical = handle->itemCanonicalTypes.find(clientHandles[index]);
        if (server == handle->itemServerHandles.end() ||
            canonical == handle->itemCanonicalTypes.end()) {
            return fail(handle, WK_OPCDA_ERR_ITEM,
                        "[OPC] SyncWrite client handle was not accepted by AddItems: " +
                            std::to_string(clientHandles[index]));
        }
        wk::opcda::opcda::SyncWriteValue converted{};
        if (!buildSyncWriteValue(values[index], canonical->second, converted, error)) {
            return fail(handle, WK_OPCDA_ERR_WRITE,
                        "[OPC] SyncWrite item " + std::to_string(index) + ": " + error);
        }
        serverHandles.push_back(server->second);
        writeValues.push_back(std::move(converted));
    }
    std::vector<uint8_t> request;
    if (!wk::opcda::opcda::encodeSyncWriteRequest(
            serverHandles, writeValues, handle->oxidResolution.comMajor,
            handle->oxidResolution.comMinor, wk::opcda::dcom::randomCid(),
            request, error)) {
        return fail(handle, WK_OPCDA_ERR_WRITE,
                    "[OPC] encode IOPCSyncIO::Write request failed: " + error);
    }
    const bool integrity =
        handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;
    const bool packetPrivacy =
        handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY;
    const std::chrono::milliseconds timeout(
        handle->requestTimeoutMs == 0 ? 10000 : handle->requestTimeoutMs);
    const uint32_t callId = handle->syncIoCallId++;
    if (handle->syncIoCallId < 3) handle->syncIoCallId = 3;
    constexpr uint16_t contextId = 0;
    constexpr uint16_t writeOperation = 4;
    wk::opcda::rpc::RpcResponse response{};
    if (!invokeProtectedRpc(
            handle->syncIoTransport, handle->syncIoSecurity, integrity,
            packetPrivacy, 1, callId, contextId, writeOperation, request,
            timeout, response, error, &handle->syncIoReference.ipid)) {
        return fail(handle, WK_OPC_WRITE_FAILED,
                    "[OPC] IOPCSyncIO::Write exchange failed: " + error);
    }
    if (response.fault) {
        return fail(handle, WK_OPC_WRITE_FAILED,
                    "[OPC] IOPCSyncIO::Write RPC Fault status=" +
                        std::to_string(response.faultStatus));
    }
    std::vector<uint32_t> decoded;
    uint32_t hresult = 0;
    if (!wk::opcda::opcda::decodeSyncWriteResponse(
            response.stub, itemCount, decoded, hresult, error)) {
        return fail(handle, WK_OPC_WRITE_FAILED,
                    "[OPC] decode IOPCSyncIO::Write response failed: " + error);
    }
    for (std::size_t index = 0; index < decoded.size(); ++index) {
        itemResults[index] = static_cast<int32_t>(decoded[index]);
    }
    handle->lastErrorCode = WK_OPCDA_OK;
    handle->lastError = "[OPC] SyncWrite completed: " +
        std::to_string(itemCount) + " items (method HRESULT=" +
        std::to_string(hresult) + ")";
    return WK_OPCDA_OK;
}

wk_opcda_error_t wk_opcda_browse(wk_opcda_handle_t* handle,
                                 const char* branchItemId,
                                 wk_opcda_browse_entry_t* entries,
                                 size_t capacity, size_t* entryCount) {
    if (handle == nullptr || entries == nullptr || entryCount == nullptr ||
        capacity == 0 || capacity > 10000) {
        return WK_OPCDA_ERR_ARGUMENT;
    }
    std::lock_guard<std::mutex> guard(handle->mutex);
    *entryCount = 0;
    if (handle->state != WK_OPCDA_STATE_CONNECTED) {
        return fail(handle, WK_DISCONNECTED,
                    "[BROWSE] browse requires a connected OPC DA session");
    }

    std::string error;
    const wk_opcda_error_t connected = ensureBrowseConnected(handle, error);
    if (connected != WK_OPCDA_OK) {
        return fail(handle, connected,
                    "[BROWSE] activate IOPCBrowseServerAddressSpace failed: " + error);
    }

    const auto resetBrowseAssociation = [&]() {
        handle->browseTransport.close();
        handle->browseHost.clear();
        handle->browsePort = 0;
        handle->browseReference = {};
        handle->browseCallId = 3;
    };
    const auto browseFailure = [&](wk_opcda_error_t result,
                                   const std::string& message) {
        resetBrowseAssociation();
        return fail(handle, result, "[BROWSE] " + message);
    };

    std::vector<uint8_t> queryRequest =
        wk::opcda::opcda::encodeQueryOrganizationRequest(
            handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
            wk::opcda::dcom::randomCid());
    wk::opcda::rpc::RpcResponse queryResponse{};
    constexpr uint16_t queryOrganizationOperation = 3;
    if (!browseInvoke(handle, queryOrganizationOperation, queryRequest,
                      queryResponse, error)) {
        return browseFailure(WK_OPC_BROWSE_FAILED,
                             "QueryOrganization exchange failed: " + error);
    }
    if (queryResponse.fault) {
        return browseFailure(
            WK_OPC_BROWSE_FAILED,
            "QueryOrganization RPC Fault status=" +
                std::to_string(queryResponse.faultStatus));
    }
    wk::opcda::opcda::BrowseOrganization organization{};
    uint32_t queryHresult = 0;
    if (!wk::opcda::opcda::decodeQueryOrganizationResponse(
            queryResponse.stub, organization, queryHresult, error)) {
        return browseFailure(WK_OPC_BROWSE_FAILED,
                             "decode QueryOrganization failed: " + error);
    }

    const std::string branch = copyString(branchItemId);
    if (organization == wk::opcda::opcda::BrowseOrganization::Hierarchical) {
        std::vector<uint8_t> positionRequest;
        if (!wk::opcda::opcda::encodeChangeBrowsePositionRequest(
                wk::opcda::opcda::BrowseDirection::To, branch,
                handle->oxidResolution.comMajor,
                handle->oxidResolution.comMinor,
                wk::opcda::dcom::randomCid(), positionRequest, error)) {
            return browseFailure(WK_OPC_BROWSE_FAILED,
                                 "encode ChangeBrowsePosition failed: " + error);
        }
        wk::opcda::rpc::RpcResponse positionResponse{};
        constexpr uint16_t changeBrowsePositionOperation = 4;
        if (!browseInvoke(handle, changeBrowsePositionOperation,
                          positionRequest, positionResponse, error)) {
            return browseFailure(WK_OPC_BROWSE_FAILED,
                                 "ChangeBrowsePosition exchange failed: " + error);
        }
        if (positionResponse.fault) {
            return browseFailure(
                WK_OPC_BROWSE_FAILED,
                "ChangeBrowsePosition RPC Fault status=" +
                    std::to_string(positionResponse.faultStatus));
        }
        uint32_t positionHresult = 0;
        if (!wk::opcda::opcda::decodeSimpleBrowseResponse(
                positionResponse.stub, positionHresult, error)) {
            return browseFailure(WK_OPC_BROWSE_FAILED,
                                 "ChangeBrowsePosition failed: " + error);
        }
    }

    std::vector<std::pair<std::string, std::string>> branches;
    std::vector<std::pair<std::string, std::string>> leaves;
    wk_opcda_error_t result = WK_OPCDA_OK;
    if (organization == wk::opcda::opcda::BrowseOrganization::Hierarchical) {
        result = enumerateBrowseType(
            handle, wk::opcda::opcda::BrowseType::Branch, true, capacity,
            branches, error);
        if (result == WK_OPCDA_OK && branches.size() < capacity) {
            result = enumerateBrowseType(
                handle, wk::opcda::opcda::BrowseType::Leaf, false,
                capacity - branches.size(), leaves, error);
        }
    } else {
        result = enumerateBrowseType(
            handle, wk::opcda::opcda::BrowseType::Flat, false, capacity,
            leaves, error);
    }
    if (result != WK_OPCDA_OK) {
        return browseFailure(result, error);
    }

    const std::size_t count = std::min(
        capacity, branches.size() + leaves.size());
    handle->browseNames.clear();
    handle->browseItemIds.clear();
    handle->browseDataTypes.clear();
    handle->browseNames.reserve(count);
    handle->browseItemIds.reserve(count);
    handle->browseDataTypes.reserve(count);
    for (const auto& item : branches) {
        if (handle->browseNames.size() >= count) break;
        handle->browseNames.push_back(item.first);
        handle->browseItemIds.push_back(item.second);
        handle->browseDataTypes.emplace_back();
    }
    for (const auto& item : leaves) {
        if (handle->browseNames.size() >= count) break;
        handle->browseNames.push_back(item.first);
        handle->browseItemIds.push_back(item.second);
        handle->browseDataTypes.emplace_back("auto");
    }
    for (std::size_t index = 0; index < handle->browseNames.size(); ++index) {
        entries[index] = {};
        entries[index].name = handle->browseNames[index].c_str();
        entries[index].item_id = handle->browseItemIds[index].c_str();
        entries[index].data_type = handle->browseDataTypes[index].c_str();
        entries[index].access_rights = 0;
        entries[index].is_branch = index < branches.size() ? 1 : 0;
    }
    *entryCount = handle->browseNames.size();
    handle->lastErrorCode = WK_OPCDA_OK;
    handle->lastError = "[BROWSE] returned " +
        std::to_string(*entryCount) + " entries under '" + branch + "'";
    return WK_OPCDA_OK;
}

wk_opcda_error_t wk_opcda_subscribe(wk_opcda_handle_t* handle,
                                    wk_opcda_data_callback_t callback,
                                    void* userData) {
    if (handle == nullptr || callback == nullptr) return WK_OPCDA_ERR_ARGUMENT;
    std::lock_guard<std::mutex> guard(handle->mutex);
    if (handle->state != WK_OPCDA_STATE_CONNECTED) {
        return fail(handle, WK_DISCONNECTED, "[SUB] cannot subscribe while disconnected");
    }
    if (handle->subscriptionCookie != 0 && handle->callbackServer &&
        handle->callbackServer->running()) {
        handle->subscriptionCallback = callback;
        handle->subscriptionUserData = userData;
        return WK_OPCDA_OK;
    }

    const bool integrity = handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;
    const bool privacy = handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY;
    const uint8_t authLevel = privacy ? 6 : integrity ? 5 : 2;
    const auto timeout = std::chrono::milliseconds(
        handle->requestTimeoutMs == 0 ? 10000 : handle->requestTimeoutMs);
    std::string error;
    const std::string groupName = handle->groupName.empty() ? "WK_OPCDA_GROUP" : handle->groupName;

    std::vector<uint8_t> getContainerRequest;
    if (!wk::opcda::opcda::encodeGetGroupByNameRequest(
            groupName, wk::opcda::opcda::connectionPointContainerIid(),
            handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
            wk::opcda::dcom::randomCid(), getContainerRequest, error)) {
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
                    "[SUB] encode IConnectionPointContainer request failed: " + error);
    }
    wk::opcda::rpc::RpcResponse containerResponse{};
    if (!invokeProtectedRpc(handle->opcServerTransport, handle->opcServerSecurity,
            integrity, privacy, 1, 100, 0, 5, getContainerRequest, timeout,
            containerResponse, error, &handle->opcServerReference.ipid) ||
        containerResponse.fault) {
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
            "[SUB] GetGroupByName(IConnectionPointContainer) failed: " +
            (containerResponse.fault ? std::to_string(containerResponse.faultStatus) : error));
    }
    std::vector<uint8_t> containerObjref;
    uint32_t methodHresult = 0;
    if (!wk::opcda::opcda::decodeInterfacePointerResponse(
            containerResponse.stub, "GetGroupByName(IConnectionPointContainer)",
            containerObjref, methodHresult, error) ||
        !wk::opcda::dcom::decodeStandardObjectReference(
            containerObjref, wk::opcda::opcda::connectionPointContainerIid(),
            handle->connectionPointContainerReference, error)) {
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED, "[SUB] " + error);
    }
    if (!handle->connectionPointContainerTransport.connect(
            handle->host, handle->opcServerPort, timeout, error, handle->localBindIp)) {
        return fail(handle, WK_RPC_CONNECT_FAILED,
                    "[SUB] connect IConnectionPointContainer failed: " + error);
    }
    const wk::opcda::rpc::SyntaxId containerSyntax{
        wk::opcda::opcda::connectionPointContainerIid(), 0, 0};
    const std::vector<uint8_t> diagnostic = wk::opcda::dcom::encodeOrpcThis(
        handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
        wk::opcda::dcom::randomCid());
    wk_opcda_error_t authResult = authenticateRpcEndpoint(
        handle, handle->connectionPointContainerTransport,
        handle->connectionPointContainerSecurity, containerSyntax, 0xffff, false,
        timeout, error, &handle->connectionPointContainerReference.ipid, &diagnostic);
    if (authResult != WK_OPCDA_OK) {
        return fail(handle, authResult,
                    "[SUB] IConnectionPointContainer association failed: " + error);
    }

    const std::vector<uint8_t> findRequest =
        wk::opcda::opcda::encodeFindConnectionPointRequest(
            wk::opcda::opcda::opcDataCallbackIid(), handle->oxidResolution.comMajor,
            handle->oxidResolution.comMinor, wk::opcda::dcom::randomCid());
    wk::opcda::rpc::RpcResponse findResponse{};
    if (!invokeProtectedRpc(handle->connectionPointContainerTransport,
            handle->connectionPointContainerSecurity, integrity, privacy, 1,
            handle->connectionPointContainerCallId++, 0, 4, findRequest, timeout,
            findResponse, error, &handle->connectionPointContainerReference.ipid) ||
        findResponse.fault) {
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED,
            "[SUB] FindConnectionPoint(IOPCDataCallback) failed: " +
            (findResponse.fault ? std::to_string(findResponse.faultStatus) : error));
    }
    std::vector<uint8_t> connectionPointObjref;
    if (!wk::opcda::opcda::decodeInterfacePointerResponse(
            findResponse.stub, "FindConnectionPoint(IOPCDataCallback)",
            connectionPointObjref, methodHresult, error) ||
        !wk::opcda::dcom::decodeStandardObjectReference(
            connectionPointObjref, wk::opcda::opcda::connectionPointIid(),
            handle->connectionPointReference, error)) {
        return fail(handle, WK_DCOM_QUERY_INTERFACE_FAILED, "[SUB] " + error);
    }
    if (!handle->connectionPointTransport.connect(
            handle->host, handle->opcServerPort, timeout, error, handle->localBindIp)) {
        return fail(handle, WK_RPC_CONNECT_FAILED,
                    "[SUB] connect IConnectionPoint failed: " + error);
    }
    const wk::opcda::rpc::SyntaxId pointSyntax{
        wk::opcda::opcda::connectionPointIid(), 0, 0};
    authResult = authenticateRpcEndpoint(
        handle, handle->connectionPointTransport, handle->connectionPointSecurity,
        pointSyntax, 0xffff, false, timeout, error,
        &handle->connectionPointReference.ipid, &diagnostic);
    if (authResult != WK_OPCDA_OK) {
        return fail(handle, authResult,
                    "[SUB] IConnectionPoint association failed: " + error);
    }

    handle->subscriptionCallback = callback;
    handle->subscriptionUserData = userData;
    const wk::opcda::auth::NtlmCredentials callbackCredentials{
        handle->username, handle->password, handle->domain, handle->workstation};
    const auto stopCallback = [handle]() {
        if (handle->callbackServer) handle->callbackServer->stop();
        handle->callbackServer.reset();
        handle->subscriptionCookie = 0;
    };
    const auto startCallback = [&](bool advertiseWinnt) {
        stopCallback();
        handle->callbackServer =
            std::make_unique<wk::opcda::rpc::CallbackServer>();
        return handle->callbackServer->start(
            handle->localBindIp, handle->host, authLevel, callbackCredentials,
            [handle](const wk::opcda::opcda::DataChange& change) {
                deliverDataChange(handle, change);
            }, error, advertiseWinnt);
    };
    const auto advise = [&](wk::opcda::rpc::RpcResponse& response) {
        error.clear();
        methodHresult = 0;
        handle->subscriptionCookie = 0;
        const std::vector<uint8_t> request =
            wk::opcda::opcda::encodeAdviseRequest(
                handle->callbackServer->callbackObjectReference(),
                handle->oxidResolution.comMajor, handle->oxidResolution.comMinor,
                wk::opcda::dcom::randomCid());
        return invokeProtectedRpc(handle->connectionPointTransport,
                   handle->connectionPointSecurity, integrity, privacy, 1,
                   handle->connectionPointCallId++, 0, 5, request, timeout,
                   response, error, &handle->connectionPointReference.ipid) &&
            !response.fault &&
            wk::opcda::opcda::decodeAdviseResponse(
                response.stub, handle->subscriptionCookie,
                methodHresult, error);
    };

    if (!startCallback(true)) {
        stopCallback();
        handle->subscriptionCallback = nullptr;
        handle->subscriptionUserData = nullptr;
        return fail(handle, WK_RPC_CONNECT_FAILED,
                    "[SUB] start callback endpoint failed: " + error);
    }

    wk::opcda::rpc::RpcResponse adviseResponse{};
    bool unauthenticatedCompatibility = false;
    if (!advise(adviseResponse)) {
        const std::string authenticatedAdviseError = adviseResponse.fault
            ? std::to_string(adviseResponse.faultStatus) : error;
        const std::string authenticatedCallbackError =
            handle->callbackServer->lastError();
        const bool missingAuthenticate =
            handle->subscriptionCookie == 0 &&
            authenticatedCallbackError.find("NTLM AUTH3 was not received") !=
                std::string::npos;

        // Some legacy OPC DA servers (including specific Matrikon builds)
        // initiate reverse NTLM but close after the Challenge even when their
        // DCOM identity is configured. Retry only this exact interoperability
        // failure with an AUTHN_NONE callback object. The listener remains
        // pinned to the configured OPC server IP, while the forward DCOM
        // association keeps its requested integrity/privacy level.
        if (missingAuthenticate && startCallback(false)) {
            adviseResponse = {};
            unauthenticatedCompatibility = advise(adviseResponse);
        }
        if (!unauthenticatedCompatibility) {
            const std::string compatibilityAdviseError = adviseResponse.fault
                ? std::to_string(adviseResponse.faultStatus) : error;
            const std::string compatibilityCallbackError =
                handle->callbackServer
                    ? handle->callbackServer->lastError() : std::string{};
            stopCallback();
            handle->subscriptionCallback = nullptr;
            handle->subscriptionUserData = nullptr;
            return fail(handle, WK_OPCDA_ERR_GROUP,
                "[SUB] IConnectionPoint::Advise failed: " +
                authenticatedAdviseError +
                (authenticatedCallbackError.empty() ? std::string{} :
                    "; authenticated callback: " + authenticatedCallbackError) +
                (missingAuthenticate
                    ? "; AUTHN_NONE compatibility retry failed: " +
                        compatibilityAdviseError +
                        (compatibilityCallbackError.empty() ? std::string{} :
                            "; callback endpoint: " + compatibilityCallbackError)
                    : std::string{}));
        }
    }
    handle->lastErrorCode = WK_OPCDA_OK;
    handle->lastError = "[SUB] IOPCDataCallback advised at " +
        handle->callbackServer->advertisedIp() + ":" +
        std::to_string(handle->callbackServer->port()) +
        ", cookie=" + std::to_string(handle->subscriptionCookie) +
        (unauthenticatedCompatibility
            ? ", reverse authentication=AUTHN_NONE compatibility (peer IP pinned)"
            : ", reverse authentication=NTLM");
    return WK_OPCDA_OK;
}

wk_opcda_error_t wk_opcda_unsubscribe(wk_opcda_handle_t* handle) {
    if (handle == nullptr) return WK_OPCDA_ERR_ARGUMENT;
    std::lock_guard<std::mutex> guard(handle->mutex);
    std::string error;
    wk_opcda_error_t result = WK_OPCDA_OK;
    if (handle->subscriptionCookie != 0 && handle->connectionPointTransport.connected()) {
        const bool integrity = handle->securityModeCode != WK_OPCDA_SECURITY_LEGACY_CONNECT;
        const bool privacy = handle->securityModeCode == WK_OPCDA_SECURITY_PACKET_PRIVACY;
        const auto timeout = std::chrono::milliseconds(
            handle->requestTimeoutMs == 0 ? 10000 : handle->requestTimeoutMs);
        const auto request = wk::opcda::opcda::encodeUnadviseRequest(
            handle->subscriptionCookie, handle->oxidResolution.comMajor,
            handle->oxidResolution.comMinor, wk::opcda::dcom::randomCid());
        wk::opcda::rpc::RpcResponse response{};
        uint32_t hresult = 0;
        if (!invokeProtectedRpc(handle->connectionPointTransport,
                handle->connectionPointSecurity, integrity, privacy, 1,
                handle->connectionPointCallId++, 0, 6, request, timeout, response,
                error, &handle->connectionPointReference.ipid) || response.fault ||
            !wk::opcda::opcda::decodeHresultResponse(
                response.stub, "IConnectionPoint::Unadvise", hresult, error)) {
            result = WK_OPCDA_ERR_GROUP;
        }
    }
    if (handle->callbackServer) {
        handle->callbackServer->stop();
        handle->callbackServer.reset();
    }
    handle->subscriptionCookie = 0;
    handle->subscriptionCallback = nullptr;
    handle->subscriptionUserData = nullptr;
    handle->connectionPointTransport.close();
    handle->connectionPointContainerTransport.close();
    if (result != WK_OPCDA_OK) {
        return fail(handle, result, "[SUB] unsubscribe failed: " + error);
    }
    handle->lastErrorCode = WK_OPCDA_OK;
    handle->lastError = "[SUB] IOPCDataCallback unadvised";
    return WK_OPCDA_OK;
}

wk_opcda_error_t wk_opcda_get_status(const wk_opcda_handle_t* handle,
                                     wk_opcda_status_t* status) {
    if (handle == nullptr || status == nullptr) return WK_OPCDA_ERR_ARGUMENT;
    std::lock_guard<std::mutex> guard(handle->mutex);
    status->state = handle->state;
    status->security_mode = handle->securityModeCode;
    status->last_error_code = handle->lastErrorCode;
    status->rpc_endpoint_port = handle->opcServerPort != 0
        ? handle->opcServerPort : handle->activationPort;
    return WK_OPCDA_OK;
}

wk_opcda_error_t wk_opcda_cancel(wk_opcda_handle_t* handle) {
    if (handle == nullptr) return WK_OPCDA_ERR_ARGUMENT;
    stopObjectLifetimeWorker(handle);
    std::lock_guard<std::mutex> guard(handle->mutex);
    if (handle->callbackServer) handle->callbackServer->stop();
    handle->endpointMapper.close();
    handle->activationTransport.close();
    handle->resolverTransport.close();
    handle->opcServerTransport.close();
    handle->itemManagementTransport.close();
    handle->syncIoTransport.close();
    handle->browseTransport.close();
    handle->connectionPointTransport.close();
    handle->connectionPointContainerTransport.close();
    return fail(handle, WK_OPCDA_ERR_CANCELLED, "operation cancelled");
}

void wk_opcda_disconnect(wk_opcda_handle_t* handle) {
    if (handle == nullptr) return;
    stopObjectLifetimeWorker(handle);
    std::lock_guard<std::mutex> guard(handle->mutex);
    if (handle->pingSetId != 0 && handle->resolverTransport.connected()) {
        std::string ignored;
        maintainObjectLifetime(handle, true, ignored);
    }
    handle->pingSetId = 0;
    handle->pingSequence = 1;
    handle->pingOids.clear();
    handle->resolverObjectCallId = 4;
    handle->opcServerReference = {};
    handle->itemManagementReference = {};
    handle->syncIoReference = {};
    handle->connectionPointContainerReference = {};
    handle->connectionPointReference = {};
    if (handle->callbackServer) {
        handle->callbackServer->stop();
        handle->callbackServer.reset();
    }
    handle->endpointMapper.close();
    handle->activationTransport.close();
    handle->resolverTransport.close();
    handle->opcServerTransport.close();
    handle->itemManagementTransport.close();
    handle->syncIoTransport.close();
    handle->browseTransport.close();
    handle->connectionPointTransport.close();
    handle->connectionPointContainerTransport.close();
    handle->subscriptionCookie = 0;
    handle->subscriptionCallback = nullptr;
    handle->subscriptionUserData = nullptr;
    handle->itemServerHandles.clear();
    handle->itemCanonicalTypes.clear();
    handle->readStrings.clear();
    handle->readFloatArrays.clear();
    handle->readStringArrays.clear();
    handle->readStringArrayPointers.clear();
    handle->readStringArrayLengths.clear();
    handle->browseNames.clear();
    handle->browseItemIds.clear();
    handle->browseDataTypes.clear();
    handle->activationPort = 0;
    handle->opcServerPort = 0;
    handle->browseHost.clear();
    handle->browsePort = 0;
    handle->browseReference = {};
    handle->browseCallId = 3;
    handle->state = WK_OPCDA_STATE_DISCONNECTED;
}

void wk_opcda_destroy(wk_opcda_handle_t* handle) {
    if (handle == nullptr) return;
    wk_opcda_disconnect(handle);
    if (handle->callbackServer) {
        handle->callbackServer->stop();
        handle->callbackServer.reset();
    }
    handle->endpointMapper.close();
    handle->activationTransport.close();
    handle->resolverTransport.close();
    handle->opcServerTransport.close();
    handle->itemManagementTransport.close();
    handle->syncIoTransport.close();
    handle->browseTransport.close();
    handle->connectionPointTransport.close();
    handle->connectionPointContainerTransport.close();
    delete handle;
}

const char* wk_opcda_last_error(const wk_opcda_handle_t* handle) {
    if (handle == nullptr) return "invalid OPC DA handle";
    std::lock_guard<std::mutex> guard(handle->mutex);
    // Stable until the caller's next last_error call; the background worker
    // only changes lastError, never the returned snapshot storage.
    handle->lastErrorSnapshot = handle->lastError;
    return handle->lastErrorSnapshot.c_str();
}

}  // extern "C"
