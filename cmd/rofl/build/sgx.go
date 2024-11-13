package build

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/sgx"
	"github.com/oasisprotocol/oasis-core/go/common/sgx/sigstruct"
	"github.com/oasisprotocol/oasis-core/go/common/version"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle/component"

	"github.com/oasisprotocol/cli/build/cargo"
	"github.com/oasisprotocol/cli/build/sgxs"
	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	sgxHeapSize  uint64
	sgxStackSize uint64
	sgxThreads   uint64

	sgxCmd = &cobra.Command{
		Use:   "sgx",
		Short: "Build an SGX-based Rust ROFL application",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			if npa.ParaTime == nil {
				cobra.CheckErr("no ParaTime selected")
			}

			// For SGX we currently only support Rust applications.

			fmt.Println("Building an SGX-based Rust ROFL application...")

			detectBuildMode(npa)
			features := sgxSetupBuildEnv()

			// Obtain package metadata.
			pkgMeta, err := cargo.GetMetadata()
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to obtain package metadata: %w", err))
			}

			// Start creating the bundle early so we can fail before building anything.
			bnd := &bundle.Bundle{
				Manifest: &bundle.Manifest{
					Name: pkgMeta.Name,
					ID:   npa.ParaTime.Namespace(),
				},
			}
			bnd.Manifest.Version, err = version.FromString(pkgMeta.Version)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("unsupported package version format: %w", err))
			}

			fmt.Printf("Name:    %s\n", bnd.Manifest.Name)
			fmt.Printf("Version: %s\n", bnd.Manifest.Version)

			// First build for the default target.
			fmt.Println("Building ELF binary...")
			elfPath, err := cargo.Build(true, "x86_64-unknown-linux-gnu", features)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to build ELF binary: %w", err))
			}

			// Then build for the SGX target.
			fmt.Println("Building SGXS binary...")
			elfSgxPath, err := cargo.Build(true, "x86_64-fortanix-unknown-sgx", nil)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to build SGXS binary: %w", err))
			}

			sgxsPath := fmt.Sprintf("%s.sgxs", elfSgxPath)
			err = sgxs.Elf2Sgxs(elfSgxPath, sgxsPath, sgxHeapSize, sgxStackSize, sgxThreads)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to generate SGXS binary: %w", err))
			}

			// Compute MRENCLAVE.
			var b []byte
			if b, err = os.ReadFile(sgxsPath); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to read SGXS binary: %w", err))
			}
			var enclaveHash sgx.MrEnclave
			if err = enclaveHash.FromSgxsBytes(b); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to compute MRENCLAVE for SGXS binary: %w", err))
			}

			fmt.Println("Creating ORC bundle...")

			// Create a random 3072-bit RSA signer and prepare SIGSTRUCT.
			// TODO: Support a specific signer to be set.
			sigKey, err := sgxGenerateKey(rand.Reader)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to generate signer key: %w", err))
			}
			sigStruct := sigstruct.New(
				sigstruct.WithBuildDate(time.Now()),
				sigstruct.WithSwDefined([4]byte{0, 0, 0, 0}),
				sigstruct.WithISVProdID(0),
				sigstruct.WithISVSVN(0),

				sigstruct.WithMiscSelect(0),
				sigstruct.WithMiscSelectMask(^uint32(0)),

				sigstruct.WithAttributes(sgx.Attributes{
					Flags: sgx.AttributeMode64Bit,
					Xfrm:  3,
				}),
				sigstruct.WithAttributesMask([2]uint64{
					^uint64(2),
					^uint64(3),
				}),

				sigstruct.WithEnclaveHash(enclaveHash),
			)
			sigData, err := sigStruct.Sign(sigKey)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to sign SIGSTRUCT: %w", err))
			}

			// Add the ROFL component.
			execName := "app.elf"
			sgxsName := "app.sgxs"
			sigName := "app.sig"

			comp := bundle.Component{
				Kind:       component.ROFL,
				Name:       pkgMeta.Name,
				Executable: execName,
				SGX: &bundle.SGXMetadata{
					Executable: sgxsName,
					Signature:  sigName,
				},
			}
			bnd.Manifest.Components = append(bnd.Manifest.Components, &comp)

			if err = bnd.Manifest.Validate(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to validate manifest: %w", err))
			}

			// Add all files.
			fileMap := map[string]string{
				execName: elfPath,
				sgxsName: sgxsPath,
			}
			for dst, src := range fileMap {
				if b, err = os.ReadFile(src); err != nil {
					cobra.CheckErr(fmt.Errorf("failed to load asset '%s': %w", src, err))
				}
				_ = bnd.Add(dst, bundle.NewBytesData(b))
			}
			_ = bnd.Add(sigName, bundle.NewBytesData(sigData))

			// Write the bundle out.
			outFn := fmt.Sprintf("%s.orc", bnd.Manifest.Name)
			if outputFn != "" {
				outFn = outputFn
			}
			if err = bnd.Write(outFn); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to write output bundle: %w", err))
			}

			fmt.Printf("ROFL app built and bundle written to '%s'.\n", outFn)
		},
	}
)

// sgxGenerateKey generates a 3072-bit RSA key with public exponent 3 as required for SGX.
//
// The code below is adopted from the Go standard library as it is otherwise not possible to
// customize the exponent.
func sgxGenerateKey(random io.Reader) (*rsa.PrivateKey, error) {
	priv := new(rsa.PrivateKey)
	priv.E = 3
	bits := 3072
	nprimes := 2

	bigOne := big.NewInt(1)
	primes := make([]*big.Int, nprimes)

NextSetOfPrimes:
	for {
		todo := bits
		// crypto/rand should set the top two bits in each prime.
		// Thus each prime has the form
		//   p_i = 2^bitlen(p_i) × 0.11... (in base 2).
		// And the product is:
		//   P = 2^todo × α
		// where α is the product of nprimes numbers of the form 0.11...
		//
		// If α < 1/2 (which can happen for nprimes > 2), we need to
		// shift todo to compensate for lost bits: the mean value of 0.11...
		// is 7/8, so todo + shift - nprimes * log2(7/8) ~= bits - 1/2
		// will give good results.
		if nprimes >= 7 {
			todo += (nprimes - 2) / 5
		}
		for i := 0; i < nprimes; i++ {
			var err error
			primes[i], err = rand.Prime(random, todo/(nprimes-i))
			if err != nil {
				return nil, err
			}
			todo -= primes[i].BitLen()
		}

		// Make sure that primes is pairwise unequal.
		for i, prime := range primes {
			for j := 0; j < i; j++ {
				if prime.Cmp(primes[j]) == 0 {
					continue NextSetOfPrimes
				}
			}
		}

		n := new(big.Int).Set(bigOne)
		totient := new(big.Int).Set(bigOne)
		pminus1 := new(big.Int)
		for _, prime := range primes {
			n.Mul(n, prime)
			pminus1.Sub(prime, bigOne)
			totient.Mul(totient, pminus1)
		}
		if n.BitLen() != bits {
			// This should never happen for nprimes == 2 because
			// crypto/rand should set the top two bits in each prime.
			// For nprimes > 2 we hope it does not happen often.
			continue NextSetOfPrimes
		}

		priv.D = new(big.Int)
		e := big.NewInt(int64(priv.E))
		ok := priv.D.ModInverse(e, totient)

		if ok != nil {
			priv.Primes = primes
			priv.N = n
			break
		}
	}

	priv.Precompute()
	return priv, nil
}

// sgxSetupBuildEnv sets up the SGX build environment and returns the list of features to enable.
func sgxSetupBuildEnv() []string {
	switch buildMode {
	case buildModeProduction, buildModeAuto:
		// Production builds.
		fmt.Println("Building in production mode.")

		for _, kv := range os.Environ() {
			key, _, _ := strings.Cut(kv, "=")
			if strings.HasPrefix(key, "OASIS_UNSAFE_") {
				os.Unsetenv(key)
			}
		}

		return nil // No features.
	case buildModeUnsafe:
		// Unsafe debug builds.
		fmt.Println("WARNING: Building in UNSAFE DEBUG mode with MOCK SGX.")
		fmt.Println("WARNING: This build will NOT BE DEPLOYABLE outside local test environments.")

		os.Setenv("OASIS_UNSAFE_SKIP_AVR_VERIFY", "1")
		os.Setenv("OASIS_UNSAFE_ALLOW_DEBUG_ENCLAVES", "1")
		os.Setenv("OASIS_UNSAFE_MOCK_SGX", "1")
		os.Unsetenv("OASIS_UNSAFE_SKIP_KM_POLICY")

		return []string{"debug-mock-sgx"}
	default:
		cobra.CheckErr(fmt.Errorf("unsupported build mode: %s", buildMode))
		return nil
	}
}

func init() {
	sgxFlags := flag.NewFlagSet("", flag.ContinueOnError)
	sgxFlags.Uint64Var(&sgxHeapSize, "sgx-heap-size", 512*1024*1024, "SGX enclave heap size")
	sgxFlags.Uint64Var(&sgxStackSize, "sgx-stack-size", 2*1024*1024, "SGX enclave stack size")
	sgxFlags.Uint64Var(&sgxThreads, "sgx-threads", 32, "SGX enclave maximum number of threads")

	sgxCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	sgxCmd.Flags().AddFlagSet(sgxFlags)
}
