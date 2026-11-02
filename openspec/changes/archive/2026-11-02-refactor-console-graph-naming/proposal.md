# Proposal

## Why

The console TUI graph code has drifted into three tangled layers: `internal/console/topo.go` (1314 lines) mixes the `topoGraph` command's own logic with the generic graph display machinery shared with the `ospfGraph` command, while the OSPF file embeds misleading "Topo" names (`ospfTopoView`, `newOspfTopoModel`, `ospfTopoExec`) into generic concepts. The half-applied convention (`graphCanvas`/`renderGraph` are generic but `topoModel`/`topoViewConfig`/`runTopoModel` are equally generic and topo-prefixed) makes it hard to tell what belongs to which command.

## What Changes

- Establish a naming convention in `internal/console`: `graph` prefix = generic graph display machinery (shared by `topoGraph` and `ospfGraph` commands); `topo` prefix = dedicated to the `topoGraph` command; `ospf` prefix stays dedicated to the `ospfGraph` command.
- Rename the core types and their family: `topoGraph` → `graph`, `topoGraphNode` → `graphNode`, `topoGraphLink` → `graphLink`, `topoPoint` → `graphPoint`, `topoModel` → `graphModel`, `topoViewConfig` → `graphViewConfig`, `newPlacedTopoModel` → `newPlacedGraphModel`, `runTopoModel` → `runGraphModel`.
- Rename misleading OSPF-side names: `ospfTopoView` → `ospfView`, `newOspfTopoModel` → `newOspfModel`, `ospfTopoExec` → `ospfViewExec`.
- Split `topo.go`: generic machinery moves to new `graph.go` (types, model, view config, legend, nodeColor) and `graph_canvas.go` (canvas, rune routing, labels, overlay); `topo.go` keeps only the `topoGraph` command (YAML parsing, `defaultTopoView`, `newTopoModel`, `runTopoView`, `topoExec`).
- Move layout files to the convention: `topo_sugiyama.go` → `graph_sugiyama.go`, `topo_subnet.go` → `graph_subnet.go` (methods stay on the `graph` type; `subnet`/`isSwitch` keep their names as ovs-vocabulary heuristics).
- Rename the OSPF command file: `topo_ospf.go` → `ospf.go`, `topo_ospf_test.go` → `ospf_test.go`; rename `topo_manual_test.go` → `graph_manual_test.go` (it exercises the generic viewer).
- Delete dead code: `isHostType` has zero callers.
- Update all test files mechanically (`prompt_models_test.go`, `ospf_test.go`, `graph_manual_test.go`).

No behavior change: same commands, same usage strings, same rendering output. Pure rename and file moves.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None — this is a pure internal refactor with no spec-level behavior change (`skip_specs: true`).

## Impact

- `internal/console/topo.go` (split), `topo_ospf.go` (renamed to `ospf.go`), `topo_subnet.go`, `topo_sugiyama.go` (renamed), test files `topo_ospf_test.go`, `topo_manual_test.go`, `prompt_models_test.go`.
- No changes to command registration in `prompt.go` (command names and usage strings unchanged), to `prompt_models.go` beyond identifier updates (`OspfAreaViewText` export unchanged), or to any wire protocol/server code.
- Verification: `go build ./...`, `go vet ./...`, `go test ./internal/console/...`.
