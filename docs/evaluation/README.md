# Distributed process evaluation

Run `make build`, then:

```sh
python3 scripts/evaluate-cluster.py --pairs 100 --concurrency 4 --output results.json
```

The script owns temporary data directories and randomly allocated loopback ports.
It starts independent 3-node and 5-node clusters, performs ten warm-up PUTs,
then 100 unique-key PUT/GET pairs with four workers. Payloads are 128 bytes;
automatic checkpoints occur every 64 entries. Every GET must match its PUT.
Workload errors fail the run instead of being silently retried or omitted.
Requests per second counts both PUT and GET; percentiles use nearest rank.

After measurement, the script kills its elected leader and measures time until
a successful consensus read returns an acknowledged key. It then writes again,
restarts that process and measures time until it reaches the previously observed
applied index. The script cleans up only the processes and temporary directories
it created. It never targets the demo cluster or Docker containers.

The checked-in JSON records the host, workload, latency percentiles, throughput,
failure recovery and catch-up observations. All nodes share one machine and disk.
This is a reproducible integration baseline, not a multi-host capacity benchmark,
an SLA, a statistical comparison between node counts or a hardware durability
certification. Snapshot work and local scheduling affect measurements. Repeat on
an otherwise idle host, report variation, and use separate hosts before making
distributed deployment performance claims. CI runs a smaller 20-pair smoke test
and uploads the raw result without enforcing machine-dependent speed thresholds.

## Remaining evaluation gates

- Repeat measurements and report spread under a documented, controlled workload.
- Review combined-fault coverage and consensus semantics independently.
- Exercise the demo from a fresh checkout and record the accepted release commit.
- Finish the presentation and final evaluation submission; a passing benchmark
  does not satisfy review or submission requirements.
