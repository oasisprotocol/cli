package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

var fromPublicKeyCmd = &cobra.Command{
	Use:   "from-public-key <public-key>",
	Short: "Convert public key to an account address",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		var pk signature.PublicKey
		err := pk.UnmarshalText([]byte(args[0]))
		cobra.CheckErr(err)

		fmt.Println(staking.NewAddress(pk))
	},
}
