#include "rpc/socket_utils.h"

#include <cassert>
#include <fcntl.h>
#include <sys/socket.h>
#include <unistd.h>

int main() {
    const int descriptor = wk::opcda::rpc::createSocketCloseOnExec(
        AF_INET, SOCK_STREAM, 0);
    assert(descriptor >= 0);
    const int flags = ::fcntl(descriptor, F_GETFD, 0);
    assert(flags >= 0);
    assert((flags & FD_CLOEXEC) != 0);
    ::close(descriptor);
    return 0;
}
