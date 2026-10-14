"""Create a private LAN CA and an IP-SAN HTTPS server certificate, once only."""

from __future__ import annotations

import ipaddress
import os
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID


def write_once(path: Path, data: bytes) -> None:
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "wb") as output:
        output.write(data)


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: generate-lan-https-cert.py <existing-protected-directory> <LAN-IP>")
    directory = Path(sys.argv[1]).resolve(strict=True)
    if not directory.is_dir():
        raise ValueError("certificate directory must already exist")
    address = ipaddress.ip_address(sys.argv[2])
    if not address.is_private:
        raise ValueError("this script is only for a private LAN address")
    if any(directory.iterdir()):
        raise FileExistsError("refusing to overwrite an existing certificate directory")

    now = datetime.now(timezone.utc)
    ca_key = ec.generate_private_key(ec.SECP256R1())
    ca_name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "Weikong EG100 LAN OTA CA 2026")])
    ca_cert = (
        x509.CertificateBuilder()
        .subject_name(ca_name)
        .issuer_name(ca_name)
        .public_key(ca_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - timedelta(minutes=10))
        .not_valid_after(now + timedelta(days=3650))
        .add_extension(x509.BasicConstraints(ca=True, path_length=0), critical=True)
        .add_extension(x509.KeyUsage(digital_signature=True, key_encipherment=False, content_commitment=False, data_encipherment=False, key_agreement=False, key_cert_sign=True, crl_sign=True, encipher_only=False, decipher_only=False), critical=True)
        .sign(ca_key, hashes.SHA256())
    )
    server_key = ec.generate_private_key(ec.SECP256R1())
    server_name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, str(address))])
    server_cert = (
        x509.CertificateBuilder()
        .subject_name(server_name)
        .issuer_name(ca_name)
        .public_key(server_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - timedelta(minutes=10))
        .not_valid_after(now + timedelta(days=365))
        .add_extension(x509.BasicConstraints(ca=False, path_length=None), critical=True)
        .add_extension(x509.SubjectAlternativeName([x509.IPAddress(address)]), critical=False)
        .add_extension(x509.ExtendedKeyUsage([ExtendedKeyUsageOID.SERVER_AUTH]), critical=False)
        .add_extension(x509.KeyUsage(digital_signature=True, key_encipherment=False, content_commitment=False, data_encipherment=False, key_agreement=False, key_cert_sign=False, crl_sign=False, encipher_only=False, decipher_only=False), critical=True)
        .sign(ca_key, hashes.SHA256())
    )
    pem = serialization.Encoding.PEM
    private_format = serialization.PrivateFormat.PKCS8
    encryption = serialization.NoEncryption()
    write_once(directory / "ca.key", ca_key.private_bytes(pem, private_format, encryption))
    write_once(directory / "ca.crt", ca_cert.public_bytes(pem))
    write_once(directory / "server.key", server_key.private_bytes(pem, private_format, encryption))
    write_once(directory / "server.crt", server_cert.public_bytes(pem))
    print(f"LAN certificate created for {address}; CA and server keys remain in protected local storage")


if __name__ == "__main__":
    main()
