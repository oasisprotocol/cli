---
title: Network
description: Managing Mainnet, Testnet or Localnet endpoints
---

# Manage Your Oasis Networks

The `network` command is used to manage the Mainnet, Testnet or Localnet
endpoints Oasis CLI will be connecting to.

The `network` command is commonly used:

- on network upgrades, because the chain domain separation context is changed
  due to a new [genesis document],
- when setting up a local `oasis-node` instance instead of relying on public
  gRPC endpoints,
- when running a private Localnet with `oasis-net-runner`,
- when examining network properties such as the native token, the network
  registry, the validator set and others.

Oasis CLI supports both **remote endpoints via the secure gRPC protocol** and
**local Unix socket endpoints**.

:::tip

When running the Oasis CLI for the first time, it will automatically configure
the current Mainnet and Testnet endpoints.

:::

[genesis document]: https://github.com/oasisprotocol/oasis-core/blob/master/docs/consensus/genesis.md#genesis-documents-hash

## Add a Network {#add}

Invoke `network add <name> <rpc-endpoint> [chain-context]` to add a new
endpoint with a specific chain domain separation context and a gRPC address.
This command is useful, if you want to connect to your own instance of the Oasis
node instead of relying on the public gRPC endpoints.

For TCP/IP endpoints, run:

![code shell](../examples/network/add-tcpip-ctx.in.static)

![code](../examples/network/add-tcpip-ctx.out.static)

For Unix sockets, use:

![code shell](../examples/network/add-unix-ctx.in.static)

![code](../examples/network/add-unix-ctx.out.static)

To automatically detect the chain context, simply omit the `[chain-context]`
argument:

![code shell](../examples/network/add-tcpip.in.static)

![code](../examples/network/add-tcpip.out.static)

## Add a Local Network {#add-local}

`network add-local <name> <rpc-endpoint>` command can be used if you are
running `oasis-node` on your local machine. In this case, Oasis CLI will
autodetect the chain domain separation context. For the Oasis Mainnet and
Testnet chains, the native token symbol, the number of decimal places and
registered ParaTimes will automatically be predefined. Otherwise, the Oasis CLI
will ask you to enter them.

```shell
oasis network add-local testnet_local unix:/node_testnet/data/internal.sock
```

To override the defaults, you can pass `--num-decimals`, `--symbol` and
`--description` parameters. This is especially useful, if you are running the
command in a [non-interactive mode](account.md#y):

```shell
oasis network add-local testnet_local unix:/node_testnet/data/internal.sock --num-decimals 9 --symbol TEST --description "Work machine - Localnet" -y
```

## List Networks {#list}

Invoke `network list` to list all configured networks.

![code shell](../examples/network/00-list.in)

![code](../examples/network/00-list.out)

The [default network](#set-default) is marked with the `(*)` sign.

## Remove a Network {#remove}

Use `network remove <name>` to remove the given network configuration including
all dependant ParaTimes.

![code shell](../examples/network/01-remove.in)

You can also delete network in non-interactive mode format by passing the
`-y` parameter:

![code shell](../examples/network/remove-y.in.static)

## Set Network Chain Context {#set-chain-context}

To change the chain context of a network, use
`network set-chain-context <name> [chain-context]`.

:::caution

Chain contexts represent a root of trust in the network, so before changing them
for production networks make sure you have verified them against a trusted
source like the [Mainnet] and [Testnet] chapters in the official Oasis
documentation.

:::

![code shell](../examples/network/04-list.in)

![code shell](../examples/network/04-list.out)

![code shell](../examples/network/05-set-chain-context-ctx.in)

![code shell](../examples/network/06-list.in)

![code shell](../examples/network/06-list.out)

To automatically detect the chain context, simply omit the `[chain-context]`
argument. This is especially useful for Localnet, where the chain context
changes each time you restart the `oasis-net-runner`:

![code shell](../examples/network/set-chain-context.in.static)

[Mainnet]: https://github.com/oasisprotocol/docs/blob/main/docs/node/mainnet/README.md
[Testnet]: https://github.com/oasisprotocol/docs/blob/main/docs/node/testnet/README.md

## Set Default Network {#set-default}

To change the default network for future Oasis CLI operations, use
`network set-default <name>`.

![code shell](../examples/network/02-list.in)

![code](../examples/network/02-list.out)

![code shell](../examples/network/03-set-default.in)

![code shell](../examples/network/04-list.in)

![code](../examples/network/04-list.out)

## Change a Network's RPC Endpoint {#set-rpc}

To change the RPC address of the already configured network, run
`network set-rpc <name> <new_endpoint>`:

![code shell](../examples/network-set-rpc/00-list.in)

![code](../examples/network-set-rpc/00-list.out)

![code shell](../examples/network-set-rpc/01-set-rpc.in)

![code shell](../examples/network-set-rpc/02-list.in)

![code](../examples/network-set-rpc/02-list.out)

## Advanced

### Governance Operations {#governance}

`network governance` command is aimed towards validators for proposing
or voting on-chain for network upgrades or changes to other crucial network
parameters.

#### `list` {#governance-list}

Use `network list` to view all past and still active governance proposals.
Each proposal has its unique subsequent ID, a submitter, an epoch when the
proposal was created and when it closes and a state.

![code shell](../examples/network-governance/list.in)

![code](../examples/network-governance/list.out)

:::info

[Network](./account.md#npa) selector is available for the
`governance list` command.

:::

#### `show` {#governance-show}

`network governance show <proposal-id>` shows detailed information on
past or opened governance proposals on the consensus layer.

![code shell](../examples/network-governance/show.in.static)

![code](../examples/network-governance/show.out.static)

You can also view individual validator votes by passing the `--show-votes`
parameter:

![code shell](../examples/network-governance/show-votes.in.static)

![code](../examples/network-governance/show-votes.out.static)

:::info

Governance proposals are not indexed and an endpoint may take some time to
respond. If you encounter timeouts, consider setting up your own gRPC endpoint!

:::

:::info

[Network](./account.md#npa) selector is available for the
`governance show` command.

:::

#### `cast-vote` {#governance-cast-vote}

`network governance cast-vote <proposal-id> { yes | no | abstain }` is used
to submit your vote on the governance proposal. The vote can either be `yes`,
`no` or `abstein`.

![code shell](../examples/network-governance/cast-vote.in.static)

![code](../examples/network-governance/cast-vote.out.static)

:::info

[Network and account](./account.md#npa) selectors are available for the
`governance cast-vote` command.

:::

#### `create-proposal` {#governance-create-proposal}

To submit a new governance proposal use `network governance create-proposal`.
The following proposal types are currently supported:

- `cancel-upgrade <proposal-id>`: Cancel network proposed upgrade. Provide the
  ID of the network upgrade proposal you wish to cancel.
- `parameter-change <module-name> <changes.json>`: Network parameter change
  proposal. Provide the consensus module name and the parameter changes JSON.
  Valid module names are: `staking`, `governance`, `keymanager`, `scheduler`,
  `registry`, and `roothash`
- `upgrade <descriptor.json>`: Network upgrade proposal. Provide a JSON file
  containing the upgrade descriptor.

:::info

[Network and account](./account.md#npa) selectors are available for all
`governance create-proposal` subcommands.

:::

### Show Network Properties {#show}

`network show` shows the network property stored in the registry, scheduler,
genesis document or on chain.

By passing `--height <block_number>` with a block number, you can obtain a
historic value of the property.

:::info

[Network](./account.md#npa) selector is available for the
`network show` command.

:::

The command expects one of the following parameters:

#### `entities` {#show-entities}

Shows all registered entities in the network registry. See the
[`account entity`] command, if you want to register or update your own entity.

[`account entity`]: ./account.md#entity

:::info

This call is not enabled on public Oasis gRPC endpoints. You will have to run
your own client node to enable this functionality.

:::

#### `nodes` {#show-nodes}

Shows all registered nodes in the network registry. See the [`account entity`],
to add a node to your entity.

:::info

This call is not enabled on public Oasis gRPC endpoints. You will have to run
your own client node to enable this functionality.

:::

#### `parameters` {#show-parameters}

Shows all consensus parameters for the following modules: consensus,
key manager, registry, roothash, staking, scheduler, beacon, and governance.

![code shell](../examples/network-show/parameters.in)

![code](../examples/network-show/parameters.out)

By passing `--format json`, the output is formatted as JSON.

#### `paratimes` {#show-paratimes}

Shows all registered ParaTimes in the network registry.

#### `validators` {#show-validators}

Shows all IDs of the nodes in the validator set.

#### `native-token` {#show-native-token}

Shows information of the network's native tokens such as its symbol, the number
of decimal points, total supply, debonding period and staking thresholds.

![code shell](../examples/network-show/native-token.in.static)

![code](../examples/network-show/native-token.out.static)

We can see that the token's name is ROSE and that 1 token corresponds to 10^9
(i.e. one billion) base units.

Next, we can observe that the **total supply** is 10 billion tokens and that
about 1.3 billion tokens are in the **common pool**.

The **staking thresholds** fields are the following:

- `entity`: The amount needed to be staked when registering an entity.
- `node-validator`, `node-compute`, `node-keymanager`: The amount needed to be
  staked to the corresponding entity for a node to run as a validator, a compute
  node or a key manager. This is the amount that will be slashed in case of
  inappropriate node behavior.
- `runtime-compute`, `runtime-keymanager`: The amount needed to be staked to an
  entity for [registering a new ParaTime or a key manager].
  Keep in mind that a ParaTime cannot be unregistered and there is no way of
  getting the staked assets back.

For example, if you wanted to register an entity running a validator and a
compute node, you would need to stake (i.e. *escrow*) at least 300 tokens.

:::info

Apart from the `node-compute` threshold above, a ParaTime may require additional
**ParaTime-specific escrow** for running a compute node. Use the
[`network show id`](#show-id) command to see it.

:::

[registering a new ParaTime or a key manager]: ./paratime.md#register

#### `gas-costs` {#show-gas-costs}

Shows minimum gas costs for each consensus transaction.

![code shell](../examples/network-show/gas-costs.in)

![code](../examples/network-show/gas-costs.out)

Above, we can see that the [maximum amount of gas](./account.md#gas-limit) our
transaction can spend must be set to at least 1000 **gas units**, otherwise it
will be rejected by the network.

#### `committees` {#committees}

Shows runtime committees.

![code shell](../examples/network-show/committees.in.static)

![code](../examples/network-show/committees.out.static)

#### `<id>` {#show-id}

The provided ID can be one of the following:

- If the [ParaTime ID] is provided, Oasis CLI shows ParaTime information stored
  in the network's registry.

  For example, at time of writing information on Sapphire stored in the Mainnet
  registry were as follows:

  ![code shell](../examples/network-show/id-paratime.in.static)

  ![code json](../examples/network-show/id-paratime.out.static)

  Network validators may be interested in the **ParaTime staking threshold**
  stored inside the `thresholds` field:

  ```shell
  oasis network show 000000000000000000000000000000000000000000000000f80306c9858e7279 | jq '.staking.thresholds."node-compute"'
  ```
  
  ```
  "5000000000000000"
  ```

  In the example above, the amount to run a Sapphire compute node on the Mainnet
  is 5,000,000 tokens and should be considered on top of the consensus-layer
  validator staking thresholds obtained by the
  [`network show native-token`](#show-native-token) command.

- If the entity ID is provided, Oasis CLI shows information on the entity and
  its corresponding nodes in the network registry. For example:

  ![code shell](../examples/network-show/id-entity.in)

  ![code json](../examples/network-show/id-entity.out)

- If the node ID is provided, Oasis CLI shows detailed information of the node
  such as the Oasis Core software version, the node's role, supported
  ParaTimes, trusted execution environment support and more. For example:

  ![code shell](../examples/network-show/id-node.in.static)

  ![code json](../examples/network-show/id-node.out.static)

[ParaTime ID]: https://github.com/oasisprotocol/oasis-core/blob/master/docs/runtime/identifiers.md

### Status of the Network's Endpoint {#status}

`network status` will connect to the gRPC endpoint and request extensive status
report from the Oasis Core node. Node operators will find important information
in the report such as:

- the last proposed consensus block,
- whether the node's storage is synchronized with the network,
- the Oasis Core software version,
- connected peers,
- similar information as above for each ParaTime, if the node is running it.

At time of writing, the following status of the official gRPC endpoint for
Mainnet was reported:

![code shell](../examples/network/status.in.static)

![code json](../examples/network/status.out.static)

By passing `--format json`, the output is formatted as JSON.

:::info

[Network](./account.md#npa) selector is available for the
`network status` command.

:::
