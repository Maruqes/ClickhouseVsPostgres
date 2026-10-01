#!/usr/bin/env python3
"""Verify the real six-service stack using only Python's standard library."""
import json
from pathlib import Path
import re
import subprocess
from urllib.error import HTTPError
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[1]


def docker(*args):
    return subprocess.check_output(["docker", *args], cwd=ROOT, text=True).strip()


def get(base, path, method="GET"):
    request = Request(base + path, method=method)
    try:
        with urlopen(request, timeout=40) as response:
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
    assert get(ch, "/classes") == page and get(ps, "/classes") == page, "Direct classes route/reload failed"
    templates = [get(address, "/api/classes/demo") for address in (ch, ps)]
    assert templates[0] == templates[1], "Demo templates differ"
    template = json.loads(templates[0][2])
    assert templates[0][0] == 200 and template["capacity"] == 20 and len(template["reservations"]) == 19
    for address in (ch, ps):
        status, _, body = get(address, "/api/classes/demo/race", "POST")
        report = json.loads(body)
        assert status == 200, report
        summary = report["summary"]
        assert [attempt["number"] for attempt in report["attempts"]] == list(range(1, 21))
        assert summary["accepted"] + summary["rejected"] + summary["errors"] == 20
        assert report["initial_bookings"] == 19 and report["capacity"] == 20
        assert summary["infrastructure_errors"] == 0
        assert report["final_bookings"] == 19 + summary["accepted"]
        assert summary["overbooked"] == max(report["final_bookings"] - 20, 0)
        if address == ps:
            assert summary["accepted"] == 1 and summary["rejected"] == 19
        print(f"Race {address}: {summary}; {report['elapsed_ms']:.2f} ms")
    metadata_response = get(ch, "/api/dataset")
    assert metadata_response[:2] == (200, "application/json")
    metadata = json.loads(metadata_response[2])
    path = f"/api/country/summary?country=ZZ&from={metadata['from']}&to={metadata['from']}"
    for address in (ch, ps, base("ch-back"), base("ps-back")):
        assert get(address, "/api/dataset") == metadata_response, "Dataset contract mismatch"
        status, content_type, body = get(address, path)
        assert (status, content_type) == (200, "application/json")
        summary = json.loads(body)
        assert summary.pop("query_ms") >= 0
        assert summary["totals"] == {"volume_kg": "0.00", "athletes": 0, "sets": 0, "reps": 0}
        assert summary["country"] == "ZZ" and summary["from"] == summary["to"] == metadata["from"]
    print("PASS: six healthy services; identical images, pages, assets, health, metadata, and analytics contracts.")


if __name__ == "__main__":
    main()
