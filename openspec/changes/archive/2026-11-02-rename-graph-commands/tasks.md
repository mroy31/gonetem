# Tasks

## 1. Command rename

- [x] 1.1 Rename the console command key `ospf` to `ospfGraph` and its `Usage` to `ospfGraph <node_name>` in `internal/console/prompt.go`, and update `internal/console/topo_ospf_test.go` so the registration-key and usage asserts target `ospfGraph`; verify with `go test ./internal/console/` (including the completer test offering `ospfGraph`) and `go build ./...`
- [x] 1.2 Rename the console command key `topo` to `topoGraph` and its `Usage` to `topoGraph` in `internal/console/prompt.go`; verify with `go build ./...` and `go test ./internal/console/` (no registration assert exists for `topo`; confirm completion/help list `topoGraph` via the completer test or a new assert)
- [x] 1.3 Confirm the clean break: verify no code path still registers or accepts `ospf` or `topo` (grep `p.commands[` shows only the new keys; the old names produce "unknown command" behavior handled by the existing prompt dispatch)

## 2. Docs touch-up

- [x] 2.1 Update `_ospfe2e/README.md` manual-check instructions to use `ospfGraph` (and `topoGraph` where the topology viewer is referenced); verify the commands as written exist in `prompt.go`
