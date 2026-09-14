#!/usr/bin/env python3
from __future__ import annotations

import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BUILD = ROOT / "scripts" / "build.sh"


def main() -> None:
    initial = subprocess.run(["bash", str(BUILD)], check=False)
    if initial.returncode != 0:
        sys.exit(initial.returncode)
    watcher = subprocess.Popen(["bash", str(BUILD), "--watch"])
    server = subprocess.Popen([sys.executable, str(ROOT / "scripts" / "devserver.py"), *sys.argv[1:]])
    try:
        while True:
            if watcher.poll() not in (None,):
                server.terminate()
                sys.exit(watcher.returncode or 1)
            if server.poll() not in (None,):
                watcher.terminate()
                sys.exit(server.returncode or 1)
            time.sleep(0.4)
    except KeyboardInterrupt:
        watcher.terminate()
        server.terminate()


if __name__ == "__main__":
    main()
