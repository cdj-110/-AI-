"""Small deployment helper for password-authenticated EG100 test units."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path

import paramiko


def connect(host: str, password: str) -> paramiko.SSHClient:
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(
        host,
        username="root",
        password=password,
        timeout=8,
        allow_agent=False,
        look_for_keys=False,
    )
    return client


def merge_opcda_config(
    sftp: paramiko.SFTPClient,
    template_path: Path,
    remote_current: str,
    remote_staged: str,
    replacements: list[tuple[str, str]],
) -> None:
    """Merge one validated OPC DA device into the live project without exposing secrets."""
    with sftp.open(remote_current, "rb") as source:
        current = json.loads(source.read().decode("utf-8"))
    payload = template_path.read_text(encoding="utf-8")
    for token, replacement in replacements:
        if token not in payload:
            raise SystemExit(f"replacement token {token!r} is not present in {template_path}")
        payload = payload.replace(token, replacement)
    template = json.loads(payload)
    candidates = [item for item in template.get("devices", []) if item.get("protocol") == "opcda"]
    if len(candidates) != 1:
        raise SystemExit("OPC DA merge template must contain exactly one opcda device")
    device = candidates[0]

    resources = current.setdefault("resources", [])
    resource = next(
        (
            item
            for item in resources
            if item.get("type") == "network"
            and (item.get("network") or {}).get("interface") == "eth0"
        ),
        None,
    )
    if resource is None:
        resource = next((item for item in resources if item.get("type") == "network"), None)
    if resource is None:
        raise SystemExit("production config has no network resource for the OPC DA channel")
    resource_key = resource.get("resourceKey")
    if not resource_key:
        raise SystemExit("selected network resource has no resourceKey")
    interface_name = (resource.get("network") or {}).get("interface") or resource.get("name") or "eth0"

    channel_key = "opcda-matrikon-test"
    channels = current.setdefault("channels", [])
    channel = {
        "channelKey": channel_key,
        "resourceKey": resource_key,
        "name": "OPC DA 测试采集通道",
        "type": "network",
        "role": "collect",
        "protocol": "opcda",
        "collectIntervalSeconds": 1,
        "collectIntervalMilliseconds": 1000,
        "interfaceName": interface_name,
        "enabled": True,
    }
    channel_index = next((index for index, item in enumerate(channels) if item.get("channelKey") == channel_key), -1)
    if channel_index >= 0:
        channels[channel_index] = channel
    else:
        channels.append(channel)

    devices = current.setdefault("devices", [])
    target_key = device.get("deviceKey") or "opcda-matrikon-test"
    existing_index = next((index for index, item in enumerate(devices) if item.get("deviceKey") == target_key), -1)
    existing_target = devices[existing_index] if existing_index >= 0 else None
    used_metrics = {
        str(point.get("metric", "")).lower()
        for item in devices
        if item is not existing_target
        for point in item.get("points", [])
        if point.get("metric")
    }
    used_metrics.update(
        str(point.get("metric", "")).lower()
        for group in (current.get("edgeComputing") or {}).get("groups", [])
        for point in group.get("points", [])
        if point.get("metric")
    )
    for point_index, point in enumerate(device.get("points", []), 1):
        preferred = "opcda_random_real8" if point_index == 1 else f"opcda_point_{point_index:03d}"
        metric, suffix = preferred, 1
        while metric.lower() in used_metrics:
            suffix += 1
            metric = f"{preferred}_{suffix:03d}"
        point["metric"] = metric
        point["deviceKey"] = target_key
        point["channelKey"] = channel_key
        point["protocol"] = "opcda"
        point["address"] = device.get("address", "")
        used_metrics.add(metric.lower())
    device["deviceKey"] = target_key
    device["channelKey"] = channel_key
    device["interfaceType"] = "network"
    device["interfaceName"] = interface_name
    if existing_index >= 0:
        devices[existing_index] = device
    else:
        devices.append(device)

    encoded = (json.dumps(current, ensure_ascii=False, indent=2) + "\n").encode("utf-8")
    with sftp.open(remote_staged, "wb") as target:
        target.write(encoded)
        target.flush()
    sftp.chmod(remote_staged, 0o600)
    point_count = sum(len(item.get("points", [])) for item in devices)
    print(
        f"staged merged config: resources={len(resources)} channels={len(channels)} "
        f"devices={len(devices)} devicePoints={point_count} opcdaDevices=1"
    )


def set_opcda_read_mode(
    sftp: paramiko.SFTPClient, remote_path: str, mode: str
) -> None:
    """Atomically update the OPC DA device and inherited point read mode."""
    if mode not in {"sync", "subscription"}:
        raise SystemExit("OPC DA read mode must be sync or subscription")
    with sftp.open(remote_path, "rb") as source:
        config = json.loads(source.read().decode("utf-8"))
    changed_devices = 0
    changed_points = 0
    for device in config.get("devices", []):
        if str(device.get("protocol", "")).lower() != "opcda":
            continue
        device["readMode"] = mode
        changed_devices += 1
        for point in device.get("points", []):
            point["readMode"] = mode
            changed_points += 1
    if changed_devices == 0:
        raise SystemExit("production config contains no OPC DA device")
    staged = remote_path + ".read-mode-new"
    encoded = (json.dumps(config, ensure_ascii=False, indent=2) + "\n").encode("utf-8")
    with sftp.open(staged, "wb") as target:
        target.write(encoded)
        target.flush()
    sftp.chmod(staged, 0o600)
    sftp.posix_rename(staged, remote_path)
    print(
        f"updated OPC DA read mode: mode={mode} devices={changed_devices} "
        f"points={changed_points}"
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("host")
    parser.add_argument("--password", default=os.environ.get("EG100_PASSWORD", ""))
    parser.add_argument("--command")
    parser.add_argument("--upload", nargs=2, metavar=("LOCAL", "REMOTE"))
    parser.add_argument(
        "--merge-opcda-config",
        nargs=3,
        metavar=("TEMPLATE", "REMOTE_CURRENT", "REMOTE_STAGED"),
    )
    parser.add_argument(
        "--set-opcda-read-mode",
        nargs=2,
        metavar=("MODE", "REMOTE_CONFIG"),
    )
    parser.add_argument("--resume-upload", action="store_true")
    parser.add_argument(
        "--replace-env",
        nargs=2,
        action="append",
        default=[],
        metavar=("TOKEN", "ENVIRONMENT_VARIABLE"),
        help="replace TOKEN from an environment variable while uploading text",
    )
    args = parser.parse_args()
    if not args.password:
        raise SystemExit("set EG100_PASSWORD or pass --password")

    client = connect(args.host, args.password)
    try:
        if args.set_opcda_read_mode:
            mode, remote_config = args.set_opcda_read_mode
            with client.open_sftp() as sftp:
                set_opcda_read_mode(sftp, remote_config, mode)
            return
        if args.merge_opcda_config:
            template, remote_current, remote_staged = args.merge_opcda_config
            replacements: list[tuple[str, str]] = []
            for token, environment_variable in args.replace_env:
                replacement = os.environ.get(environment_variable)
                if replacement is None:
                    raise SystemExit(f"environment variable {environment_variable} is not set")
                replacements.append((token, replacement))
            with client.open_sftp() as sftp:
                merge_opcda_config(
                    sftp,
                    Path(template).resolve(),
                    remote_current,
                    remote_staged,
                    replacements,
                )
            return
        if args.upload:
            local, remote = args.upload
            with client.open_sftp() as sftp:
                local_path = Path(local).resolve()
                replacements: list[tuple[str, str]] = []
                for token, environment_variable in args.replace_env:
                    replacement = os.environ.get(environment_variable)
                    if replacement is None:
                        raise SystemExit(f"environment variable {environment_variable} is not set")
                    replacements.append((token, replacement))
                if replacements and args.resume_upload:
                    raise SystemExit("--replace-env cannot be combined with --resume-upload")
                if replacements:
                    payload = local_path.read_text(encoding="utf-8")
                    for token, replacement in replacements:
                        if token not in payload:
                            raise SystemExit(f"replacement token {token!r} is not present in {local_path}")
                        payload = payload.replace(token, replacement)
                    with sftp.open(remote, "wb") as target:
                        target.write(payload.encode("utf-8"))
                        target.flush()
                    return
                offset = 0
                if args.resume_upload:
                    try:
                        offset = sftp.stat(remote).st_size
                    except OSError:
                        offset = 0
                if offset > local_path.stat().st_size:
                    offset = 0
                with local_path.open("rb") as source, sftp.open(remote, "ab" if offset else "wb") as target:
                    source.seek(offset)
                    while chunk := source.read(256 * 1024):
                        target.write(chunk)
                    target.flush()
        if args.command:
            _, stdout, stderr = client.exec_command(args.command, timeout=600)
            output = stdout.read().decode("utf-8", "replace")
            error = stderr.read().decode("utf-8", "replace")
            if output:
                print(output, end="")
            if error:
                print(error, end="")
            status = stdout.channel.recv_exit_status()
            if status:
                raise SystemExit(status)
    finally:
        client.close()


if __name__ == "__main__":
    main()
