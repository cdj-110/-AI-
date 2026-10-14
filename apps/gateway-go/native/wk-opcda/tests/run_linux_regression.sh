#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
out=${1:?build output directory required}
mkdir -p "$out"
g++ -std=c++17 -O2 -Wall -Wextra -pthread -fPIC -shared -Wl,-soname,libwk_opcda.so.0 -I. api/wk_opcda.cpp auth/*.cpp dcom/*.cpp opcda/*.cpp rpc/*.cpp -o "$out/libwk_opcda.so.0.1.0"
g++ -std=c++17 -O2 -I. tests/ping_test.cpp rpc/ndr.cpp -o "$out/ping_test"
"$out/ping_test"
for test in rpc_pdu variant_data_integrity socket_utils native_state; do
    g++ -std=c++17 -O2 -pthread -I. "tests/${test}_test.cpp" -L"$out" -l:libwk_opcda.so.0.1.0 -Wl,-rpath,"$out" -o "$out/${test}_test"
    # Runtime SONAME symlink is required by these regression executables.
    ln -sf libwk_opcda.so.0.1.0 "$out/libwk_opcda.so.0"
    "$out/${test}_test"
done
echo 'Native regression tests passed'
