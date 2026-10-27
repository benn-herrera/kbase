package main

import (
	"strconv"

	"github.com/spf13/cobra"

	"kbase/internal/config"
	toolresult "kbase/internal/result"
)

// askConcurrencyFlag is the flag naming an invocation's model calls in flight.
const askConcurrencyFlag = "ask-concurrency"

// registerAskConcurrencyFlag declares --ask-concurrency on cmd, into value.
func registerAskConcurrencyFlag(cmd *cobra.Command, value *int) {
	cmd.Flags().IntVar(value, askConcurrencyFlag, 0,
		"model calls in flight at once, across every role; at least 1 (default: [asks] concurrency in config.toml, else "+strconv.Itoa(config.DefaultAskConcurrency)+")")
}

// givenAskConcurrency is value where the invocation gave --ask-concurrency,
// nil where it did not.
func givenAskConcurrency(cmd *cobra.Command, value int) *int {
	if !cmd.Flags().Changed(askConcurrencyFlag) {
		return nil
	}
	return &value
}

// askConcurrency is the invocation's model calls in flight: the
// --ask-concurrency flag where given, else the [asks] concurrency key, else
// config.DefaultAskConcurrency. A flag under 1 is refused, naming it; the
// key's own value was checked at load.
func askConcurrency(flag, key *int) (int, error) {
	switch {
	case flag != nil && *flag < 1:
		return 0, toolresult.Refusal{{Check: checkUsage, Key: "--" + askConcurrencyFlag,
			Detail: "--" + askConcurrencyFlag + " counts model calls in flight and is at least 1"}}
	case flag != nil:
		return *flag, nil
	case key != nil:
		return *key, nil
	}
	return config.DefaultAskConcurrency, nil
}
