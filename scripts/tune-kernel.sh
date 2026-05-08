#!/usr/bin/env bash
# Kernel + ulimit tweaks for holding 50k+ WebSocket connections.
# Linux only. Idempotent — re-run safely. Requires root.
set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "Linux only — macOS uses a different stack and these knobs do not apply."
  exit 0
fi

if [[ $EUID -ne 0 ]]; then
  echo "must run as root (use sudo)" >&2
  exit 1
fi

# 1. file descriptors per process
#    Each WebSocket conn = 1 fd. 50k conns + epoll fds + listen socket + ...
#    100 000 gives plenty of headroom.
ulimit -n 100000 || true
cat >/etc/security/limits.d/realtimehub.conf <<'EOF'
*  soft  nofile  100000
*  hard  nofile  100000
EOF

# 2. tcp tuning
sysctl -w net.core.somaxconn=4096
sysctl -w net.ipv4.tcp_max_syn_backlog=8192
sysctl -w net.ipv4.ip_local_port_range="1024 65535"
sysctl -w net.ipv4.tcp_tw_reuse=1
sysctl -w net.ipv4.tcp_fin_timeout=15
sysctl -w net.core.netdev_max_backlog=16384

# 3. memory for buffers (modest bump; nbio is per-conn lean)
sysctl -w net.core.rmem_max=16777216
sysctl -w net.core.wmem_max=16777216

echo
echo "applied. verify with:"
echo "  sysctl net.core.somaxconn net.ipv4.tcp_max_syn_backlog"
echo "  ulimit -n"
