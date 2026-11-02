# Proposal

## Why

The console's two graph commands, `ospf` and `topo`, are the only prompt commands that hide their graph nature behind a bare protocol/domain word, while every other multi-word command uses camelCase (`ifState`, `saveAs`, `viewConfig`). Renaming them to `ospfGraph` and `topoGraph` makes the graph-view family self-evident in help and completion.

## What Changes

- **BREAKING** Console command `ospf <node_name>` is renamed to `ospfGraph <node_name>`. The old name is removed with no alias (clean break).
- **BREAKING** Console command `topo` is renamed to `topoGraph`. The old name is removed with no alias.
- Command descriptions, usage strings, and prompt completion follow automatically from the registration map; no new behavior is added.
- Internal Go identifiers (`ospfExec`, `topoExec`, vertex types `ospf.router`/`ospf.transit`, view config helpers) are unchanged — the rename is user-facing only.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `ospf-visualization`: the requirement currently named "ospf command" is renamed to "ospfGraph command" and its text and scenarios are updated to the `ospfGraph` command name. No behavior changes.

The `topo` command is not named in any existing spec (only referenced stylistically as "the topology graph view"), so its rename produces no spec delta.

## Impact

- `internal/console/prompt.go`: two command registration keys and their `Usage` strings.
- `internal/console/topo_ospf_test.go`: assertions on the registered command key and usage string.
- `_ospfe2e/README.md`: manual-check instructions referencing `ospf R1`.
- Spec `openspec/specs/ospf-visualization/spec.md`: synced at archive time from this change's delta.
- Users' muscle memory and any scripts invoking `ospf`/`topo` (accepted: clean break, pre-release project).
