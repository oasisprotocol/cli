---
title: Account
description: Using CLI for performing account-related tasks
---

# Account-related Tasks

The `account` command is the home for most consensus and ParaTime-layer
on-chain transactions that are signed with one of your accounts such as:

- getting the account balance including delegated assets,
- sending tokens,
- delegating or undelegating tokens to or from validators (*staking*),
- depositing and withdrawing tokens to or from a ParaTime,
- managing withdrawal beneficiaries of your accounts,
- validator utils such as entity registration, setting the commission schedule,
  unfreezing your node and similar.

## Network, ParaTime and Account Selectors {#npa}

Before we dig into `account` subcommands, let's look at the three most common
selectors.

### Network

The `--network <network_name>` parameter specifies the [network] which the
Oasis CLI should connect to.

For example:

![code shell](../examples/account/show-testnet.in)

![code](../examples/account/show-testnet.out)

![code shell](../examples/account/show-mainnet.in)

![code](../examples/account/show-mainnet.out)

### ParaTime

The `--paratime <paratime_name>` sets which [ParaTime] Oasis CLI should use.
If you do not want to use any ParaTime, for example to perform a consensus
layer operation, pass the `--no-paratime` flag explicitly.

![code shell](../examples/account/show-no-paratime.in)

![code](../examples/account/show-no-paratime.out)

### Account

The `--account <account_name>` specifies which account in your wallet the
Oasis CLI should use to sign the transaction with.

![code shell](../examples/account/transfer-eth.y.in)

![code](../examples/account/transfer-eth.y.out)

![code shell](../examples/account/transfer-eth2.y.in)

![code](../examples/account/transfer-eth2.y.out)

:::tip

You can also set **the default [network][network-set-default],
[ParaTime][paratime-set-default] or [account][wallet-set-default] to use**, if
no network, ParaTime or account selectors are provided.

:::

[network]: ./network.md
[paratime]: ./paratime.md
[network-set-default]: ./network.md#set-default
[paratime-set-default]: ./paratime.md#set-default
[wallet-set-default]: ./wallet.md#set-default

## Show the Balance of an Account {#show}

The `account show [address]` command prints the balance, delegated assets
and other validator information corresponding to:

- a given address,
- the name of the [address book entry] or
- the name of one of the accounts in your wallet.

The address is looked up both on the consensus layer and the ParaTime, if
selected.

Running the command without arguments will show you the balance
of your default account on the default network and ParaTime:

![code shell](../examples/account/show.in)

![code](../examples/account/show.out)

You can also pass the name of the account in your wallet or address book, or one
of the [built-in named addresses](#reserved-addresses):

![code shell](../examples/account/show-named.in)

![code](../examples/account/show-named.out)

![code shell](../examples/account/show-named-pool.in)

![code](../examples/account/show-named-pool.out)

Or, you can check the balance of an arbitrary account address by passing the
native or Ethereum-compatible addresses.

![code shell](../examples/account/show-oasis.in)

![code](../examples/account/show-oasis.out)

![code shell](../examples/account/show-eth.in)

![code](../examples/account/show-eth.out)

To also include any staked assets in the balance, pass the `--show-delegations`
flag. For example:

![code shell](../examples/account/show-delegations.in.static)

![code](../examples/account/show-delegations.out.static)

Let's look more closely at the figures above. The account's **nonce** is the
incremental number starting from 0 that must be unique for each account's
transaction. In our case, the nonce is 32. This means there have been that many
transactions made with this account as the source. The next transaction should
have nonce equal to 32.

We can see that the total account's **balance** on the consensus layer is \~973
tokens:

- \~951 tokens can immediately be transferred.
- \~16.3 tokens (15,000,000,0000 shares) are staked (delegated).
- \~5.4 tokens are debonding and will be available for spending in the epoch
  26558.
- up to \~270 tokens are [allowed](#allow) to be transferred to accounts
  `oasis1qqczuf3x6glkgjuf0xgtcpjjw95r3crf7y2323xd` and
  `oasis1qrydpazemvuwtnp3efm7vmfvg3tde044qg6cxwzx` without the signature of the
  account above.

Separately, you can notice there are \~7 tokens currently [deposited](#deposit)
in Sapphire.

:::info

The `--show-delegations` flag is not enabled by default, because account
delegations are not indexed on-chain. This means that the endpoint needs
to scan block by block to retrieve this information and takes some time
often leading to the timeout on public endpoints due to denial-of-service
protection.

:::

Next, let's look at how the account of a validator typically looks like. For
example:

![code shell](../examples/account/show-delegations-validator.in.static)

![code](../examples/account/show-delegations-validator.out.static)

We can see there is a total of \~1833 tokens delegated to this validator. One
delegation was done by the account itself and then there are five more
delegators. Sometimes, we also refer to accounts with delegated assets to it as
*escrow accounts*.

Next, we can see a *commission schedule*. A validator can charge commission for
tokens that are delegated to it in form of the commission schedule **rate
steps** (7%, 11%, 14% and 18% activated on epochs 15883, 15994, 16000 and 16134
respectively) and the commission schedule **rate bound steps** (0-10% on
epoch 15883 and then 0-20% activated on epoch 15993). For more details, see the
[account amend-commission-schedule](./account#amend-commission-schedule)
command.

An escrow account may also accumulate one or more **stake claims** as seen
above. The network ensures that all claims are satisfied at any given point.
Adding a new claim is only possible if **all of the existing claims plus the
new claim can be satisfied**.

We can observe that the stake accumulator currently has the following claims:

- The `registry.RegisterEntity` claim is for registering an entity. It needs to
  satisfy the global threshold for
  [registering the `entity`][show-native-token].

- The `registry.RegisterNode.LAdHWnCkjFR5NUkFHVpfGuKFfZW1Cqjzu6wTFY6v2JI=`
  claim is for registering the validator node with the public key
  `LAdHWnCkjFR5NUkFHVpfGuKFfZW1Cqjzu6wTFY6v2JI=`. The claim needs to satisfy the
  [`node-validator`][show-native-token] global staking threshold parameter.

- The `registry.RegisterNode.xk58fx5ys6CSO33ngMQkgOL5UUHSgOSt0QbqWGGuEF8=`
  claim is for registering the three compute nodes with the public key
  `xk58fx5ys6CSO33ngMQkgOL5UUHSgOSt0QbqWGGuEF8==`. The claim needs to satisfy
  three [`node-compute`][show-native-token] global staking threshold parameters.

For more details on registering entities, nodes and ParaTimes, see the
[Oasis Core Registry service][oasis-core-registry].

[address book entry]: ./addressbook.md
[show-native-token]: ./network#show-native-token

:::info

[Network and ParaTime](#npa) selectors are available for the
`account show` command.

:::

## Transfer {#transfer}

Use `account transfer <amount> <to>` command to transfer funds between two
accounts on the consensus layer or between two accounts inside the same
ParaTime.

The following command will perform a token transfer inside default ParaTime:

![code shell](../examples/account/transfer-named.y.in)

![code](../examples/account/transfer-named.y.out)

Consensus layer token transfers:

![code shell](../examples/account/transfer-named-no-paratime.y.in)

![code](../examples/account/transfer-named-no-paratime.y.out)

:::info

[Network, ParaTime and account](#npa) selectors are available for the
`account transfer` command.

:::

:::info

The [`--subtract-fee`](#subtract-fee) flag is available both for consensus
and ParaTime transfers.

:::

## Allowance {#allow}

`account allow <beneficiary> <amount>` command makes your funds withdrawable by
a 3rd party beneficiary at consensus layer. For example, instead of paying your
partner for a service directly, you can ask for their address and enable
**them** to withdraw the amount which you agreed on from your account. This is a
similar mechanism to how payment checks were used in the past.

![code shell](../examples/account/allow.y.in)

![code](../examples/account/allow.y.out)

The allowance command uses relative amount. For example, if your run the
above command 3 times, Logan will be allowed to withdraw 30 ROSE.

:::tip

To reduce the allowed amount or completely **disallow** the withdrawal, use the
negative amount. To avoid flag ambiguity in the shell, you will first need to
pass all desired flags and parameters except the negative amount, then append
`--` to mark the end of options, and finally append the negative amount.

![code shell](../examples/account/allow-negative.in.static)

![code](../examples/account/allow-negative.out.static)

:::

The allowance transaction is also required if you want to deposit funds from
your consensus account to a ParaTime. The ParaTime will **withdraw** the amount
from your consensus account and fund your ParaTime account with the same
amount deducted by the deposit fee. Oasis CLI can derive the address of the
ParaTime beneficiary, if you use `paratime:<paratime name>` as the beneficiary
address.

![code shell](../examples/account/allow-paratime.y.in)

![code](../examples/account/allow-paratime.y.out)

:::info

[Network and account](#npa) selectors are available for the `account allow`
command.

:::

## Deposit Tokens to a ParaTime {#deposit}

`account deposit <amount> [address]` will deposit funds from your consensus
account to the target address inside the selected ParaTime.

![code shell](../examples/account/deposit-named.y.in)

![code](../examples/account/deposit-named.y.out)

If no address is provided, the deposit will be made to the address
corresponding to your consensus account inside the ParaTime.

![code shell](../examples/account/deposit.y.in)

![code](../examples/account/deposit.y.out)

Currently, deposit transactions are free of charge, hence the `--gas-price 0`
parameter to avoid spending unnecessary gas fees. Also, keep in
mind that **deposit and withdrawal fees are always paid by your ParaTime
account.** If it doesn't contain any ROSE, you will not able to cover the fees.

You can also make a deposit to an account with arbitrary address inside a
ParaTime. For example, let's deposit to some native address inside the
ParaTime:

![code shell](../examples/account/deposit-oasis.y.in)

![code](../examples/account/deposit-oasis.y.out)

Or to some address in the Ethereum format:

![code shell](../examples/account/deposit-eth.y.in)

![code](../examples/account/deposit-eth.y.out)

:::info

[Network, ParaTime and account](#npa) selectors are available for the
`account deposit` command.

:::

## Withdraw Tokens from the ParaTime {#withdraw}

`account withdraw <amount> [to]` will withdraw funds from your ParaTime account
to a consensus address:

![code shell](../examples/account/withdraw-named.y.in)

![code](../examples/account/withdraw-named.y.out)

If the address is not provided, the address of the account inside ParaTime will
be used as a consensus address:

![code shell](../examples/account/withdraw.y.in)

![code](../examples/account/withdraw.y.out)

:::caution

Withdrawal transactions are not free of charge and the fee will be deducted
**from your ParaTime balance**.

:::

Similar to the [`account deposit`](#deposit) command, you can also specify an
arbitrary Oasis address which you want to withdraw your tokens to.

![code shell](../examples/account/withdraw-oasis.y.in)

![code](../examples/account/withdraw-oasis.y.out)

:::caution

You cannot use the destination address of your `secp256k1` account or any other
Ethereum-formatted address for the withdrawal, because this signature scheme is
not supported on the consensus layer!

:::

:::info

[Network, ParaTime and account](#npa) selectors are available for the
`account withdraw` command.

:::

:::info

The [`--subtract-fee`](#subtract-fee) flag is available for withdrawal
transactions.

:::

## Delegate Tokens to a Validator {#delegate}

To stake your tokens on the consensus layer, run
`account delegate <amount> <to>`. This will delegate the specified amount of
tokens to a validator.

You can either delegate directly on the consensus layer:

![code shell](../examples/account/delegate.y.in)

![code](../examples/account/delegate.y.out)

Or you can delegate from inside a ParaTime that supports delegations:

![code shell](../examples/account/delegate-paratime.y.in)

![code](../examples/account/delegate-paratime.y.out)

Once your tokens are staked, they are converted into *shares* since the number
of tokens may change over time based on the
[staking reward schedule][token-metrics] or if your validator is subject to
[slashing]. The number of shares on the other hand will remain constant. Also,
shares are always interpreted as a whole number, whereas the amount of tokens is
usually a rational number and may lead to rounding errors when managing your
delegations.

To find out how many shares did you delegate, run [`account show`](#show) and
look for the `shares` under the active delegations section.

:::info

[Network, ParaTime and account](#npa) selectors are available for the
`account delegate` command.

:::

[token-metrics]: https://github.com/oasisprotocol/docs/blob/main/docs/general/oasis-network/token-metrics-and-distribution.mdx#staking-incentives
[slashing]: https://github.com/oasisprotocol/docs/blob/main/docs/general/manage-tokens/terminology.md#slashing

## Undelegate Tokens from the Validator {#undelegate}

To reclaim your delegated assets, use `account undelegate <shares> <from>`. You
will need to specify the **number of shares instead of tokens** and the
validator address you want to reclaim your assets from.

Depending on where the tokens have been delegated from, you can either reclaim
delegated tokens directly on the consensus layer:

![code shell](../examples/account/undelegate.y.in)

![code](../examples/account/undelegate.y.out)

Or you can reclaim from inside a ParaTime that supports delegations:

![code shell](../examples/account/undelegate-paratime.y.in)

![code](../examples/account/undelegate-paratime.y.out)

After submitting the transaction, a [debonding period] will
commence. After the period has passed, the network will automatically move your
assets back to your account. Note that during the debonding period, your
assets may still be [slashed][slashing].

:::info

[Network, ParaTime and account](#npa) selectors are available for the
`account undelegate` command.

:::

[debonding period]: ./network.md#show

## Advanced

### Public Key to Address {#from-public-key}

`account from-public-key <public_key>` converts the Base64-encoded public key
to the [Oasis native address](../terminology.md#address).

![code shell](../examples/account/from-public-key.in)

![code](../examples/account/from-public-key.out)

This command is most often used by the network validators for converting the
public key of their entity to a corresponding address. You can find your
entity's ID in the `id` field of the `entity.json` file.

:::tip

Oasis consensus transactions hold the public key of the signer instead of their
*from* address. This command can be used for debugging to determine the
signer's staking address on the network.

:::

### Non-Interactive Mode {#y}

Add `-y` flag to any operation, if you want to use Oasis CLI in
non-interactive mode. This will answer "yes to all" for yes/no questions
and for all other prompts it will keep the proposed default values.

### Output Transaction to File {#output-file}

Use `--output-file <filename>` parameter to save the resulting transaction to a
file instead of broadcasting it to the network. You can then use the
[`transaction`] command to verify and submit it.

Check out the [`--unsigned`] flag, if you wish to store the unsigned version of
the transaction and the [`--format`] parameter for a different transaction
encoding.

[`transaction`]: ./transaction.md
[`--unsigned`]: #unsigned
[`--format`]: #format

### Do Not Sign the Transaction {#unsigned}

If you wish to *prepare* a transaction to be signed by a specific account in
the future, use the `--unsigned` flag. This will cause Oasis CLI to skip the
signing and broadcasting steps. The transaction will be printed to the
standard output instead.

You can also use [`--output-file`] to store the transaction to a file. This
setup is ideal when you want to sign a transaction with the
[offline/air-gapped machine] machine:

1. First, generate an unsigned transaction on a networked machine,
2. copy it over to an air-gapped machine,
3. [sign it][transaction-sign] on the air-gapped machine,
4. copy it over to the networked machine,
5. [broadcast the transaction][transaction-submit] on the networked machine.

Use the CBOR format, if you are using a 3rd party tool in step 3 to sign the
transaction content directly. Check out the [`--format`] parameter to learn
more.

[`--output-file`]: #output-file
[transaction-sign]: ./transaction.md#sign
[transaction-submit]: ./transaction.md#submit
[offline/air-gapped machine]: https://en.wikipedia.org/wiki/Air_gap_\(networking\)

### Output format {#format}

Use `--format json` or `--format cbor` to select the output file
format. By default the JSON encoding is selected so that the file is
human-readable and that 3rd party applications can easily manage it. If you want
to output the transaction in the same format that will be stored on-chain or you
are using a 3rd party tool for signing the content of the transaction file
directly use the CBOR encoding.

This parameter only works together with [`--unsigned`] and/or
[`--output-file`] parameters.

### Offline Mode {#offline}

To generate a transaction without accessing the network and also without
broadcasting it, add `--offline` flag. In this case Oasis CLI will require that
you provide all necessary transaction details (e.g. [account nonce](#nonce),
[gas limit](#gas-limit), [gas price](#gas-price)) which would otherwise be
automatically obtained from the network. Oasis CLI will print the transaction to
the standard output for you to examine. Use [`--output-file`](#output-file), if
you wish to save the transaction to the file and submit it to the network
afterwards by using the [`transaction submit`][transaction-submit] command.

### Subtract fee {#subtract-fee}

To include the transaction fee inside the given amount, pass the
`--subtract-fee` flag. This comes handy, if you want to drain the account or
keep it rounded to some specific number.

![code shell](../examples/account/transfer-subtract-fee.y.in)

![code shell](../examples/account/transfer-subtract-fee.y.out)

### Account's Nonce {#nonce}

`--nonce <nonce_number>` will override the detection of the account's nonce used
to sign the transaction with the specified one.

### Gas Price {#gas-price}

`--gas-price <price_in_base_units>` sets the transaction's price per gas unit in
base units.

### Gas Limit {#gas-limit}

`--gas-limit <limit>` sets the maximum amount of gas that can be spend by the
transaction.

### Entity Management {#entity}

#### Initialize Entity {#entity-init}

When setting up a validator node for the first time, you will need to provide
the path to the file containing your entity descriptor as well as register it in
the network registry. Use `account entity init` to generate the entity
descriptor file containing the public key of the selected account.

![code shell](../examples/account/entity-init.in.static)

![code json](../examples/account/entity-init.out.static)

By default, the file content will be printed to the standard output. You can use
`-o` parameter to store it to a file, for example:

![code shell](../examples/account/entity-init-o.y.in)

:::info

[Account](#account) selector is available for the
`account entity init` command.

:::

#### Register your Entity {#entity-register}

In order for validators to become part of the validator set and/or the compute
committee, they first need to register as an entity inside the network's
registry. Use the `account entity register <entity.json>` command to register
your entity and provide a JSON file with the Entity descriptor. You can use the
[`network show`][network-show] command to see existing entities and
then examine specific ones to see how entity descriptors of the currently
registered entities look like.

[network-show]: ./network.md#show
[oasis-core-registry]: https://github.com/oasisprotocol/oasis-core/blob/master/docs/consensus/services/registry.md#entities-and-nodes

![code shell](../examples/account/entity-register.y.in)

![code](../examples/account/entity-register.y.out)

:::info

[Network and account](#npa) selectors are available for the
`account entity register` command.

:::

#### Deregister Your Entity {#entity-deregister}

To remove an entity from the network's registry, invoke
`account entity deregister`. No additional arguments are required since each
account can only deregister their own entity, if one exists in the registry.

![code shell](../examples/account/entity-deregister.y.in)

![code](../examples/account/entity-deregister.y.out)

:::info

[Network and account](#npa) selectors are available for the
`account entity deregister` command.

:::

### Change Your Commission Schedule {#amend-commission-schedule}

Validators can use `account amend-commission-schedule` to add or remove
their commission bounds and rates at consensus layer. Rate bounds can be
defined by using the `--bounds <epoch>/<min_rate>/<max_rate>` parameter.
Actual rates which can be subject to change every epoch can be defined with the
`--rates <epoch>/<rate>` parameter. Rates are specified in milipercents
(100% = 100000m%). The new commission schedule will replace any previous
schedules.

![code shell](../examples/account/amend-commission-schedule.y.in)

![code](../examples/account/amend-commission-schedule.y.out)

To learn more on commission rates read the  section inside the Oasis Core
[Staking service][staking-service-commission-schedule] chapter.

:::info

[Network and account](#npa) selectors are available for the
`account amend-commission-schedule` command.

:::

[staking-service-commission-schedule]: https://github.com/oasisprotocol/oasis-core/blob/master/docs/consensus/services/staking.md#amend-commission-schedule

### Unfreeze Your Node {#node-unfreeze}

Once the validators, based on their stake, get elected into the validator set,
it is important that their nodes are actively participating in proposing new
blocks and submitting votes for other proposed blocks. For regular node
upgrades and maintenance, the validators should follow the
[Shutting Down a Node] instructions. Nevertheless, if the network froze your
node, the only way to unfreeze it is to execute the `account node-unfreeze`

![code shell](../examples/account/node-unfreeze.y.in)

![code](../examples/account/node-unfreeze.y.out)

:::info

[Network and account](#npa) selectors are available for the
`account node-unfreeze` command.

:::

[Shutting Down a Node]: https://github.com/oasisprotocol/docs/blob/main/docs/node/run-your-node/maintenance/shutting-down-a-node.md

### Burn Tokens {#burn}

`account burn <amount>` command will permanently destroy the amount of tokens
in your account and remove them from circulation. This command should not be
used on public networks since not only no one will be able to access burnt
assets anymore, but will also permanently remove the tokens from circulation.

![code shell](../examples/account/burn.y.in)

![code](../examples/account/burn.y.out)

:::info

[Network and account](#npa) selectors are available for the `account burn`
command.

:::

### Pools and Reserved Addresses {#reserved-addresses}

The following literals are used in the Oasis CLI to denote special reserved
addresses which cannot be directly used in the ledger:

#### Consensus layer

- `pool:consensus:burn`: The token burn address.
- `pool:consensus:common`: The common pool address.
- `pool:consensus:fee-accumulator`: The per-block fee accumulator address.
- `pool:consensus:governance-deposits`: The governance deposits address.

#### ParaTime layer

- `pool:paratime:common`: The common pool address.
- `pool:paratime:fee-accumulator`: The per-block fee accumulator address.
- `pool:paratime:pending-withdrawal`: The internal pending withdrawal address.
- `pool:paratime:pending-delegation`: The internal pending delegation address.
- `pool:paratime:rewards`: The reward pool address.
