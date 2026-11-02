# Tasks

## 1. Identifier renames

- [x] 1.1 Rename the generic type family across `internal/console` (sources and tests): `topoGraph` → `graph`, `topoGraphNode` → `graphNode`, `topoGraphLink` → `graphLink`, `topoPoint` → `graphPoint`, `topoModel` → `graphModel`, `topoViewConfig` → `graphViewConfig`, `newPlacedTopoModel` → `newPlacedGraphModel`, `runTopoModel` → `runGraphModel`; verify with `go build ./... && go test ./internal/console/...`
- [x] 1.2 Rename OSPF-side names and their call sites: `ospfTopoView` → `ospfView`, `newOspfTopoModel` → `newOspfModel`, `ospfTopoExec` → `ospfViewExec`; delete dead `isHostType` (zero callers); verify with `go build ./... && go test ./internal/console/...`

## 2. Split topo.go

- [x] 2.1 Create `graph.go` and move the generic machinery out of `topo.go`: `graph`/`graphNode`/`graphLink`/`graphPoint` types, `graphModel` with its Update/View/scroll/selection/root-picker methods, `graphViewConfig`, `newPlacedGraphModel`, `runGraphModel`, `renderGraph`, `legend`, `nodeColor`, `linksOf`, `endpoint`; verify with `go build ./... && go test ./internal/console/...`
- [x] 2.2 Create `graph_canvas.go` and move from `topo.go`: `graphCanvas` and its methods, rune/arm routing helpers (`opp`, `runeArms`, `armsRune`, `joinRunes`, `cornerOrStraight`, `dirBit`, `firstRune`), label placement (`labelSeg`, `makeLabelSeg`, `paintLabel`, `placeLabel`, `placeLabelBeside`), `gridCell`, `stamp`, `overlay`, `lines`, `window`; `topo.go` then holds only the `topoGraph` command (`topoYAML`, `parseTopology`, `splitPeer`, `splitIface`, `defaultTopoView`, `newTopoModel`, `runTopoView`, `topoExec`); verify with `go build ./... && go test ./internal/console/...`

## 3. File renames

- [x] 3.1 `git mv topo_sugiyama.go graph_sugiyama.go` and `git mv topo_subnet.go graph_subnet.go` (contents unchanged); verify with `go build ./... && go test ./internal/console/...`
- [x] 3.2 `git mv topo_ospf.go ospf.go`, `git mv topo_ospf_test.go ospf_test.go`, `git mv topo_manual_test.go graph_manual_test.go`; verify with `go build ./... && go test ./internal/console/...`

## 4. Integration check

- [x] 4.1 Run `gofmt -l internal/console`, `go vet ./...`, `go build ./...`, `go test ./...` and confirm all clean; inspect `git diff -M --stat` to confirm the change is renames and moves only, with no command names, usage strings, or rendering behavior altered

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.
