package network

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	consensusPretty "github.com/oasisprotocol/oasis-core/go/common/prettyprint"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-core/go/staking/api/token"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/table"
)

type propertySelector int

const (
	selInvalid propertySelector = iota
	selEntities
	selNodes
	selRuntimes
	selValidators
	selNativeToken
	selGasCosts
	selCommittees
	selParameters
)

var showCmd = &cobra.Command{
	Use:     "show { <id> | committees | entities | gas-costs | native-token | nodes | parameters | paratimes | validators }",
	Short:   "Show network properties",
	Long:    "Show network property stored in the registry, scheduler, genesis document or chain. Query by ID, hash or a specified kind.",
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"s"},
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)

		id, err := parseIdentifier(npa, args[0])
		cobra.CheckErr(err)

		// Establish connection with the target network.
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		consensusConn := conn.Consensus()
		registryConn := consensusConn.Registry()
		roothashConn := consensusConn.RootHash()

		// Figure out the height to use if "latest".
		height, err := common.GetActualHeight(
			ctx,
			consensusConn,
		)
		cobra.CheckErr(err)

		// This command just takes a brute-force "do-what-I-mean" approach
		// and queries everything it can till it finds what the user is
		// looking for.

		prettyPrint := func(b interface{}) error {
			data, err := json.MarshalIndent(b, "", "  ")
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", data)
			return nil
		}

		switch v := id.(type) {
		case signature.PublicKey:
			idQuery := &registry.IDQuery{
				Height: height,
				ID:     v,
			}

			if entity, err := registryConn.GetEntity(ctx, idQuery); err == nil {
				err = prettyPrint(entity)
				cobra.CheckErr(err)
				return
			}

			if nodeStatus, err := registryConn.GetNodeStatus(ctx, idQuery); err == nil {
				if node, err2 := registryConn.GetNode(ctx, idQuery); err2 == nil {
					err = prettyPrint(node)
					cobra.CheckErr(err)
				}

				err = prettyPrint(nodeStatus)
				cobra.CheckErr(err)
				return
			}

			nsQuery := &registry.GetRuntimeQuery{
				Height: height,
			}
			copy(nsQuery.ID[:], v[:])

			if runtime, err := registryConn.GetRuntime(ctx, nsQuery); err == nil {
				err = prettyPrint(runtime)
				cobra.CheckErr(err)
				return
			}
		case *types.Address:
			addr := staking.Address(*v)

			entities, err := registryConn.GetEntities(ctx, height)
			cobra.CheckErr(err) // If this doesn't work the other large queries won't either.
			for _, entity := range entities {
				if staking.NewAddress(entity.ID).Equal(addr) {
					err = prettyPrint(entity)
					cobra.CheckErr(err)
					return
				}
			}

			nodes, err := registryConn.GetNodes(ctx, height)
			cobra.CheckErr(err)
			for _, node := range nodes {
				if staking.NewAddress(node.ID).Equal(addr) {
					err = prettyPrint(node)
					cobra.CheckErr(err)
					return
				}
			}

			// Probably don't need to bother querying the runtimes by address.
		case propertySelector:
			switch v {
			case selEntities:
				entities, err := registryConn.GetEntities(ctx, height)
				cobra.CheckErr(err)
				for _, entity := range entities {
					err = prettyPrint(entity)
					cobra.CheckErr(err)
				}
				return
			case selNodes:
				nodes, err := registryConn.GetNodes(ctx, height)
				cobra.CheckErr(err)
				for _, node := range nodes {
					err = prettyPrint(node)
					cobra.CheckErr(err)
				}
				return
			case selRuntimes:
				runtimes, err := registryConn.GetRuntimes(ctx, &registry.GetRuntimesQuery{
					Height:           height,
					IncludeSuspended: true,
				})
				cobra.CheckErr(err)
				for _, runtime := range runtimes {
					err = prettyPrint(runtime)
					cobra.CheckErr(err)
				}
				return
			case selValidators:
				schedulerConn := consensusConn.Scheduler()
				validators, err := schedulerConn.GetValidators(ctx, height)
				cobra.CheckErr(err)
				for _, validator := range validators {
					err = prettyPrint(validator)
					cobra.CheckErr(err)
				}
				return
			case selNativeToken:
				stakingConn := consensusConn.Staking()
				showNativeToken(ctx, height, npa, stakingConn)
				return
			case selGasCosts:
				stakingConn := consensusConn.Staking()
				consensusParams, err := stakingConn.ConsensusParameters(ctx, height)
				cobra.CheckErr(err)

				fmt.Printf("Gas costs for network %s:", npa.PrettyPrintNetwork())
				fmt.Println()

				// Print costs ordered by kind.
				kinds := make([]string, 0, len(consensusParams.GasCosts))
				for k := range consensusParams.GasCosts {
					kinds = append(kinds, string(k))
				}
				sort.Strings(kinds)
				for _, k := range kinds {
					fmt.Printf("  - %-26s %d", k+":", consensusParams.GasCosts[transaction.Op(k)])
					fmt.Println()
				}
				return
			case selCommittees:
				runtimes, err := registryConn.GetRuntimes(ctx, &registry.GetRuntimesQuery{
					Height:           height,
					IncludeSuspended: false,
				})
				cobra.CheckErr(err)

				for _, runtime := range runtimes {
					if runtime.Kind != registry.KindCompute {
						continue
					}
					table := table.New()
					table.SetHeader([]string{"Entity ID", "Node ID", "Role"})

					runtimeID := runtime.ID
					paratimeName := getParatimeName(cfg, runtimeID.String())

					fmt.Println("=== COMMITTEE ===")
					fmt.Printf("Paratime: %s(%s)\n", paratimeName, runtimeID)
					fmt.Printf("Height:   %d\n", height)
					fmt.Println()

					state, _ := roothashConn.GetRuntimeState(ctx, &roothash.RuntimeRequest{
						Height:    height,
						RuntimeID: runtimeID,
					})
					cobra.CheckErr(err)

					var output [][]string
					for _, member := range state.Committee.Members {
						nodeQuery := &registry.IDQuery{
							Height: height,
							ID:     member.PublicKey,
						}

						node, err := consensusConn.Registry().GetNode(ctx, nodeQuery)
						cobra.CheckErr(err)

						output = append(output, []string{
							node.EntityID.String(),
							member.PublicKey.String(),
							member.Role.String(),
						})
					}

					table.AppendBulk(output)
					table.Render()
					fmt.Println()
				}
				return
			case selParameters:
				showParameters(ctx, npa, height, consensusConn)
				return

			default:
				// Should never happen.
			}
		}

		cobra.CheckErr(fmt.Errorf("id '%s' not found", id))
	},
}

func parseIdentifier(
	npa *common.NPASelection,
	s string,
) (interface{}, error) { // TODO: Use `any`
	if sel := selectorFromString(s); sel != selInvalid {
		return sel, nil
	}

	addr, _, err := common.ResolveAddress(npa.Network, s)
	if err == nil {
		return addr, nil
	}

	var pk signature.PublicKey
	if err = pk.UnmarshalText([]byte(s)); err == nil {
		return pk, nil
	}
	if err = pk.UnmarshalHex(s); err == nil {
		return pk, nil
	}

	return nil, fmt.Errorf("unrecognized id: '%s'", s)
}

func selectorFromString(s string) propertySelector {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "entities":
		return selEntities
	case "nodes":
		return selNodes
	case "paratimes", "runtimes":
		return selRuntimes
	case "validators":
		return selValidators
	case "native-token":
		return selNativeToken
	case "gas-costs":
		return selGasCosts
	case "committees":
		return selCommittees
	case "parameters":
		return selParameters
	}
	return selInvalid
}

func showNativeToken(ctx context.Context, height int64, npa *common.NPASelection, stakingConn staking.Backend) {
	fmt.Printf("%-25s %s", "Network:", npa.PrettyPrintNetwork())
	fmt.Println()

	tokenSymbol, err := stakingConn.TokenSymbol(ctx, height)
	cobra.CheckErr(err)
	tokenValueExponent, err := stakingConn.TokenValueExponent(ctx, height)
	cobra.CheckErr(err)

	ctx = context.WithValue(
		ctx,
		consensusPretty.ContextKeyTokenSymbol,
		tokenSymbol,
	)
	ctx = context.WithValue(
		ctx,
		consensusPretty.ContextKeyTokenValueExponent,
		tokenValueExponent,
	)

	fmt.Printf("%-25s %s", "Token's ticker symbol:", tokenSymbol)
	fmt.Println()
	fmt.Printf("%-25s %d", "Token's base-10 exponent:", tokenValueExponent)
	fmt.Println()

	totalSupply, err := stakingConn.TotalSupply(ctx, height)
	cobra.CheckErr(err)
	fmt.Printf("%-25s ", "Total supply:")
	token.PrettyPrintAmount(ctx, *totalSupply, os.Stdout)
	fmt.Println()

	commonPool, err := stakingConn.CommonPool(ctx, height)
	cobra.CheckErr(err)
	fmt.Printf("%-25s ", "Common pool:")
	token.PrettyPrintAmount(ctx, *commonPool, os.Stdout)
	fmt.Println()

	lastBlockFees, err := stakingConn.LastBlockFees(ctx, height)
	cobra.CheckErr(err)
	fmt.Printf("%-25s ", "Last block fees:")
	token.PrettyPrintAmount(ctx, *lastBlockFees, os.Stdout)
	fmt.Println()

	governanceDeposits, err := stakingConn.GovernanceDeposits(ctx, height)
	cobra.CheckErr(err)
	fmt.Printf("%-25s ", "Governance deposits:")
	token.PrettyPrintAmount(ctx, *governanceDeposits, os.Stdout)
	fmt.Println()

	consensusParams, err := stakingConn.ConsensusParameters(ctx, height)
	cobra.CheckErr(err)

	fmt.Printf("%-25s %d epoch(s)", "Debonding interval:", consensusParams.DebondingInterval)
	fmt.Println()

	fmt.Println("\n=== STAKING THRESHOLDS ===")
	thresholdsToQuery := []staking.ThresholdKind{
		staking.KindEntity,
		staking.KindNodeValidator,
		staking.KindNodeCompute,
		staking.KindNodeKeyManager,
		staking.KindRuntimeCompute,
		staking.KindRuntimeKeyManager,
	}
	for _, kind := range thresholdsToQuery {
		threshold, err := stakingConn.Threshold(
			ctx,
			&staking.ThresholdQuery{
				Kind:   kind,
				Height: height,
			},
		)
		cobra.CheckErr(err)
		fmt.Printf("  %-19s ", kind.String()+":")
		token.PrettyPrintAmount(ctx, *threshold, os.Stdout)
		fmt.Println()
	}
}

func showParameters(ctx context.Context, npa *common.NPASelection, height int64, cons consensus.ClientBackend) {
	checkErr := func(what string, err error) {
		if err != nil {
			cobra.CheckErr(fmt.Errorf("%s: %w", what, err))
		}
	}

	// Get these two from the genesis document, since cons.GetParameters is
	// not allowed on the public grpc node and the keymanager would require a
	// ServicesBackend instead of the ClientBackend that the Oasis Client SDK
	// provides.
	genesisDoc, err := cons.GetGenesisDocument(ctx)
	checkErr("GetGenesisDocument", err)
	consensusParams := genesisDoc.Consensus
	keymanagerParams := genesisDoc.KeyManager

	// Get live consensus parameters from all the other backends.
	registryParams, err := cons.Registry().ConsensusParameters(ctx, height)
	checkErr("Registry", err)

	roothashParams, err := cons.RootHash().ConsensusParameters(ctx, height)
	checkErr("RootHash", err)

	stakingParams, err := cons.Staking().ConsensusParameters(ctx, height)
	checkErr("Staking", err)

	schedulerParams, err := cons.Scheduler().ConsensusParameters(ctx, height)
	checkErr("Scheduler", err)

	beaconParams, err := cons.Beacon().ConsensusParameters(ctx, height)
	checkErr("Beacon", err)

	governanceParams, err := cons.Governance().ConsensusParameters(ctx, height)
	checkErr("Governance", err)

	doc := make(map[string]interface{})

	doSection := func(name string, params interface{}) {
		if common.OutputFormat() == common.FormatJSON {
			doc[name] = params
		} else {
			fmt.Printf("=== %s PARAMETERS ===\n", strings.ToUpper(name))
			out := common.PrettyPrint(npa, "  ", params)
			fmt.Printf("%s\n", out)
			fmt.Println()
		}
	}

	doSection("consensus", consensusParams)
	doSection("keymanager", keymanagerParams)
	doSection("registry", registryParams)
	doSection("roothash", roothashParams)
	doSection("staking", stakingParams)
	doSection("scheduler", schedulerParams)
	doSection("beacon", beaconParams)
	doSection("governance", governanceParams)

	if common.OutputFormat() == common.FormatJSON {
		pp, err := json.MarshalIndent(doc, "", "  ")
		cobra.CheckErr(err)
		fmt.Printf("%s\n", pp)
	}
}

func init() {
	showCmd.Flags().AddFlagSet(common.SelectorNFlags)
	showCmd.Flags().AddFlagSet(common.HeightFlag)
	showCmd.Flags().AddFlagSet(common.FormatFlag)
}
