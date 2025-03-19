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

[rofl]: https://github.com/oasisprotocol/docs/blob/main/docs/build/rofl/README.mdx

## Initialize a new ROFL app manifest {#init}

The `rofl init` command will prepare a new ROFL app manifest in the given
directory (defaults to the current directory). The manifest is a YAML file named
`rofl.yaml` which defines the versions of all components, upgrade policies, etc.
needed to manage, build and deploy the ROFL app.

![code shell](../examples/rofl/init.in.static)

![code](../examples/rofl/init.out.static)

Note that by default the manifest will not contain any deployments. In order to
create deployments, use `rofl create`.

## Create a new ROFL app on the network {#create}

Use `rofl create` to register a new ROFL app on the network using an existing
manifest.

You can also define specific [Network, ParaTime and Account][npa] parameters
as those get recorded into the manfiest so you don't need to specify them on
each invocation:

![code shell](../examples/rofl/create.in.static)

![code](../examples/rofl/create.out.static)

Returned is the unique ROFL app ID starting with `rofl1` and which you
will refer to for managing your ROFL app in the future. The manifest is
automatically updated with the newly assigned app identifier.

:::info

In order to prevent spam attacks registering a ROFL app requires a
certain amount to be deposited from your account until you decide to
[remove it](#remove). The deposit remains locked for the lifetime of the app.
Check out the [ROFL chapter][app] to view the current staking requirements.

:::

With the `--scheme` parameter, you can select one of the following ROFL app ID
derivation schemes:

- `cn` for the ROFL app creator address (the account you're using to sign the
  transaction) combined with the account's nonce (default). This behavior is
  similar to the one of the Ethereum [smart contract address derivation] and is
  deterministic.
- `cri` uses the ROFL app creator address combined with the block round the
  transaction will be validated in and its position inside that block.

[app]: https://github.com/oasisprotocol/oasis-sdk/blob/main/docs/rofl/app.mdx
[smart contract address derivation]: https://ethereum.org/en/developers/docs/accounts/#contract-accounts

## Build ROFL {#build}

The `rofl build` command will execute a series of build commands depending on
the target Trusted Execution Environment (TEE) and produce the Oasis Runtime
Container (ORC) bundle.

Additionally, the following flags are available:

- `--output` the filename of the output ORC bundle. Defaults to the pattern
  `<name>.<deployment>.orc` where `<name>` is the app name from the manifest and
  `<deployment>` is the deployment name from the manifest.

- `--verify` also verifies the locally built enclave identity against the
  identity that is currently defined in the manifest and also against the
  identity that is currently set in the on-chain policy.

- `--no-update-manifest` do not update the enclave identity stored in the app
  manifest.

:::info

Building ROFL apps does not require a working TEE on your machine. However, you
do need to install all corresponding tools. Check out the [ROFL Prerequisites]
chapter for details.

:::

[ROFL Prerequisites]: https://github.com/oasisprotocol/oasis-sdk/blob/main/docs/rofl/prerequisites.md
[npa]: ./account.md#npa

## Update ROFL app config {#update}

Use `rofl update` command to update the ROFL app's configuration on chain:

![code shell](../examples/rofl/update.in.static)

![code shell](../examples/rofl/update.out.static)

## Remove ROFL app from the network {#remove}

Run `rofl remove` to deregister your ROFL app:

![code shell](../examples/rofl/remove.in.static)

![code](../examples/rofl/remove.out.static)

The deposit required to register the ROFL app will be returned to the current
administrator account.

## Show ROFL information {#show}

Run `rofl show` to obtain the information from the network on the ROFL admin
account, staked amount, current ROFL policy and running instances:

![code shell](../examples/rofl/show.in.static)

![code](../examples/rofl/show.out.static)

## Deploy ROFL app {#deploy}

Run `rofl deploy` to automatically deploy your app to the provider on-chain.

## Advanced

### Show ROFL identity {#identity}

Run `rofl identity` to compute the **cryptographic identity** of the ROFL app:

![code shell](../examples/rofl/identity.in.static)

![code](../examples/rofl/identity.out.static)

The output above is Base64-encoded enclave identity which depends on the ROFL
source code and the build environment. Enclave identities should be reproducible
on any computer and are used to prove and verify the integrity of ROFL binaries
on the network. See the [Reproducibility] chapter to learn more.

[Reproducibility]: https://github.com/oasisprotocol/oasis-sdk/blob/main/docs/runtime/reproducibility.md

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
