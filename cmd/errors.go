package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

func usageArgs(validator cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validator(cmd, args); err != nil {
			return usageError{err: err}
		}
		return nil
	}
}

func asUsage(err error) error {
	if err == nil {
		return nil
	}
	var usage usageError
	if errors.As(err, &usage) {
		return err
	}
	return usageError{err: err}
}
