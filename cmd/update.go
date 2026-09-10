package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/huh/v2"
	"github.com/LickABrick/inpakker/internal/cliui"
	"github.com/LickABrick/inpakker/internal/updater"
	"github.com/spf13/cobra"
)

var newUpdateService = updater.New

func newUpdateCmd() *cobra.Command {
	var checkOnly, assumeYes, jsonOutput bool
	command := &cobra.Command{
		Use:   "update",
		Short: "Check for and install Inpakker updates",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if assumeYes && (jsonOutput || checkOnly) {
				return asUsage(errors.New("--yes cannot be combined with --check or --json"))
			}
			service := newUpdateService(currentVersion)
			result, err := service.Check(cmd.Context(), false)
			if err != nil {
				return err
			}
			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}
			console := newConsole(cmd.OutOrStdout(), cmd.ErrOrStderr())
			if !result.Available {
				console.successDetail("Inpakker", "v"+result.CurrentVersion+" is up to date")
				return nil
			}
			console.warningDetail("Update available", "v"+result.CurrentVersion+" → v"+result.LatestVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "Release notes: %s\n", result.ReleaseURL)
			if checkOnly {
				return nil
			}
			confirmed := assumeYes
			if !confirmed {
				if !interactive(cmd) {
					return errors.New("confirmation is required; rerun with --yes")
				}
				form := huh.NewForm(huh.NewGroup(
					huh.NewConfirm().Title("Install v" + result.LatestVersion + " now?").Affirmative("Install").Negative("Cancel").Value(&confirmed),
				))
				if err := runForm(cmd, form); err != nil {
					return err
				}
			}
			if !confirmed {
				fmt.Fprintln(cmd.OutOrStdout(), "Update cancelled.")
				return nil
			}
			if interactive(cmd) {
				_, err = cliui.Run(cmd.Context(), cmd.InOrStdin(), cmd.ErrOrStderr(), "Updating Inpakker", 4, func(ctx context.Context, emit func(cliui.Event)) (any, error) {
					return nil, service.Install(ctx, result, func(event updater.Event) {
						emit(cliui.Event{Current: event.Current, Completed: event.Current, Total: event.Total, Phase: event.Phase})
					})
				})
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Updating Inpakker to v%s\n", result.LatestVersion)
				err = service.Install(cmd.Context(), result, nil)
			}
			if err != nil {
				return err
			}
			console.successDetail("Inpakker", "updated to v"+result.LatestVersion)
			fmt.Fprintln(cmd.OutOrStdout(), "Restart Inpakker to use the new version.")
			return nil
		},
	}
	command.Flags().BoolVar(&checkOnly, "check", false, "Check for an update without installing it")
	command.Flags().BoolVarP(&assumeYes, "yes", "y", false, "Install without asking for confirmation")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Write machine-readable update information without installing")
	return command
}

type updateNoticeKey struct{}

func automaticUpdateCheck(cmd *cobra.Command) {
	if !shouldCheckForUpdates(cmd) {
		return
	}
	notices := startAutomaticUpdateCheck(cmd.Context(), newUpdateService(currentVersion))
	cmd.SetContext(context.WithValue(cmd.Context(), updateNoticeKey{}, notices))
}

func startAutomaticUpdateCheck(parent context.Context, service *updater.Service) <-chan *updateNotice {
	notices := make(chan *updateNotice, 1)
	go func() {
		defer close(notices)
		ctx, cancel := context.WithTimeout(parent, 2*time.Second)
		defer cancel()
		result, err := service.Check(ctx, true)
		if err == nil && result.Available && service.CLINoticeDue() {
			notices <- &updateNotice{service: service, result: result}
		}
	}()
	return notices
}

func printAutomaticUpdateNotice(cmd *cobra.Command) {
	notices, _ := cmd.Context().Value(updateNoticeKey{}).(<-chan *updateNotice)
	select {
	case notice := <-notices:
		if notice == nil {
			return
		}
		console := newConsole(cmd.ErrOrStderr(), cmd.ErrOrStderr())
		fmt.Fprintln(cmd.ErrOrStderr())
		console.warningDetail("Update available", "Inpakker v"+notice.result.LatestVersion+"; run 'inpakker update' to install")
		_ = notice.service.MarkCLINotified()
	default:
		// A background check must never delay completion of the user's command.
	}
}

type updateNotice struct {
	service *updater.Service
	result  updater.Result
}

func shouldCheckForUpdates(cmd *cobra.Command) bool {
	if cmd.Parent() == nil || !interactive(cmd) || updateChecksDisabled() {
		return false
	}
	for current := cmd; current != nil; current = current.Parent() {
		switch current.Name() {
		case "update", "tui", "completion", "help":
			return false
		}
	}
	if flag := cmd.Flags().Lookup("json"); flag != nil && flag.Value.String() == "true" {
		return false
	}
	return true
}

func updateChecksDisabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("INPAKKER_NO_UPDATE_CHECK")))
	return value != "" && value != "0" && value != "false" && value != "no"
}

func init() {
	rootCmd.AddCommand(newUpdateCmd())
}
