package wallet

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

var Cmd = &cobra.Command{
	Use:     "wallet",
	Short:   "Manage accounts in the local wallet",
	Aliases: []string{"w"},
}

type accountEntitySignerFactory struct {
	signer signature.Signer
}

func (sf *accountEntitySignerFactory) EnsureRole(
	role signature.SignerRole,
) error {
	if role != signature.SignerEntity {
		return signature.ErrInvalidRole
	}
	return nil
}

func (sf *accountEntitySignerFactory) Generate(
	_ signature.SignerRole,
	_ io.Reader,
) (signature.Signer, error) {
	// The remote signer should never require this.
	return nil, fmt.Errorf("refusing to generate new signing keys")
}

func (sf *accountEntitySignerFactory) Load(
	role signature.SignerRole,
) (signature.Signer, error) {
	if err := sf.EnsureRole(role); err != nil {
		return nil, err
	}
	return sf.signer, nil
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(showCmd)
	Cmd.AddCommand(rmCmd)
	Cmd.AddCommand(renameCmd)
	Cmd.AddCommand(setDefaultCmd)
	Cmd.AddCommand(importCmd)
	Cmd.AddCommand(importFileCmd)
	Cmd.AddCommand(exportCmd)
	Cmd.AddCommand(remoteSignerCmd)
}
