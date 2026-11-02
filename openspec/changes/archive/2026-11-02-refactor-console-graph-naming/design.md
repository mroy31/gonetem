# Design

## Context

See proposal.md - Why. `internal/console/topo.go` currently holds three layers at once (the `topoGraph` command, the generic graph display machinery, and layout heuristics), and `topo_ospf.go` — the `ospfGraph` command — is misfiled under the `topo` prefix and embeds "Topo" in generic names. The code already half-follows the target convention: every `graph`-prefixed identifier that exists today (`graphCanvas`, `newGraphCanvas`, `renderGraph`, `buildOspfGraph`) is generic machinery.

## Goals / Non-Goals

**Goals:**
- One convention: `graph*` = generic display machinery shared by both commands; `topo*` = the `topoGraph` command only; `ospf*` = the `ospfGraph` command only.
- File layout mirrors the convention so `ls` reads: machinery (`graph*.go`), one file per command (`topo.go`, `ospf.go`).
- Byte-for-byte identical behavior: same command names, usage strings, rendering.

**Non-Goals:**
- No change to the viewer's features, keybindings, or rendering.
- No generalization of the type-vocabulary seam (`nodeColor`/`legend`/`isSwitch` keep knowing `docker.*`, `ovs`, and `ospf.*` strings inline). If a third graph command ever appears, a per-view color/legend hook can be extracted then.
- No changes to `prompt.go` command registration or `OspfAreaViewText`'s exported name.

## Decisions

### Core type name is bare `graph`

`topoGraph` → `graph` (in package `console`), with family `graphNode`, `graphLink`, `graphPoint`, `graphModel`, `graphViewConfig`. Alternative considered: a qualified name (`placedGraph`) to avoid a very generic type name, rejected — inside `console` it never stutters and it names the machinery exactly as generically as it is. Methods on the types (`neighbors`, `legend`, `linksOf`, `degree`, `components`, layout methods) keep their names: the receiver carries the prefix.

### Full rename map

Generic machinery (moves to `graph.go` / `graph_canvas.go`):

| Old | New |
|-----|-----|
| `topoGraph` | `graph` |
| `topoGraphNode` | `graphNode` |
| `topoGraphLink` | `graphLink` |
| `topoPoint` | `graphPoint` |
| `topoModel` | `graphModel` |
| `topoViewConfig` | `graphViewConfig` |
| `newPlacedTopoModel` | `newPlacedGraphModel` |
| `runTopoModel` | `runGraphModel` |

Unchanged (already `graph`-prefixed or methods): `graphCanvas`, `newGraphCanvas`, `renderGraph`, `endpoint`, `isSwitch`, canvas/rune/label/overlay helpers, `gridCell`, `panelLine`, `sortedKeys`, `abs`.

`topo.go` keeps only the `topoGraph` command: `topoYAML`, `parseTopology`, `splitPeer`, `splitIface`, `defaultTopoView`, `newTopoModel`, `runTopoView`, `topoExec`. `isHostType` is deleted (dead code, zero callers).

`ospf.go` (was `topo_ospf.go`) sheds misleading "Topo":

| Old | New | Rationale |
|-----|-----|-----------|
| `ospfTopoView` | `ospfView` | pairs with `defaultTopoView`, one view config per command |
| `newOspfTopoModel` | `newOspfModel` | pairs with `newTopoModel` |
| `ospfTopoExec` | `ospfViewExec` | the DB-fetch + open-view step under `ospfExec` |

`ospfExec`, `runOspfView`, `OspfAreaViewText`, `buildOspfGraph`, and the `ospf*DB/Lsa/Link` JSON types keep their names.

### File split and moves

| From | To | Contents |
|------|----|----------|
| `topo.go` (generic half) | `graph.go` | `graph`/`graphNode`/`graphLink`/`graphPoint`, `graphModel` + Update/View/scroll/selection/root picker, `graphViewConfig`, `newPlacedGraphModel`, `runGraphModel`, `renderGraph`, `legend`, `nodeColor`, `linksOf`, `endpoint` |
| `topo.go` (canvas half) | `graph_canvas.go` | `graphCanvas` + methods, rune/arm routing helpers (`opp`, `runeArms`, `armsRune`, `joinRunes`, `cornerOrStraight`, `dirBit`), `firstRune`, label placement (`labelSeg`, `makeLabelSeg`, `paintLabel`, `placeLabel`, `placeLabelBeside`), `gridCell`, `stamp`, `overlay`, `lines`, `window` |
| `topo.go` (command half) | stays `topo.go` | see above |
| `topo_sugiyama.go` | `graph_sugiyama.go` | unchanged contents |
| `topo_subnet.go` | `graph_subnet.go` | unchanged contents |
| `topo_ospf.go` | `ospf.go` | renames above |
| `topo_ospf_test.go` | `ospf_test.go` | mechanical updates |
| `topo_manual_test.go` | `graph_manual_test.go` | exercises the generic viewer |

`graph_subnet.go` keeps the `subnet`/`isSwitch` names: they are ovs-vocabulary heuristics that are part of the shared layout path (`subnetMap` is consumed by `graph_sugiyama.go`) and legitimately degenerate to the single "misc" group for OSPF graphs (no `ovs`-typed nodes).

### Mechanical execution

Rename via `gopls rename`/`govim`-style identifier rename or careful `gofmt`-safe text substitution, then `git mv` the files in the same commit so history follows (`git log --follow`). Test files are updated mechanically; no test logic changes.

## Risks / Trade-offs

- [Mechanical rename touches every test in the package] → Verification is trivial and complete: `go build ./... && go vet ./... && go test ./internal/console/...`; behavior-preserving by construction, and `prompt_models_test.go`/`ospf_test.go` pin the command names and usage strings so any accidental user-visible drift fails loudly.
- [`graph` as a bare type name can collide with local variables named `graph`] → Only new scopes matter; existing locals of that name will shadow within their scope and the compiler will catch any conflict immediately.
- [`topo_` → `graph_` file renames churn blame history] → Mitigated by `git mv` in one commit; content moves in `topo.go` are separable in review by doing renames first, moves second.
