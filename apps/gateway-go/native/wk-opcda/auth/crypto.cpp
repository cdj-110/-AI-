#include "auth/crypto.h"

#include <algorithm>
#include <array>
#include <cstring>

namespace wk::opcda::auth {
namespace {

uint32_t readU32(const uint8_t* value) {
    return static_cast<uint32_t>(value[0]) |
           (static_cast<uint32_t>(value[1]) << 8) |
           (static_cast<uint32_t>(value[2]) << 16) |
           (static_cast<uint32_t>(value[3]) << 24);
}

void writeU32(uint8_t* output, uint32_t value) {
    output[0] = static_cast<uint8_t>(value);
    output[1] = static_cast<uint8_t>(value >> 8);
    output[2] = static_cast<uint8_t>(value >> 16);
    output[3] = static_cast<uint8_t>(value >> 24);
}

uint32_t rotateLeft(uint32_t value, uint32_t count) {
    return (value << count) | (value >> (32 - count));
}

std::vector<uint8_t> paddedMessage(const uint8_t* data, std::size_t size) {
    const std::size_t paddedSize = ((size + 9 + 63) / 64) * 64;
    std::vector<uint8_t> message(paddedSize, 0);
    if (size != 0) std::copy(data, data + size, message.begin());
    message[size] = 0x80;
    const uint64_t bits = static_cast<uint64_t>(size) * 8;
    for (int index = 0; index < 8; ++index) {
        message[paddedSize - 8 + static_cast<std::size_t>(index)] =
            static_cast<uint8_t>(bits >> (index * 8));
    }
    return message;
}

}  // namespace

Digest16 md4(const uint8_t* data, std::size_t size) {
    uint32_t a = 0x67452301;
    uint32_t b = 0xefcdab89;
    uint32_t c = 0x98badcfe;
    uint32_t d = 0x10325476;
    const std::vector<uint8_t> message = paddedMessage(data, size);
    for (std::size_t offset = 0; offset < message.size(); offset += 64) {
        uint32_t x[16]{};
        for (int index = 0; index < 16; ++index) x[index] = readU32(message.data() + offset + index * 4);
        uint32_t aa = a, bb = b, cc = c, dd = d;
        const auto f = [](uint32_t x0, uint32_t y, uint32_t z) { return (x0 & y) | (~x0 & z); };
        const auto g = [](uint32_t x0, uint32_t y, uint32_t z) { return (x0 & y) | (x0 & z) | (y & z); };
        const auto h = [](uint32_t x0, uint32_t y, uint32_t z) { return x0 ^ y ^ z; };
#define WK_MD4_STEP(fn, av, bv, cv, dv, k, s, add) av = rotateLeft(av + fn(bv, cv, dv) + x[k] + add, s)
        WK_MD4_STEP(f, a, b, c, d, 0, 3, 0); WK_MD4_STEP(f, d, a, b, c, 1, 7, 0);
        WK_MD4_STEP(f, c, d, a, b, 2, 11, 0); WK_MD4_STEP(f, b, c, d, a, 3, 19, 0);
        WK_MD4_STEP(f, a, b, c, d, 4, 3, 0); WK_MD4_STEP(f, d, a, b, c, 5, 7, 0);
        WK_MD4_STEP(f, c, d, a, b, 6, 11, 0); WK_MD4_STEP(f, b, c, d, a, 7, 19, 0);
        WK_MD4_STEP(f, a, b, c, d, 8, 3, 0); WK_MD4_STEP(f, d, a, b, c, 9, 7, 0);
        WK_MD4_STEP(f, c, d, a, b, 10, 11, 0); WK_MD4_STEP(f, b, c, d, a, 11, 19, 0);
        WK_MD4_STEP(f, a, b, c, d, 12, 3, 0); WK_MD4_STEP(f, d, a, b, c, 13, 7, 0);
        WK_MD4_STEP(f, c, d, a, b, 14, 11, 0); WK_MD4_STEP(f, b, c, d, a, 15, 19, 0);

        constexpr uint32_t k2 = 0x5a827999;
        WK_MD4_STEP(g, a, b, c, d, 0, 3, k2); WK_MD4_STEP(g, d, a, b, c, 4, 5, k2);
        WK_MD4_STEP(g, c, d, a, b, 8, 9, k2); WK_MD4_STEP(g, b, c, d, a, 12, 13, k2);
        WK_MD4_STEP(g, a, b, c, d, 1, 3, k2); WK_MD4_STEP(g, d, a, b, c, 5, 5, k2);
        WK_MD4_STEP(g, c, d, a, b, 9, 9, k2); WK_MD4_STEP(g, b, c, d, a, 13, 13, k2);
        WK_MD4_STEP(g, a, b, c, d, 2, 3, k2); WK_MD4_STEP(g, d, a, b, c, 6, 5, k2);
        WK_MD4_STEP(g, c, d, a, b, 10, 9, k2); WK_MD4_STEP(g, b, c, d, a, 14, 13, k2);
        WK_MD4_STEP(g, a, b, c, d, 3, 3, k2); WK_MD4_STEP(g, d, a, b, c, 7, 5, k2);
        WK_MD4_STEP(g, c, d, a, b, 11, 9, k2); WK_MD4_STEP(g, b, c, d, a, 15, 13, k2);

        constexpr uint32_t k3 = 0x6ed9eba1;
        WK_MD4_STEP(h, a, b, c, d, 0, 3, k3); WK_MD4_STEP(h, d, a, b, c, 8, 9, k3);
        WK_MD4_STEP(h, c, d, a, b, 4, 11, k3); WK_MD4_STEP(h, b, c, d, a, 12, 15, k3);
        WK_MD4_STEP(h, a, b, c, d, 2, 3, k3); WK_MD4_STEP(h, d, a, b, c, 10, 9, k3);
        WK_MD4_STEP(h, c, d, a, b, 6, 11, k3); WK_MD4_STEP(h, b, c, d, a, 14, 15, k3);
        WK_MD4_STEP(h, a, b, c, d, 1, 3, k3); WK_MD4_STEP(h, d, a, b, c, 9, 9, k3);
        WK_MD4_STEP(h, c, d, a, b, 5, 11, k3); WK_MD4_STEP(h, b, c, d, a, 13, 15, k3);
        WK_MD4_STEP(h, a, b, c, d, 3, 3, k3); WK_MD4_STEP(h, d, a, b, c, 11, 9, k3);
        WK_MD4_STEP(h, c, d, a, b, 7, 11, k3); WK_MD4_STEP(h, b, c, d, a, 15, 15, k3);
#undef WK_MD4_STEP
        a += aa; b += bb; c += cc; d += dd;
    }
    Digest16 output{};
    writeU32(output.data(), a); writeU32(output.data() + 4, b);
    writeU32(output.data() + 8, c); writeU32(output.data() + 12, d);
    return output;
}

Digest16 md5(const uint8_t* data, std::size_t size) {
    static constexpr uint32_t shifts[64] = {
        7,12,17,22, 7,12,17,22, 7,12,17,22, 7,12,17,22,
        5,9,14,20, 5,9,14,20, 5,9,14,20, 5,9,14,20,
        4,11,16,23, 4,11,16,23, 4,11,16,23, 4,11,16,23,
        6,10,15,21, 6,10,15,21, 6,10,15,21, 6,10,15,21,
    };
    static constexpr uint32_t constants[64] = {
        0xd76aa478,0xe8c7b756,0x242070db,0xc1bdceee,0xf57c0faf,0x4787c62a,0xa8304613,0xfd469501,
        0x698098d8,0x8b44f7af,0xffff5bb1,0x895cd7be,0x6b901122,0xfd987193,0xa679438e,0x49b40821,
        0xf61e2562,0xc040b340,0x265e5a51,0xe9b6c7aa,0xd62f105d,0x02441453,0xd8a1e681,0xe7d3fbc8,
        0x21e1cde6,0xc33707d6,0xf4d50d87,0x455a14ed,0xa9e3e905,0xfcefa3f8,0x676f02d9,0x8d2a4c8a,
        0xfffa3942,0x8771f681,0x6d9d6122,0xfde5380c,0xa4beea44,0x4bdecfa9,0xf6bb4b60,0xbebfbc70,
        0x289b7ec6,0xeaa127fa,0xd4ef3085,0x04881d05,0xd9d4d039,0xe6db99e5,0x1fa27cf8,0xc4ac5665,
        0xf4292244,0x432aff97,0xab9423a7,0xfc93a039,0x655b59c3,0x8f0ccc92,0xffeff47d,0x85845dd1,
        0x6fa87e4f,0xfe2ce6e0,0xa3014314,0x4e0811a1,0xf7537e82,0xbd3af235,0x2ad7d2bb,0xeb86d391,
    };
    uint32_t a0=0x67452301,b0=0xefcdab89,c0=0x98badcfe,d0=0x10325476;
    const std::vector<uint8_t> message = paddedMessage(data, size);
    for (std::size_t offset=0; offset<message.size(); offset+=64) {
        uint32_t m[16]{}; for(int i=0;i<16;++i)m[i]=readU32(message.data()+offset+i*4);
        uint32_t a=a0,b=b0,c=c0,d=d0;
        for(uint32_t i=0;i<64;++i){uint32_t f=0,g=0;
            if(i<16){f=(b&c)|(~b&d);g=i;}else if(i<32){f=(d&b)|(~d&c);g=(5*i+1)%16;}
            else if(i<48){f=b^c^d;g=(3*i+5)%16;}else{f=c^(b|~d);g=(7*i)%16;}
            const uint32_t next=d;d=c;c=b;b=b+rotateLeft(a+f+constants[i]+m[g],shifts[i]);a=next;
        }
        a0+=a;b0+=b;c0+=c;d0+=d;
    }
    Digest16 output{};writeU32(output.data(),a0);writeU32(output.data()+4,b0);
    writeU32(output.data()+8,c0);writeU32(output.data()+12,d0);return output;
}

Digest16 hmacMd5(const uint8_t* key, std::size_t keySize,
                 const uint8_t* data, std::size_t dataSize) {
    std::array<uint8_t,64> normalized{};
    if(keySize>normalized.size()){const Digest16 hashed=md5(key,keySize);std::copy(hashed.begin(),hashed.end(),normalized.begin());}
    else if(keySize!=0)std::copy(key,key+keySize,normalized.begin());
    std::vector<uint8_t> inner(64+dataSize),outer(64+16);
    for(std::size_t i=0;i<64;++i){inner[i]=normalized[i]^0x36;outer[i]=normalized[i]^0x5c;}
    if(dataSize!=0)std::copy(data,data+dataSize,inner.begin()+64);
    const Digest16 innerDigest=md5(inner);std::copy(innerDigest.begin(),innerDigest.end(),outer.begin()+64);
    return md5(outer);
}

bool Rc4::initialize(const uint8_t* key, std::size_t size) {
    if(key==nullptr||size==0){initialized_=false;return false;}
    for(std::size_t index=0;index<state_.size();++index)state_[index]=static_cast<uint8_t>(index);
    uint8_t j=0;for(std::size_t index=0;index<state_.size();++index){j=static_cast<uint8_t>(j+state_[index]+key[index%size]);std::swap(state_[index],state_[j]);}
    i_=0;j_=0;initialized_=true;return true;
}

void Rc4::process(uint8_t* data,std::size_t size){if(!initialized_||data==nullptr)return;for(std::size_t index=0;index<size;++index){i_=static_cast<uint8_t>(i_+1);j_=static_cast<uint8_t>(j_+state_[i_]);std::swap(state_[i_],state_[j_]);data[index]^=state_[static_cast<uint8_t>(state_[i_]+state_[j_])];}}

std::vector<uint8_t> Rc4::processCopy(const uint8_t* data,std::size_t size){std::vector<uint8_t> output(size);if(size!=0)std::copy(data,data+size,output.begin());process(output.data(),output.size());return output;}

}  // namespace wk::opcda::auth
