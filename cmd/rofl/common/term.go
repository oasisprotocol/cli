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

// FormatTerm formats a roflmarket.Term into a human-readable string.
func FormatTerm(term roflmarket.Term) string {
	switch term {
	case roflmarket.TermHour:
		return TermHour
	case roflmarket.TermMonth:
		return TermMonth
	case roflmarket.TermYear:
		return TermYear
	default:
		return fmt.Sprintf("<unknown: %d>", term)
	}
}

// FormatTermAdjectival formats a roflmarket.Term into an adjectival form (e.g., "hourly").
func FormatTermAdjectival(term roflmarket.Term) string {
	switch term {
	case roflmarket.TermHour:
		return "hourly"
	case roflmarket.TermMonth:
		return "monthly"
	case roflmarket.TermYear:
		return "yearly"
	default:
		return fmt.Sprintf("term_%d", term)
	}
}

// FormatTeeType formats a roflmarket.TeeType into a human-readable string.
func FormatTeeType(tee roflmarket.TeeType) string {
	switch tee {
	case roflmarket.TeeTypeSGX:
		return "sgx"
	case roflmarket.TeeTypeTDX:
		return "tdx"
	default:
		return fmt.Sprintf("<unknown: %d>", tee)
	}
}
