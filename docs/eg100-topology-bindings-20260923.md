# EG100 topology binding correction

## Root causes

- The topology treated the non-empty string `forwardProtocol: "none"` as an active forwarding protocol, even for explicit collection channels. This also selected the wrong device array and protocol label.
- The sidebar used the shared configuration store while the topology independently read runtime bootstrap configuration. For small projects this could show different versions of the project.
- Initial hardware profile application cleared channel definitions but retained devices and their channel references, losing custom channel identities such as a Siemens S7 channel.

## Changes

- Sidebar grouping and topology use the same role resolver: explicit role takes precedence; legacy forwarding inference excludes empty and `none` values.
- Topology reads and reloads the shared configuration store, including its compact large-project mode. Collection health continues to come from the runtime status endpoint.
- Invalid channel/role associations remain visible as unassigned devices rather than disappearing or appearing healthy based on stale telemetry.
- First activation builds the licensed port inventory and matches existing channels by physical interface/serial path. It retains channel keys and device/point references. Ports that cannot be matched retain their project configuration as disabled, unbound channels; their collection/forwarding devices and flat points are disabled too. They require explicit rebinding and enabling, never guessed migration.
- A zero-port hardware profile cannot resurrect old resources through legacy defaults.

## Scope and recovery

No live gateway configuration or license is changed by this code update. No new OTA release or deployment is performed. Already-lost channel definitions cannot be reconstructed safely from names alone; inspect the retained project backup or have the operator explicitly choose their intended resource/channel. The earlier device backup remains at `/userdata/weikong/backups/activation-reset-20260923-093400`.

## Verification

- Topology and sidebar grouping unit tests cover `none`, explicit collection role, forwarding, health, unassigned devices and navigation.
- Browser regression covers stale runtime bootstrap versus saved project, compact large-project mode, collection/forwarding counts, protocol labels, filtering, polling failures/recovery and mobile layout.
- Go configuration/initial activation tests cover retained S7 channel keys, physical-port remapping, removed ports, zero-port profiles, signed first activation and replacement protections.

Targeted Go tests, frontend type-check/build, topology/grouping unit tests and browser regressions passed. The full `go test ./internal/config ./internal/web -count=1` run did not pass: two activation tests reported local hardware identity mismatch, SQLite-dependent tests could not use the CGO-disabled driver, and the system-version test's expected value differed from the current build version. These failures were not suppressed or changed for this task.

## Device deployment (2026-09-23)

Subsequently deployed at the user's explicit request to `WK20265AF2F5C6` (`192.168.1.214`). This direct repair build retains the V0.9 label and the existing OTA release marker `2026092204`; no OTA package was published. The Linux ARM build retained strict license/hardware/secure-chip validation and the existing OTA public-key trust ring. Linux build-container configuration and remote-access tests passed.

- New binary SHA-256: `196e4d91998fb532fb66bd6f667c9049b69388db222694cf1e827233c5fc8aed`.
- Previous binary SHA-256: `b8d7334ac8c8d58f6af61cd96a8b5cd332ec35ad743f9901b94f446db2beaa67`.
- Verified rollback backup: `/media/udisk1/.weikong-deploy/activation-entry-20260923-095615`.
- Local deployment result and original binary archive: `artifacts/eg100-topology-deployment-20260923/`.
- Configuration, active license, license ledger, OTA marker, service init script and native OPC DA library remained byte-for-byte unchanged; network addressing/routes were unchanged.
- Health/readiness passed with a stable process and 7 configured collection points. The live authenticated topology page was not exercised because no web-session credential was available; browser interaction was tested locally against fixture data and the deployed binary hash was verified.

## Confirmed S7 channel restoration

After examining the live configuration, `device-003` was still an input to `edge-group-001/derived-001` (`s7-P01`). The user explicitly chose restoration rather than device deletion. Restored only the original `channel-001` collection channel (`siemens-s7`, named 通道1) from the authorization-reset backup and bound it to the current 网口2/eth1 resource. Kept the current resource's network settings, all existing channels, device points and calculation formula intact.

The final pre-change backup is `/userdata/weikong/backups/s7-channel-restore-20260923-101314`. An earlier attempt safely rolled back when verification assumed that a just-started health response already included a sample timestamp; verification now waits for two real, advancing samples. Final health/readiness and collection progress passed, with 7 points. Firmware, license, OTA ledger, other config fields and network routes remained unchanged.

The local repair is complete. Runtime calculation values were not directly inspected: the factory gateway is currently UNBOUND in the cloud (no tenant telemetry target), no gateway web credential is available in the task environment, and the existing-browser inspection connector returned a connection error. Do not describe this as verified S7 point quality or verified calculation output.
