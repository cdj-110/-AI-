"""Non-disruptive activation binding validation against the live EG100."""

import json
import os
from pathlib import Path

from mysql_live_20260912 import Gateway


LOCAL_BINARY = Path("build/remote-access-auto-release-20260916/weikong-gateway")
REMOTE_DIR = "/mnt/udisk/.weikong-deploy/activation-binding-check-20260916"
REMOTE_BINARY = REMOTE_DIR + "/weikong-gateway"
CONFIG_PATH = "/userdata/weikong/config/config.local.json"


def read_json(sftp, path):
    with sftp.open(path, "r") as handle:
        return json.loads(handle.read().decode("utf-8"))


def write_json(sftp, path, value):
    with sftp.open(path, "w") as handle:
        handle.write(json.dumps(value, ensure_ascii=False, indent=2).encode("utf-8"))
    sftp.chmod(path, 0o600)


def run(gateway, command):
    _, stdout, stderr = gateway.ssh.exec_command(command, timeout=90)
    output = stdout.read().decode("utf-8", "replace")
    error = stderr.read().decode("utf-8", "replace")
    return stdout.channel.recv_exit_status(), output + error


def main():
    gateway = Gateway()
    sftp = gateway.ssh.open_sftp()
    mismatch_config_path = REMOTE_DIR + "/config-mismatch.json"
    mismatch_activation_path = REMOTE_DIR + "/activation-mismatch.json"
    try:
        gateway.shell(f"mkdir -p {REMOTE_DIR} && chmod 700 {REMOTE_DIR}")
        sftp.put(str(LOCAL_BINARY), REMOTE_BINARY)
        sftp.chmod(REMOTE_BINARY, 0o700)

        current_status, current_output = run(
            gateway,
            f"{REMOTE_BINARY} -check-config -config {CONFIG_PATH}",
        )
        if current_status != 0:
            raise RuntimeError("current activation was rejected: " + current_output)

        config = read_json(sftp, CONFIG_PATH)
        activation_ref = config.get("activation", {}).get("file", "activation.local.json")
        activation_path = activation_ref if activation_ref.startswith("/") else "/userdata/weikong/config/" + Path(activation_ref).name
        activation = read_json(sftp, activation_path)
        activation["hardwareId"] = "00000000000000000000000000000000"
        write_json(sftp, mismatch_activation_path, activation)
        config["activation"] = {"enabled": True, "file": mismatch_activation_path}
        write_json(sftp, mismatch_config_path, config)

        copied_status, copied_output = run(
            gateway,
            f"{REMOTE_BINARY} -check-config -config {mismatch_config_path}",
        )
        if copied_status == 0 or "does not match" not in copied_output:
            raise RuntimeError("mismatched hardware activation was not rejected")

        print(json.dumps({
            "verified": True,
            "currentHardwareAccepted": True,
            "copiedHardwareRejected": True,
        }, ensure_ascii=False))
    finally:
        try:
            gateway.shell(f"rm -rf {REMOTE_DIR}")
        finally:
            sftp.close()
            gateway.ssh.close()


if __name__ == "__main__":
    if not os.environ.get("WK_EG100_PASSWORD"):
        raise SystemExit("WK_EG100_PASSWORD is required")
    main()
