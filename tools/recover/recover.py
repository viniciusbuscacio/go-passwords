#!/usr/bin/env python3
"""recover.py — standalone recovery tool for go-passwords vaults.

Independent implementation of the vault format: your data stays recoverable
even if the main app is gone.

Dependencies:
    pip install argon2-cffi cryptography

Usage:
    python recover.py <vault.gpw>

Asks for the master password and prints the decrypted JSON to stdout.
"""

import base64
import getpass
import json
import sys

from argon2.low_level import Type, hash_secret_raw
from cryptography.hazmat.primitives.ciphers.aead import AESGCM


def die(msg: str) -> None:
    print(f"Error: {msg}", file=sys.stderr)
    sys.exit(1)


def decrypt(field: str, key: bytes, decode) -> bytes:
    """Open an 'iv:tag:ciphertext' string with AES-256-GCM."""
    try:
        iv_s, tag_s, ct_s = field.split(":")
    except ValueError:
        die("expected iv:tag:ciphertext")
    iv, tag, ct = decode(iv_s), decode(tag_s), decode(ct_s)
    # AESGCM.decrypt expects ciphertext||tag.
    return AESGCM(key).decrypt(iv, ct + tag, None)


def main() -> None:
    if len(sys.argv) != 2:
        die("usage: python recover.py <vault.gpw>")
    with open(sys.argv[1], "rb") as f:
        header = json.load(f)

    if header.get("format") != "go-passwords-vault" or header.get("version") != 3:
        die(f"unsupported format {header.get('format')!r} version {header.get('version')!r}")
    kdf = header["kdf"]
    if kdf["algo"] != "argon2id":
        die(f"unsupported kdf {kdf['algo']!r}")

    password = getpass.getpass("Master password: ")
    key = hash_secret_raw(
        secret=password.encode(),
        salt=bytes.fromhex(header["salt"]),
        time_cost=kdf["time"],
        memory_cost=kdf["memory_kib"],
        parallelism=kdf["threads"],
        hash_len=32,
        type=Type.ID,
    )
    try:
        mount_key = decrypt(header["mount_key_ciphered"], key, bytes.fromhex)
    except Exception:
        die("wrong master password")
    try:
        payload = decrypt(header["payload"], mount_key, base64.b64decode)
    except Exception as e:
        die(f"payload corrupted: {e}")

    print(json.dumps(json.loads(payload), indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
