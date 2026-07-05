package rofl

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	buildDotenv "github.com/oasisprotocol/cli/build/dotenv"
	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
)

const publicVarMetadataPrefix = "env."

var (
	publicVarCmd = &cobra.Command{
		Use:   "public-var",
		Short: "Public variable management commands",
		Long: "Public variable management commands.\n\n" +
			"Values are stored unencrypted in ROFL app metadata and exposed to containers as\n" +
			"environment variables. Use `oasis rofl secret` for confidential values.",
	}

	publicVarSetCmd = &cobra.Command{
		Use:   "set <name> <file>|-",
		Short: "Set a public variable in the manifest, reading the value from file or stdin",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			publicVarName := args[0]
			publicVarFn := args[1]

			manifest, deployment, _ := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: false,
				NeedAdmin: false,
			})

			key, err := publicVarMetadataKey(publicVarName)
			if err != nil {
				cobra.CheckErr(err)
			}
			cobra.CheckErr(checkNoSecretNamed(deployment.Secrets, publicVarName))

			// Read public variable.
			var publicVarValueRaw []byte
			if publicVarFn == "-" {
				publicVarValueRaw, err = io.ReadAll(os.Stdin)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to read public variable from standard input: %w", err))
				}
			} else {
				publicVarValueRaw, err = os.ReadFile(publicVarFn)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to read public variable from file: %w", err))
				}
			}
			publicVarValue, err := parsePublicVarValue(publicVarValueRaw)
			if err != nil {
				cobra.CheckErr(err)
			}

			if existing, ok := deployment.Metadata[key]; ok {
				if existing == publicVarValue {
					fmt.Printf("Public variable '%s' did not change; no update needed.\n", publicVarName)
					return
				}
				common.CheckForceErr(fmt.Errorf("the public variable named '%s' for deployment '%s' already exists", publicVarName, roflCommon.DeploymentName))
			}
			if deployment.Metadata == nil {
				deployment.Metadata = make(map[string]string)
			}
			deployment.Metadata[key] = publicVarValue

			// Update manifest.
			if err = manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}

			fmt.Printf("Run `oasis rofl update` to update your ROFL app's on-chain configuration.\n")
		},
	}

	// publicVarImportCmd bulk-imports public variables from a .env file (key=value with # comments).
	// Supports '-' to read from stdin. Existing public variables are replaced only with --force.
	publicVarImportCmd = &cobra.Command{
		Use:   "import <dot-env-file>|-",
		Short: "Import multiple public variables from a .env file",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			envFn := args[0]

			manifest, deployment, _ := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: false,
				NeedAdmin: false,
			})

			// Read .env data (supports stdin via "-").
			var (
				raw []byte
				err error
			)
			if envFn == "-" {
				raw, err = io.ReadAll(os.Stdin)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to read .env from standard input: %w", err))
				}
			} else {
				raw, err = os.ReadFile(envFn)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to read .env file: %w", err))
				}
			}

			entries, err := buildDotenv.Parse(string(raw))
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to parse .env: %w", err))
			}
			if len(entries) == 0 {
				fmt.Println("No key=value pairs found in the provided .env input.")
				return
			}

			// Deterministic iteration for stable updates.
			names := make([]string, 0, len(entries))
			for k := range entries {
				names = append(names, k)
			}
			sort.Strings(names)

			if deployment.Metadata == nil {
				deployment.Metadata = make(map[string]string)
			}

			var imported, updated int
			for _, name := range names {
				key, err := publicVarMetadataKey(name)
				if err != nil {
					cobra.CheckErr(err)
				}
				cobra.CheckErr(checkNoSecretNamed(deployment.Secrets, name))

				if existing, ok := deployment.Metadata[key]; ok {
					if existing == entries[name] {
						continue
					}
					common.CheckForceErr(fmt.Errorf("the public variable named '%s' for deployment '%s' already exists", name, roflCommon.DeploymentName))
					updated++
				} else {
					imported++
				}
				deployment.Metadata[key] = entries[name]
			}

			src := envFn
			if envFn == "-" {
				src = "stdin"
			}
			if imported == 0 && updated == 0 {
				fmt.Printf("No public variables were imported or updated using '%s'.\n", src)
				return
			}

			// Update manifest.
			if err = manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}

			fmt.Printf("Imported %d public variables, updated %d existing from '%s'.\n", imported, updated, src)
			fmt.Printf("Run `oasis rofl update` to update your ROFL app's on-chain configuration.\n")
		},
	}

	publicVarGetCmd = &cobra.Command{
		Use:   "get <name>",
		Short: "Show the given public variable",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			publicVarName := args[0]

			_, deployment, _ := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: false,
				NeedAdmin: false,
			})

			key, err := publicVarMetadataKey(publicVarName)
			if err != nil {
				cobra.CheckErr(err)
			}
			value, ok := deployment.Metadata[key]
			if !ok {
				cobra.CheckErr(fmt.Errorf("public variable named '%s' does not exist for deployment '%s'", publicVarName, roflCommon.DeploymentName))
				return // Lint doesn't know that cobra.CheckErr never returns.
			}

			fmt.Printf("Name:        %s\n", publicVarName)
			fmt.Printf("Value:       %s\n", value)
			fmt.Printf("Size:        %d bytes\n", len(value))
		},
	}

	publicVarRmCmd = &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove the given public variable from the manifest",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			publicVarName := args[0]

			manifest, deployment, _ := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: false,
				NeedAdmin: false,
			})

			key, err := publicVarMetadataKey(publicVarName)
			if err != nil {
				cobra.CheckErr(err)
			}
			if _, ok := deployment.Metadata[key]; !ok {
				cobra.CheckErr(fmt.Errorf("public variable named '%s' does not exist for deployment '%s'", publicVarName, roflCommon.DeploymentName))
			}
			delete(deployment.Metadata, key)
			if len(deployment.Metadata) == 0 {
				deployment.Metadata = nil
			}

			// Update manifest.
			if err := manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}

			fmt.Printf("Run `oasis rofl update` to update your ROFL app's on-chain configuration.\n")
		},
	}
)

func publicVarMetadataKey(name string) (string, error) {
	if err := validatePublicVarName(name); err != nil {
		return "", err
	}
	return publicVarMetadataPrefix + name, nil
}

// checkNoSecretNamed returns an error if a secret exposed under the given environment variable
// name exists among the secrets.
func checkNoSecretNamed(secrets []*buildRofl.SecretConfig, name string) error {
	for _, sc := range secrets {
		if secretEnvVarName(sc.Name) == name {
			return fmt.Errorf("the secret named '%s' for deployment '%s' already exists", sc.Name, roflCommon.DeploymentName)
		}
	}
	return nil
}

// checkNoPublicVarNamed returns an error if a public variable exposed under the same environment
// variable name as a secret with the given name exists in the deployment metadata.
func checkNoPublicVarNamed(metadata map[string]string, name string) error {
	envName := secretEnvVarName(name)
	if _, ok := metadata[publicVarMetadataPrefix+envName]; ok {
		return fmt.Errorf("the public variable named '%s' for deployment '%s' already exists", envName, roflCommon.DeploymentName)
	}
	return nil
}

// secretEnvVarName returns the environment variable name under which rofl-containers exposes a
// secret with the given name.
func secretEnvVarName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, " ", "_"))
}

func validatePublicVarName(name string) error {
	if name == "" {
		return fmt.Errorf("public variable name cannot be empty")
	}
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			continue
		}
		return fmt.Errorf("public variable name '%s' is invalid, only ASCII letters, digits and '_' are allowed", name)
	}
	return nil
}

func parsePublicVarValue(raw []byte) (string, error) {
	if !utf8.Valid(raw) {
		return "", fmt.Errorf("public variable values must be valid UTF-8")
	}
	return string(raw), nil
}

func init() {
	publicVarSetCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	publicVarSetCmd.Flags().AddFlagSet(common.ForceFlag)
	publicVarCmd.AddCommand(publicVarSetCmd)

	publicVarImportCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	publicVarImportCmd.Flags().AddFlagSet(common.ForceFlag)
	publicVarCmd.AddCommand(publicVarImportCmd)

	publicVarGetCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	publicVarCmd.AddCommand(publicVarGetCmd)

	publicVarRmCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	publicVarCmd.AddCommand(publicVarRmCmd)
}
