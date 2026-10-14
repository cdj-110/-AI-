#ifndef WK_OPCDA_RPC_TRANSPORT_H
#define WK_OPCDA_RPC_TRANSPORT_H

#include <chrono>
#include <cstddef>
#include <cstdint>
#include <string>
#include <vector>

namespace wk::opcda::rpc {

class TcpTransport {
public:
    TcpTransport() = default;
    ~TcpTransport();
    TcpTransport(const TcpTransport&) = delete;
    TcpTransport& operator=(const TcpTransport&) = delete;

    bool connect(const std::string& host, uint16_t port, std::chrono::milliseconds timeout,
                 std::string& error, const std::string& localBindIp = {});
    bool sendAll(const uint8_t* data, std::size_t size, std::chrono::milliseconds timeout, std::string& error);
    bool receiveExact(uint8_t* data, std::size_t size, std::chrono::milliseconds timeout, std::string& error);
    bool receiveFragment(std::vector<uint8_t>& fragment, std::chrono::milliseconds timeout, std::string& error);
    void close() noexcept;
    bool connected() const noexcept { return fd_ >= 0; }

private:
    bool wait(short events, std::chrono::steady_clock::time_point deadline, std::string& error);
    int fd_ = -1;
};

}  // namespace wk::opcda::rpc

#endif
