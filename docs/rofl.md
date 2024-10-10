---
title: ROFL
description: Manage "Runtime OFfchain Logic" apps
---

# Manage ROFL Apps

The `rofl` command combines a series of actions for managing the [Runtime
OFfchain Logic (ROFL)][rofl] apps:

- build ROFL locally,
- verify the ROFL bundle,
- register, deregister and update ROFL apps on the network,
- show information about the registered ROFL apps,
- other convenient tooling for ROFL app developers.

[rofl]: https://github.com/oasisprotocol/docs/blob/main/docs/rofl/README.mdx

## Build ROFL {#build}

The `build` command will execute a series of build commands depending on the
target Trusted Execution Environment (TEE) and produce the Oasis Runtime
Container (ORC) bundle.

Building a ROFL bundle requires the [Network and ParaTime][npa] selectors.
Additionally, the following flags are available:

- `--mode` specifies a `production` (enabled SGX attestations suitable for the
  Mainnet and Testnet) or `unsafe` build (using mocked SGX for debugging
  and testing). The default behavior is set to `auto` which, based on the
  selected [Network and ParaTime][npa], determines the build mode.
- `--output` the filename of the output ORC bundle. Defaults to the package name
  inside `Cargo.toml` and the `.orc` extension.

:::info

Building ROFL apps involves **cross compilation**, so you do not need a working
TEE on your machine. However, you do need to install all corresponding compilers
and toolchains. Check out the [ROFL Prerequisites] chapter for details.

:::

[ROFL Prerequisites]: https://github.com/oasisprotocol/oasis-sdk/blob/main/docs/rofl/prerequisites.md
[npa]: ./account.md#npa

### For SGX {#build-sgx}

To build an SGX-based ROFL for the default [Network and ParaTime][npa], run
`build sgx`.

![code shell](../examples/rofl/build-sgx.in.static)

If you want to build a version for debugging and you're not running a [Sapphire
Localnet] at build time, you can force the *unsafe* build mode:

![code shell](../examples/rofl/build-sgx-unsafe.in.static)

The following SGX-specific flags are supported:

- `--sgx-heap-size` is the heap size in bytes.
- `--sgx-stack-size` is the stack size in bytes.
- `--sgx-threads` is the maximum number of threads that an enclave can spawn.
  This number must be at least the number of threads required by the [Oasis
  Core runtime], otherwise the app may crash during the execution under actual
  SGX. It can be greater, if the app needs to spawn more threads.

[Sapphire Localnet]: https://github.com/oasisprotocol/sapphire-paratime/blob/main/docs/guide.mdx#running-a-private-oasis-network-locally
[Oasis Core runtime]: https://github.com/oasisprotocol/oasis-core/blob/master/runtime/THREADS.md

<!-- TODO
### For TDX {#build-tdx}

To build an TDX container for the default [Network and ParaTime][npa], run
`build tdx`.

-->

## Show ROFL identity {#identity}

Run `rofl identity` to compute the **cryptographic identity** of the ROFL app:

![code shell](../examples/rofl/identity.in.static)

![code](../examples/rofl/identity.out.static)

The output above is Base64-encoded enclave identity which depends on the ROFL
source code and the build environment. Enclave identities should be reproducible
on any computer and are used to prove and verify the integrity of ROFL binaries
on the network. See the [Reproducibility] chapter to learn more.

[Reproducibility]: https://github.com/oasisprotocol/oasis-sdk/blob/main/docs/runtime/reproducibility.md

## Create a new ROFL app on the network {#create}

Use `rofl create` to register a new ROFL app on the network using a
specific [policy] file:

![code shell](../examples/rofl/create.in.static)

![code](../examples/rofl/create.out.static)

Returned is the unique ROFL app ID starting with `rofl1` and which you
will refer to for managing your ROFL app in the future.

:::info

In order to prevent spam attacks registering a ROFL app requires a
certain amount to be deposited from your account until you decide to
[remove it](#remove). The deposit remains locked for the lifetime of the app.
Check out the [ROFL chapter][policy] to view the current staking requirements.

:::

You can also define specific [Network, ParaTime and Account][npa] parameters:

![code shell](../examples/rofl/create-npa.in.static)

With the `--scheme` parameter, you can select one of the following ROFL app ID
derivation schemes:

- `cn` for the ROFL app creator address (the account you're using to sign the
  transaction) combined with the account's nonce (default). This behavior is
  similar to the one of the Ethereum [smart contract address derivation] and is
  deterministic.
- `cri` uses the ROFL app creator address combined with the block round the
  transaction will be validated in and its position inside that block.

[policy]: https://github.com/oasisprotocol/oasis-sdk/blob/main/docs/rofl/deployment.md#register-the-app
[smart contract address derivation]: https://ethereum.org/en/developers/docs/accounts/#contract-accounts

## Update ROFL policy {#update}

Use `rofl update` command to set the new policy and the new administrator of the
ROFL app:

![code shell](../examples/rofl/update.in.static)

![code shell](../examples/rofl/update.out.static)

For the administrator, you can also specify an account name in your wallet or
address book.

To keep the existing administrator, pass `self`:

![code shell](../examples/rofl/update-self.in.static)

You can also define specific [Network, ParaTime and Account][npa] parameters:

![code shell](../examples/rofl/update-npa.in.static)

## Remove ROFL app from the network {#remove}

Run `rofl remove` to deregister your ROFL app:

![code shell](../examples/rofl/remove.in.static)

![code](../examples/rofl/remove.out.static)

The deposit required to register the ROFL app will be returned to the current
administrator account.

You can also define specific [Network, ParaTime and Account][npa] parameters:

![code shell](../examples/rofl/remove-npa.in.static)

## Show ROFL information {#show}

Run `rofl show` to obtain the information from the network on the ROFL admin
account, staked amount, current ROFL policy and running instances:

![code shell](../examples/rofl/show.in.static)

![code](../examples/rofl/show.out.static)

You can also define specific [Network and ParaTime][npa] parameters:

![code shell](../examples/rofl/show-np.in.static)

## Advanced

### Show the current trust-root {#trust-root}

In order the ROFL app can trust the environment it is executed in, it
needs to have a hardcoded *trust root*. Typically, it consists of:

- the [ParaTime ID],
- the [chain domain separation context],
- the specific consensus block hash and its height.

To obtain the latest trust root in rust programming language, run
`oasis rofl trust-root`:

![code shell](../examples/rofl/trust-root.in.static)

![code](../examples/rofl/trust-root.out.static)

You can also define specific [Network and ParaTime][npa] parameters:

![code shell](../examples/rofl/trust-root-np.in.static)

[ParaTime ID]: https://github.com/oasisprotocol/oasis-core/blob/master/docs/runtime/identifiers.md
[chain domain separation context]: https://github.com/oasisprotocol/oasis-core/blob/master/docs/crypto.md#chain-domain-separation
