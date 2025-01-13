# Setup

## Download and Run

Download the latest release [here][cli-releases] and extract it to your
favorite application folder.

:::info

Oasis is currently providing official amd64 for Linux and ARM builds for MacOS.
If you want to run it on another platform, you will have to
[build it from source][cli-source].

:::

:::info

We suggest that you update your system path to include a directory containing
the `oasis` binary or create a symbolic link to `oasis` in your
system path, so you can access it globally.

:::

Run the Oasis CLI by typing `oasis`.

![code](../examples/setup/first-run.out)

When running the Oasis CLI for the first time, it will generate a configuration
file and populate it with the current Mainnet and Testnet networks. It will also
configure all [ParaTimes supported by the Oasis Foundation][paratimes].

## Configuration

The configuration folder of Oasis CLI is located:

- on Windows:
  - `%USERPROFILE%\AppData\Local\oasis\`
- on macOS:
  - `/Users/$USER/Library/Application Support/oasis/`
- on Linux:
  - `$HOME/.config/oasis/`

There, you will find `cli.toml` which contains the configuration of the
networks, ParaTimes and your wallet. Additionally, each file-based account in
your wallet will have a separate, password-encrypted JSON file in the same
folder named after the name of the account with the `.wallet` extension.

## Multiple Profiles

You can utilize multiple profiles of your Oasis CLI. To create a new profile,
move your existing configuration folder to another place, for example:

```shell
mv $HOME/.config/oasis $HOME/.config/oasis_dev
```

Then, invoke `oasis` with arbitrary command to set up a fresh configuration
folder under `~/.config/oasis`. For example:

![code shell](../examples/setup/wallet-list.in)

![code](../examples/setup/wallet-list.out)

Now you can switch between the `oasis_dev` and the new profile by passing
`--config` parameter pointing to `cli.toml` in the desired configuration folder.

![code shell](../examples/setup/wallet-list-config.in.static)

![code](../examples/setup/wallet-list-config.out.static)

## Back Up Your Wallet

To back up your complete Oasis CLI configuration including your wallet, archive
the configuration folder containing `cli.toml` and `.wallet` files.

[cli-releases]: https://github.com/oasisprotocol/cli/releases
[cli-source]: https://github.com/oasisprotocol/cli
[paratimes]: https://github.com/oasisprotocol/docs/blob/main/docs/build/tools/other-paratimes/README.mdx
