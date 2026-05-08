# Load Test — Ramp to 50,000 WebSocket Connections

`ramp-50k.js` drives a single RealtimeHub node up to 50 000 concurrent WebSocket connections, holds for 5 minutes, and drains. The point is to prove the headline number for the README.

## Prerequisites

### On the **server** host

```bash
sudo bash scripts/tune-kernel.sh
```

This sets `ulimit -n 100000` and the relevant `sysctl` knobs (somaxconn, tcp_max_syn_backlog, ip_local_port_range, tcp_tw_reuse, etc.).

The server itself needs the `ulimit` to also apply to its process. If running under systemd, add `LimitNOFILE=100000` to the unit. If running under Docker, pass `--ulimit nofile=100000:100000`.

### On the **load-generator** host

```bash
ulimit -n 100000
brew install k6   # or apt install k6 / see https://k6.io
```

If the generator is the same machine as the server, you'll be fighting for ports. Ideally use a second machine.

## Run

```bash
ulimit -n 100000
API_BASE=http://localhost:8090 WS_URL=ws://localhost:8090 \
  k6 run loadtest/ramp-50k.js
```

In a separate terminal, watch the live count:

```bash
watch -n 1 'curl -s http://localhost:8090/healthz | jq .connections'
```

## What to expect (target numbers)

On a 4 vCPU / 8 GB RAM commodity VM with the sysctl tweaks above:

| Stage | Connections | RAM | CPU |
|---|---|---|---|
| ramp | 0 → 25k | <1 GB | 5-10 % |
| sustain | 50k | ~3-4 GB | 15-25 % |
| drain | 50k → 0 | — | — |

If RAM creeps higher, check the `WriteBuffer` config (default 64) and `MaxLoad` in nbio (default 1M). If CPU is the limit before RAM, you've likely hit the netpoll worker pool default — increase `runtime.GOMAXPROCS` or scale horizontally with another node (Redis pub-sub will fan-out for you).

## Recording results

After the run, record the numbers in the project README under the "Benchmarks" section so the claim is reproducible.
