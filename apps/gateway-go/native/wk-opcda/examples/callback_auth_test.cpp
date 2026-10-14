#include "rpc/callback_server.h"

#include <chrono>
#include <cstdlib>
#include <iostream>
#include <string>
#include <thread>

namespace {

std::string environment(const char* name) {
    const char* value = std::getenv(name);
    return value == nullptr ? std::string{} : std::string(value);
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) {
        std::cerr << "usage: callback_auth_test <bind-ip> [auth-level] [seconds]\n";
        return 2;
    }
    const uint8_t authenticationLevel = static_cast<uint8_t>(
        argc > 2 ? std::stoi(argv[2]) : 5);
    const int seconds = argc > 3 ? std::stoi(argv[3]) : 120;
    wk::opcda::auth::NtlmCredentials credentials{
        environment("WK_OPCDA_TEST_USERNAME"),
        environment("WK_OPCDA_TEST_PASSWORD"),
        environment("WK_OPCDA_TEST_DOMAIN"),
        environment("WK_OPCDA_TEST_WORKSTATION")};
    if (credentials.username.empty() || credentials.password.empty()) {
        std::cerr << "WK_OPCDA_TEST_USERNAME and WK_OPCDA_TEST_PASSWORD are required\n";
        return 2;
    }

    wk::opcda::rpc::CallbackServer server;
    std::string error;
    if (!server.start(argv[1], argv[1], authenticationLevel, credentials,
                      [](const wk::opcda::opcda::DataChange&) {}, error)) {
        std::cerr << error << '\n';
        return 1;
    }
    std::cout << "CALLBACK_ENDPOINT=" << server.advertisedIp() << ':'
              << server.port() << std::endl;
    std::string previousError;
    for (int index = 0; index < seconds * 10; ++index) {
        const std::string currentError = server.lastError();
        if (!currentError.empty() && currentError != previousError) {
            std::cout << "CALLBACK_DIAGNOSTIC=" << currentError << std::endl;
            previousError = currentError;
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
    server.stop();
    return previousError.empty() ? 0 : 1;
}
