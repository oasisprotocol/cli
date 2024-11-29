package wallet

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature/signers/remote"
	"github.com/oasisprotocol/oasis-core/go/common/grpc"
	"github.com/oasisprotocol/oasis-core/go/common/identity"
	"github.com/oasisprotocol/oasis-core/go/common/logging"
	"github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/common/background"

	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
)

var remoteSignerCmd = &cobra.Command{
	Use:   "remote-signer <name> <socket-path>",
	Short: "Act as a oasis-node remote entity signer over AF_LOCAL",
	Args:  cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		name, socketPath := args[0], args[1]

		acc := common.LoadAccount(config.Global(), name)

		sf := &accountEntitySignerFactory{
			signer: acc.ConsensusSigner(),
		}
		if sf.signer == nil {
			cobra.CheckErr("account not compatible with consensus layer usage")
		}

		// The domain separation is entirely handled on the client side.
		signature.UnsafeAllowUnregisteredContexts()

		// Suppress oasis-core logging.
		err := logging.Initialize(
			nil,
			logging.FmtLogfmt,
			logging.LevelInfo,
			nil,
		)
		cobra.CheckErr(err)

		// Setup the gRPC service.
		srvCfg := &grpc.ServerConfig{
			Name:     "remote-signer",
			Path:     socketPath, // XXX: Maybe fix this up to be nice.
			Identity: &identity.Identity{},
		}
		srv, err := grpc.NewServer(srvCfg)
		cobra.CheckErr(err)
		remote.RegisterService(srv.Server(), sf)

		// Start the service and wait for graceful termination.
		err = srv.Start()
		cobra.CheckErr(err)

		fmt.Printf("Address: %s\n", acc.Address())
		fmt.Printf("Node Args:\n  --signer.backend=remote \\\n  --signer.remote.address=unix:%s\n", socketPath)
		fmt.Printf("\n*** REMOTE SIGNER READY ***\n")

		sm := background.NewServiceManager(logging.GetLogger("remote-signer"))
		sm.Register(srv)
		defer sm.Cleanup()
		sm.Wait()
	},
}
