#include "rpc/rpc_transport.h"
#include "rpc/socket_utils.h"

#include <arpa/inet.h>
#include <cerrno>
#include <cstring>
#include <fcntl.h>
#include <netdb.h>
#include <poll.h>
#include <sys/socket.h>
#include <unistd.h>

#include <algorithm>
#include <limits>
#include <sstream>

namespace wk::opcda::rpc {
namespace {

std::string systemError(const char* operation, int errorNumber = errno) {
    std::ostringstream stream;
    stream << operation << ": " << std::strerror(errorNumber) << " (errno=" << errorNumber << ')';
    return stream.str();
}

int remainingMilliseconds(std::chrono::steady_clock::time_point deadline) {
    const auto now = std::chrono::steady_clock::now();
    if (now >= deadline) {
        return 0;
    }
    const auto remaining = std::chrono::duration_cast<std::chrono::milliseconds>(deadline - now).count();
    return static_cast<int>(std::min<int64_t>(remaining, std::numeric_limits<int>::max()));
}

}  // namespace

TcpTransport::~TcpTransport() {
    close();
}

bool TcpTransport::connect(const std::string& host, uint16_t port, std::chrono::milliseconds timeout,
                           std::string& error, const std::string& localBindIp) {
    close();
    addrinfo hints{};
    hints.ai_family = AF_INET;
    hints.ai_socktype = SOCK_STREAM;
    hints.ai_protocol = IPPROTO_TCP;
    addrinfo* addresses = nullptr;
    const std::string service = std::to_string(port);
    const int lookup = ::getaddrinfo(host.c_str(), service.c_str(), &hints, &addresses);
    if (lookup != 0) {
        error = std::string("resolve ") + host + ": " + gai_strerror(lookup);
        return false;
    }

    const auto deadline = std::chrono::steady_clock::now() + timeout;
    for (addrinfo* address = addresses; address != nullptr; address = address->ai_next) {
        const int candidate = createSocketCloseOnExec(
            address->ai_family, address->ai_socktype, address->ai_protocol);
        if (candidate < 0) {
            continue;
        }
        if (!localBindIp.empty()) {
            sockaddr_in local{};
            local.sin_family = AF_INET;
            local.sin_port = 0;
            if (::inet_pton(AF_INET, localBindIp.c_str(), &local.sin_addr) != 1) {
                error = "invalid local IPv4 bind address " + localBindIp;
                ::close(candidate);
                break;
            }
            if (::bind(candidate, reinterpret_cast<const sockaddr*>(&local), sizeof(local)) < 0) {
                error = "bind local address " + localBindIp + " failed: " + systemError("bind");
                ::close(candidate);
                continue;
            }
        }
        const int originalFlags = ::fcntl(candidate, F_GETFL, 0);
        if (originalFlags < 0 || ::fcntl(candidate, F_SETFL, originalFlags | O_NONBLOCK) < 0) {
            ::close(candidate);
            continue;
        }
        const int result = ::connect(candidate, address->ai_addr, address->ai_addrlen);
        if (result == 0) {
            fd_ = candidate;
        } else if (errno == EINPROGRESS) {
            fd_ = candidate;
            if (!wait(POLLOUT, deadline, error)) {
                ::close(candidate);
                fd_ = -1;
                continue;
            }
            int socketError = 0;
            socklen_t length = sizeof(socketError);
            if (::getsockopt(candidate, SOL_SOCKET, SO_ERROR, &socketError, &length) < 0 || socketError != 0) {
                error = systemError("connect", socketError == 0 ? errno : socketError);
                ::close(candidate);
                fd_ = -1;
                continue;
            }
        } else {
            error = systemError("connect");
            ::close(candidate);
            continue;
        }
        ::freeaddrinfo(addresses);
        return true;
    }
    ::freeaddrinfo(addresses);
    if (error.empty()) {
        error = "no reachable IPv4 address for " + host;
    }
    return false;
}

bool TcpTransport::wait(short events, std::chrono::steady_clock::time_point deadline, std::string& error) {
    pollfd descriptor{fd_, events, 0};
    while (true) {
        const int remaining = remainingMilliseconds(deadline);
        if (remaining <= 0) {
            error = "socket operation timed out";
            return false;
        }
        const int result = ::poll(&descriptor, 1, remaining);
        if (result > 0) {
            if ((descriptor.revents & (POLLERR | POLLHUP | POLLNVAL)) != 0) {
                error = "socket closed while waiting for I/O";
                return false;
            }
            return (descriptor.revents & events) != 0;
        }
        if (result == 0) {
            error = "socket operation timed out";
            return false;
        }
        if (errno != EINTR) {
            error = systemError("poll");
            return false;
        }
    }
}

bool TcpTransport::sendAll(const uint8_t* data, std::size_t size, std::chrono::milliseconds timeout, std::string& error) {
    const auto deadline = std::chrono::steady_clock::now() + timeout;
    std::size_t sent = 0;
    while (sent < size) {
        if (!wait(POLLOUT, deadline, error)) {
            return false;
        }
        const ssize_t count = ::send(fd_, data + sent, size - sent, MSG_NOSIGNAL);
        if (count > 0) {
            sent += static_cast<std::size_t>(count);
            continue;
        }
        if (count < 0 && (errno == EAGAIN || errno == EWOULDBLOCK || errno == EINTR)) {
            continue;
        }
        error = count == 0 ? "socket closed during send" : systemError("send");
        return false;
    }
    return true;
}

bool TcpTransport::receiveExact(uint8_t* data, std::size_t size, std::chrono::milliseconds timeout, std::string& error) {
    const auto deadline = std::chrono::steady_clock::now() + timeout;
    std::size_t received = 0;
    while (received < size) {
        if (!wait(POLLIN, deadline, error)) {
            return false;
        }
        const ssize_t count = ::recv(fd_, data + received, size - received, 0);
        if (count > 0) {
            received += static_cast<std::size_t>(count);
            continue;
        }
        if (count < 0 && (errno == EAGAIN || errno == EWOULDBLOCK || errno == EINTR)) {
            continue;
        }
        error = count == 0 ? "peer closed the RPC connection" : systemError("recv");
        return false;
    }
    return true;
}

bool TcpTransport::receiveFragment(std::vector<uint8_t>& fragment, std::chrono::milliseconds timeout, std::string& error) {
    constexpr std::size_t headerSize = 16;
    uint8_t header[headerSize]{};
    if (!receiveExact(header, sizeof(header), timeout, error)) {
        return false;
    }
    const uint16_t fragmentLength = static_cast<uint16_t>(header[8]) | static_cast<uint16_t>(header[9] << 8);
    if (fragmentLength < headerSize) {
        error = "invalid DCE/RPC fragment length " + std::to_string(fragmentLength);
        return false;
    }
    fragment.assign(header, header + headerSize);
    fragment.resize(fragmentLength);
    return receiveExact(fragment.data() + headerSize, fragmentLength - headerSize, timeout, error);
}

void TcpTransport::close() noexcept {
    if (fd_ >= 0) {
        ::shutdown(fd_, SHUT_RDWR);
        ::close(fd_);
        fd_ = -1;
    }
}

}  // namespace wk::opcda::rpc
