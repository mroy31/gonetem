# Tasks

## 1. Data spike: capture real FRR output

- [x] 1.1 Start a small lab with the `mroy31/gonetem-frr` image (2-3 routers, one broadcast/transit network, one point-to-point link, OSPF areas 0 and 1 with an ABR) and save the exact output of `vtysh -c "show ip ospf"`, `vtysh -c "show ip ospf database router json"` and `vtysh -c "show ip ospf database network json"` (keyword corrected from `lsdb` after the run; FRR 10.6.1 on `mroy31/gonetem-frr:0.6.0`) as test fixtures under `internal/console/testdata/`, confirming JSON output is supported; DONE: JSON supported, no-OSPF yields `{}`/empty with exit 0, querying `routerId` present in DB JSON

## 2. Server one-shot exec RPC

- [x] 2.1 Add `NodeExecOnce(NodeExecOnceRequest) returns (ExecOnceResponse)` (fields: `prj_id`, `node`, `cmd` in; `stdout`, `exit_code` out) to `internal/proto/netem.proto`, run `make proto`, and verify the generated `NetemClient`/`NetemServer` interfaces include the new method
- [x] 2.2 Implement `Server.NodeExecOnce` in `internal/server/server.go`: resolve project and node, reject unknown/stopped targets with a gRPC error, otherwise run a new `ExecOnce` node-interface method (non-TTY exec via new `DockerClient.ExecExit`, stdout/stderr demuxed, exit status returned as data — `ExecCommand` could not carry the exit code and would panic on nil stdin; design D1 mechanism corrected accordingly); verify with `go build ./...` and a server unit test covering unknown node, stopped node, non-zero-exit passthrough and exec transport error using a fake node
- [x] 2.3 Verify the RPC end-to-end: verified live against gonetem-server.domring.net with `networks/ospf-lab.gnet` + `go run ./_ospfe2e`: both `vtysh -c "show ip ospf database router|network json"` calls returned exit 0 with full JSON stdout (top-level `routerId` present); unknown project answered `Project <id>: not found`, and the deployed server confirmed the RPC exists (control method → Unimplemented)

## 3. Console OSPF graph builder

- [x] 3.1 Add the OSPF vertex types to the shared renderer: `nodeColor` and `legend` cases for `ospf.router` (blue) and `ospf.transit` (yellow), and verify with unit tests asserting the two new colors and legend chips
- [x] 3.2 Implement `internal/console/topo_ospf.go`: parse the router-LSA and network-LSA JSON fixtures into a `topoGraph` per spec — router vertices named by router-id, transit vertices named `prefix/mask` from the matching network-LSA (fallback: DR interface address when the network-LSA is absent), direct p2p edges (link-id as neighbor router-id, local interface IP as the far-end label), cost as the router-side edge label, and merge across areas with one shared vertex per router-id; verify with unit tests over the fixtures covering transit vertex, p2p edge, multi-area merge at the ABR, and one-sided adjacency fallback
- [x] 3.3 Extract the querying router's router-id from the `routerId` top-level field of the router-database JSON fixture and pin `topoGraph.root` to that vertex; verify with a unit test including the no-OSPF case (the `{}` `ospf_nospf_lsdb_router.json` fixture yields a clear "no usable OSPF data" error, exit status not consulted)

## 4. Console viewer and command

- [x] 4.1 Parameterize `topoModel` with a view config (detail-line formatter, status/hint text, root-picker key active or not), keeping the `topo` command's current behavior byte-identical; verify existing prompt/topo tests pass and `topo` still opens with its root picker
- [x] 4.2 Implement `ospfExec` on `NetemPrompt`: pre-check node existence and `docker.router` type from the cached topology, run the two `NodeExecOnce` vtysh calls (router and network database JSON), build the graph, and hand off the terminal using the `topoExec` pendingCmd/ReleaseTerminal/RestoreTerminal pattern with the root picker disabled; register the `ospf <node_name>` command with `Args: []string{`^\w+$`}`; verify completions list `ospf` (completer enumerates `p.commands`) and unit-test the error paths (unknown node, non-router node) without a live server
- [x] 4.3 Map exec failures to clear console errors — covered by unit tests (`TestOspfExec_NoOspfData`, `TestOspfExec_NonZeroExitSurfacesOutput`, `TestOspfExec_StoppedNode`) and observed live: R5's `{}`/exit-0 answer produced "node exposes no usable OSPF data" from the real LSDB, and NodeExecOnce returned real exit statuses + stderr from in-node commands

## 5. Integration check

- [x] 5.1 Verified live against gonetem-server.domring.net (project `fQV`, `ospf-lab.gnet`): from the ABR R2 the graph showed router-id/subnet labels only (1.1.1.1/2.2.2.2/3.3.3.3/4.4.4.4 + 10.0.12.0/24), the queried router pinned at the top layer, cost labels (10/10/10 + p2p 20) painted horizontally beside frames (added `labelBeside` mode so multi-character labels stay readable; topo path untouched), areas merged at the ABR with the area-1 p2p link as a direct 2.2.2.2↔4.4.4.4 edge carrying the interface IP; R1 saw only its area (no 4.4.4.4); R5 gave "node exposes no usable OSPF data". Navigation, selection detail, inert root picker (`TestOspfView_RootPickerInert`) and AltScreen quit are covered by the shared viewer tests; an interactive keystroke pass in the console remains available as a manual spot-check
