#!/usr/bin/env python3
"""Download frontend runtime packages and esbuild without npm install."""
from __future__ import annotations

import json
import tarfile
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
NM = ROOT / "node_modules"
VENDOR = ROOT / ".vendor"
TOOLS = ROOT / "tools"

PACKAGES = {
    "react": "19.1.1",
    "react-dom": "19.1.1",
    "scheduler": "0.26.0",
    "react-router": "7.8.2",
    "react-router-dom": "7.8.2",
    "cookie": "1.0.2",
    "set-cookie-parser": "2.7.1",
}
ESBUILD_VERSION = "0.25.9"


def tarball_url(name: str, version: str) -> str:
    filename = f"{name.split('/')[-1]}-{version}.tgz"
    return f"https://registry.npmjs.org/{name}/-/{filename}"


def extract_pkg(name: str, version: str) -> None:
    dest = NM / name
    dest.mkdir(parents=True, exist_ok=True)
    tgz = VENDOR / f"{name.replace('/', '-')}={version}.tgz"
    url = tarball_url(name, version)
    print(f"fetch {url}")
    urllib.request.urlretrieve(url, tgz)
    with tarfile.open(tgz, "r:gz") as tar:
        for member in tar.getmembers():
            parts = Path(member.name).parts
            if len(parts) <= 1:
                continue
            member.name = str(Path(*parts[1:]))
            tar.extract(member, path=dest)
    meta = json.loads((dest / "package.json").read_text())
    print(f" extracted {name} {meta['version']}")


def extract_esbuild() -> None:
    url = f"https://registry.npmjs.org/@esbuild/linux-x64/-/linux-x64-{ESBUILD_VERSION}.tgz"
    tgz = VENDOR / f"esbuild-linux-x64-{ESBUILD_VERSION}.tgz"
    print(f"fetch {url}")
    urllib.request.urlretrieve(url, tgz)
    TOOLS.mkdir(parents=True, exist_ok=True)
    with tarfile.open(tgz, "r:gz") as tar:
        for member in tar.getmembers():
            if member.name.endswith("bin/esbuild"):
                member.name = "esbuild"
                tar.extract(member, path=TOOLS)
                break
    binary = TOOLS / "esbuild"
    binary.chmod(0o755)
    print(f"esbuild {binary} ({binary.stat().st_size} bytes)")


def main() -> None:
    NM.mkdir(exist_ok=True)
    VENDOR.mkdir(exist_ok=True)
    for name, version in PACKAGES.items():
        extract_pkg(name, version)
    extract_esbuild()


if __name__ == "__main__":
    main()
