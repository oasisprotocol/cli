---
title: Oasis CLI
description: Powerful CLI for managing Oasis networks, nodes, tokens and dApps
---

# Oasis Command Line Interface

Oasis command-line interface (CLI) is a powerful all-in-one tool for
interacting with the Oasis Network. You can download the latest release
binaries from the [GitHub repository].

It boasts a number of handy features:

- Flexible setup:
  - supports Mainnet, Testnet, Localnet or any other deployment of the Oasis
    network
  - consensus layer configuration with arbitrary token
  - configuration of custom ParaTimes with arbitrary token
  - connecting to remote (via TCP/IP) or local (Unix socket) Oasis node
    instance
- Powerful wallet features:
  - standard token operations (transfers, allowances, deposits, withdrawals and
    balance queries)
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
  - Oasis node inspection and healthchecks
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
