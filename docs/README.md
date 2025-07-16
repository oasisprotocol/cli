---
title: Oasis CLI
description: Powerful CLI for managing Oasis networks, nodes, tokens and dApps
---

# Oasis Command Line Interface

The Oasis Command Line Interface (CLI) is a powerful, all-in-one tool for
interacting with the Oasis Network. [5] You can download the latest release
binaries from the [GitHub repository].

It boasts a number of handy features:

- **Flexible setup**:
  - Supports Mainnet, Testnet, Localnet, or any other deployment of the Oasis
    network. [5]
  - Consensus layer configuration with an arbitrary token. [5]
  - Configuration of custom ParaTimes with an arbitrary token. [5]
  - Connecting to a remote (via TCP/IP) or local (Unix socket) Oasis node
    instance. [5]
- **Powerful wallet features**:
  - Standard token operations (transfers, allowances, deposits, withdrawals,
    and balance queries). [5]
  - File-based wallet with password protection. [5]
  - Full Ledger hardware wallet support. [5]
  - Address book. [5]
  - Generation, signing, and submitting transactions in non-interactive
    (headless) mode. [5]
  - Offline transaction generation for air-gapped machines. [5]
  - Transaction encryption with X25519-Deoxys-II envelope. [5]
  - Support for Ed25519, Ethereum-compatible Secp256k1, and Sr25519 signature
    schemes. [5]
  - Raw, BIP-44, ADR-8, and Ledger's legacy derivation paths. [5]
- **Node operator features**:
  - Oasis node inspection and health checks. [5]
  - Network governance transactions. [5]
  - Staking reward schedule transactions. [5]
- **Developer features**:
  - Built-in testing accounts compatible with the Oasis test runner, the Oasis
    CI, and the official Sapphire and Emerald Localnet Docker images. [5]
  - Oasis ROFL app compilation, deployment, and management. [5]
  - Oasis Wasm smart contract code deployment, instantiation, management, and
    calls. [5]
  - Debugging tools for deployed Wasm contracts. [5]
  - Inspection of blocks, transactions, results, and events. [5]

## Installation

### macOS

This guide covers the installation or update process for the Oasis CLI on macOS
(compatible with Apple Silicon like M1/M2/M3). It assumes you have Terminal
access and basic command-line knowledge.

#### Prerequisites

- macOS (tested on Apple Silicon).
- To avoid modifying system directories, we'll install the binary in a
  user-specific path (`~/.local/bin`). Ensure `~/.local/bin` is in your
  `PATH`. If not, add it by editing your shell configuration file (e.g.,
  `~/.zshrc` for zsh):

    ```
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
    source ~/.zshrc
    ```

- Create the directory if it doesn't exist: `mkdir -p ~/.local/bin`.

#### Installation Steps

1. **Download the Binary**:
   - Visit the Oasis CLI GitHub releases page:
     <https://github.com/oasisprotocol/cli/releases>.
   - Download the latest macOS archive, e.g.,
     `oasis_cli_X.Y.Z_darwin_all.tar.gz` (replace X.Y.Z with the version,
     like 0.14.1).

2. **Extract the Archive**:
   - Open Terminal and navigate to your Downloads folder:

        ```
        cd ~/Downloads
        ```

   - Extract the file (replace with your version):

        ```
        tar -xzf oasis_cli_0.14.1_darwin_all.tar.gz
        ```

   - This creates a directory like `oasis_cli_0.14.1_darwin_all`.

3. **Navigate to the Extracted Directory**:

    ```
    cd oasis_cli_0.14.1_darwin_all
    ```

4. **Move the Binary to User Path**:

    ```
    mv oasis ~/.local/bin/
    ```

5. **Bypass macOS Gatekeeper (if prompted with a security warning)**:

    ```
    xattr -d com.apple.quarantine ~/.local/bin/oasis
    ```

   - If a dialog appears, go to System Settings > Privacy & Security >
     Security, and click "Open Anyway".

6. **Verify Installation**:

    ```
    oasis --version
    ```

   - This should display the CLI version. If successful, the Oasis CLI is
     installed and ready to use.

## Updating the Oasis CLI

To update the Oasis CLI to a newer version, simply overwrite the binary with
the latest version. This preserves your configurations (e.g., wallets and
settings).

1. **Check Current Version**:

    ```
    oasis --version
    ```

    Compare with the latest on <https://github.com/oasisprotocol/cli/releases>
    to see if an update is available.

2. **Update Process**:
   Follow the same installation steps above with the new version. The key
   difference is that you're overwriting the existing binary rather than
   installing it fresh. Your configurations will be preserved.

## Notes

- Clean up: Optionally delete the downloaded .tar.gz and extracted directory
  after installation/update.
- For major version updates, review the release notes on GitHub for any
  breaking changes or additional steps.

[GitHub repository]: https://github.com/oasisprotocol/cli/releases
