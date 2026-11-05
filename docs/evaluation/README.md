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

## Repeated measurements on 5 November 2026

Three repetitions per size completed 1,200 requests without workload errors.
Every repetition checked retained data after leader kill and restarted-node
catch-up. Raw files are `repeat-1/2/3-2026-11-05.json` in this directory.
The source baseline is `563ea61` (included unchanged in the integration branch).
Existing local demo processes and build/test activity shared this host during
these runs. This is observed development-environment variability, not an idle
host experiment or a statistically significant ranking of cluster sizes.

| Nodes | Requests/s range | PUT p95 ms range | GET p95 ms range | Recovery seconds range |
|---|---|---|---|---|
| 3 | 124.75–137.21 | 49.16–69.27 | 55.30–70.22 | 1.19–1.72 |
| 5 | 97.27–113.65 | 53.39–103.92 | 93.06–105.84 | 0.96–1.04 |

Repeat with the documented command three times and distinct output paths.
The earlier 4 November single-sample result is retained for provenance, not
combined with these runs. Percentile ranges are ranges of per-run percentiles,
not pooled percentiles. Operator review and final evaluation acceptance remain.
