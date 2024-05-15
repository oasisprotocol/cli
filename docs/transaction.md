---
title: Transaction
description: Use CLI to decode, verify, sign and submit a transaction
---

# Transaction Tools

The `transaction` command offers convenient tools for processing raw
consensus or ParaTime transactions stored in a JSON file:

- decoding and displaying the transaction,
- verifying transaction's signature,
- signing the transaction,
- broadcasting the transaction.

## Decode, Verify and Show a Transaction {#show}

To show the transaction, invoke `transaction show <filename.json>` and provide
a filename containing a previously generated transaction by `oasis-node` or the
Oasis CLI's [`--output-file`][account-output-file] parameter.

[account-output-file]: ./account.md#output-file

For example, let's take the following transaction transferring `1.0 TEST` from
`test:alice` to `test:bob` on Testnet consensus layer and store it to
`testtx.json`:

![code json](../examples/transaction/testtx.json "testtx.json")

We can decode and verify the transaction as follows:

![code shell](../examples/transaction/show.in)

![code](../examples/transaction/show.out)

Since the signature depends on the [chain domain separation context], the
transaction above will be invalid on other networks such as the Mainnet. In this
case the Oasis CLI will print the `[INVALID SIGNATURE]` warning below the
signature:

![code shell](../examples/transaction/show-invalid.in)

![code text {4}](../examples/transaction/show-invalid.out)

The `show` command is also compatible with ParaTime transactions. Take the
following transaction which transfers `1.0 TEST` from `test:alice` to `test:bob`
inside Sapphire ParaTime on the Testnet:

![code json](../examples/transaction/testtx2.json "testtx2.json")

The Oasis CLI will be able to verify a transaction only for the **exact network
and ParaTime combination** since both are used to derive the chain domain
separation context for signing the transaction.

![code shell](../examples/transaction/show-paratime-tx.in)

![code](../examples/transaction/show-paratime-tx.out)

## Sign a Transaction {#sign}

To sign a [previously unsigned transaction][unsigned] transaction or to append
another signature to the transaction (*multisig*), run
`transaction sign <filename.json>`.

For example, let's transfer `1.0 TEST` from `test:alice` to `test:bob` on
Testnet consensus layer, but don't sign it and store it to
`testtx_unsigned.json`:

![code json](../examples/transaction/testtx_unsigned.json
  "testtx_unsigned.json")

Comparing this transaction to [`testtx.json`](#show) which was signed, we can
notice that the transaction is not wrapped inside the `untrusted_raw_value`
envelope with the `signature` field.

Decoding unsigned transaction gives us similar output:

![code shell](../examples/transaction/show-unsigned.in)

![code](../examples/transaction/show-unsigned.out)

Finally, let's sign the transaction:

![code shell](../examples/transaction/sign.y.in)

![code](../examples/transaction/sign.y.out)

We can also use [`--output-file`][account-output-file] here and store the
signed transaction back to another file instead of showing it.

:::info

[Network and Account][npa] selectors are available for the `transaction sign`
command.

:::

[npa]: ./account.md#npa
[unsigned]: ./account.md#unsigned

## Submit a Transaction {#submit}

Invoking `transaction submit <filename.json>` will broadcast the consensus or
ParaTime transaction to the selected network or ParaTime. If the transaction
hasn't been signed yet, Oasis CLI will first sign it with the selected account
in your wallet and then broadcast it.

```shell
oasis tx submit testtx.json --network testnet --no-paratime
```

```
Broadcasting transaction...
Transaction executed successfully.
Transaction hash: a81a1dcd203bba01761a55527f2c44251278110a247e63a12f064bf41e07f13a
```

```shell
oasis tx submit testtx2.json --network testnet --paratime sapphire
```

```
Broadcasting transaction...
Transaction included in block successfully.
Round:            946461
Transaction hash: 25f0b2a92b6171969e9cd41d047bc20b4e2307c3a329ddef41af73df69d95b5d
```

[chain domain separation context]: ../../../core/crypto.md#chain-domain-separation
