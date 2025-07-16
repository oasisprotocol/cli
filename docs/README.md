---
title: Oasis CLI
description: Powerful CLI for managing Oasis networks, nodes, tokens and dApps
---

# Oasis Command Line Interface

Oasis command-line interface (CLI) is a powerful all-in-one tool for
interacting with the Oasis Network. You can download the latest release
binaries from the [GitHub repository] (see the "Installing or Updating Oasis
CLI on macOS" section below for macOS-specific instructions).

It boasts a number of handy features:

- Flexible setup:
  - supports Mainnet, Testnet, Localnet or any other deployment of the Oasis
    network
  - consensus layer configuration with arbitrary token
  - configuration of custom ParaTimes with arbitrary token
  - connecting to remote (via TCP/IP) or local (Unix socket) Oasis node
    instance
- Powerful wallet features:
  - standard token operations (transfers, allowances, deposits, withdrawals
    and balance queries)
  - file-based wallet with password protection
  - full Ledger hardware wallet support
  - address book
  - generation, signing and submitting transactions in non-interactive
    (headless) mode
  - offline transaction generation for air-gapped machines
  - transaction encryption with X25519-Deoxys-II envelope
  - support for Ed25519, Ethereum-compatible Secp256k1 and Sr25519 signature
    schemes
  - raw, BIP-44, ADR-8 and Ledger's legacy derivation paths
- Node operator features:
  - Oasis node inspection and health-checks
  - network governance transactions
  - staking reward schedule transactions
- Developer features:
  - built-in testing accounts compatible with the Oasis test runner, the Oasis
    CI and the official Sapphire and Emerald Localnet Docker images
  - Oasis ROFL app compilation, deployment and management
  - Oasis Wasm smart contract code deployment, instantiation, management and
    calls
  - debugging tools for deployed Wasm contracts
  - inspection of blocks, transactions, results and events

[GitHub repository]: https://github.com/oasisprotocol/cli/releases

## Installing or Updating Oasis CLI on macOS

This guide covers the installation or update process for the Oasis CLI on
macOS (compatible with Apple Silicon like M1/M2/M3). It assumes you have
Terminal access and basic command-line knowledge.

### Prerequisites

- macOS (tested on Apple Silicon).
- To avoid modifying system directories, we'll install the binary in a
  user-specific path (`~/.local/bin`). Ensure `~/.local/bin` is in your `PATH`.
  If not, add it by editing your shell configuration file (e.g., `~/.zshrc`
  for zsh):

  ```shell
  echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
  source ~/.zshrc
  ```

  Create the directory if it doesn't exist:

  ```shell
  mkdir -p ~/.local/bin
  ```

### Installation Steps

1. **Download the Binary**

   - Visit the Oasis CLI [GitHub repository].
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
   Security** and click **Open Anyway.**
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

   Compare the output with the latest version on the [GitHub repository] to
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
