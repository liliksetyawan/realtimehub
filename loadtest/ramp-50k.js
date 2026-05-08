// k6 load test: ramp to 50,000 concurrent WebSocket connections, sustain
// for 5 minutes, then drain.
//
// Prereq:
//   - run scripts/tune-kernel.sh on the *server* host (Linux only)
//     OR equivalent ulimit -n 100000 + sysctl tweaks.
//   - on the load-generator host (the machine running k6), also bump
//     ulimit -n 100000 — every WS conn = 1 fd on this side too.
//
// Usage:
//   ulimit -n 100000
//   API_BASE=http://localhost:8090 WS_URL=ws://localhost:8090 \
//     k6 run loadtest/ramp-50k.js
//
// Tip: monitor in another terminal:
//   watch -n 1 'curl -s http://localhost:8090/healthz | jq .connections'

import ws from "k6/ws";
import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  scenarios: {
    ramp: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 5000 },
        { duration: "1m", target: 25000 },
        { duration: "2m", target: 50000 },
        { duration: "5m", target: 50000 }, // sustain
        { duration: "30s", target: 0 },
      ],
      gracefulRampDown: "30s",
      gracefulStop: "30s",
    },
  },
  thresholds: {
    ws_connecting: ["p(95)<2000"], // 95% of handshakes <2s
    ws_session_duration: ["avg>200000"], // sessions average >200s (we hold for 240s)
  },
};

const API_BASE = __ENV.API_BASE || "http://localhost:8090";
const WS_URL = __ENV.WS_URL || "ws://localhost:8090";

// One-time setup: log in as the demo users and capture their tokens.
// All 50k VUs reuse this small pool — the test is about connection
// scale, not auth load. Each VU picks a token by `__VU % len`.
export function setup() {
  const creds = [
    { username: "alice", password: "password123" },
    { username: "bob", password: "password123" },
    { username: "charlie", password: "password123" },
  ];
  const tokens = [];
  for (const c of creds) {
    const res = http.post(`${API_BASE}/v1/auth/login`, JSON.stringify(c), {
      headers: { "Content-Type": "application/json" },
    });
    if (res.status === 200) {
      tokens.push(JSON.parse(res.body).token);
    } else {
      console.error(`login failed for ${c.username}: ${res.status}`);
    }
  }
  if (tokens.length === 0) {
    throw new Error("setup failed: no tokens; is the server up?");
  }
  return { tokens };
}

export default function (data) {
  const token = data.tokens[__VU % data.tokens.length];
  const url = `${WS_URL}/ws?token=${token}`;

  const res = ws.connect(url, {}, function (socket) {
    socket.on("open", function () {
      // first frame the server sends is `welcome` — receive then idle.
    });

    socket.on("message", function (raw) {
      let frame;
      try {
        frame = JSON.parse(raw);
      } catch {
        return;
      }
      // Respond to server pings so the reaper doesn't evict us.
      if (frame.type === "server_ping") {
        socket.send(JSON.stringify({ type: "pong" }));
      }
    });

    // hold the connection for ~4 minutes, well within the sustain stage
    socket.setTimeout(function () {
      socket.close();
    }, 240000);

    socket.on("error", function (e) {
      if (e.error() != "websocket: close sent") console.error("ws error:", e.error());
    });
  });

  check(res, { "handshake 101": (r) => r && r.status === 101 });
  sleep(1);
}
