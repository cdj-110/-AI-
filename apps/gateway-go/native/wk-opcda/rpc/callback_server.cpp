#include "rpc/callback_server.h"
#include "rpc/socket_utils.h"

#include "dcom/object_reference.h"
#include "opcda/connection_point.h"
#include "rpc/rpc_pdu.h"

#include <arpa/inet.h>
#include <cerrno>
#include <chrono>
#include <cstdlib>
#include <cstring>
#include <poll.h>
#include <random>
#include <sys/socket.h>
#include <unistd.h>

#include <algorithm>
#include <sstream>

namespace wk::opcda::rpc {
namespace {

constexpr uint8_t kFirstLast = 0x03;
constexpr uint8_t kObjectUuidFlag = 0x80;
constexpr uint8_t kSupportHeaderSign = 0x04;
constexpr uint8_t kGssNegotiateAuthenticationType = 9;
constexpr uint8_t kNtlmAuthenticationType = 10;

void appendDerLength(std::vector<uint8_t>& output, std::size_t length) {
    if (length < 0x80) {
        output.push_back(static_cast<uint8_t>(length));
        return;
    }
    std::array<uint8_t, sizeof(std::size_t)> bytes{};
    std::size_t count = 0;
    while (length != 0) {
        bytes[bytes.size() - 1 - count] = static_cast<uint8_t>(length);
        length >>= 8;
        ++count;
    }
    output.push_back(static_cast<uint8_t>(0x80 | count));
    output.insert(output.end(), bytes.end() - static_cast<std::ptrdiff_t>(count),
                  bytes.end());
}

std::vector<uint8_t> derValue(uint8_t tag, const std::vector<uint8_t>& value) {
    std::vector<uint8_t> encoded;
    encoded.reserve(value.size() + 5);
    encoded.push_back(tag);
    appendDerLength(encoded, value.size());
    encoded.insert(encoded.end(), value.begin(), value.end());
    return encoded;
}

bool extractNtlmToken(const std::vector<uint8_t>& token,
                      std::vector<uint8_t>& ntlm, std::string& error) {
    static constexpr std::array<uint8_t, 8> signature{
        'N', 'T', 'L', 'M', 'S', 'S', 'P', 0};
    const auto found = std::search(token.begin(), token.end(),
                                   signature.begin(), signature.end());
    if (found == token.end()) {
        std::ostringstream message;
        message << "SPNEGO token does not contain NTLMSSP (size="
                << token.size() << ", prefix=";
        const std::size_t shown = std::min<std::size_t>(token.size(), 12);
        message << std::hex;
        for (std::size_t index = 0; index < shown; ++index) {
            if (index != 0) message << ':';
            message.width(2);
            message.fill('0');
            message << static_cast<unsigned>(token[index]);
        }
        message << ')';
        error = message.str();
        return false;
    }
    ntlm.assign(found, token.end());
    return true;
}

std::vector<uint8_t> encodeSpnegoChallenge(const std::vector<uint8_t>& ntlm) {
    // NegTokenResp ::= [1] SEQUENCE {
    //   negState [0] ENUMERATED accept-incomplete,
    //   supportedMech [1] NTLMSSP OID,
    //   responseToken [2] OCTET STRING NTLM_CHALLENGE }
    static const std::vector<uint8_t> ntlmOid{
        0x06, 0x0a, 0x2b, 0x06, 0x01, 0x04, 0x01, 0x82,
        0x37, 0x02, 0x02, 0x0a};
    const std::vector<uint8_t> state = derValue(0xa0, {0x0a, 0x01, 0x01});
    const std::vector<uint8_t> mechanism = derValue(0xa1, ntlmOid);
    const std::vector<uint8_t> responseToken = derValue(0xa2, derValue(0x04, ntlm));
    std::vector<uint8_t> sequence;
    sequence.insert(sequence.end(), state.begin(), state.end());
    sequence.insert(sequence.end(), mechanism.begin(), mechanism.end());
    sequence.insert(sequence.end(), responseToken.begin(), responseToken.end());
    return derValue(0xa1, derValue(0x30, sequence));
}

uint16_t readU16(const uint8_t* data) {
    return static_cast<uint16_t>(data[0]) |
        static_cast<uint16_t>(static_cast<uint16_t>(data[1]) << 8);
}

uint32_t readU32(const uint8_t* data) {
    return static_cast<uint32_t>(data[0]) |
        (static_cast<uint32_t>(data[1]) << 8) |
        (static_cast<uint32_t>(data[2]) << 16) |
        (static_cast<uint32_t>(data[3]) << 24);
}

uint64_t readU64(const uint8_t* data) {
    return static_cast<uint64_t>(readU32(data)) |
        (static_cast<uint64_t>(readU32(data + 4)) << 32);
}

void appendU16(std::vector<uint8_t>& data, uint16_t value) {
    data.push_back(static_cast<uint8_t>(value));
    data.push_back(static_cast<uint8_t>(value >> 8));
}

void appendU32(std::vector<uint8_t>& data, uint32_t value) {
    data.push_back(static_cast<uint8_t>(value));
    data.push_back(static_cast<uint8_t>(value >> 8));
    data.push_back(static_cast<uint8_t>(value >> 16));
    data.push_back(static_cast<uint8_t>(value >> 24));
}

void appendU64(std::vector<uint8_t>& data, uint64_t value) {
    appendU32(data, static_cast<uint32_t>(value));
    appendU32(data, static_cast<uint32_t>(value >> 32));
}

bool receiveExact(int socket, uint8_t* data, std::size_t size,
                  const std::atomic<bool>& running) {
    std::size_t received = 0;
    while (received < size && running.load()) {
        pollfd descriptor{socket, POLLIN, 0};
        const int ready = ::poll(&descriptor, 1, 1000);
        if (ready == 0) continue;
        if (ready < 0) {
            if (errno == EINTR) continue;
            return false;
        }
        if ((descriptor.revents & (POLLERR | POLLHUP | POLLNVAL)) != 0) return false;
        const ssize_t count = ::recv(socket, data + received, size - received, 0);
        if (count <= 0) {
            if (count < 0 && (errno == EINTR || errno == EAGAIN || errno == EWOULDBLOCK)) continue;
            return false;
        }
        received += static_cast<std::size_t>(count);
    }
    return received == size;
}

bool receiveFragment(int socket, std::vector<uint8_t>& fragment,
                     const std::atomic<bool>& running) {
    uint8_t header[16]{};
    if (!receiveExact(socket, header, sizeof(header), running)) return false;
    const uint16_t length = readU16(header + 8);
    if (length < 16) return false;
    fragment.assign(header, header + 16);
    fragment.resize(length);
    return receiveExact(socket, fragment.data() + 16, length - 16, running);
}

bool sendAll(int socket, const std::vector<uint8_t>& data) {
    std::size_t sent = 0;
    while (sent < data.size()) {
        const ssize_t count = ::send(socket, data.data() + sent,
                                     data.size() - sent, MSG_NOSIGNAL);
        if (count > 0) {
            sent += static_cast<std::size_t>(count);
            continue;
        }
        if (count < 0 && errno == EINTR) continue;
        return false;
    }
    return true;
}

std::array<uint8_t, 16> randomUuid() {
    std::array<uint8_t, 16> value{};
    std::random_device random;
    for (uint8_t& byte : value) byte = static_cast<uint8_t>(random());
    value[7] = static_cast<uint8_t>((value[7] & 0x0f) | 0x40);
    value[8] = static_cast<uint8_t>((value[8] & 0x3f) | 0x80);
    return value;
}

uint64_t randomU64() {
    std::random_device random;
    return (static_cast<uint64_t>(random()) << 32) | random();
}

std::string routeAddress(const std::string& remoteHost) {
    int socket = createSocketCloseOnExec(AF_INET, SOCK_DGRAM, IPPROTO_UDP);
    if (socket < 0) return {};
    sockaddr_in remote{};
    remote.sin_family = AF_INET;
    remote.sin_port = htons(135);
    if (::inet_pton(AF_INET, remoteHost.c_str(), &remote.sin_addr) != 1 ||
        ::connect(socket, reinterpret_cast<sockaddr*>(&remote), sizeof(remote)) != 0) {
        ::close(socket);
        return {};
    }
    sockaddr_in local{};
    socklen_t size = sizeof(local);
    std::string result;
    if (::getsockname(socket, reinterpret_cast<sockaddr*>(&local), &size) == 0) {
        char address[INET_ADDRSTRLEN]{};
        if (::inet_ntop(AF_INET, &local.sin_addr, address, sizeof(address))) result = address;
    }
    ::close(socket);
    return result;
}

std::string hexBytes(const std::vector<uint8_t>& value) {
    static constexpr char digits[] = "0123456789abcdef";
    std::string output;
    output.reserve(value.size() * 2);
    for (const uint8_t byte : value) {
        output.push_back(digits[byte >> 4]);
        output.push_back(digits[byte & 0x0f]);
    }
    return output;
}

struct BindContext {
    uint16_t id = 0;
    SyntaxId syntax{};
    bool ndr32 = false;
    bool bindTimeFeatures = false;
    bool accepted = false;
};

bool isBindTimeFeatureSyntax(const SyntaxId& syntax) {
    static constexpr std::array<uint8_t, 8> prefix{
        0x2c, 0x1c, 0xb7, 0x6c, 0x12, 0x98, 0x40, 0x45};
    return std::equal(prefix.begin(), prefix.end(), syntax.uuid.begin()) &&
        syntax.major == 1 && syntax.minor == 0;
}

bool decodeBindContexts(const std::vector<uint8_t>& request,
                        std::vector<BindContext>& contexts,
                        std::string& error,
                        PacketType expectedPacketType = PacketType::Bind) {
    CommonHeader header{};
    contexts.clear();
    if (!decodeCommonHeader(request.data(), request.size(), header, error) ||
        header.packetType != expectedPacketType || request.size() < 28) return false;
    const uint8_t count = request[24];
    if (count == 0) {
        error = "callback bind contains no presentation contexts";
        return false;
    }
    const SyntaxId ndr = ndr32Syntax();
    std::size_t offset = 28;
    for (uint8_t index = 0; index < count; ++index) {
        if (offset + 24 > request.size()) {
            error = "callback bind presentation context is truncated";
            return false;
        }
        BindContext context{};
        context.id = readU16(request.data() + offset);
        const uint8_t transferCount = request[offset + 2];
        std::copy_n(request.data() + offset + 4, 16, context.syntax.uuid.begin());
        context.syntax.major = readU16(request.data() + offset + 20);
        context.syntax.minor = readU16(request.data() + offset + 22);
        offset += 24;
        if (transferCount == 0 || offset + static_cast<std::size_t>(transferCount) * 20 > request.size()) {
            error = "callback bind transfer syntax list is truncated";
            return false;
        }
        for (uint8_t transfer = 0; transfer < transferCount; ++transfer) {
            SyntaxId offered{};
            std::copy_n(request.data() + offset, 16, offered.uuid.begin());
            offered.major = readU16(request.data() + offset + 16);
            offered.minor = readU16(request.data() + offset + 18);
            if (offered.uuid == ndr.uuid && offered.major == ndr.major &&
                offered.minor == ndr.minor) {
                context.ndr32 = true;
            }
            if (isBindTimeFeatureSyntax(offered)) {
                context.bindTimeFeatures = true;
            }
            offset += 20;
        }
        contexts.push_back(context);
    }
    return true;
}

std::vector<uint8_t> encodeBindAck(
    uint32_t callId, const std::vector<BindContext>& contexts,
    uint32_t associationGroup,
    uint8_t authenticationType = kNtlmAuthenticationType,
    uint8_t authenticationLevel = 0, uint32_t authenticationContextId = 0,
    const std::vector<uint8_t>& authenticationToken = {},
    bool supportsHeaderSigning = false,
    PacketType packetType = PacketType::BindAck,
    const std::string& secondaryAddress = {},
    uint16_t maximumTransmitFragment = 4280,
    uint16_t maximumReceiveFragment = 4280) {
    const SyntaxId ndr = ndr32Syntax();
    std::vector<uint8_t> response{
        5, 0, static_cast<uint8_t>(packetType), kFirstLast,
        0x10, 0, 0, 0, 0, 0, 0, 0};
    appendU32(response, callId);
    appendU16(response, maximumTransmitFragment);
    appendU16(response, maximumReceiveFragment);
    appendU32(response, associationGroup);
    appendU16(response, static_cast<uint16_t>(secondaryAddress.size() + 1));
    response.insert(response.end(), secondaryAddress.begin(),
                    secondaryAddress.end());
    response.push_back(0);
    while (response.size() % 4 != 0) response.push_back(0);
    response.push_back(static_cast<uint8_t>(contexts.size()));
    response.insert(response.end(), {0, 0, 0});
    for (const BindContext& context : contexts) {
        if (context.bindTimeFeatures) {
            appendU16(response, 3);  // negotiate_ack
            appendU16(response, 0);  // no optional bind-time features enabled
            response.insert(response.end(), 20, 0);
            continue;
        }
        appendU16(response, context.accepted ? 0 : 2);
        appendU16(response, context.accepted ? 0 : (context.ndr32 ? 1 : 2));
        response.insert(response.end(), ndr.uuid.begin(), ndr.uuid.end());
        appendU16(response, ndr.major);
        appendU16(response, ndr.minor);
    }
    if (!authenticationToken.empty()) {
        if (supportsHeaderSigning) response[3] |= kSupportHeaderSign;
        // Connection-oriented BindAck authentication verifiers are aligned to
        // the NDR/RPC 4-byte boundary. Windows does not add the 16-byte request
        // stub padding used by protected ORPC Request/Response PDUs here.
        const uint8_t padding = static_cast<uint8_t>(
            (4 - (response.size() % 4)) % 4);
        response.insert(response.end(), padding, 0);
        response.insert(response.end(), {authenticationType, authenticationLevel, padding, 0});
        appendU32(response, authenticationContextId);
        response.insert(response.end(), authenticationToken.begin(),
                        authenticationToken.end());
        const uint16_t authLength =
            static_cast<uint16_t>(authenticationToken.size());
        response[10] = static_cast<uint8_t>(authLength);
        response[11] = static_cast<uint8_t>(authLength >> 8);
    }
    const uint16_t length = static_cast<uint16_t>(response.size());
    response[8] = static_cast<uint8_t>(length);
    response[9] = static_cast<uint8_t>(length >> 8);
    return response;
}

std::vector<uint8_t> encodeResponse(uint32_t callId, uint16_t contextId,
                                    const std::vector<uint8_t>& stub) {
    std::vector<uint8_t> response{
        5, 0, static_cast<uint8_t>(PacketType::Response), kFirstLast,
        0x10, 0, 0, 0, 0, 0, 0, 0};
    appendU32(response, callId);
    appendU32(response, static_cast<uint32_t>(stub.size()));
    appendU16(response, contextId);
    response.push_back(0);
    response.push_back(0);
    response.insert(response.end(), stub.begin(), stub.end());
    const uint16_t length = static_cast<uint16_t>(response.size());
    response[8] = static_cast<uint8_t>(length);
    response[9] = static_cast<uint8_t>(length >> 8);
    return response;
}

std::vector<uint8_t> successOrpcThat() {
    // ORPCTHAT is flags (ULONG) plus a null unique extensions pointer.
    return std::vector<uint8_t>(8, 0);
}

bool decodeRemQueryInterfaceRequest(
    const std::vector<uint8_t>& stub,
    const std::array<uint8_t, 16>& expectedIpid,
    uint32_t& publicReferences,
    std::vector<std::array<uint8_t, 16>>& iids,
    std::string& error) {
    if (stub.size() < 60) {
        error = "IRemUnknown::RemQueryInterface request is truncated";
        return false;
    }
    if (!std::equal(expectedIpid.begin(), expectedIpid.end(), stub.begin() + 32)) {
        error = "IRemUnknown::RemQueryInterface targets an unknown IPID";
        return false;
    }
    publicReferences = readU32(stub.data() + 48);
    const uint16_t count = readU16(stub.data() + 52);
    const uint32_t maximum = readU32(stub.data() + 56);
    if (count == 0 || maximum != count ||
        60 + static_cast<std::size_t>(count) * 16 != stub.size()) {
        error = "IRemUnknown::RemQueryInterface IID array is invalid";
        return false;
    }
    iids.resize(count);
    for (std::size_t index = 0; index < count; ++index) {
        std::copy_n(stub.data() + 60 + index * 16, 16, iids[index].begin());
    }
    return true;
}

void appendStdObjref(std::vector<uint8_t>& stub, uint32_t publicReferences,
                     uint64_t oxid, uint64_t oid,
                     const std::array<uint8_t, 16>& ipid) {
    appendU32(stub, 0);
    appendU32(stub, publicReferences);
    appendU64(stub, oxid);
    appendU64(stub, oid);
    stub.insert(stub.end(), ipid.begin(), ipid.end());
}

std::vector<uint8_t> encodeRemQueryInterfaceResponse(
    const std::vector<std::array<uint8_t, 16>>& iids,
    uint32_t publicReferences, uint64_t oxid, uint64_t oid,
    const std::array<uint8_t, 16>& unknownIpid,
    const std::array<uint8_t, 16>& dataCallbackIpid) {
    constexpr uint32_t kENoInterface = 0x80004002;
    std::vector<uint8_t> response = successOrpcThat();
    appendU32(response, 0x00020000);
    appendU32(response, static_cast<uint32_t>(iids.size()));
    for (const auto& requested : iids) {
        const bool wantsUnknown = requested == opcda::unknownIid();
        const bool wantsCallback = requested == opcda::opcDataCallbackIid();
        appendU32(response, wantsUnknown || wantsCallback ? 0 : kENoInterface);
        // REMQIRESULT is NDR-marshaled here (not the 44-byte packet form).
        // Its embedded STDOBJREF has 8-byte alignment because it contains
        // OXID/OID hyper integers, so four bytes of padding follow HRESULT.
        appendU32(response, 0);
        if (wantsUnknown || wantsCallback) {
            appendStdObjref(response, publicReferences, oxid, oid,
                            wantsCallback ? dataCallbackIpid : unknownIpid);
        } else {
            response.insert(response.end(), 40, 0);
        }
    }
    appendU32(response, 0);
    return response;
}

}  // namespace

CallbackServer::CallbackServer()
    : oxid_(randomU64()), oid_(randomU64()), callbackIpid_(randomUuid()),
      dataCallbackIpid_(randomUuid()), remoteUnknownIpid_(randomUuid()) {}

CallbackServer::~CallbackServer() { stop(); }

bool CallbackServer::start(const std::string& bindIp,
                           const std::string& remoteHost,
                           uint8_t authenticationLevel,
                           const auth::NtlmCredentials& callbackCredentials,
                           DataChangeHandler handler, std::string& error,
                           bool advertiseWinnt) {
    stop();
    // A stopped callback listener must never be restarted under the same
    // exporter identity. Windows caches OXID resolution, including the
    // dynamic TCP endpoint; reusing the OXID after a UI hot reload makes COM
    // reconnect to the stale listener and Advise fails with CONNECT_E_CANNOTCONNECT.
    oxid_ = randomU64();
    oid_ = randomU64();
    callbackIpid_ = randomUuid();
    dataCallbackIpid_ = randomUuid();
    remoteUnknownIpid_ = randomUuid();
    associationGroup_ = static_cast<uint32_t>(randomU64());
    if (associationGroup_ == 0) associationGroup_ = 1;
    {
        std::lock_guard<std::mutex> guard(mutex_);
        lastError_.clear();
    }
    advertisedIp_ = bindIp.empty() ? routeAddress(remoteHost) : bindIp;
    if (advertisedIp_.empty()) {
        error = "cannot determine the callback IPv4 address; configure localBindIp";
        return false;
    }
    expectedPeerIp_ = remoteHost;
    sockaddr_in expectedPeer{};
    restrictPeerIp_ = ::inet_pton(
        AF_INET, expectedPeerIp_.c_str(), &expectedPeer.sin_addr) == 1;
    listener_ = createSocketCloseOnExec(AF_INET, SOCK_STREAM, IPPROTO_TCP);
    if (listener_ < 0) {
        error = std::string("create callback listener failed: ") + std::strerror(errno);
        return false;
    }
    int reuse = 1;
    ::setsockopt(listener_, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse));
    sockaddr_in address{};
    address.sin_family = AF_INET;
    address.sin_port = 0;
    if (::inet_pton(AF_INET, advertisedIp_.c_str(), &address.sin_addr) != 1 ||
        ::bind(listener_, reinterpret_cast<sockaddr*>(&address), sizeof(address)) != 0 ||
        ::listen(listener_, 8) != 0) {
        error = std::string("bind callback listener on ") + advertisedIp_ +
            " failed: " + std::strerror(errno);
        ::close(listener_);
        listener_ = -1;
        return false;
    }
    socklen_t addressSize = sizeof(address);
    if (::getsockname(listener_, reinterpret_cast<sockaddr*>(&address), &addressSize) != 0) {
        error = "read callback listener port failed";
        ::close(listener_);
        listener_ = -1;
        return false;
    }
    port_ = ntohs(address.sin_port);
    authenticationLevel_ = authenticationLevel;
    advertiseWinnt_ = advertiseWinnt;
    callbackCredentials_ = callbackCredentials;
    handler_ = std::move(handler);
    running_.store(true);
    thread_ = std::thread(&CallbackServer::run, this);
    return true;
}

void CallbackServer::stop() noexcept {
    running_.store(false);
    if (listener_ >= 0) {
        ::shutdown(listener_, SHUT_RDWR);
        ::close(listener_);
        listener_ = -1;
    }
    if (thread_.joinable()) thread_.join();
    {
        std::lock_guard<std::mutex> guard(connectionMutex_);
        for (int connection : connections_) ::shutdown(connection, SHUT_RDWR);
    }
    for (std::thread& connectionThread : connectionThreads_) {
        if (connectionThread.joinable()) connectionThread.join();
    }
    for (int connection : connections_) ::close(connection);
    connections_.clear();
    connectionThreads_.clear();
    connectionFinished_.clear();
    port_ = 0;
    handler_ = {};
    callbackCredentials_ = {};
    expectedPeerIp_.clear();
    restrictPeerIp_ = false;
}

std::vector<uint8_t> CallbackServer::callbackObjectReference() const {
    return dcom::encodeStandardObjectReference(
        opcda::unknownIid(), oxid_, oid_, callbackIpid_,
        advertisedIp_, port_, 1, advertiseWinnt_);
}

std::string CallbackServer::lastError() const {
    std::lock_guard<std::mutex> guard(mutex_);
    return lastError_;
}

void CallbackServer::setError(std::string error) {
    std::lock_guard<std::mutex> guard(mutex_);
    if (error.empty() || lastError_.find(error) != std::string::npos) return;
    if (!lastError_.empty()) lastError_ += " | ";
    lastError_ += std::move(error);
    if (lastError_.size() > 2048) {
        lastError_.erase(0, lastError_.size() - 2048);
    }
}

void CallbackServer::run() noexcept {
    while (running_.load()) {
        reapFinishedConnections();
        pollfd descriptor{listener_, POLLIN, 0};
        const int ready = ::poll(&descriptor, 1, 1000);
        if (ready == 0 || (ready < 0 && errno == EINTR)) continue;
        if (ready < 0 || !running_.load()) break;
        sockaddr_in peer{};
        socklen_t peerSize = sizeof(peer);
        const int accepted = acceptCloseOnExec(
            listener_, reinterpret_cast<sockaddr*>(&peer), &peerSize);
        if (accepted < 0) continue;
        if (restrictPeerIp_) {
            char peerIp[INET_ADDRSTRLEN]{};
            const char* decoded = ::inet_ntop(
                AF_INET, &peer.sin_addr, peerIp, sizeof(peerIp));
            if (decoded == nullptr || expectedPeerIp_ != peerIp) {
                setError("rejected callback connection from unexpected peer " +
                    std::string(decoded == nullptr ? "<invalid>" : peerIp) +
                    "; expected " + expectedPeerIp_);
                ::close(accepted);
                continue;
            }
        }
        std::lock_guard<std::mutex> guard(connectionMutex_);
        auto finished = std::make_shared<std::atomic<bool>>(false);
        connections_.push_back(accepted);
        connectionFinished_.push_back(finished);
        connectionThreads_.emplace_back(
            [this, accepted, finished]() noexcept {
                serveConnection(accepted);
                finished->store(true, std::memory_order_release);
            });
    }
}

void CallbackServer::reapFinishedConnections() noexcept {
    std::lock_guard<std::mutex> guard(connectionMutex_);
    for (std::size_t index = 0; index < connectionThreads_.size();) {
        if (!connectionFinished_[index]->load(std::memory_order_acquire)) {
            ++index;
            continue;
        }
        if (connectionThreads_[index].joinable()) {
            connectionThreads_[index].join();
        }
        ::close(connections_[index]);
        connections_.erase(connections_.begin() + index);
        connectionThreads_.erase(connectionThreads_.begin() + index);
        connectionFinished_.erase(connectionFinished_.begin() + index);
    }
}

void CallbackServer::serveConnection(int socket) noexcept {
    std::vector<uint8_t> fragment;
    if (!receiveFragment(socket, fragment, running_)) return;
    CommonHeader bindHeader{};
    std::string error;
    if (!decodeCommonHeader(fragment.data(), fragment.size(), bindHeader, error)) {
        setError(error);
        return;
    }
    const SyntaxId exporter = objectExporterSyntax();
    const SyntaxId callback{opcda::opcDataCallbackIid(), 0, 0};
    const SyntaxId remoteUnknown{opcda::remoteUnknownIid(), 0, 0};
    const SyntaxId remoteUnknown2{opcda::remoteUnknown2Iid(), 0, 0};
    std::vector<BindContext> contexts;
    const bool decoded = decodeBindContexts(fragment, contexts, error);
    SecurityTrailer bindSecurity{};
    std::size_t bindBodyEnd = 0;
    const bool securityDecoded = decoded && decodeSecurityTrailer(
        fragment, bindHeader, bindSecurity, bindBodyEnd, error);
    bool anyAccepted = false;
    if (securityDecoded) {
        for (BindContext& context : contexts) {
            context.accepted = context.ndr32 &&
                ((context.syntax.uuid == exporter.uuid &&
                  context.syntax.major == exporter.major) ||
                 (context.syntax.uuid == callback.uuid &&
                  context.syntax.major == callback.major) ||
                 (context.syntax.uuid == remoteUnknown.uuid &&
                  context.syntax.major == remoteUnknown.major) ||
                 (context.syntax.uuid == remoteUnknown2.uuid &&
                  context.syntax.major == remoteUnknown2.major));
            anyAccepted = anyAccepted || context.accepted;
        }
    }
    // A few legacy COM runtimes apply a process-wide WINNT proxy blanket even
    // when the callback OBJREF explicitly advertises RPC_C_AUTHN_NONE. In the
    // compatibility listener, accept that Bind as unauthenticated by omitting
    // the challenge from BindAck. This is deliberately limited to an OBJREF
    // that was published without WINNT and the listener is still peer-IP
    // pinned by start().
    const bool authenticated = advertiseWinnt_ && securityDecoded &&
        bindHeader.authLength != 0;
    const bool performNtlmHandshake = authenticated;
    const bool spnego = performNtlmHandshake &&
        bindSecurity.authenticationType == kGssNegotiateAuthenticationType;
    auth::NtlmChallenge ntlmChallenge{};
    std::vector<uint8_t> negotiateToken;
    std::vector<uint8_t> challengeToken;
    if (performNtlmHandshake) {
        const std::string callbackComputer =
            callbackCredentials_.workstation.empty()
                ? std::string("EG100")
                : callbackCredentials_.workstation;
        if ((bindSecurity.authenticationType != kNtlmAuthenticationType && !spnego) ||
            (bindSecurity.authenticationLevel != 2 &&
             bindSecurity.authenticationLevel != 5 &&
             bindSecurity.authenticationLevel != 6) ||
            !(spnego
                ? extractNtlmToken(bindSecurity.token, negotiateToken, error)
                : (negotiateToken = bindSecurity.token, true)) ||
            !auth::encodeNtlmChallenge(
                negotiateToken, callbackComputer, callbackComputer,
                ntlmChallenge, error)) {
            if (error.empty()) {
                error = "unsupported inbound callback authentication: type=" +
                    std::to_string(bindSecurity.authenticationType) +
                    ", level=" +
                    std::to_string(bindSecurity.authenticationLevel);
            }
            setError(error);
            return;
        }
        challengeToken = spnego
            ? encodeSpnegoChallenge(ntlmChallenge.rawMessage)
            : ntlmChallenge.rawMessage;
    }
    const std::vector<uint8_t> bindAckResponse = encodeBindAck(
            bindHeader.callId, contexts, associationGroup_,
            performNtlmHandshake ? bindSecurity.authenticationType
                          : kNtlmAuthenticationType,
            performNtlmHandshake ? bindSecurity.authenticationLevel : 0,
            performNtlmHandshake ? bindSecurity.contextId : 0, challengeToken,
            performNtlmHandshake &&
                bindSecurity.authenticationLevel >= 5 &&
                (bindHeader.flags & kSupportHeaderSign) != 0,
            PacketType::BindAck, std::to_string(port_),
            readU16(fragment.data() + 16),
            readU16(fragment.data() + 18));
    if (!securityDecoded ||
        !sendAll(socket, bindAckResponse) ||
        !anyAccepted) {
        if (!anyAccepted) setError(error.empty() ? "unsupported callback bind syntax" : error);
        return;
    }
    auth::NtlmSignatureState inboundSecurity;
    const bool packetProtected = authenticated &&
        (bindSecurity.authenticationLevel == 5 ||
         bindSecurity.authenticationLevel == 6);
    const bool packetPrivacy = authenticated && bindSecurity.authenticationLevel == 6;
    if (performNtlmHandshake) {
        std::vector<uint8_t> auth3;
        if (!receiveFragment(socket, auth3, running_)) {
            std::string acceptedContexts;
            for (const BindContext& context : contexts) {
                if (!context.accepted) continue;
                if (!acceptedContexts.empty()) acceptedContexts += ',';
                if (context.syntax.uuid == exporter.uuid) acceptedContexts += "IObjectExporter";
                else if (context.syntax.uuid == callback.uuid) acceptedContexts += "IOPCDataCallback";
                else if (context.syntax.uuid == remoteUnknown.uuid) acceptedContexts += "IRemUnknown";
                else if (context.syntax.uuid == remoteUnknown2.uuid) acceptedContexts += "IRemUnknown2";
            }
            std::string auth3Error =
                "callback NTLM AUTH3 was not received; the Windows/Matrikon "
                "process did not provide outbound NTLM credentials "
                "(configure the Matrikon DCOM identity as the OPC account or "
                "allow an unauthenticated callback): type=" +
                std::to_string(bindSecurity.authenticationType) +
                ", level=" + std::to_string(bindSecurity.authenticationLevel) +
                ", bindFlags=" + std::to_string(bindHeader.flags) +
                ", contexts=" + acceptedContexts;
            if (std::getenv("WK_OPCDA_AUTH_DIAGNOSTICS") != nullptr) {
                auth3Error +=
                    ", negotiateFlags=" +
                        std::to_string(readU32(negotiateToken.data() + 12)) +
                    ", challengeFlags=" +
                        std::to_string(ntlmChallenge.flags) +
                    ", negotiateHex=" + hexBytes(negotiateToken) +
                    ", bindHex=" + hexBytes(fragment) +
                    ", challengeHex=" + hexBytes(ntlmChallenge.rawMessage) +
                    ", bindAckHex=" + hexBytes(bindAckResponse);
            }
            setError(std::move(auth3Error));
            return;
        }
        CommonHeader auth3Header{};
        SecurityTrailer authenticateSecurity{};
        std::size_t authenticateBodyEnd = 0;
        auth::NtlmAuthenticateResult authenticatedResult{};
        std::vector<uint8_t> authenticateToken;
        if (!decodeCommonHeader(auth3.data(), auth3.size(), auth3Header, error) ||
            auth3Header.packetType != PacketType::Auth3 ||
            !decodeSecurityTrailer(
                auth3, auth3Header, authenticateSecurity,
                authenticateBodyEnd, error) ||
            authenticateSecurity.authenticationType !=
                bindSecurity.authenticationType ||
            authenticateSecurity.contextId != bindSecurity.contextId ||
            !(spnego
                ? extractNtlmToken(
                    authenticateSecurity.token, authenticateToken, error)
                : (authenticateToken = authenticateSecurity.token, true))) {
            setError(error.empty() ? "callback NTLM AUTH3 validation failed" : error);
            return;
        }
        static constexpr std::array<uint8_t, 8> ntlmSignature{
            'N', 'T', 'L', 'M', 'S', 'S', 'P', 0};
        const bool anonymousConnect = bindSecurity.authenticationLevel == 2 &&
            authenticateToken.size() >= 64 &&
            std::equal(ntlmSignature.begin(), ntlmSignature.end(),
                       authenticateToken.begin()) &&
            readU32(authenticateToken.data() + 8) == 3 &&
            readU16(authenticateToken.data() + 20) == 0 &&
            (readU32(authenticateToken.data() + 60) & 0x00000800u) != 0;
        if (!anonymousConnect &&
            (!auth::acceptNtlmAuthenticate(
                callbackCredentials_, negotiateToken, ntlmChallenge,
                authenticateToken, authenticatedResult, error) ||
             !inboundSecurity.initialize(authenticatedResult))) {
            setError(error.empty() ? "callback NTLM AUTH3 validation failed" : error);
            return;
        }
    }

    while (running_.load()) {
        fragment.clear();
        if (!receiveFragment(socket, fragment, running_)) return;
        CommonHeader header{};
        if (!decodeCommonHeader(fragment.data(), fragment.size(), header, error)) {
            setError(error);
            return;
        }
        if (packetProtected && !verifyPacketIntegrityClient(
                fragment, bindSecurity.contextId, inboundSecurity,
                error, packetPrivacy, bindSecurity.authenticationType)) {
            setError(error);
            return;
        }
        if (header.packetType == PacketType::AlterContext) {
            std::vector<BindContext> additions;
            SecurityTrailer alterSecurity{};
            std::size_t alterBodyEnd = 0;
            if (!decodeBindContexts(
                    fragment, additions, error, PacketType::AlterContext) ||
                !decodeSecurityTrailer(
                    fragment, header, alterSecurity, alterBodyEnd, error)) {
                setError(error.empty()
                    ? "invalid callback AlterContext request" : error);
                return;
            }
            if (header.authLength != 0 &&
                (!packetProtected ||
                 alterSecurity.authenticationType !=
                     bindSecurity.authenticationType ||
                 alterSecurity.contextId != bindSecurity.contextId)) {
                setError("callback AlterContext uses an unexpected security context");
                return;
            }
            bool acceptedAddition = false;
            for (BindContext& addition : additions) {
                addition.accepted = addition.ndr32 &&
                    ((addition.syntax.uuid == exporter.uuid &&
                      addition.syntax.major == exporter.major) ||
                     (addition.syntax.uuid == callback.uuid &&
                      addition.syntax.major == callback.major) ||
                     (addition.syntax.uuid == remoteUnknown.uuid &&
                      addition.syntax.major == remoteUnknown.major) ||
                     (addition.syntax.uuid == remoteUnknown2.uuid &&
                      addition.syntax.major == remoteUnknown2.major));
                acceptedAddition = acceptedAddition || addition.accepted;
            }
            std::vector<uint8_t> response = encodeBindAck(
                header.callId, additions, associationGroup_,
                kNtlmAuthenticationType, 0, 0, {}, false,
                PacketType::AlterContextResponse, std::to_string(port_),
                readU16(fragment.data() + 16),
                readU16(fragment.data() + 18));
            if (packetProtected && !protectPacketIntegrityServer(
                    response, bindSecurity.contextId, inboundSecurity,
                    error, packetPrivacy,
                    bindSecurity.authenticationType)) {
                setError(error);
                return;
            }
            if (!sendAll(socket, response) || !acceptedAddition) {
                if (!acceptedAddition) {
                    setError("unsupported callback AlterContext syntax");
                }
                return;
            }
            for (BindContext& addition : additions) {
                const auto existing = std::find_if(
                    contexts.begin(), contexts.end(),
                    [&addition](const BindContext& candidate) {
                        return candidate.id == addition.id;
                    });
                if (existing == contexts.end()) {
                    contexts.push_back(std::move(addition));
                } else {
                    *existing = std::move(addition);
                }
            }
            continue;
        }
        if (header.packetType != PacketType::Request || fragment.size() < 24) return;
        SecurityTrailer requestSecurity{};
        std::size_t requestBodyEnd = 0;
        if (!decodeSecurityTrailer(
                fragment, header, requestSecurity, requestBodyEnd, error)) {
            setError(error);
            return;
        }
        if (header.authLength != 0 &&
            (requestSecurity.authenticationType !=
                 bindSecurity.authenticationType ||
             requestSecurity.contextId != bindSecurity.contextId)) {
            setError("callback request uses an unexpected security context");
            return;
        }
        const uint16_t requestContext = readU16(fragment.data() + 20);
        const auto context = std::find_if(
            contexts.begin(), contexts.end(),
            [requestContext](const BindContext& candidate) {
                return candidate.accepted && candidate.id == requestContext;
            });
        if (context == contexts.end()) {
            setError("callback request uses an unaccepted presentation context");
            return;
        }
        const uint16_t opnum = readU16(fragment.data() + 22);
        std::size_t stubOffset = 24;
        if ((header.flags & kObjectUuidFlag) != 0) stubOffset += 16;
        if (stubOffset > requestBodyEnd) return;
        std::vector<uint8_t> requestStub(fragment.begin() + static_cast<std::ptrdiff_t>(stubOffset),
                                         fragment.begin() + static_cast<std::ptrdiff_t>(requestBodyEnd));
        std::vector<uint8_t> responseStub;
        if (context->syntax.uuid == exporter.uuid) {
            if (opnum == 3) {
                appendU32(responseStub, 0);
            } else if (opnum == 5) {
                responseStub = dcom::encodeServerAlive2Response(
                    advertisedIp_, 5, 5, 0, advertiseWinnt_);
            } else if (opnum == 0 || opnum == 4) {
                if (requestStub.size() < 8) {
                    setError("truncated IObjectExporter ResolveOxid request");
                    return;
                }
                const uint64_t requestedOxid = readU64(requestStub.data());
                if (requestedOxid != oxid_) {
                    setError("IObjectExporter ResolveOxid OXID mismatch: requested=" +
                        std::to_string(requestedOxid) + ", expected=" +
                        std::to_string(oxid_));
                    return;
                }
                responseStub = opnum == 4
                    ? dcom::encodeResolveOxid2Response(
                        advertisedIp_, port_, remoteUnknownIpid_,
                        1, 5, 5, 0, advertiseWinnt_)
                    : dcom::encodeResolveOxidResponse(
                        advertisedIp_, port_, remoteUnknownIpid_,
                        1, 0, advertiseWinnt_);
            } else {
                setError("unsupported IObjectExporter operation " + std::to_string(opnum));
                return;
            }
        } else if (context->syntax.uuid == remoteUnknown.uuid ||
                   context->syntax.uuid == remoteUnknown2.uuid) {
            if (opnum == 3) {
                uint32_t publicReferences = 0;
                std::vector<std::array<uint8_t, 16>> requestedIids;
                if (!decodeRemQueryInterfaceRequest(
                        requestStub, callbackIpid_, publicReferences,
                        requestedIids, error)) {
                    setError(error);
                    return;
                }
                responseStub = encodeRemQueryInterfaceResponse(
                    requestedIids, publicReferences, oxid_, oid_,
                    callbackIpid_, dataCallbackIpid_);
            } else if (opnum == 4) {
                if (requestStub.size() < 34) {
                    setError("truncated IRemUnknown::RemAddRef request");
                    return;
                }
                const uint16_t count = readU16(requestStub.data() + 32);
                responseStub = successOrpcThat();
                appendU32(responseStub, count);
                for (uint16_t index = 0; index < count; ++index) {
                    appendU32(responseStub, 0);
                }
                appendU32(responseStub, 0);
            } else if (opnum == 5) {
                responseStub = successOrpcThat();
                appendU32(responseStub, 0);
            } else if (opnum == 6 &&
                       context->syntax.uuid == remoteUnknown2.uuid) {
                setError("IRemUnknown2::RemQueryInterface2 received: request size=" +
                    std::to_string(requestStub.size()));
                return;
            } else {
                setError("unsupported IRemUnknown operation " + std::to_string(opnum));
                return;
            }
        } else if (opnum == 3 || opnum == 4) {
            opcda::DataChange change{};
            if (!opcda::decodeDataChangeRequest(requestStub, change, error)) {
                setError(error);
                return;
            }
            if (handler_) handler_(change);
            responseStub = successOrpcThat();
            appendU32(responseStub, 0);  // HRESULT S_OK
        } else if (opnum == 5 || opnum == 6) {
            responseStub = successOrpcThat();
            appendU32(responseStub, 0);  // HRESULT S_OK
        } else {
            setError("unsupported IOPCDataCallback operation " + std::to_string(opnum));
            return;
        }
        std::vector<uint8_t> response =
            encodeResponse(header.callId, requestContext, responseStub);
        if (packetProtected && !protectPacketIntegrityServer(
                response, bindSecurity.contextId, inboundSecurity,
                error, packetPrivacy, bindSecurity.authenticationType)) {
            setError(error);
            return;
        }
        if (!sendAll(socket, response)) return;
    }
}

}  // namespace wk::opcda::rpc
