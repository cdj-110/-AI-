#ifndef WK_OPCDA_RPC_CALLBACK_SERVER_H
#define WK_OPCDA_RPC_CALLBACK_SERVER_H

#include "opcda/data_callback.h"
#include "auth/ntlm.h"

#include <array>
#include <atomic>
#include <cstdint>
#include <functional>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

namespace wk::opcda::rpc {

class CallbackServer {
public:
    using DataChangeHandler = std::function<void(const opcda::DataChange&)>;

    CallbackServer();
    ~CallbackServer();
    CallbackServer(const CallbackServer&) = delete;
    CallbackServer& operator=(const CallbackServer&) = delete;

    bool start(const std::string& bindIp, const std::string& remoteHost,
               uint8_t authenticationLevel,
               const auth::NtlmCredentials& callbackCredentials,
               DataChangeHandler handler,
               std::string& error,
               bool advertiseWinnt = true);
    void stop() noexcept;

    bool running() const noexcept { return running_.load(); }
    uint16_t port() const noexcept { return port_; }
    const std::string& advertisedIp() const noexcept { return advertisedIp_; }
    uint64_t oxid() const noexcept { return oxid_; }
    uint64_t oid() const noexcept { return oid_; }
    const std::array<uint8_t, 16>& callbackIpid() const noexcept { return callbackIpid_; }
    std::vector<uint8_t> callbackObjectReference() const;
    std::string lastError() const;

private:
    void run() noexcept;
    void serveConnection(int socket) noexcept;
    void reapFinishedConnections() noexcept;
    void setError(std::string error);

    std::atomic<bool> running_{false};
    int listener_ = -1;
    std::vector<int> connections_;
    std::vector<std::thread> connectionThreads_;
    std::vector<std::shared_ptr<std::atomic<bool>>> connectionFinished_;
    std::mutex connectionMutex_;
    uint16_t port_ = 0;
    uint8_t authenticationLevel_ = 0;
    auth::NtlmCredentials callbackCredentials_{};
    std::string advertisedIp_;
    std::string expectedPeerIp_;
    bool restrictPeerIp_ = false;
    bool advertiseWinnt_ = true;
    uint64_t oxid_ = 0;
    uint64_t oid_ = 0;
    uint32_t associationGroup_ = 0;
    std::array<uint8_t, 16> callbackIpid_{};
    std::array<uint8_t, 16> dataCallbackIpid_{};
    std::array<uint8_t, 16> remoteUnknownIpid_{};
    DataChangeHandler handler_;
    mutable std::mutex mutex_;
    std::string lastError_;
    std::thread thread_;
};

}  // namespace wk::opcda::rpc

#endif
