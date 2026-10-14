# IEC61850 MMS build

The IEC61850 point browser can be built directly into the Go backend through
libiec61850 and CGO. This removes the need for an external browser command.

Requirements:

- Install libiec61850 headers and library.
- Ensure `iec61850_client.h` is on the C include path.
- Ensure the `iec61850` library is on the linker path.
- Build with CGO enabled and the `iec61850_mms` tag.

Windows/MSYS2 example:

```powershell
$env:PATH = "C:\msys64\mingw64\bin;" + $env:PATH
$env:CGO_ENABLED = "1"
$env:CC = "C:/msys64/mingw64/bin/clang.exe"
$env:CGO_CFLAGS = "-IC:/msys64/mingw64/include/libiec61850"
$env:CGO_LDFLAGS = "-LC:/msys64/mingw64/lib"

go build -tags iec61850_mms ./cmd/gateway
```

Keep `C:\msys64\mingw64\bin` on `PATH` when running the built gateway, or copy
`libiec61850.dll` next to the gateway executable.

The default build still works without libiec61850, but `/api/iec61850/browse`
returns HTTP 501 with a capability explanation. Connection testing also returns
501 without MMS support; a TCP connection alone is never reported as MMS success.

## EG100 ARMv7 release

Run `deploy/eg100/build_mms_armv7.sh` from the gateway-go root in Linux with Go,
make, native GCC, and `gcc-arm-linux-gnueabihf` installed. Prepare the official
libiec61850 v1.6.1 source at `build/iec61850-mms-armv7/source` (commit
`a13961110b8238d2d8ea577c1fb7592ba3017ad8`) and the existing OPC DA link libraries
at `build/remote-phase1-armv7/lib`.

The script runs native MMS browser/reader/forwarder and Web tests, statically
links libiec61850, and builds with **both** `opcda,iec61850_mms` and CGO enabled.
SQLite remains enabled. Output: `build/iec61850-mms-armv7/weikong-gateway`.
The only gateway-specific shared dependency remains `libwk_opcda.so.0` in
`/userdata/weikong/lib`; no additional libiec61850 shared library is needed.
Verify `go version -m` contains both tags before any subsequent release.

`cmd/mms-probe` is a read-only diagnostic command using the same Go collector:
build with `-tags iec61850_mms` and run `mms-probe -address HOST:102 -samples 5`
to inspect model counts and a few readable leaves without modifying device or
point configuration. It is a temporary diagnostic tool, not a second service.
