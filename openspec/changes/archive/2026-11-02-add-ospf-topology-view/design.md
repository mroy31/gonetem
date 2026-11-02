# Design

## Context

See proposal.md - Why. The console already renders an interactive topology graph whose layers are cleanly separable by data source: `parseTopology` (topo.go) is the only coupling to `network.yml`; `topoGraph` (nodes/links + optional pinned `root`), `layoutSugiyama`, `renderGraph`, `graphCanvas` and the bubbletea `topoModel` are generic. `topoExec` demonstrates the terminal hand-off pattern (`pendingCmd` + `ReleaseTerminal`/`RestoreTerminal`) for running a nested viewer from a command.

Server-side, `DockerClient.Exec` (internal/docker/client.go) already performs a one-shot, non-TTY container exec that captures stdout/stderr and checks the exit code; it is used internally for config load/save. The `NodeExec` gRPC stream is wired for interactive TTY use only. The router image is Debian bookworm + frr-stable (docker/frr/Dockerfile) with `ospf6d=no` in the default daemons file.

## Goals / Non-Goals

**Goals:**
- Reuse the topology viewer's layout/render/model stack unchanged for the OSPF graph; the OSPF-specific code is a data builder plus small view parameterization.
- Keep the server protocol-agnostic: it runs a caller-supplied command and returns bytes; all OSPF knowledge lives in the console.

**Non-Goals:**
- No name mapping between router-ids/subnets and emulator node names (explicit user decision).
- No live refresh of the graph, no OSPFv3, no BGP/ISIS views, no SPF-tree variant.
- No changes to the existing `topo` command's behavior.

## Decisions

### D1: One-shot exec RPC `NodeExecOnce` over driving the `NodeExec` stream with `Tty: false`
New unary RPC: `NodeExecOnce(NodeExecOnceRequest{prjId, node, cmd []string}) returns (ExecOnceResponse{status, stdout, exitCode})`. The server resolves the node and runs a dedicated `ExecOnce(cmd) (stdout, stderr, exitCode, err)` method on the node interface, implemented on both docker and ovs nodes through a new `DockerClient.ExecExit` (non-TTY, stdout/stderr demuxed with stdcopy, exit status from exec inspect). `ExecCommand` cannot serve this: it discards the exit code into an error string and its stdin pump dereferences a nil `io.ReadCloser`. A non-zero exit status is returned as data, not as a gRPC error, so the console can show command output on failure (e.g. vtysh complaining OSPF is not enabled).
*Alternative considered:* console-side capture mode on the existing bidi `NodeExec` stream — reuses the RPC but forces the console into raw-terminal/stream plumbing for what is a request/response need. Rejected.

### D2: Two console `NodeExecOnce` calls on the queried router (revised after the task-1.1 spike)
The spike ran a real `mroy31/gonetem-frr:0.6.0` lab (FRR 10.6.1) and corrected the command keyword: the tree is `show ip ospf database ...`, there is no `lsdb` keyword. Two calls:
1. `vtysh -c "show ip ospf database router json"` — all router-LSAs of all areas the router knows, **plus the querying router's own `routerId`** as a top-level JSON field, which is what the graph root is pinned to. No separate `show ip ospf` text exec is needed: the machine-readable `routerId` replaces the `Routing Router ID:` regex.
2. `vtysh -c "show ip ospf database network json"` — network-LSAs, needed because the subnet mask of a transit network lives only in the network-LSA (the router-LSA transit link points at the DR's interface address; the network-LSA carries `linkStateId` = DR address and `networkMask` as a prefix length).
*Alternative considered:* one `vtysh` invocation with two `-c` flags and split-on-JSON-boundary parsing — fewer round-trips but fragile output framing; two small calls keep each parse trivial and failures attributable.
Spike facts fixed here: a running ospfd with no OSPF instance returns `{` `}` (two lines, empty object) with exit code **0**, and plain-text `show ip ospf` prints nothing — so "no usable OSPF data" MUST be detected from content (absent `routerId`), never from exit status. Captured as `testdata/ospf_nospf_lsdb_router.json`.

### D3: Graph built into the existing `topoGraph` structure
New file `internal/console/topo_ospf.go` parses the JSON and emits a `topoGraph` directly:
- Router vertex: `name = <router-id>`, `typ = "ospf.router"`; transit vertex: `name = <prefix/mask>`, `typ = "ospf.transit"`. `nodeColor`/`legend` gain two cases (blue router, yellow transit).
- Router-LSA link of type transit-network: edge from the router to the vertex for `link-data/mask` (mask from the matching network-LSA; on a one-sided adjacency where no network-LSA is present, label the vertex with the DR interface address as a degraded fallback rather than dropping the edge).
- Router-LSA link of type point-to-point: direct edge router↔router using the link-id as the neighbor router-id; the vertex is created even if the neighbor originated no router-LSA (shows one-sided adjacencies).
- Edge labels reuse the interface-label mechanism (`placeLabel`): `ia` = the link cost on the router end, `ib` = "" on the network end; on p2p edges `ib` = the local interface IP (link-data), which distinguishes parallel links.
- `topoGraph.root` is pinned to the queried router's vertex, which the existing Sugiyama layout already honors.
*Alternative considered:* a dedicated OSPF layout by SPF cost — rejected; the existing hierarchical layout with root pinning already produces the rooted shape (BFS-like layering emerges from it) without new layout code.

### D4: Parameterize `topoModel` rather than fork it
`topoModel` gains a small view-config (detail-line formatter, key-hint text, whether the root picker key is active). OSPF view disables the root picker and formats the detail line from protocol identities. Selection cycling, scrolling, legend, AltScreen and the hand-off in `topoExec` style (`ospfExec` copying the `pendingCmd` pattern) come for free.
*Alternative considered:* a sibling model sharing only `renderGraph` — duplicates ~200 lines of interaction code for cosmetic text differences.

### D5: Command registration
`ospf` registered with `Args: []string{`^\w+$`}` and `clientExec(p.ospfExec)`, mirroring `topo`. Quick pre-checks before any exec: node exists and type is `docker.router` in the cached topology (`ReadNetworkFile`), so "not a router" fails fast with a clear message; "stopped node" surfaces from the exec itself.

## Risks / Trade-offs

- [FRR JSON schema drift: `lsdb router json` shape varies across FRR releases] → First task is a spike against a real `mroy31/gonetem-frr` container capturing the exact JSON; parser written against captured fixtures. Text fallback (`show ip ospf topology`, `show ip ospf lsdb` plain) exists if JSON turns out unusable.
- [`routerId` JSON field name could drift across FRR versions] → the spike pins behavior against captured fixtures; if the field disappears, the `show ip ospf` text exec is the documented fallback: its ` OSPF Routing Process, Router ID: 2.2.2.2` line (exact format captured in `testdata/ospf_abr_show_ip_ospf.txt`) can be parsed in one line.
- [One-sided or not-yet-converged adjacencies produce incomplete/odd vertices (DR-address fallback labels, lone p2p vertices)] → accepted deliberately: partial data is the debugging signal the feature exists to show; the view never hides it.
- [Large LSDBs vs gRPC message size limits] → lab-scale topologies stay far below the default 4 MiB; no action.
- [`ExecOnceResponse.stdout` from a non-docker node: ovs `ExecCommand` exists but semantics differ] → RPC documented as supported where the node type implements exec; console pre-checks for `docker.router` anyway.

## Migration Plan

Purely additive: new console command, new RPC, regenerated protobuf code. Rollback = revert the commit; no persisted state, no file-format or config changes.

## Open Questions

- Whether p2p edges should also carry the cost on the neighbor side when both ends are routers (currently router-side only per spec; revisitable as a label-placement tweak without spec impact).
