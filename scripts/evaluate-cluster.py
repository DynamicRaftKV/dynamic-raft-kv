#!/usr/bin/env python3
"""Disposable local-process evaluation. Never touches demo data or existing nodes.

Build bin/raftkv first. Results are measurements, not performance thresholds.
"""
import argparse
import concurrent.futures
import datetime
import json
import math
import pathlib
import platform
import socket
import subprocess
import tempfile
import time
import urllib.request


def percentile(values, p):
    return sorted(values)[max(0, math.ceil(len(values) * p) - 1)]


def evaluate(binary, count, operations, concurrency):
    with tempfile.TemporaryDirectory(prefix="raftkv-evaluation-") as root:
        root = pathlib.Path(root)
        reservations = []
        peers = []
        for i in range(count):
            ports = []
            for _ in range(2):
                s = socket.socket()
                s.bind(("127.0.0.1", 0))
                reservations.append(s)
                ports.append(s.getsockname()[1])
            peers.append(dict(id=f"eval{i}", http_address=f"http://127.0.0.1:{ports[0]}",
                              grpc_address=f"127.0.0.1:{ports[1]}"))
        config = root / "cluster.json"
        config.write_text(json.dumps({"peers": peers}))
        processes = {}
        logs = []
        for s in reservations:
            s.close()

        def start(i):
            log = open(root / f"node{i}.log", "ab")
            logs.append(log)
            p = peers[i]
            processes[i] = subprocess.Popen([str(binary), "--node-id", p["id"],
                "--cluster-config", str(config), "--data-dir", str(root / p["id"]),
                "--http-addr", p["http_address"].removeprefix("http://"),
                "--grpc-addr", p["grpc_address"], "--snapshot-entries", "64"],
                stdout=log, stderr=log)

        def call(i, path, value=None):
            data = None if value is None else json.dumps({"value": value}).encode()
            req = urllib.request.Request(peers[i]["http_address"] + path, data=data,
                method="GET" if value is None else "PUT", headers={"Content-Type": "application/json"})
            with urllib.request.urlopen(req, timeout=3) as r:
                return json.load(r)

        def leader():
            candidates = []
            for i, process in processes.items():
                if process.poll() is not None:
                    continue
                try:
                    s = call(i, "/cluster/status")
                    if s["role"] == "leader":
                        candidates.append((s["term"], i))
                except (OSError, ValueError):
                    pass
            return max(candidates)[1] if candidates else None

        def eventually(fn):
            until = time.monotonic() + 30
            last = None
            while time.monotonic() < until:
                try:
                    value = fn()
                    if value is not None and value is not False:
                        return value
                except (OSError, ValueError, KeyError) as e:
                    last = e
                time.sleep(.03)
            raise RuntimeError(f"cluster did not recover: {last}")

        try:
            for i in range(count):
                start(i)
            first = eventually(leader)
            call(first, "/kv/proof", "retained")
            for i in range(10):
                call(first, f"/kv/warmup{i}", "x" * 128)

            def workload(i):
                begin = time.perf_counter()
                # Unique keys avoid overwrites between workers; every read is
                # checked. Neither writes nor timeouts are silently retried.
                call(first, f"/kv/sample{i}", "x" * 128)
                written = time.perf_counter()
                result = call(first, f"/kv/sample{i}")
                if not result.get("found") or result.get("value") != "x" * 128:
                    raise RuntimeError("read did not match acknowledged write")
                return (written - begin) * 1000, (time.perf_counter() - written) * 1000

            begin = time.perf_counter()
            with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as pool:
                samples = list(pool.map(workload, range(operations)))
            seconds = time.perf_counter() - begin
            killed = time.perf_counter()
            processes[first].kill()
            processes[first].wait()

            def recovered_read():
                elected = leader()
                if elected is None:
                    return None
                result = call(elected, "/kv/proof")
                if result.get("value") != "retained":
                    raise RuntimeError("acknowledged data lost after leader kill")
                return elected

            replacement = eventually(recovered_read)
            failover_ms = (time.perf_counter() - killed) * 1000
            call(replacement, "/kv/after-failover", "healthy")
            target = call(replacement, "/cluster/status")["last_applied"]
            restarted = time.perf_counter()
            start(first)
            eventually(lambda: call(first, "/cluster/status")["last_applied"] >= target)
            catchup_ms = (time.perf_counter() - restarted) * 1000
            def summary(v):
                return {"p50_ms": round(percentile(v, .5), 3),
                        "p95_ms": round(percentile(v, .95), 3),
                        "p99_ms": round(percentile(v, .99), 3)}
            return {"nodes": count, "pairs": operations, "concurrency": concurrency,
                    "payload_bytes": 128, "snapshot_entries": 64,
                    "completed_requests": 2 * operations, "errors": 0,
                    "requests_per_second": round(2 * operations / seconds, 2),
                    "put": summary([p for p, _ in samples]),
                    "get": summary([g for _, g in samples]),
                    "leader_kill_to_verified_read_ms": round(failover_ms, 3),
                    "restarted_node_catchup_ms": round(catchup_ms, 3)}
        except Exception:
            for path in root.glob("*.log"):
                print(path.name, path.read_text()[-4000:])
            raise
        finally:
            for process in processes.values():
                if process.poll() is None:
                    process.terminate()
            for process in processes.values():
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            for log in logs:
                log.close()


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--binary", type=pathlib.Path, default=pathlib.Path("bin/raftkv"))
    p.add_argument("--pairs", type=int, default=100)
    p.add_argument("--concurrency", type=int, default=4)
    p.add_argument("--output", type=pathlib.Path, required=True)
    args = p.parse_args()
    if args.pairs < 1 or args.concurrency < 1:
        p.error("pairs and concurrency must be positive")
    report = {"timestamp_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "platform": platform.platform(), "python": platform.python_version(),
              "scope": "Local independent processes sharing one host and disk; no retries in workload; not an SLA",
              "results": [evaluate(args.binary.resolve(), n, args.pairs, args.concurrency) for n in (3, 5)]}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
