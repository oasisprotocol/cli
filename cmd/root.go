package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/oasisprotocol/cli/cmd/account"
	"github.com/oasisprotocol/cli/cmd/network"
	"github.com/oasisprotocol/cli/cmd/paratime"
	"github.com/oasisprotocol/cli/cmd/rofl"
	"github.com/oasisprotocol/cli/cmd/wallet"
	"github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/version"
	_ "github.com/oasisprotocol/cli/wallet/file"   // Register file wallet backend.
	_ "github.com/oasisprotocol/cli/wallet/ledger" // Register ledger wallet backend.
)

var (
	cfgFile string

	rootCmd = &cobra.Command{
		Use:     "oasis",
		Short:   "CLI for interacting with the Oasis network",
		Version: version.Software,
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func initVersions() {
	cobra.AddTemplateFunc("toolchain", func() interface{} { return version.Toolchain })
	cobra.AddTemplateFunc("sdk", func() interface{} { return version.GetOasisSDKVersion() })
	cobra.AddTemplateFunc("core", func() interface{} { return version.GetOasisCoreVersion() })

	rootCmd.SetVersionTemplate(`Software version: {{.Version}}
Oasis SDK version: {{ sdk }}
Oasis Core version: {{ core }}
Go toolchain version: {{ toolchain }}
`)
}

func initConfig() {
	v := viper.New()

	if cfgFile != "" {
		// Use config file from the flag.
		v.SetConfigFile(cfgFile)
	} else {
		const configFilename = "cli.toml"
		configDir := config.DefaultDirectory()
		configPath := filepath.Join(configDir, configFilename)

		v.AddConfigPath(configDir)
		v.SetConfigType("toml")
		v.SetConfigName(configFilename)

		// Ensure the configuration file exists.
		_ = os.MkdirAll(configDir, 0o700)
		if _, err := os.Stat(configPath); errors.Is(err, fs.ErrNotExist) {
			if _, err := os.Create(configPath); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to create configuration file: %w", err))
			}

			// Populate the initial configuration file with defaults.
			config.ResetDefaults()
			_ = config.Save(v)
		}
	}

	_ = v.ReadInConfig()

	// Load and validate global configuration.
	err := config.Load(v)
	cobra.CheckErr(err)
	changes, err := config.Global().Migrate()
	cobra.CheckErr(err)
	if changes {
		err = config.Global().Save()
		cobra.CheckErr(err)
	}
	err = config.Global().Validate()
	cobra.CheckErr(err)
}

func init() {
	initVersions()

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file to use")

	rootCmd.AddCommand(network.Cmd)
	rootCmd.AddCommand(paratime.Cmd)
	rootCmd.AddCommand(wallet.Cmd)
	rootCmd.AddCommand(account.Cmd)
	rootCmd.AddCommand(addressBookCmd)
	rootCmd.AddCommand(contractCmd)
	rootCmd.AddCommand(txCmd)
	rootCmd.AddCommand(rofl.Cmd)
}
