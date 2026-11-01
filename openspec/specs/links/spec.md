# links

## Purpose

Define point-to-point emulated links between node interfaces and the per-direction
network impairments (netem loss/delay/jitter, tbf rate/buffer) that make the
emulation realistic. Links are declared in the topology and can also be added,
retuned and removed live on a running topology, optionally synchronized back into
`network.yml`.

## Requirements

### Requirement: Point-to-point link establishment
Each link SHALL connect exactly two endpoints addressed as `<node>.<ifIndex>`,
realized as a veth pair whose ends appear inside each node as its `ethN` interface,
admin-up and attached to the node. Links of the topology SHALL be established during
topology run, and link endpoints SHALL be usable immediately by running nodes.

#### Scenario: Simple host-to-router link
- **WHEN** a topology declares `host1.1 <--> router1.1` and is run
- **THEN** both nodes show an up interface carrying the link and frames flow between
  them

#### Scenario: Endpoint reuse rejected
- **WHEN** a topology reuses one `<node>.<ifIndex>` endpoint in a second link
- **THEN** validation rejects the topology

### Requirement: Impairment configuration in the topology
Links SHALL accept symmetric impairment parameters — `loss` percent, `delay` ms,
`jitter` ms, `rate` kbps, `buffer` BDP scale factor — and per-peer overrides that
replace them per direction. Delay/jitter/loss SHALL be enforced with a netem qdisc on
the affected direction; a non-zero `rate` SHALL additionally enforce a rate limit with
buffer sized from the BDP (default 1.0 when unset). Rate and buffer are topology-time
parameters fixed at link setup.

#### Scenario: Asymmetric impairment
- **WHEN** a link declares a per-peer loss/delay override for one side only
- **THEN** only that direction's interface carries the netem qdisc with those values

#### Scenario: Rate limiting
- **WHEN** a link declares `rate` with a non-zero `delay`
- **THEN** both directions enforce the rate limit with buffer derived from
  rate x delay

### Requirement: Live link add, retune and delete
On a running topology the server SHALL add a new link between two running nodes
(failing if that link already exists), SHALL retune a link's netem impairment
parameters (loss, delay, jitter) per direction while traffic flows, and SHALL delete
a link (removing its veth), failing when the addressed link is not present.

#### Scenario: Add a link live
- **WHEN** a client adds `router1.3 <--> router2.3` to a running topology
- **THEN** the endpoints come up connected without restarting either node

#### Scenario: Duplicate live add
- **WHEN** the same pair of endpoints is added again
- **THEN** the request fails with an already-exists error

#### Scenario: Retune impairments live
- **WHEN** a client updates a link's loss to 10 percent
- **THEN** the running interfaces adopt the new loss without re-creating the link

#### Scenario: Delete an unknown link
- **WHEN** a client deletes a link whose endpoints are not connected
- **THEN** the request fails with a not-found error

### Requirement: Persist link changes with sync
Every live link operation SHALL accept a sync flag; when set, the topology manager
SHALL rewrite `network.yml` so the running link set is what a later reload or save
would restore.

#### Scenario: Sync keeps the project file truthful
- **WHEN** a client adds a link live with sync enabled
- **THEN** the project's `network.yml` contains the new link, and a reload preserves
  it
