---
title: ParaTime
description: Managing ParaTimes
---

# Managing Your ParaTimes

The `paratime` command lets you manage your ParaTime configurations bound to a
specific [network]. If you are a ParaTime developer, the command allows you to
register a new ParaTime into the public network's registry. The command
also supports examining specific block and a transaction inside the ParaTime
and printing different validator-related statistics.

:::tip

When running the Oasis CLI for the first time, it will automatically configure
official Oasis ParaTimes running on the [Mainnet] and [Testnet] networks.

:::

## Add a ParaTime {#add}

Invoke `paratime add <network> <name> <id>` to add a new ParaTime to your Oasis
CLI configuration. Beside the name of the corresponding network and the unique
ParaTime name inside that network, you will also need to provide the
[ParaTime ID]. This is a unique identifier of the ParaTime on the network, and
it remains the same even when the network and ParaTime upgrades occur. You can
always check the IDs of the official Oasis ParaTimes on the respective
[Mainnet] and [Testnet] pages.

Each ParaTime also has a native token denomination symbol defined with specific
number of decimal places which you will need to specify.

```shell
oasis paratime add testnet sapphire2 000000000000000000000000000000000000000000000000a6d1e3ebf60dff6d
```

```
? Description:
? Denomination symbol: TEST
? Denomination decimal places: 18
```

You can also enable [non-interactive mode](account.md#y) and pass
`--num-decimals`, `--symbol` and `--description` parameters directly:

```shell
oasis paratime add testnet sapphire2 000000000000000000000000000000000000000000000000a6d1e3ebf60dff6d --num-decimals 18 --symbol TEST --description "Testnet Sapphire 2" -y
```

:::danger Decimal places of the native and ParaTime token may differ!

Emerald and Sapphire use **18 decimals** for compatibility with
Ethereum tooling. The Oasis Mainnet and Testnet consensus layer tokens and the
token native to Cipher have **9 decimals**.

Configuring the wrong number of decimal places will lead to incorrect amount
of tokens to be deposited, withdrawn or transferred from or into the ParaTime!

:::

:::tip

If you configured your network with the [`network add-local`] command, then all
registered ParaTimes of that network will be detected and added to your Oasis
CLI config automatically.

:::

[network]: ./network.md
[`network add-local`]: ./network.md#add-local
[ParaTime ID]: https://github.com/oasisprotocol/oasis-core/blob/master/docs/runtime/identifiers.md
[Mainnet]: https://github.com/oasisprotocol/docs/blob/main/docs/node/mainnet/README.md
[Testnet]: https://github.com/oasisprotocol/docs/blob/main/docs/node/testnet/README.md

## List ParaTimes {#list}

Invoke `paratime list` to list all configured ParaTimes across the networks.

For example, at time of writing this section the following ParaTimes were
preconfigured by the Oasis CLI:

![code shell](../examples/paratime/00-list.in)

![code](../examples/paratime/00-list.out)

The [default ParaTime](#set-default) for each network is marked with the `(*)`
sign.

:::info

ParaTimes on this list are configured inside your Oasis CLI instance. They
may not actually exist on the network.

:::

## Remove a ParaTime {#remove}

To remove a configuration of a ParaTime for a specific network, use
`paratime remove <network> <name>`. For example, let's remove the
[previously added](#add) ParaTime:

![code shell](../examples/paratime-remove/00-list.in)

![code](../examples/paratime-remove/00-list.out)

![code shell](../examples/paratime-remove/01-remove.in)

![code shell](../examples/paratime-remove/02-list.in)

![code](../examples/paratime-remove/02-list.out)

## Set Default ParaTime {#set-default}

To change the default ParaTime for Oasis CLI transactions on the specific
network, use `paratime set-default <network> <name>`.

For example, to set the Cipher ParaTime default on the Testnet, run:

![code shell](../examples/paratime/01-set-default.in)

![code shell](../examples/paratime/02-list.in)

![code](../examples/paratime/02-list.out)

## Show {#show}

Use `paratime show` to investigate a specific ParaTime block or other
parameters.

### `<round>` {#show-round}

Providing the block round or `latest` literal will print its header and other
information.

![code shell](../examples/paratime-show/show.in.static)

![code](../examples/paratime-show/show.out.static)

To show the details of the transaction stored inside the block including the
transaction status and any emitted events, pass the transaction index in the
block or its hash:

![code shell](../examples/paratime-show/show-tx.in.static)

![code](../examples/paratime-show/show-tx.out.static)

Encrypted transactions can also be examined, although the data chunk will be
encrypted:

![code shell](../examples/paratime-show/show-tx-encrypted.in.static)

![code](../examples/paratime-show/show-tx-encrypted.out.static)

### `parameters` {#show-parameters}

This will print various ParaTime-specific parameters such as the ROFL stake
thresholds.

![code shell](../examples/paratime-show/show-parameters.in)

![code](../examples/paratime-show/show-parameters.out)

By passing `--format json`, the output is formatted as JSON.

### `events` {#show-events}

This will return all Paratime events emitted in the block.

Use `--round <round>` to specify the round number.

![code shell](../examples/paratime-show/show-events.in)

![code](../examples/paratime-show/show-events.out)

By passing `--format json`, the output is formatted as JSON.

## Set information about a denomination {#denom-set}

To set information about a denomination on the specific network and paratime use
`paratime denom set <network> <paratime> <denomination> <number_of_decimals>
--symbol <symbol>`. To use this command a denomination must already exist in the
actual paratime.

![code shell](../examples/paratime-denom/00-denom-set.in)

## Set information about the native denomination {#denom-set-native}

To set information about the native denomination on the specific network and
paratime use `paratime denom set-native <network> <paratime> <denomination>
<number_of_decimals>`.

The native denomination is already mandatory in the [`paratime add`](#add)
command.

![code shell](../examples/paratime-denom/01-denom-set-native.in)

## Remove denomination {#denom-remove}

To remove an existing denomination on the specific network and paratime use
`paratime denom remove <network> <paratime> <denomination>`.

The native denomination cannot be removed.

![code shell](../examples/paratime-denom/02-denom-remove.in)

## Advanced

### Register a New ParaTime {#register}

ParaTime developers may add a new ParaTime to the network's registry by
invoking the `paratime register <desc.json>` command and providing a JSON file
with the ParaTime descriptor. You can use the
[`network show`][network-show-id] command passing the ParaTime ID to
see how descriptors of the currently registered ParaTimes look like.

To learn more about registering your own ParaTime, check the
[Oasis Core Registry service].

[network-show-id]: ./network.md#show-id
[Oasis Core Registry service]: https://github.com/oasisprotocol/oasis-core/blob/master/docs/consensus/services/registry.md#register-runtime

### Statistics {#statistics}

`paratime statistics [<start-round> [<end-round>]]` will examine the voting
details for a range of blocks. First, it will print you aggregated statistics
showing you the number of successful rounds in that range, epoch transitions
and also anomalies such as the proposer timeouts, failed rounds and
discrepancies. Then, it will print out detailed validator per-entity
statistics for that range of blocks.

The passed block number should be enumerated based on the round
inside the ParaTime. The start round can be one of the following:

- If no round given, the validation of the last block will be examined.
- If a negative round number `N` is passed, the last `N` blocks will be
  examined.
- If `0` is given, the oldest block available to the Oasis endpoint will be
  considered as a starting block.
- A positive number will be considered as a start round.

At time of writing, the following statistics was available:

![code shell](../examples/paratime/statistics.in.static)

![code](../examples/paratime/statistics.out.static)

To extend statistics to, say 5 last blocks, you can run:

![code shell](../examples/paratime/statistics-negative.in.static)

![code](../examples/paratime/statistics-negative.out.static)

For further analysis, you can easily export entity statistics to a CSV file by
passing the `--output-file` parameter and the file name:

```shell
oasis paratime statistics -o stats.csv
```

:::info

The analysis of the range of blocks may require some time or even occasionally
fail due to denial-of-service protection. If you encounter such issues,
consider setting up your own gRPC endpoint!

:::
