package common

import (
	"context"
	"fmt"
	"strings"

	flag "github.com/spf13/pflag"

	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
)

var (
	// HeightFlag is the flag for specifying block height.
	HeightFlag *flag.FlagSet

	// ForceFlag is a force mode switch.
	ForceFlag *flag.FlagSet

	// AnswerYesFlag answers yes to all questions.
	AnswerYesFlag *flag.FlagSet

	// FormatFlag specifies the command's output format (text/json).
	FormatFlag *flag.FlagSet
)

// FormatType specifies the type of format for output of commands.
type FormatType string

// Supported output formats for the format flag.
const (
	// Output plain text.
	FormatText FormatType = "text"
	// Output JSON.
	FormatJSON FormatType = "json"
)

// String returns a string representation of the output format type.
func (f *FormatType) String() string {
	return string(*f)
}

// Set sets the value of the type to the argument given.
func (f *FormatType) Set(v string) error {
	switch strings.ToLower(v) {
	case string(FormatText), string(FormatJSON):
		*f = FormatType(v)
		return nil
	default:
		return fmt.Errorf("unknown output format type, must be one of: %s", strings.Join([]string{string(FormatText), string(FormatJSON)}, ", "))
	}
}

// Type returns the type of the flag.
func (f *FormatType) Type() string {
	return "FormatType"
}

var (
	selectedHeight int64
	force          bool
	answerYes      bool
	outputFormat   = FormatText
)

// GetHeight returns the user-selected block height.
func GetHeight() int64 {
	return selectedHeight
}

// IsForce returns force mode.
func IsForce() bool {
	return force
}

// GetAnswerYes returns whether all interactive questions should be answered
// with yes.
func GetAnswerYes() bool {
	return answerYes
}

// OutputFormat returns the format of the command's output.
func OutputFormat() FormatType {
	return outputFormat
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

	AnswerYesFlag = flag.NewFlagSet("", flag.ContinueOnError)
	AnswerYesFlag.BoolVarP(&answerYes, "yes", "y", false, "answer yes to all questions")

	FormatFlag = flag.NewFlagSet("", flag.ContinueOnError)
	FormatFlag.Var(&outputFormat, "format", "output format ["+strings.Join([]string{string(FormatText), string(FormatJSON)}, ",")+"]")
}
