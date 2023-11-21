---
title: Address book
description: Storing your blockchain contacts for future use
---

# Address Book

If you repeatedly transfer tokens to the same recipients or if you just want to
store an arbitrary address for future use, you can use the `addressbook`
command to **name the address and store it in your address book**. Entries
in your address book are behaving similarly to the
[accounts stored in your wallet][wallet], for example when checking the balance
of the account or sending tokens to. Of course, you cannot sign any
transactions with the address stored in your address book since you do not
possess the private key of that account. Both the Oasis native and the
Ethereum-compatible addresses can be stored.

:::info

The name of the address book entry may not clash with any of the account names
in your wallet. The Oasis CLI will prevent you from doing so.

:::

[wallet]: wallet.md

## Add a New Entry {#add}

Use `addressbook add <name> <address>` to name the address and store it in your
address book.

![code shell](../examples/addressbook/00-add-oasis.in)

![code shell](../examples/addressbook/01-add-eth.in)

Then, you can for example use the entry name in you address book to send the
tokens to. In this case, we're sending `2.5 TEST` to `meghan` on Sapphire
Testnet:

![code shell](../examples/addressbook/02-transfer.y.in)

![code](../examples/addressbook/02-transfer.y.out)

## List Entries {#list}

You can list all entries in your address book by invoking `addressbook list`.

![code shell](../examples/addressbook/03-list.in)

![code](../examples/addressbook/03-list.out)

## Show Entry Details {#show}

You can check the details such as the native Oasis address of the Ethereum
account or simply check, if an entry exists in the address book, by running
`addressbook show <name>`:

![code shell](../examples/addressbook/04-show-eth.in)

![code](../examples/addressbook/04-show-eth.out)

![code shell](../examples/addressbook/05-show-oasis.in)

![code](../examples/addressbook/05-show-oasis.out)

## Rename an Entry {#rename}

You can always rename the entry in your address book by using
`addressbook rename <old_name> <new_name>`:

![code shell](../examples/addressbook/03-list.in)

![code](../examples/addressbook/03-list.out)

![code shell](../examples/addressbook/06-rename.in)

![code shell](../examples/addressbook/07-list.in)

![code](../examples/addressbook/07-list.out)

## Remove an Entry {#remove}

To delete an entry from your address book invoke
`addressbook remove <name>`.

![code shell](../examples/addressbook/03-list.in)

![code](../examples/addressbook/03-list.out)

![code shell](../examples/addressbook/09-remove.in)

![code shell](../examples/addressbook/10-list.in)

![code](../examples/addressbook/10-list.out)
