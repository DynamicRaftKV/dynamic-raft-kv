#!/usr/bin/env python3
"""Acceptance for the disposable five-node Compose test cluster.

Run after `docker compose --profile node4 --profile node5 up -d --build`.
This deliberately kills/restarts containers. It never deletes volumes.
"""
import json
import subprocess
import time
import urllib.request

def call(node, path, payload=None, method=None):
    data = None if payload is None else json.dumps(payload).encode()
    req = urllib.request.Request(f"http://127.0.0.1:{8000+node}{path}", data=data,
                                 headers={"Content-Type": "application/json"}, method=method)
    with urllib.request.urlopen(req, timeout=8) as response:
        return json.load(response)

def eventually(work, description, seconds=90):
    deadline = time.monotonic() + seconds
    last = None
    while time.monotonic() < deadline:
        try:
            result = work()
            if result:
                return result
        except Exception as error:
            last = error
        time.sleep(.2)
    raise RuntimeError(f"{description} timed out: {last}")

def leader(exclude=()):
    candidates = []
    for node in range(1, 6):
        if node in exclude:
            continue
        try:
            status = call(node, "/cluster/status")
            if status["role"] == "leader":
                candidates.append((status["term"], node))
        except Exception:
            pass
    return max(candidates)[1] if candidates else None

def stable(size):
    node = leader()
    if not node:
        return False
    status = call(node, "/cluster/status")
    return node if not status["membership_pending"] and len(status["peers"]) == size else False

def main():
    first = eventually(lambda: stable(3), "initial election")
    call(first, "/kv/compose-proof", {"value": "durable"}, "PUT")
    for joining in (4, 5):
        def admit():
            node = leader()
            if not node:
                return False
            status = call(node, "/cluster/status")
            if any(p["id"] == f"node{joining}" for p in status["peers"]):
                return True
            return call(node, "/cluster/join", {"id": f"node{joining}",
                        "grpc_address": f"node{joining}:{9000+joining}",
                        "http_address": f"http://127.0.0.1:{8000+joining}"})
        eventually(admit, "learner admission")
        def promote():
            node = stable(joining)
            if not node:
                return False
            status = call(node, "/cluster/status")
            if any(p["id"] == f"node{joining}" and not p["learner"] for p in status["peers"]):
                return True
            return call(node, "/cluster/promote", {"id": f"node{joining}"})
        eventually(promote, "learner promotion")
        eventually(lambda: stable(joining), "finalize membership")
    node = eventually(lambda: stable(5), "five-node cluster")
    # Check every node has applied finalized membership before taking one down.
    def synchronized():
        for n in range(1, 6):
            s = call(n, "/cluster/status")
            if s["membership_pending"] or len(s["peers"]) != 5 or any(p["learner"] for p in s["peers"]):
                return False
        return True
    eventually(synchronized, "membership replication")
    subprocess.run(["docker", "compose", "kill", "-s", "SIGKILL", f"node{node}"], check=True)
    replacement = eventually(lambda: leader((node,)), "replacement election")
    assert call(replacement, "/kv/compose-proof")["value"] == "durable"
    subprocess.run(["docker", "compose", "--profile", "node4", "--profile", "node5", "kill", "-s", "SIGKILL"], check=True)
    subprocess.run(["docker", "compose", "--profile", "node4", "--profile", "node5", "up", "-d"], check=True)
    recovered = eventually(lambda: stable(5), "full-cluster recovery")
    assert call(recovered, "/kv/compose-proof")["value"] == "durable"
    call(recovered, "/kv/after-restart", {"value": "healthy"}, "PUT")
    assert call(recovered, "/kv/after-restart")["value"] == "healthy"
    print("PASS: 3→5 nodes, promotion, leader kill, durable full restart, writes and reads")

if __name__ == "__main__":
    main()
