"""India API db bridge: lets psql and ordinary Postgres drivers reach a throwaway database,
which is served over a WebSocket. It listens on localhost and carries each connection to
the API. Standard library only. MIT-0.

    python -m india_api.bridge [--port 5432] [--url wss://api.dharun.dev/v1/db/ws]
    psql "postgresql://USER:PASSWORD@localhost:5432/NAME?sslmode=disable"
"""

from __future__ import annotations

import argparse
import base64
import os
import socket
import ssl
import struct
import sys
import threading
from urllib.parse import urlsplit


def _connect(url: str) -> socket.socket:
    parts = urlsplit(url)
    secure = parts.scheme == "wss"
    port = parts.port or (443 if secure else 80)
    raw = socket.create_connection((parts.hostname, port), timeout=15)
    sock = ssl.create_default_context().wrap_socket(raw, server_hostname=parts.hostname) if secure else raw
    key = base64.b64encode(os.urandom(16)).decode()
    path = (parts.path or "/") + ("?" + parts.query if parts.query else "")
    sock.sendall((f"GET {path} HTTP/1.1\r\nHost: {parts.netloc}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"
                  f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\nUser-Agent: india-api-python-bridge\r\n\r\n").encode())
    head = b""
    while b"\r\n\r\n" not in head:
        chunk = sock.recv(1)
        if not chunk:
            raise ConnectionError("the server closed the connection during the handshake")
        head += chunk
    status = head.split(b"\r\n", 1)[0]
    if b" 101 " not in status + b" ":
        raise ConnectionError("the server refused the WebSocket: " + status.decode(errors="replace"))
    sock.settimeout(None)
    return sock


def _send(sock: socket.socket, opcode: int, payload: bytes, lock: threading.Lock) -> None:
    mask = os.urandom(4)
    length = len(payload)
    if length < 126:
        header = struct.pack("!BB", 0x80 | opcode, 0x80 | length)
    elif length < 65536:
        header = struct.pack("!BBH", 0x80 | opcode, 0x80 | 126, length)
    else:
        header = struct.pack("!BBQ", 0x80 | opcode, 0x80 | 127, length)
    masked = bytes(b ^ mask[i % 4] for i, b in enumerate(payload))
    with lock:
        sock.sendall(header + mask + masked)


def _read_exact(sock: socket.socket, n: int) -> bytes:
    data = b""
    while len(data) < n:
        chunk = sock.recv(n - len(data))
        if not chunk:
            raise ConnectionError("closed")
        data += chunk
    return data


def _pipe(client: socket.socket, url: str) -> None:
    try:
        ws = _connect(url)
    except Exception as error:  # noqa: BLE001
        print("db bridge:", error, file=sys.stderr)
        client.close()
        return
    lock = threading.Lock()

    def upstream() -> None:
        try:
            while True:
                data = client.recv(65536)
                if not data:
                    break
                _send(ws, 0x2, data, lock)
        except OSError:
            pass
        try:
            _send(ws, 0x8, b"", lock)
        except OSError:
            pass

    threading.Thread(target=upstream, daemon=True).start()
    try:
        while True:
            first, second = _read_exact(ws, 2)
            opcode, length = first & 0x0F, second & 0x7F
            if length == 126:
                length = struct.unpack("!H", _read_exact(ws, 2))[0]
            elif length == 127:
                length = struct.unpack("!Q", _read_exact(ws, 8))[0]
            payload = _read_exact(ws, length)
            if opcode in (0x0, 0x1, 0x2):
                client.sendall(payload)
            elif opcode == 0x9:
                _send(ws, 0xA, payload, lock)
            elif opcode == 0x8:
                break
    except (OSError, ConnectionError):
        pass
    client.close()
    ws.close()


def main() -> None:
    parser = argparse.ArgumentParser(description="India API db bridge")
    parser.add_argument("--port", type=int, default=5432)
    parser.add_argument("--url", default="wss://api.dharun.dev/v1/db/ws")
    args = parser.parse_args()
    listener = socket.create_server(("127.0.0.1", args.port))
    print(f"db bridge: localhost:{args.port} -> {args.url}", file=sys.stderr)
    while True:
        client, _ = listener.accept()
        threading.Thread(target=_pipe, args=(client, args.url), daemon=True).start()


if __name__ == "__main__":
    main()
