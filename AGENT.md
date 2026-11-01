# AGENT.md

Guidance for AI agents working in this repository.

## Project Overview

gonetem is a network emulator written in Go. It emulates nodes (hosts, servers,
routers) with Docker containers and switches with Open vSwitch. It is composed
of two binaries:

- `gonetem-server` (`cmd/gonetem-server`): core emulator, requires root access
  to manage Docker containers, OVS switches, links and netlink/tc.
- `gonetem-console` (`cmd/gonetem-console`): interactive TUI client that talks
  to the server over gRPC.

Module: `github.com/mroy31/gonetem`, Go 1.26. License: GPLv3+.

## Build

```sh
make build-amd64        # both binaries for linux/amd64 -> ./bin
make build-arm64        # linux/arm64
make build-console-darwin-arm64  # console only, darwin/arm64 (dev on macOS)
make clean              # removes ./bin
```

Binaries are plain `go build` of the two `cmd/*/main.go` files with
`CGO_ENABLED=0` and `-ldflags "-X main.Version=$(VERSION)"` (version comes
from the `VERSION` file).

## Protobuf / gRPC

The server/console API is defined in `internal/proto/netem.proto`. Generated
files (`netem.pb.go`, `netem_grpc.pb.go`) are committed. After editing the
proto, regenerate with:

```sh
make proto    # requires protoc + protoc-gen-go + protoc-gen-go-grpc
```

## Testing

```sh
make test               # runs: sudo go test ./...
```

Tests must run with sudo: many touch Docker, netlink, OVS or tc. Some are
manual (e.g. `internal/console/topo_manual_test.go`). For quick checks without
root, run targeted packages, e.g. `go test ./internal/utils/...`.

## Layout

- `cmd/gonetem-server`, `cmd/gonetem-console` — entrypoints (cobra CLI via `internal/options`)
- `internal/server` — gRPC server: projects, nodes, topology, mgmt
- `internal/console` — bubbletea/bubbles/lipgloss TUI, commands, completion, topology graph rendering
- `internal/proto` — proto definition + generated gRPC code
- `internal/docker` — Docker client wrapper, node lifecycle
- `internal/ovs` — Open vSwitch instance and switch nodes
- `internal/link` — links between nodes, tc (netem/impairments)
- `internal/options` — CLI options and config file handling (`conf/config.yaml`)
- `internal/utils` — small helpers (archive, paths, random, strings)
- `networks/*.gnet` — example/topology project files used for testing/demos
- `docker/` — Dockerfiles for emulated node images (frr, host, server, ovs, bmv2, vyos)
- `docs/` — Sphinx documentation (`make -C docs html`)
- `scripts/` — helpers: TLS cert generation, topology generators, pynetem migration

## Conventions & Notes

- Logging uses `github.com/sirupsen/logrus`; CLI flags via `spf13/cobra`.
- Console UI is built with charmbracelet (`bubbletea`/`bubbles`/`lipgloss`) v2.
- Keep server-side logic in `internal/server` + `internal/docker`/`ovs`/`link`;
  the console should only talk to the server through the gRPC API
  (`internal/console/client.go`), never directly to Docker/OVS.
- `.gnet` project files are archives (see `internal/utils/archive.go`);
  `test.gnet` at the repo root is a scratch test topology.
- Before adding a console command, look at `internal/console/commands.go` and
  add tab-completion in `internal/console/completer.go` (there are tests).
- The remote UI is a separate repo (gonetem-ui); TLS support lives in
  `internal/options/tls.go` and `scripts/generate-cert.sh`.
- Comments and some docs are in French; match the tone of the file you edit.
