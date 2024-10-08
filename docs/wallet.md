---
title: Wallet
description: Manage accounts in your CLI wallet
---

# Managing Accounts in Your Wallet

The `wallet` command is used to manage accounts in your wallet. The wallet
can contain file-based accounts which are stored along your Oasis CLI
configuration, or a reference to an account stored on your hardware wallet.

The following encryption algorithms and derivation paths are supported by the
Oasis CLI for your accounts:

- `ed25519-adr8`: [Ed25519] keypair using the [ADR-8] derivation path in order
  to obtain a private key from the mnemonic. This is the default setting
  suitable for accounts on the Oasis consensus layer and Cipher.
- `secp256k1-bip44`: [Secp256k1] Ethereum-compatible keypair using [BIP-44]
  with ETH coin type to derive a private key. This setting is
  used for accounts living on EVM-compatible ParaTimes such as Sapphire or
  Emerald. The same account can be imported into Metamask and other Ethereum
  wallets.
- `ed25519-raw`: [Ed25519] keypair imported directly from the Base64-encoded
  private key. No key derivation is involved. This setting is primarily used by
  the network validators to sign the governance and other consensus-layer
  transactions.
- `ed25519-legacy`: [Ed25519] keypair using a legacy 5-component derivation
  path. This is the preferred setting for Oasis accounts stored on a hardware
  wallet like Ledger. It is called legacy, because it was first implemented
  before the [ADR-8] was standardized.
- `sr25519-adr8`: [Sr25519] keypair using the [ADR-8] derivation path. This is
  an alternative signature scheme for signing ParaTime transactions.
- `secp256k1-raw` and `sr25519-raw`: Respective Secp256k1 and Sr25519 keypairs
  imported directly from the Hex- or Base64-encoded private key. No key
  derivation is involved.

:::tip

For compatibility with Ethereum, each `secp256k1` account corresponds to two
addresses:

- 20-byte hex-encoded Ethereum-compatible address, e.g.
  `0xDCbF59bbcC0B297F1729adB23d7a5D721B481BA9`
- Bech32-encoded Oasis native address, e.g.
  `oasis1qq3agel5x07pxz08ns3d2y7sjrr3xf9paquhhhzl`.

There exists a [mapping][eth-oasis-address-mapping] from the Ethereum address
to the native Oasis address as in the example above, but **there is no reverse
mapping**.

:::

[ADR-8]: ../../../adrs/0008-standard-account-key-generation.md
[BIP-44]: https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki
[Ed25519]: https://en.wikipedia.org/wiki/EdDSA
[Secp256k1]: https://en.bitcoin.it/wiki/Secp256k1
[Sr25519]: https://wiki.polkadot.network/docs/learn-cryptography
[eth-oasis-address-mapping]: https://github.com/oasisprotocol/oasis-sdk/blob/c36a7ee194abf4ca28fdac0edbefe3843b39bf69/client-sdk/go/types/address.go#L135-L142

## Create an Account {#create}

The `wallet create <name>` command is used add a new account into your Oasis
CLI wallet by:

- generating a new mnemonic and storing it into a file-based wallet, or
- creating a reference to an account stored on your hardware wallet.

By default, a password-encrypted file-based wallet will be used for storing the
private key. You will have to enter the password for this account each time to
access use it for signing the transactions (e.g. to send tokens). The account
address is public and can be accessed without entering the passphrase.

![code shell](../examples/wallet/create.in.static)

![code](../examples/wallet/create.out.static)

:::tip

The first account you create or import will become your **default account**.
This means it will automatically be selected as a source for sending funds or
calling smart contracts unless specified otherwise by using `--account <name>`
flag. You can always [change the default account](#set-default) later.

:::

To use your hardware wallet, add `--kind ledger` parameter and Oasis CLI will
store a reference to an account on your hardware wallet:

![code shell](../examples/wallet/create-ledger.in.static)

A specific account kind (`ed25519-adr8`, `secp256k1-bip44`) and the derivation
path number can be passed with `--file.algorithm` and `--file.number` or
`--ledger.algorithm` and `--ledger.number` respectively. For example:

![code shell](../examples/wallet/create-ledger-secp256k1.in.static)

:::tip

When creating a hardware wallet account, Oasis CLI will:

1. obtain the public key of the account from your hardware wallet,
2. compute the corresponding native address, and
3. store the Oasis native address into the Oasis CLI.

If you try to open the same account with a different Ledger device or
reset your Ledger with a new mnemonic, Oasis CLI will abort because the address
of the account obtained from the new device will not match the one stored in
your config.

![code shell](../examples/wallet/show-ledger-error.in.static)

![code](../examples/wallet/show-ledger-error.out.static)

:::

## Import an Existing Keypair or a Mnemonic {#import}

If you already have a mnemonic or a raw private key, you can import it
as a new account by invoking `wallet import`. You will be asked
interactively to select an account kind (`mnemonic` or `private key`),
encryption algorithm (`ed25519` or `secp256k1`) and then provide either the
mnemonic with the derivation number, or the raw private key in the corresponding
format.

Importing an account with a mnemonic looks like this:

![code shell](../examples/wallet/import-secp256k1-bip44.in.static)

![code](../examples/wallet/import-secp256k1-bip44.out.static)

Let's make another Secp256k1 account and entering a hex-encoded raw private key:

![code shell](../examples/wallet/import-secp256k1-raw.in.static)

![code](../examples/wallet/import-secp256k1-raw.out.static)

To override the defaults, you can pass `--algorithm`, `--number` and `--secret`
parameters. This is especially useful, if you are running the command in a
non-interactive mode:

![code](../examples/wallet/import-secp256k1-bip44-y.in.static)

:::danger Be cautious when importing accounts in non-interactive mode

Since the account's secret is provided as a command line parameter in the
non-interactive mode, make sure you **read the account's secret from a file or
an environment variable**. Otherwise, the secret may be stored and exposed in
your shell history.

Also, protecting your account with a password is currently not supported in the
non-interactive mode.

:::

## List Accounts Stored in Your Wallet {#list}

You can list all available accounts in your wallet with `wallet list`:

![code shell](../examples/wallet/00-list.in)

![code](../examples/wallet/00-list.out)

Above, you can see the native Oasis addresses of all local accounts. The
[default account](#set-default) has a special `(*)` sign next to its name.

## Show Account Configuration Details {#show}

To verify whether an account exists in your wallet, use `wallet show <name>`.
This will print the account's native address and the public key which requires
entering your account's password.

![code shell](../examples/wallet/show.in.static)

![code](../examples/wallet/show.out.static)

For `secp256k1` accounts Ethereum's hex-encoded address will also be printed.

![code shell](../examples/wallet/show-secp256k1.in.static)

![code](../examples/wallet/show-secp256k1.out.static)

Showing an account stored on your hardware wallet will require connecting it to
your computer:

![code shell](../examples/wallet/show-ledger.in.static)

![code](../examples/wallet/show-ledger.out.static)

## Export the Account's Secret {#export}

You can obtain the secret material of a file-based account such as the mnemonic
or the private key by running `wallet export <name>`.

For example:

![code shell](../examples/wallet/export.in.static)

![code](../examples/wallet/export.out.static)

The same goes for your Secp256k1 accounts:

![code shell](../examples/wallet/export-secp256k1-bip44.in.static)

![code](../examples/wallet/export-secp256k1-bip44.out.static)

![code shell](../examples/wallet/export-secp256k1-raw.in.static)

![code](../examples/wallet/export-secp256k1-raw.out.static)

Trying to export an account stored on your hardware wallet will only
export its public key:

![code shell](../examples/wallet/export-ledger.in.static)

![code](../examples/wallet/export-ledger.out.static)

## Renaming the Account {#rename}

To rename an account, run `wallet rename <old_name> <new_name>`.

For example:

![code shell](../examples/wallet/00-list.in)

![code](../examples/wallet/00-list.out)

![code shell](../examples/wallet/01-rename.in)

![code shell](../examples/wallet/02-list.in)

![code](../examples/wallet/02-list.out)

## Deleting an Account {#remove}

To irreversibly delete the accounts from your wallet use
`wallet remove [names]`. For file-based accounts this will delete the file
containing the private key from your disk. For hardware wallet accounts this
will delete the Oasis CLI reference, but the private keys will remain intact on
your hardware wallet.

For example, let's delete `lenny` account:

![code shell](../examples/wallet/00-list.in)

![code](../examples/wallet/00-list.out)

![code shell](../examples/wallet/remove.in.static)

![code](../examples/wallet/remove.out.static)

```shell
oasis wallet list
```

```
ACCOUNT         KIND                            ADDRESS                                        
emma            file (secp256k1-raw)            oasis1qph93wnfw8shu04pqyarvtjy4lytz3hp0c7tqnqh
eugene          file (secp256k1-bip44:0)        oasis1qrvzxld9rz83wv92lvnkpmr30c77kj2tvg0pednz
logan           ledger (ed25519-legacy:0)       oasis1qpl4axynedmdrrgrg7dpw3yxc4a8crevr5dkuksl
oscar (*)       file (ed25519-raw)              oasis1qp87hflmelnpqhzcqcw8rhzakq4elj7jzv090p3e
```

You can also delete accounct in non-interactive mode format by passing the
`-y` parameter:

![code shell](../examples/wallet/remove-y.in.static)

## Set Default Account {#set-default}

To change your default account, use `wallet set-default <name>` and the
name of the desired default account.

![code shell](../examples/wallet/00-list.in)

![code](../examples/wallet/00-list.out)

![code shell](../examples/wallet/04-set-default.in)

![code shell](../examples/wallet/05-list.in)

![code](../examples/wallet/05-list.out)

## Advanced

### Import an Existing Keypair from PEM file {#import-file}

Existing node operators may already use their Ed25519 private key for running
their nodes stored in a PEM-encoded file typically named `entity.pem`. In order
to submit their governance transaction, for example to vote on the network
upgrade using the Oasis CLI, they need to import the key into the Oasis CLI
wallet:

![code shell](../examples/wallet/import-file.in.static)

![code](../examples/wallet/import-file.out.static)

The key is now safely stored and encrypted inside the Oasis CLI.

```shell
oasis wallet list
```

```
ACCOUNT                         KIND                            ADDRESS                                        
my_entity                       file (ed25519-raw)              oasis1qpe0vnm0ahczgc353vytvtz9r829le4pjux8lc5z
```

### Remote Signer for `oasis-node` {#remote-signer}

You can bind the account in your Oasis CLI wallet with a local instance of
`oasis-node`. To do this, use
`wallet remote-signer <account_name> <socket_path>`, pick the account you wish
to expose and provide a path to the new unix socket:

![code shell](../examples/wallet/remote-signer.in.static)

![code](../examples/wallet/remote-signer.out.static)

### Test Accounts {#test-accounts}

Oasis CLI comes with the following hardcoded test accounts:

- `test:alice`: Ed25519 test account used by Oasis core tests
- `test:bob`: Ed25519 test account used by Oasis core tests
- `test:charlie`: Secp256k1 test account
- `test:cory`: Ed25519 account used by `oasis-net-runner`
- `test:dave`: Secp256k1 test account
- `test:erin`: Sr25519 test account
- `test:frank`: Sr25519 test account

:::danger Do not use these accounts on public networks

Private keys for these accounts are well-known. Do not fund them on public
networks, because anyone can drain them!

:::

We suggest that you use these accounts for Localnet development or for
reproducibility when you report bugs to the Oasis core team. You can access the
private key of a test account the same way as you would for ordinary accounts
by invoking the [`oasis wallet export`](#export) command.
