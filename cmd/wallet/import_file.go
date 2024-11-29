package wallet

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/wallet"
	walletFile "github.com/oasisprotocol/cli/wallet/file"
)

var importFileCmd = &cobra.Command{
	Use:   "import-file <name> <entity.pem>",
	Short: "Import an existing account from file",
	Long:  "Import the private key from an existing PEM file",
	Args:  cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		cfg := config.Global()
		name := args[0]
		filename := args[1]

		checkAccountExists(cfg, name)

		rawFile, err := os.ReadFile(filename)
		cobra.CheckErr(err)

		block, _ := pem.Decode(rawFile)
		if block == nil { //nolint: staticcheck
			cobra.CheckErr(fmt.Errorf("failed to decode PEM file"))
		}

		algorithm, err := detectAlgorithm(block.Type) //nolint: staticcheck
		cobra.CheckErr(err)

		// Ask for passphrase.
		passphrase := common.AskNewPassphrase()

		accCfg := &config.Account{
			Kind: walletFile.Kind,
			Config: map[string]interface{}{
				"algorithm": algorithm,
			},
		}

		src := &wallet.ImportSource{
			Kind: wallet.ImportKindPrivateKey,
			Data: encodeKeyData(algorithm, block.Bytes), //nolint: staticcheck
		}

		err = cfg.Wallet.Import(name, passphrase, accCfg, src)
		cobra.CheckErr(err)

		err = cfg.Save()
		cobra.CheckErr(err)
	},
}

// detectAlgorithm detects the key type based on the PEM type.
func detectAlgorithm(pemType string) (string, error) {
	switch pemType {
	case "ED25519 PRIVATE KEY":
		return wallet.AlgorithmEd25519Raw, nil
	case "EC PRIVATE KEY":
		return wallet.AlgorithmSecp256k1Raw, nil
	case "SR25519 PRIVATE KEY":
		return wallet.AlgorithmSr25519Raw, nil
	}

	return "", fmt.Errorf("unsupported PEM type: %s", pemType)
}

// encodeKeyData re-encodes the key in raw bytes back to the user-readable string for import.
func encodeKeyData(algorithm string, rawKey []byte) string {
	switch algorithm {
	case wallet.AlgorithmEd25519Raw, wallet.AlgorithmSr25519Raw:
		return base64.StdEncoding.EncodeToString(rawKey)
	case wallet.AlgorithmSecp256k1Raw:
		return hex.EncodeToString(rawKey)
	}

	return ""
}
