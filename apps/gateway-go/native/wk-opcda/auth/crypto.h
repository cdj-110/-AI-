#ifndef WK_OPCDA_AUTH_CRYPTO_H
#define WK_OPCDA_AUTH_CRYPTO_H

#include <array>
#include <cstddef>
#include <cstdint>
#include <vector>

namespace wk::opcda::auth {

using Digest16 = std::array<uint8_t, 16>;

Digest16 md4(const uint8_t* data, std::size_t size);
Digest16 md5(const uint8_t* data, std::size_t size);
Digest16 hmacMd5(const uint8_t* key, std::size_t keySize,
                 const uint8_t* data, std::size_t dataSize);

inline Digest16 md4(const std::vector<uint8_t>& data) { return md4(data.data(), data.size()); }
inline Digest16 md5(const std::vector<uint8_t>& data) { return md5(data.data(), data.size()); }
inline Digest16 hmacMd5(const std::vector<uint8_t>& key, const std::vector<uint8_t>& data) {
    return hmacMd5(key.data(), key.size(), data.data(), data.size());
}

class Rc4 {
public:
    Rc4() = default;
    explicit Rc4(const uint8_t* key, std::size_t size) { initialize(key, size); }
    explicit Rc4(const Digest16& key) { initialize(key.data(), key.size()); }

    bool initialize(const uint8_t* key, std::size_t size);
    void process(uint8_t* data, std::size_t size);
    std::vector<uint8_t> processCopy(const uint8_t* data, std::size_t size);
    bool initialized() const noexcept { return initialized_; }

private:
    std::array<uint8_t, 256> state_{};
    uint8_t i_ = 0;
    uint8_t j_ = 0;
    bool initialized_ = false;
};

}  // namespace wk::opcda::auth

#endif
