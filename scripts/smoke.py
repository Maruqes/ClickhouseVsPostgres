#!/usr/bin/env python3
"""Verify the real six-service stack using only Python's standard library."""
import json
from pathlib import Path
import re
import subprocess
from urllib.error import HTTPError
from urllib.request import urlopen

ROOT = Path(__file__).resolve().parents[1]


def docker(*args):
    return subprocess.check_output(["docker", *args], cwd=ROOT, text=True).strip()


def get(base, path):
    try:
        with urlopen(base + path, timeout=5) as response:
            return response.status, response.headers.get_content_type(), response.read()
    except HTTPError as response:
        return response.code, response.headers.get_content_type(), response.read()


def main():
    expected = {"ch-front", "ch-back", "ch-db", "ps-front", "ps-back", "ps-db"}
    config = json.loads(docker("compose", "config", "--format", "json"))
    assert set(config["services"]) == expected, "Default stack must have exactly six services"
    ids = docker("compose", "ps", "-q").splitlines()
    assert len(ids) == 6, "Start all services with docker compose up --build -d --wait"
    containers = json.loads(docker("inspect", *ids))
    by_service = {c["Config"]["Labels"]["com.docker.compose.service"]: c for c in containers}
    assert set(by_service) == expected
    for name, container in by_service.items():
        assert container["State"]["Status"] == "running", f"{name} is not running"
        assert container["State"]["Health"]["Status"] == "healthy", f"{name} is not healthy"
    for role in ("front", "back"):
        assert by_service[f"ch-{role}"]["Image"] == by_service[f"ps-{role}"]["Image"], f"{role} images differ"

    def base(name):
        port = config["services"][name]["ports"][0]["published"]
        return f"http://127.0.0.1:{port}"

    ch, ps = base("ch-front"), base("ps-front")
    page = get(ch, "/")
    assert page[0:2] == (200, "text/html")
    assert page == get(ps, "/"), "Frontend pages differ"
    assets = re.findall(r'(?:src|href)="(/assets/[^\"]+)"', page[2].decode())
    assert assets, "No built frontend assets found"
    for asset in assets:
        result = get(ch, asset)
        assert result[0] == 200 and result == get(ps, asset), f"Asset mismatch: {asset}"
    for address in (ch, ps, base("ch-back"), base("ps-back")):
        status, content_type, body = get(address, "/api/health")
        assert (status, content_type) == (200, "application/json")
        assert body == b'{"status":"ok"}\n', f"Unexpected health response at {address}"
        assert get(address, "/api/missing")[0] == 404, "Unknown API routes must return 404"
    for name in ("ch-back", "ps-back"):
        assert get(base(name), "/livez")[0] == 200
    assert get(ch, "/future-route") == page, "SPA fallback is missing"
    assert get(ps, "/future-route") == page, "SPA fallback is missing"
    print("PASS: six healthy services; identical images, pages, assets, and API contracts.")


if __name__ == "__main__":
    main()
