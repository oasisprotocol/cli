package common

import (
	"context"
	"fmt"

	flag "github.com/spf13/pflag"

	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
)

var (
	selectedHeight int64
	force          bool
)

// HeightFlag is the flag for specifying block height.
var HeightFlag *flag.FlagSet

// ForceFlag is a force mode switch.
var ForceFlag *flag.FlagSet

// GetHeight returns the user-selected block height.
func GetHeight() int64 {
	return selectedHeight
}

// IsForce returns force mode.
func IsForce() bool {
	return force
}

// GetActualHeight returns the user-selected block height if explicitly
// specified, or the current latest height.
func GetActualHeight(
	ctx context.Context,
	consensusConn consensus.ClientBackend,
) (int64, error) {
	height := GetHeight()
	if height != consensus.HeightLatest {
		return height, nil
	}
	blk, err := consensusConn.GetBlock(ctx, height)
	if err != nil {
		return 0, fmt.Errorf("failed to query current height: %w", err)
	}
	return blk.Height, nil
}

func init() {
	HeightFlag = flag.NewFlagSet("", flag.ContinueOnError)
	HeightFlag.Int64Var(&selectedHeight, "height", consensus.HeightLatest, "explicitly set block height to use")

	ForceFlag = flag.NewFlagSet("", flag.ContinueOnError)
	ForceFlag.BoolVarP(&force, "force", "f", false, "treat safety check errors as warnings")
}
