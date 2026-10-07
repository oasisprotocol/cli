package network

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	cmnGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var addLocalCmd = &cobra.Command{
	Use:   "add-local <name> <rpc-endpoint>",
	Short: "Add a new local network",
	Args:  cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		name, rpc := args[0], args[1]

		// Validate initial network configuration early.
		cobra.CheckErr(config.ValidateIdentifier(name))

		// Check if a local file name was given without protocol.
		if !strings.HasPrefix(rpc, "unix:") {
			if info, err := os.Stat(rpc); err == nil {
				if !info.IsDir() {
					rpc = "unix:" + rpc
				}
			}
		}

		AddLocalNetwork(name, rpc)
	},
}

func AddLocalNetwork(name string, rpc string) {
	cfg := cliConfig.Global()

	net := config.Network{
		RPC: rpc,
	}

	if !cmnGrpc.IsLocalAddress(net.RPC) {
		cobra.CheckErr(fmt.Errorf("rpc-endpoint '%s' is not local", net.RPC))
	}

	// Extract absolute path for given local endpoint.
	var err error
	net.RPC, err = extractAbsPath(net.RPC)
	if err != nil {
		cobra.CheckErr(err)
	}

	// Connect to the network and query the chain context.
	ctx := context.Background()
	conn, err := connection.ConnectNoVerify(ctx, &net)
	cobra.CheckErr(err)

	chainContext, err := conn.Consensus().Core().GetChainContext(ctx)
	cobra.CheckErr(err)
	net.ChainContext = chainContext
	cobra.CheckErr(net.Validate())

	// Try to clone config details from any of the hardcoded defaults.
	for _, defaultNet := range config.DefaultNetworks.All {
		if defaultNet.ChainContext != chainContext {
			continue
		}

		net.Denomination = defaultNet.Denomination
		net.ParaTimes = defaultNet.ParaTimes
		break
	}

	// Let the user change detected parameters, if needed.
	networkDetailsFromSurvey(&net)

	err = cfg.Networks.Add(name, &net)
	cobra.CheckErr(err)

	err = cfg.Save()
	cobra.CheckErr(err)
}

// Extract absolute path for given local RPC endpoint.
func extractAbsPath(rpc string) (string, error) {
	// First split the input local RPC enpoint into scheme and path.
	extracted := strings.Split(rpc, ":")
	if len(extracted) != 2 {
		return "", fmt.Errorf("malformed local RPC endpoint")
	}
	scheme := extracted[0]
	path := extracted[1]

	// Expand home directory shorthand if applicable.
	if strings.HasPrefix(path, "~/") {
		// Expand to user's home directory.
		home, grr := os.UserHomeDir()
		if grr != nil {
			return "", fmt.Errorf("unable to get user's home directory: %w", grr)
		}
		path = filepath.Join(home, path[2:])
	}

	// Get absolute path.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("malformed path in RPC endpoint: %w", err)
	}

	// Assemble path back into valid local RPC enpoint.
	return strings.Join([]string{scheme, absPath}, ":"), nil
}

func init() {
	addLocalCmd.Flags().AddFlagSet(common.AnswerYesFlag)
	addLocalCmd.Flags().AddFlagSet(common.DenominationFlags)
}
