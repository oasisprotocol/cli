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
Check out the [Stake Requirements] chapter for more information.

:::

With the `--scheme` parameter, you can select one of the following ROFL app ID
derivation schemes:

- `cn` for the ROFL app creator address (the account you're using to sign the
  transaction) combined with the account's nonce (default). This behavior is
  similar to the one of the Ethereum [smart contract address derivation] and is
  deterministic.
- `cri` uses the ROFL app creator address combined with the block round the
  transaction will be validated in and its position inside that block.

[Stake Requirements]: https://github.com/oasisprotocol/docs/blob/main/docs/node/run-your-node/prerequisites/stake-requirements.md
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
  identity that is currently set in the on-chain policy. It does not update the
  manifest file with new entity id(s).

- `--no-update-manifest` do not update the enclave identity stored in the app
  manifest.

:::info

Building ROFL apps does not require a working TEE on your machine. However, you
do need to install all corresponding tools. Check out the [ROFL Prerequisites]
chapter for details.

:::

[ROFL Prerequisites]: https://github.com/oasisprotocol/oasis-sdk/blob/main/docs/rofl/prerequisites.md
[npa]: ./account.md#npa

## Secrets management {#secret}

### Set secret {#secret-set}

Run `rofl secret set <secret_name> <filename>|-` command to end-to-end encrypt a
secret with a key derived from the selected deployment network and store it to
the manifest file.

If you have your secret in a file, run:

![code shell](../examples/rofl/secret-set-file.in.static)

You can also feed the secret from a standard input like this:

![code shell](../examples/rofl/secret-set-stdin.in.static)

Once the secret is encrypted and stored, **there is no way of obtaining the
unencrypted value back again apart from within the TEE on the designated ROFL
deployment**.

Additionally, the following flags are available:

- `--force` replaces an existing secret.
- `--public-name <public-name>` defines the name of the secret that will be
  publicly exposed e.g. in the Oasis Explorer. By default, the public name is
  the same as the name of the secret.

:::danger Shells store history

Passing secrets as a command line argument will store them in your shell history
file as well! Use this approach only for testing. In production, always use
file-based secrets.

:::

### Import secrets from `.env` files {#secret-import}

Run `rofl secret import <dot-env-file>|-` to bulk-import secrets from a
[dotenv](https://github.com/motdotla/dotenv) compatible file (key=value with
`#` comments). This is handy for files like `.env`, `.env.production`,
`.env.testnet`, or symlinks such as `.env → .env.production`. You can also
pass `-` to read from standard input.

Each `KEY=VALUE` pair becomes a separate secret entry in your manifest.
Quoted values may span multiple physical lines;
newline characters are preserved.
Double-quoted values also support common escapes (`\n`, `\r`, `\t`, `\"`, `\\`).
Lines starting with `#` are ignored. Unquoted values stop at an unquoted `#`
comment.

![code shell](../examples/rofl/secret-import.in.static)

```bash
oasis rofl secret import .env
```

By default, if a secret with the same name already exists,
the command will
fail. Use `--force` to replace existing secrets.

After importing, **run**:

```bash
oasis rofl update
```

to push the updated secrets on-chain.

### Get secret info {#secret-get}

Run `rofl secret get <secret-name>` to check whether the secret exists in your
manifest file.

![code shell](../examples/rofl/secret-get.in.static)

![code](../examples/rofl/secret-get.out.static)

### Remove secret {#secret-rm}

Run `rofl secret rm <secret-name>` to remove the secret from your manifest file.

![code shell](../examples/rofl/secret-rm.in.static)

## Update ROFL app config on-chain {#update}

Use `rofl update` command to push the ROFL app's configuration to the chain:

![code shell](../examples/rofl/update.in.static)

![code shell](../examples/rofl/update.out.static)

The current on-chain policy, metadata and secrets will be replaced with the ones
in the manifest file. Keep in mind that ROFL replicas need to be restarted in
order for changes to take effect.

## Show ROFL information {#show}

Run `rofl show` to obtain the information from the network on the ROFL admin
account, staked amount, current ROFL policy and running instances:

![code shell](../examples/rofl/show.in.static)

![code](../examples/rofl/show.out.static)

## Deploy ROFL app {#deploy}

Run `rofl deploy` to automatically deploy your app to a machine obtained from
the [ROFL marketplace]. If a machine is already configured in your manifest file
a new version of your ROFL app will be deployed there. If no machines are rented
yet, you can use the following arguments to select a specific provider and
offer:

- `--provider <address>` specifies the provider to rent the machine from. On
  Sapphire Testnet, the Oasis-managed provider will be selected by default.
- `--offer <offer_name>` specifies the offer of the machine to rent. By default
  it takes the most recent offer. Run `--show-offers` to list offers and
  specifications.
- `--term <hour|month|year>` specifies the base rent period. It takes the first
  available provider term by default.
- `--term-count <number>` specifies the multiplier. Default is `1`.

![code shell](../examples/rofl/deploy.in.static)

![code](../examples/rofl/deploy.out.static)

[ROFL marketplace]: https://github.com/oasisprotocol/oasis-sdk/blob/main/docs/rofl/features/marketplace.mdx

## Manage a deployed ROFL machine {#machine}

Once a ROFL app is deployed, you can manage the machine it's running on using
the `oasis rofl machine` subcommands.

### Show machine information {#machine-show}

To view details about a deployed machine, including its status, expiration,
and any proxy URLs, run `oasis rofl machine show`:

![code shell](../examples/rofl/machine-show.in.static)

![code](../examples/rofl/machine-show.out.static)

If you have published ports in your `compose.yaml`, the output will include
a `Proxy` section with public URLs to access your services. For more details on
how to configure the proxy and for troubleshooting, see the [ROFL Proxy]
feature page.

[ROFL Proxy]: https://github.com/oasisprotocol/oasis-sdk/blob/main/docs/rofl/features/proxy.mdx

### Top-up payment for the machine {#machine-top-up}

Run `rofl machine top-up` to extend the rental of the machine obtained from
the [ROFL marketplace]. You can check the current expiration date of your
machine in the `Paid until` field from
the [`oasis rofl machine show` output](#machine-show).
The rental is extended under the terms of the original
offer. Specify the extension period with [`--term`][term-flags] and
[`--term-count`][term-flags] parameters.

![code shell](../examples/rofl/machine-top-up.in.static)

![code](../examples/rofl/machine-top-up.out.static)

[term-flags]: #deploy

### Show machine logs {#machine-logs}

You can fetch logs from your running ROFL app using `oasis rofl machine logs`.

![code shell](../examples/rofl/machine-logs.in.static)

:::danger Logs are not encrypted!

While only the app admin can access the logs, they are stored
**unencrypted on the ROFL node**. In production, make sure
you never print any confidential data to the standard or error outputs!

:::

### Restart a machine {#machine-restart}

To restart a running machine, use `oasis rofl machine restart`.

If you wish to clear the machine's persistent storage,
pass the [`--wipe-storage`] flag.

[`--wipe-storage`]: #deploy

### Stop a machine {#machine-stop}

To stop a machine, use `oasis rofl machine stop`.
To start it back again, use [`oasis rofl machine restart`].

[`oasis rofl machine restart`]: #machine-restart

### Remove a machine {#machine-remove}

To cancel the rental and permanently remove a machine,
including its persistent storage, use `oasis rofl machine remove`.

:::info

Canceling a machine rental will not refund any payment for the already paid
term.

:::

## Advanced

### Upgrade ROFL app dependencies {#upgrade}

Run `rofl upgrade` to bump ROFL bundle TDX artifacts in your manifest file to
their latest versions. This includes:

- the firmware
- the kernel
- stage two boot
- ROFL containers middleware (for TDX containers kind only)

![code shell](../examples/rofl/upgrade.in.static)

### Remove ROFL app from the network {#remove}

Run `rofl remove` to deregister your ROFL app:

![code shell](../examples/rofl/remove.in.static)

![code](../examples/rofl/remove.out.static)

The deposit required to register the ROFL app will be returned to the current
administrator account.

:::danger Secrets will be permanently lost

All secrets stored on-chain will be permanently lost when the ROFL app is
deregistered! If you backed up your manifest file, those secrets will also be
unretrievable since they were encrypted with a ROFL deployment-specific keypair.

:::

### ROFL provider tooling {#provider}

The `rofl provider` commands offers tools for managing your on-chain provider
information and your offers.

An example provider configuration file looks like this:

```yaml title="rofl-provider.yaml"
# Network name in your Oasis CLI
network: testnet
# ParaTime name in your Oasis CLI
paratime: sapphire
# Account name in your Oasis CLI
provider: rofl_provider
# List of Base64-encoded node IDs allowed to execute ROFL apps
nodes:
  -
# Address of the scheduler app
scheduler_app: rofl1qrqw99h0f7az3hwt2cl7yeew3wtz0fxunu7luyfg 
# Account name or address of who receives ROFL machine rental payments
payment_address: rofl_provider
offers:
  - id: small # Short human-readable name
    resources:
      tee: tdx # Possible values: sgx, tdx
      memory: 4096 # In MiB
      cpus: 2
      storage: 20000 # In MiB
    payment:
      native: # Possible keys: native, evm
        terms:
          hourly: 10 # Possible keys: hourly, monthly, yearly
    capacity: 50 # Max number of actively rented machines
```

#### Initialize a ROFL provider {#provider-init}

The `rofl provider init` initializes a new provider configuration file.

:::info

[Network and ParaTime](./account.md#npa) selectors are available for the
`rofl provider init` command.

:::

#### Create a ROFL provider on-chain {#provider-create}

Run `rofl provider create` to register your account as a provider on the
configured network and ParaTime.

![code shell](../examples/rofl/provider-create.in.static)

![code](../examples/rofl/provider-create.out.static)

:::info

In order to prevent spam attacks registering a ROFL provider requires a
certain amount to be deposited from your account until you decide to
[remove it](#provider-remove). The deposit remains locked for the lifetime of
the provider entity. Check out the [Stake Requirements] chapter for more
information.

:::

#### Update ROFL provider policies {#provider-update}

Use `rofl provider update` to update the list of endorsed nodes, the scheduler
app address, the payment recipient address and other provider settings.

![code shell](../examples/rofl/provider-update.in.static)

![code](../examples/rofl/provider-update.out.static)

To update your offers, run
[`rofl provider update-offers`](#provider-update-offers) instead.

#### Update ROFL provider offers {#provider-update-offers}

Use `rofl provider update-offers` to replace the on-chain offers with the ones
in your provider manifest file.

![code shell](../examples/rofl/provider-update-offers.in.static)

![code](../examples/rofl/provider-update-offers.out.static)

To update your provider policies, run [`rofl provider update`](#provider-update)
instead.

#### Remove ROFL provider from the network {#provider-remove}

Run `rofl provider remove` to deregister your ROFL provider account:

![code shell](../examples/rofl/provider-remove.in.static)

![code](../examples/rofl/provider-remove.out.static)

The deposit required to register the ROFL provider will be returned to its
address.

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
