#pragma once

#include <cerrno>
#include <fcntl.h>
#include <sys/socket.h>
#include <unistd.h>

namespace wk::opcda::rpc {

inline bool setCloseOnExec(int descriptor) noexcept {
    const int flags = ::fcntl(descriptor, F_GETFD, 0);
    return flags >= 0 && ::fcntl(descriptor, F_SETFD, flags | FD_CLOEXEC) == 0;
}

inline int createSocketCloseOnExec(int domain, int type, int protocol) noexcept {
#ifdef SOCK_CLOEXEC
    const int closeOnExecSocket = ::socket(domain, type | SOCK_CLOEXEC, protocol);
    if (closeOnExecSocket >= 0) return closeOnExecSocket;
    if (errno != EINVAL && errno != EPROTONOSUPPORT) return -1;
#endif

    const int descriptor = ::socket(domain, type, protocol);
    if (descriptor < 0) return -1;
    if (setCloseOnExec(descriptor)) return descriptor;

    const int errorNumber = errno;
    ::close(descriptor);
    errno = errorNumber;
    return -1;
}

inline int acceptCloseOnExec(int listener, sockaddr* address,
                             socklen_t* addressSize) noexcept {
#if defined(__linux__) && defined(SOCK_CLOEXEC)
    const int closeOnExecSocket = ::accept4(listener, address, addressSize, SOCK_CLOEXEC);
    if (closeOnExecSocket >= 0) return closeOnExecSocket;
    if (errno != ENOSYS && errno != EINVAL) return -1;
#endif

    const int descriptor = ::accept(listener, address, addressSize);
    if (descriptor < 0) return -1;
    if (setCloseOnExec(descriptor)) return descriptor;

    const int errorNumber = errno;
    ::close(descriptor);
    errno = errorNumber;
    return -1;
}

}  // namespace wk::opcda::rpc
