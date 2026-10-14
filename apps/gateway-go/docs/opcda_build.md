# Linux OPC DA V1.1 build and implementation status

The Go gateway integration uses the `opcda` build tag and ABI v2 in
`native/wk-opcda/api/wk_opcda.h`.

Current native milestone:

- Linux IPv4 TCP transport with bounded connect/send/receive operations;
- DCE/RPC common header and Bind PDU encoding;
- BindAck validation;
- independent NDR primitives for aligned integers, UUIDs, pointers and conformant/varying arrays;
- RPC AlterContext/Auth3 structures, Request fragmentation and Response/Fault reassembly;
- Windows RPC Endpoint Mapper `ept_map` request, correct top-level NDR pointer
  layout, TCP/IP tower decoding and dynamic-port connection;
- TCP/135 `IRemoteSCMActivator` fallback when Windows exposes the activation
  interface on the well-known object-resolver endpoint but has no dynamic EPM entry;
- NTLMv2 Negotiate/Challenge/Authenticate, including TargetInfo validation,
  MIC generation, key exchange and independent client/server signing state;
- RPC Packet Integrity request signing and response verification with negotiated
  header signing;
- side-effect-free `IObjectExporter::ServerAlive2` integrity diagnostic;
- `IRemoteSCMActivator::RemoteCreateInstance` (`opnum 4`) on context 0,
  including the four activation-property structures and extraction of the
  returned `IID_IOPCServer` OBJREF;
- strict `OBJREF_STANDARD`, `STDOBJREF` and `DUALSTRINGARRAY` decoding;
- signed `IObjectExporter::ResolveOxid2` OXID resolution and ncacn_ip_tcp
  object-exporter endpoint selection, including configured-IP fallback for
  Windows NetBIOS names that are not resolvable on Linux;
- `IID_IOPCServer` version 0.0 authenticated Bind on the resolved exporter
  endpoint, with the OBJREF IPID in the RPC object UUID and a valid ORPCTHIS;
- signed `IOPCServer::GetStatus` (`opnum 6`) with `OPCSERVERSTATUS`, FILETIME
  and vendor-string decoding;
- signed `IOPCServer::AddGroup` (`opnum 3`) with IDL-accurate `[ref]` / `[unique]`
  NDR pointer handling, revised update-rate decoding, and extraction of the
  returned `IOPCItemMgt` OBJREF;
- authenticated `IID_IOPCItemMgt` association on the group IPID and signed
  `IOPCItemMgt::AddItems` (`opnum 3`), including deferred OPCITEMDEF strings,
  VARTYPE selection, OPCITEMRESULT decoding, per-item HRESULTs, and client-to-server
  handle mapping;
- signed `IOPCServer::GetGroupByName` (`opnum 5`) lookup of the same group's
  `IID_IOPCSyncIO` interface, followed by an authenticated group-IPID association;
- signed `IOPCSyncIO::Read` (`opnum 3`) for cache/device sources, including
  batched server handles, OPCITEMSTATE, scalar/BSTR/DATE VARIANT decoding,
  Quality, FILETIME timestamps, and per-item HRESULTs;
- V1.1 `Workstation` and `SecurityMode` configuration contract;
- `modern` defaults to Packet Integrity, while `legacy_connect` must be selected explicitly;
- no automatic fallback from Modern/Packet Integrity to Legacy Connect;
- layered V1.1 error codes and stable Browse/Write/Subscribe/Status C API entry points;
- Go Write adapter plumbing with explicit per-point `read` / `write` / `read_write` access;
- Go device-session batching and Value/Quality/Timestamp conversion contract.

NTLMv2 authenticated Bind/Auth3 and Packet Integrity were validated from an
x86_64 Linux container against the Windows test server on 2026-08-25. The test
completed two independent secure associations: the activation association
accepted a signed request, and the object-exporter association returned a
`ServerAlive2` response whose signature was verified locally. Credentials are
provided only through process environment variables by the diagnostic program.

`RemoteCreateInstance`, `ResolveOxid2`, and the resulting `IID_IOPCServer` Bind
were validated against the same Windows server using Packet Integrity. The
server returned a NetBIOS binding and dynamic TCP endpoint; the Linux client
used the configured server IP with that returned endpoint and confirmed IPID
dispatch using a protected ORPC. `GetStatus` and `AddGroup` were then validated
on the same Packet Integrity association; the returned group interface was
decoded as an `IOPCItemMgt` standard object reference. The group interface Bind
and `AddItems` were also validated with Packet Integrity using the live
`Random.Real8` item; the server returned `VT_R8`, read access, a server handle,
and per-item `S_OK`. `GetGroupByName(IID_IOPCSyncIO)` and a device-source
`SyncRead` were then validated against the same group: the server returned a
`VT_R8` value, Good (`0xC0`) quality, FILETIME timestamp, and per-item `S_OK`.
A successful sequence is reported as `WK_OPCDA_STATE_CONNECTED`; this completes
the V1.1 synchronous collection path used by the Go gateway.

Gateway APIs redact OPC DA passwords and connection testing resolves an already
masked password only from the current in-memory device configuration. Password
encryption at rest still requires the product key-management integration and is
not claimed as complete by this milestone.

## Target matrix

V1.1 requires ARM64 and x86_64 Linux deliverables. The generic wrapper supports
both, and retains ARMv7 for actual RK3506/Cortex-A7 hardware. Do not label an
ARMv7 RK3506 binary as ARM64; select the architecture from the SDK and CPU used
by the specific gateway model.

```sh
WK_TARGET_ARCH=arm64 \
WK_TOOLCHAIN_ROOT=/opt/arm64-buildroot/host \
WK_SYSROOT=/opt/arm64-buildroot/sysroot \
./native/wk-opcda/build-linux.sh

WK_TARGET_ARCH=x86_64 \
WK_TOOLCHAIN_ROOT=/opt/x86_64-buildroot/host \
WK_SYSROOT=/opt/x86_64-buildroot/sysroot \
./native/wk-opcda/build-linux.sh
```

For an RK3506/Cortex-A7 SDK, the compatibility wrapper still builds ARMv7:

```sh
RK3506_TOOLCHAIN_ROOT=/opt/rk3506-sdk/host \
RK3506_SYSROOT=/opt/rk3506-sdk/sysroot \
./native/wk-opcda/build-rk3506.sh
```

`WK_TOOLCHAIN_PREFIX` can override the default compiler triplet for a vendor
SDK. Build outputs are kept per architecture so ARM64, x86_64 and ARMv7 cannot
overwrite each other.

The default build remains available without CGO. It accepts and preserves OPC
DA project configuration but returns a clear capability error when collection
or connection testing is attempted.

## EG100 ARMv7 deployment validation (2026-08-25)

The production EG100 at `192.168.1.214` reports `armv7l` on RK3506 and
Buildroot glibc 2.38. The deployed binaries target 32-bit ARM hard-float and
require no newer than glibc 2.17. The gateway embeds the runtime search path
`$ORIGIN/../lib`, so the native runtime is installed at
`/userdata/weikong/lib/libwk_opcda.so.0`.

Validated production hashes:

- gateway: `5a4886cde51f575be9c97a7da3abeb583d577b525342b78a6e383b2c850d451e`;
- native library: `780c966ba2e935d3c58b5d4bdfb888b6a3002dcc66dd397b4c186171169e6c05`;
- pre-upgrade gateway after decompressing the rollback archive:
  `4cccb24019a4da7db4dc708217a25521e2084a7016a5d6a090e72effe20dacca`.

An isolated one-second collection run used `Random.Real8` with NTLMv2 Packet
Integrity. Five consecutive samples changed value, returned OPC quality
`0xC0` (`GOOD`), carried a valid source FILETIME, and had no point error. The
final write-only diagnostic also completed Create, RemoteActivation, GetStatus,
AddGroup, AddItems, and device-source SyncRead against the production-installed
library. The Windows test server clock was about 12 seconds ahead of the EG100
receive clock during this run; source timestamps were preserved rather than
rewritten.

The production supervisor was restarted successfully and the loopback health
check remained healthy after cutover. Its existing production configuration was
left unchanged; the validated OPC DA test project is stored separately at
`/userdata/weikong/config/config.opcda-periodic-test.json` with mode 0600. The
compressed rollback binary is stored under
`/userdata/weikong/backups/opcda-v11-20260825/`.
