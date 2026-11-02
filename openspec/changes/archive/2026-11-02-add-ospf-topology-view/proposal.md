# Proposal

## Why

Users running OSPF labs cannot see what a router actually believes about the area: today `topo` shows only the configured (physical) topology, so debugging adjacencies or LSDB inconsistencies requires manually running `vtysh` commands on each router. A command that renders one router's OSPF area view as an interactive graph, rooted at that router, turns "is my area correct?" from manual CLI archaeology into a single glance.

## What Changes

- New console command `ospf <node_name>` that opens an interactive graph, in the same TUI style and terminal hand-off as `topo`, showing the OSPF area as seen by the named router.
- Graph content comes from the router's own link-state database, obtained by executing `vtysh` commands inside the node: router vertices labeled by router-id, transit-network vertices labeled by subnet prefix, point-to-point links as direct router-to-router edges. No emulator node names are shown; users associate IPs with names themselves.
- The queried router is pinned as the root of the hierarchical layout; its links are highlighted. Link costs are displayed as edge labels.
- New server RPC for one-shot, non-interactive exec on a node that returns captured stdout (the console's existing `NodeExec` usage is interactive-TTY-only, although the server already has a non-TTY exec primitive internally).
- IPv4 OSPF (ospfd) only; OSPFv3 is not enabled in the default router image.

## Capabilities

### New Capabilities
- `ospf-visualization`: the console's OSPF graph command — data acquisition from a router's LSDB, graph model and labeling, root pinning, rendering behavior, and error reporting.

### Modified Capabilities
- `node-operations`: add a one-shot exec requirement — a single request/response operation that runs a command on a running node without a tty and returns its captured output and exit status.

## Impact

- Console: new command registration in `internal/console/prompt.go`, new `ospf` view code under `internal/console/` (graph builder beside `topo.go`), parameterization of the shared `topoModel`/renderer (currently hardcoded to topology-view text and the root picker).
- Server: new `NodeExecOnce` RPC in `internal/proto/*.proto` wrapping the existing `DockerClient.Exec` capture primitive; regeneration of protobuf/gRPC code.
- Dependencies: relies on FRR 10.x JSON output of `show ip ospf database router|network json`, confirmed on the real `mroy31/gonetem-frr:0.6.0` image (fixtures under `internal/console/testdata/`).
- No breaking changes: purely additive command and RPC.
