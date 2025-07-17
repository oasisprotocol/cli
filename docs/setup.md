# Setup

## Download and Run

Download the latest release from our [GitHub repository][cli-releases] and
extract it to your favorite application folder.

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

## Installing or Updating Oasis CLI on macOS

This guide covers the installation or update process for the Oasis CLI on
macOS (compatible with Apple Silicon M1/M2/M3). It assumes you have Terminal
access and basic command-line knowledge.

### Prerequisites

- macOS (tested on Apple Silicon).
- To avoid modifying system directories, we’ll install the binary in a
  user-specific path (`~/.local/bin`). Ensure `~/.local/bin` is in your `PATH`.
  If not, add it by editing your shell configuration file (e.g., `~/.zshrc`
  for zsh):

  ```shell
  echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
  source ~/.zshrc
  ```

  Create the directory if it doesn’t exist:

  ```shell
  mkdir -p ~/.local/bin
  ```

### Installation Steps

1. **Download the Binary**

   - Visit the Oasis CLI [cli-releases] page.
   - Download the latest macOS archive, e.g.,
     `oasis_cli_X.Y.Z_darwin_all.tar.gz` (replace `X.Y.Z` with the version,
     like `0.14.1`).

2. **Extract the Archive**

   ```shell
   cd ~/Downloads
   tar -xzf oasis_cli_X.Y.Z_darwin_all.tar.gz   # replace with your version
   ```

   This creates a directory such as `oasis_cli_X.Y.Z_darwin_all`.

3. **Navigate to the Extracted Directory**

   ```shell
   cd oasis_cli_X.Y.Z_darwin_all
   ```

4. **Move the Binary to the User Path**

   ```shell
   mv oasis ~/.local/bin/
   ```

5. **Bypass macOS Gatekeeper** (if you encounter a security warning)

   ```shell
   xattr -d com.apple.quarantine ~/.local/bin/oasis
   ```

   If a dialog appears, go to **System Settings → Privacy & Security →
   Security** and click **Open Anyway**.

6. **Verify Installation**

   ```shell
   oasis --version
   ```

   The CLI version should be displayed. If so, the Oasis CLI is installed and
   ready to use.

### Updating the Oasis CLI

To update the Oasis CLI to a newer version, simply overwrite the binary with
the latest version. Your configurations (wallets, settings) are preserved.

1. **Check Current Version**

   ```shell
   oasis --version
   ```

   Compare the output with the latest version on the [cli-releases] page to
   see if an update is available.

2. **Update Process**

   Follow the same installation steps above using the new version archive.
   Because you are overwriting the existing binary, no additional cleanup is
   required.

### Notes

- Clean-up: You can delete the downloaded `.tar.gz` file and extracted
  directory after installation or update.
- For major version updates, review the release notes on GitHub for any
  breaking changes or additional steps.

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
