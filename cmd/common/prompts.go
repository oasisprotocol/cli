package common

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// SurveyStdio is the standard survey option to direct prompts to stderr.
var SurveyStdio = survey.WithStdio(os.Stdin, os.Stderr, os.Stderr)

// Ask wraps survey.AskOne while forcing prompts to stderr.
func Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	return survey.AskOne(p, response, append([]survey.AskOpt{SurveyStdio}, opts...)...)
}

// AskMulti wraps survey.Ask while forcing prompts to stderr.
func AskMulti(qs []*survey.Question, response interface{}, opts ...survey.AskOpt) error {
	return survey.Ask(qs, response, append([]survey.AskOpt{SurveyStdio}, opts...)...)
}

var (
	// PromptPassphrase is the standard passphrase prompt.
	PromptPassphrase = &survey.Password{
		Message: "Passphrase:",
	}

	// PromptCreatePassphrase is the standard create a new passphrase prompt.
	PromptCreatePassphrase = &survey.Password{
		Message: "Choose a new passphrase:",
	}

	// PromptRepeatPassphrase is the standard repeat a new passphrase prompt.
	PromptRepeatPassphrase = &survey.Password{
		Message: "Repeat passphrase:",
	}
)

// Confirm asks the user for confirmation and aborts when rejected.
func Confirm(msg, abortMsg string) {
	if answerYes {
		fmt.Fprintf(os.Stderr, "? %s Yes\n", msg)
		return
	}

	var proceed bool
	err := Ask(&survey.Confirm{Message: msg}, &proceed)
	cobra.CheckErr(err)
	if !proceed {
		cobra.CheckErr(abortMsg)
	}
}

// DenominationDetails holds the editable description/symbol/decimal-places fields used by the
// network add, add-local, and paratime add commands.
type DenominationDetails struct {
	Description string
	Symbol      string
	Decimals    uint8
}

// AskDenominationDetails prompts for description, symbol and num-decimals unless -y given.
func AskDenominationDetails(d *DenominationDetails) {
	if denominationDescription != "" {
		d.Description = denominationDescription
	}
	if denominationSymbol != "" {
		d.Symbol = denominationSymbol
	}
	if denominationNumDecimals != 0 {
		d.Decimals = uint8(denominationNumDecimals) //nolint: gosec
	}

	if GetAnswerYes() {
		return
	}

	questions := []*survey.Question{
		{
			Name:   "description",
			Prompt: &survey.Input{Message: "Description:", Default: d.Description},
		},
		{
			Name:   "symbol",
			Prompt: &survey.Input{Message: "Denomination symbol:", Default: d.Symbol},
		},
		{
			Name: "decimals",
			Prompt: &survey.Input{
				Message: "Denomination decimal places:",
				Default: fmt.Sprintf("%d", d.Decimals),
			},
			Validate: survey.Required,
		},
	}
	answers := struct {
		Description string
		Symbol      string
		Decimals    uint8
	}{}
	err := AskMulti(questions, &answers)
	cobra.CheckErr(err)

	d.Description = answers.Description
	d.Symbol = answers.Symbol
	d.Decimals = answers.Decimals
}

// AskNewPassphrase asks the user to create a new passphrase.
func AskNewPassphrase() string {
	var answers struct {
		Passphrase  string
		Passphrase2 string
	}
	questions := []*survey.Question{
		{
			Name:   "passphrase",
			Prompt: PromptCreatePassphrase,
		},
		{
			Name:   "passphrase2",
			Prompt: PromptRepeatPassphrase,
			Validate: func(ans interface{}) error {
				if ans.(string) != answers.Passphrase {
					return fmt.Errorf("passphrases do not match")
				}
				return nil
			},
		},
	}
	err := AskMulti(questions, &answers)
	cobra.CheckErr(err)

	return answers.Passphrase
}
