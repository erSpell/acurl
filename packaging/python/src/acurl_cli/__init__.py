"""Runs the acurl CLI (https://acurl.dev), a free, open-source tool by tragent.io.

On first run the release binary matching this package version is downloaded
from GitHub Releases, verified against the release SHA-256 checksums and
cached in the user cache directory.
"""

from __future__ import annotations

import hashlib
import io
import os
import platform
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from pathlib import Path

__version__ = "1.0.0"
REPO = "tragent-io/acurl"


def target(system: str | None = None, machine: str | None = None) -> tuple[str, str]:
    """Return (archive name, binary name) for this platform."""
    system = (system or platform.system()).lower()
    machine = (machine or platform.machine()).lower()
    os_name = {"darwin": "darwin", "linux": "linux", "windows": "windows"}.get(system)
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(machine)
    if not os_name or not arch:
        raise RuntimeError(
            f"acurl has no prebuilt binary for {system}/{machine}. "
            f"Install it with Go instead: go install github.com/{REPO}/cmd/acurl@latest"
        )
    ext = "zip" if os_name == "windows" else "tar.gz"
    binary = "acurl.exe" if os_name == "windows" else "acurl"
    return f"acurl_{__version__}_{os_name}_{arch}.{ext}", binary


def expected_sum(checksums: str, archive: str) -> str:
    for line in checksums.splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1] == archive:
            return parts[0]
    raise RuntimeError(f"{archive} is not listed in checksums.txt")


def cache_dir() -> Path:
    if sys.platform == "win32":
        base = Path(os.environ.get("LOCALAPPDATA", Path.home() / "AppData" / "Local"))
    elif sys.platform == "darwin":
        base = Path.home() / "Library" / "Caches"
    else:
        base = Path(os.environ.get("XDG_CACHE_HOME", Path.home() / ".cache"))
    return base / "acurl" / __version__


def _get(url: str) -> bytes:
    req = urllib.request.Request(url, headers={"User-Agent": "acurl-pip"})
    with urllib.request.urlopen(req, timeout=60) as resp:  # noqa: S310 (https only)
        return resp.read()


def ensure_binary() -> Path:
    archive, binary = target()
    dest = cache_dir() / binary
    if dest.exists():
        return dest
    base = f"https://github.com/{REPO}/releases/download/v{__version__}"
    data = _get(f"{base}/{archive}")
    sums = _get(f"{base}/checksums.txt").decode()
    if hashlib.sha256(data).hexdigest() != expected_sum(sums, archive):
        raise RuntimeError(f"checksum mismatch for {archive}")

    dest.parent.mkdir(parents=True, exist_ok=True)
    if archive.endswith(".zip"):
        with zipfile.ZipFile(io.BytesIO(data)) as zf:
            payload = zf.read(binary)
    else:
        with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as tf:
            member = tf.extractfile(binary)
            if member is None:
                raise RuntimeError(f"{binary} not found in {archive}")
            payload = member.read()
    with tempfile.NamedTemporaryFile(dir=dest.parent, delete=False) as tmp:
        tmp.write(payload)
    os.chmod(tmp.name, 0o755)
    os.replace(tmp.name, dest)
    return dest


def main() -> None:
    try:
        binary = ensure_binary()
    except Exception as err:  # report download problems plainly
        print(f"acurl: {err}", file=sys.stderr)
        sys.exit(2)
    args = [str(binary), *sys.argv[1:]]
    if sys.platform == "win32":
        sys.exit(subprocess.call(args))
    os.execv(args[0], args)
