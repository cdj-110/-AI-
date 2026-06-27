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
will return a clear error telling you to rebuild with `-tags iec61850_mms`.
