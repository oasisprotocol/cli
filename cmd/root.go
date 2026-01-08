package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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

// isCompletionCommand checks if the CLI is being invoked for shell completion.
// This is used to skip side effects (file creation, migrations) during tab-completion.
// We check only the first non-flag argument to avoid false positives when a
// positional argument happens to be named "completion" (e.g., an address book entry).
func isCompletionCommand() bool {
	skipNext := false
	for _, arg := range os.Args[1:] {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			// Skip flag, and if it's not --flag=value format, skip the next arg too.
			if !strings.Contains(arg, "=") {
				skipNext = true
			}
			continue
		}
		// First non-flag arg is the subcommand.
		return arg == "completion" || arg == "__complete" || arg == "__completeNoDesc"
	}
	return false
}

// ensureConfigExists creates the config file with defaults if it doesn't exist.
func ensureConfigExists(v *viper.Viper, configPath string) {
	if _, err := os.Stat(configPath); !errors.Is(err, fs.ErrNotExist) {
		return
	}
	if _, err := os.Create(configPath); err != nil {
		cobra.CheckErr(fmt.Errorf("failed to create configuration file: %w", err))
	}
	config.ResetDefaults()
	_ = config.Save(v)
}

func initConfig() {
	v := viper.New()
	completionMode := isCompletionCommand()

	// cfgFile is set by cobra flag parsing, but OnInitialize runs before
	// flags are parsed. For completion mode, manually check os.Args.
	configPath := cfgFile
	if configPath == "" && completionMode {
		for i, arg := range os.Args {
			if arg == "--config" && i+1 < len(os.Args) {
				configPath = os.Args[i+1]
				break
			}
			if strings.HasPrefix(arg, "--config=") {
				configPath = strings.TrimPrefix(arg, "--config=")
				break
			}
		}
	}
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		const configFilename = "cli.toml"
		configDir := config.DefaultDirectory()

		v.AddConfigPath(configDir)
		v.SetConfigType("toml")
		v.SetConfigName(configFilename)

		// Skip file creation during completion to avoid side effects.
		if !completionMode {
			_ = os.MkdirAll(configDir, 0o700)
			ensureConfigExists(v, filepath.Join(configDir, configFilename))
		}
	}

	if err := v.ReadInConfig(); err != nil {
		// If config file doesn't exist and we're in completion mode,
		// use defaults so completions still show default networks/paratimes.
		if completionMode {
			config.ResetDefaults()
			return
		}
	}

	// Load global configuration.
	err := config.Load(v)
	cobra.CheckErr(err)

	// Skip migrations and validation during completion to avoid side effects.
	if completionMode {
		return
	}

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
