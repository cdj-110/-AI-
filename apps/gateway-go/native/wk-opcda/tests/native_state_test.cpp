// White-box regression for association state and keepalive worker shutdown.
// Support codecs/transports are supplied by libwk_opcda; the API implementation
// is included so the test can inject an already-connected loopback association.
#include "api/wk_opcda.cpp"
#include <cassert>
#include <sys/socket.h>
#include <netinet/in.h>
#include <unistd.h>

int main() {
    int listener = socket(AF_INET, SOCK_STREAM, 0);
    assert(listener >= 0);
    sockaddr_in address{};
    address.sin_family = AF_INET;
    address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    assert(bind(listener, reinterpret_cast<sockaddr*>(&address), sizeof(address)) == 0);
    socklen_t length = sizeof(address);
    assert(getsockname(listener, reinterpret_cast<sockaddr*>(&address), &length) == 0);
    assert(listen(listener, 1) == 0);
    std::thread server([&] {
        int client = accept(listener, nullptr, nullptr);
        assert(client >= 0);
        for (int i = 0; i < 2; ++i) {
            uint8_t header[16];
            assert(recv(client, header, 16, MSG_WAITALL) == 16);
            size_t count = header[8] | (header[9] << 8);
            std::vector<uint8_t> body(count - 16);
            assert(recv(client, body.data(), body.size(), MSG_WAITALL) == static_cast<ssize_t>(body.size()));
            // Valid RPC response envelope, deliberately truncated OPC stub.
            std::vector<uint8_t> response = {5,0,2,3,0x10,0,0,0,24,0,0,0,
                header[12],header[13],header[14],header[15],0,0,0,0,0,0,0,0};
            assert(send(client, response.data(), response.size(), 0) == static_cast<ssize_t>(response.size()));
        }
        close(client);
    });
    auto* handle = new wk_opcda_handle_t;
    handle->state = WK_OPCDA_STATE_CONNECTED;
    handle->securityModeCode = WK_OPCDA_SECURITY_LEGACY_CONNECT;
    handle->requestTimeoutMs = 1000;
    handle->itemServerHandles[1] = 101;
    std::string error;
    assert(handle->syncIoTransport.connect("127.0.0.1", ntohs(address.sin_port),
        std::chrono::seconds(1), error));
    uint32_t item = 1;
    wk_opcda_value_t value{};
    for (int i = 0; i < 2; ++i) {
        assert(wk_opcda_read(handle, WK_OPCDA_READ_DEVICE, &item, 1, &value) == WK_OPC_READ_FAILED);
        assert(handle->state == WK_OPCDA_STATE_CONNECTED);
        assert(handle->lastError.find("decode IOPCSyncIO") != std::string::npos);
    }
    server.join();
    close(listener);
    const char* snapshot = wk_opcda_last_error(handle);
    const std::string original(snapshot);
    std::thread errorWriter([&] {
        std::lock_guard<std::mutex> guard(handle->mutex);
        fail(handle, WK_DISCONNECTED, "simulated keepalive failure");
    });
    errorWriter.join();
    assert(std::string(snapshot) == original);
    // With no ping-required OIDs, the worker waits without any network I/O.
    for (int i = 0; i < 20; ++i) {
        handle->state = WK_OPCDA_STATE_CONNECTED;
        assert(startObjectLifetimeWorker(handle, error));
        auto before = std::chrono::steady_clock::now();
        wk_opcda_disconnect(handle);
        assert(!handle->pingThread.joinable());
        assert(std::chrono::steady_clock::now() - before < std::chrono::seconds(1));
    }
    wk_opcda_destroy(handle);
}
