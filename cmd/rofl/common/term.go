package common

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
)

// Machine payment terms.
const (
	TermHour  = "hour"
	TermMonth = "month"
	TermYear  = "year"
)

// ParseMachineTerm parses the given machine payment term.
//
// Terminates the process in case of errors.
func ParseMachineTerm(term string) roflmarket.Term {
	switch term {
	case TermHour:
		return roflmarket.TermHour
	case TermMonth:
		return roflmarket.TermMonth
	case TermYear:
		return roflmarket.TermYear
	default:
		cobra.CheckErr(fmt.Sprintf("invalid machine payment term: %s", term))
		return 0
	}
}
