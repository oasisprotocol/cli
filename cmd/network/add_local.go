package network

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	cmnGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	symbol      string
	numDecimals uint
	description string

	addLocalCmd = &cobra.Command{
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
)

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

	// With a very high probability, the user is going to be
	// adding a local endpoint for an existing network, so try
	// to clone config details from any of the hardcoded
	// defaults.
	var clonedDefault bool
	for _, defaultNet := range config.DefaultNetworks.All {
		if defaultNet.ChainContext != chainContext {
			continue
		}

		// Yep.
		net.Denomination = defaultNet.Denomination
		net.ParaTimes = defaultNet.ParaTimes
		clonedDefault = true
		break
	}

	if symbol != "" {
		net.Denomination.Symbol = symbol
	}

	if numDecimals != 0 {
		net.Denomination.Decimals = uint8(numDecimals) //nolint: gosec
	}

	if description != "" {
		net.Description = description
	}

	// If we failed to crib details from a hardcoded config,
	// and user did not set -y flag ask the user.
	if !clonedDefault && !common.GetAnswerYes() {
		networkDetailsFromSurvey(&net)
	}

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

	symbolFlag := flag.NewFlagSet("", flag.ContinueOnError)
	symbolFlag.StringVar(&symbol, "symbol", "", "network's symbol")
	addLocalCmd.Flags().AddFlagSet(symbolFlag)

	numDecimalsFlag := flag.NewFlagSet("", flag.ContinueOnError)
	numDecimalsFlag.UintVar(&numDecimals, "num-decimals", 0, "network's number of decimals")
	addLocalCmd.Flags().AddFlagSet(numDecimalsFlag)

	descriptionFlag := flag.NewFlagSet("", flag.ContinueOnError)
	descriptionFlag.StringVar(&description, "description", "", "network's description")
	addLocalCmd.Flags().AddFlagSet(descriptionFlag)
}
