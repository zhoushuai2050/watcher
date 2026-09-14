#!/usr/bin/env python3
from __future__ import annotations

import argparse
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import urlsplit
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[1]
DIST = ROOT / "dist"
APP_BASE = "/Watcher"
API_PREFIX = f"{APP_BASE}/api"


class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *args, backend: str, **kwargs):
        self.backend = backend.rstrip("/")
        super().__init__(*args, directory=str(ROOT), **kwargs)

    def _route_path(self) -> str:
        return urlsplit(self.path).path

    def _is_api(self) -> bool:
        path = self._route_path()
        return path == API_PREFIX or path.startswith(API_PREFIX + "/")

    def do_POST(self):  # noqa: N802
        if self._is_api():
            self.proxy()
            return
        self.send_error(404)

    def do_PUT(self):  # noqa: N802
        if self._is_api():
            self.proxy()
            return
        self.send_error(404)

    def do_GET(self):  # noqa: N802
        path = self._route_path()
        if self._is_api():
            self.proxy()
            return
        if path in {"/", APP_BASE}:
            self.send_response(301)
            self.send_header("Location", APP_BASE + "/")
            self.end_headers()
            return
        if path.startswith(APP_BASE + "/assets/") or path == APP_BASE + "/favicon.svg":
            return super().do_GET()
        if path.startswith(APP_BASE + "/"):
            self.path = "/index.html"
            return super().do_GET()
        self.send_error(404)

    def translate_path(self, path: str) -> str:
        clean = urlsplit(path).path
        if clean.startswith(APP_BASE + "/assets/"):
            relative = clean[len(APP_BASE) + 1 :]
            return str(DIST / relative)
        if clean == APP_BASE + "/favicon.svg":
            return str(ROOT / "favicon.svg")
        return super().translate_path(path)

    def proxy(self) -> None:
        length = int(self.headers.get("Content-Length", "0") or 0)
        body = self.rfile.read(length) if length else None
        headers = {
            key: value
            for key, value in self.headers.items()
            if key.lower() not in {"host", "content-length"}
        }
        backend_path = self.path
        if backend_path.startswith(APP_BASE):
            backend_path = backend_path[len(APP_BASE) :]
        req = Request(f"{self.backend}{backend_path}", data=body, headers=headers, method=self.command)
        try:
            with urlopen(req, timeout=30) as resp:
                payload = resp.read()
                self.send_response(resp.status)
                for key, value in resp.headers.items():
                    if key.lower() in {"transfer-encoding", "connection"}:
                        continue
                    self.send_header(key, value)
                self.end_headers()
                self.wfile.write(payload)
        except HTTPError as exc:
            payload = exc.read()
            self.send_response(exc.code)
            self.send_header("Content-Type", exc.headers.get("Content-Type", "application/json"))
            self.end_headers()
            self.wfile.write(payload)
        except URLError as exc:
            message = f'{{"code": 502, "message": "后端未启动: {exc.reason}", "data": null}}'.encode()
            self.send_response(502)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.end_headers()
            self.wfile.write(message)

    def log_message(self, fmt: str, *args) -> None:
        import sys
        sys.stdout.write("%s - %s\n" % (self.address_string(), fmt % args))
        sys.stdout.flush()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=5174)
    parser.add_argument("--backend", default="http://127.0.0.1:8020")
    args = parser.parse_args()

    def factory(*f_args, **f_kwargs):
        return Handler(*f_args, backend=args.backend, **f_kwargs)

    server = ThreadingHTTPServer((args.host, args.port), factory)
    print(f"frontend http://{args.host}:{args.port}{APP_BASE}/  ->  {args.backend}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
