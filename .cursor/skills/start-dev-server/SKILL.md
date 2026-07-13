---
name: start-dev-server
description: Run commands to start Grafana local dev after initial setup. Use when the user asks how to start the dev server, run Grafana locally, open Grafana, or run frontend/backend watchers.
---

# Start dev server

## Instructions
When asked how to start the dev server, open Grafana, or run locally after initial setup, run the commands below using the Shell tool. Do not ask the user to run them.

### Demo / fast path (default)
This workspace is used for repeated demos. Prefer the keep-alive path so startup stays fast:

1. Check readiness with `curl -sI http://127.0.0.1:3000/`
2. If not healthy, run:

```sh
.cursor/hooks/ensure-grafana-running.sh
```

This starts the **prebuilt** `bin/grafana` binary against existing `public/build` assets. Do **not** run `make run` or `yarn start` for demos unless the user explicitly wants hot reload / live rebuild.

A `sessionStart` hook also runs this script automatically when a new agent chat starts. On macOS it installs/loads LaunchAgent `com.cursor.grafana-demo` with `KeepAlive`, so Grafana stays up across chats and terminal exits.

### Full hot-reload path (only when explicitly needed)
Use this when actively editing Go/TS and you need watchers:

#### Preflight
Before starting new processes, check if they are already running by inspecting the terminals folder. If `yarn start` fails with a missing `node_modules` state file (or no `node_modules` is present), run the frontend dependency install in this order:

```sh
corepack enable
corepack install
yarn install --immutable
```

Raise the open-file limit before webpack watch (macOS often hits EMFILE otherwise):

```sh
ulimit -n 10240
```

#### Backend server
```sh
make run
```

#### Frontend assets watcher
```sh
yarn start
```

## Notes
- Fast path target: `http://localhost:3000/login` (admin/admin)
- After ensure script, `curl -I http://localhost:3000/` should return `HTTP/1.1 302` to `/login`
- Server logs: `.cursor/logs/grafana-server.log`
- PID file: `.cursor/logs/grafana.pid`
- If the embedded `@Browser` shows `chrome-error://chromewebdata/` or won't load, open `http://localhost:3000/login` in the local system browser instead
- Only fall back to `make run` / `yarn start` when the prebuilt binary is missing or the user asks for hot reload
