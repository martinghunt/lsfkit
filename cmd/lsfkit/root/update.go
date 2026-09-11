package root

import (
	"fmt"
	"os"

	"github.com/martinghunt/lsfkit/internal/buildinfo"
	"github.com/martinghunt/lsfkit/internal/selfupdate"
	"github.com/spf13/cobra"
)

var runSelfUpdate = selfupdate.Update

func newUpdateCommand() *cobra.Command {
	var checkOnly bool
	var force bool
	command := &cobra.Command{
		Use:   "update",
		Short: "Update lsfkit to the latest GitHub release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := runSelfUpdate(cmd.Context(), selfupdate.Options{
				CurrentVersion: buildinfo.Version,
				CheckOnly:      checkOnly,
				Force:          force,
				Token:          githubToken(),
			})
			if err != nil {
				return err
			}
			return printUpdateResult(cmd, result)
		},
	}
	command.Flags().BoolVar(&checkOnly, "check", false, "check for an update without installing")
	command.Flags().BoolVar(&force, "force", false, "install the latest release even when the current version is not older")
	return command
}

func githubToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GH_TOKEN")
}

func printUpdateResult(cmd *cobra.Command, result selfupdate.Result) error {
	out := cmd.OutOrStdout()
	switch {
	case result.Updated && displayVersion(result.CurrentVersion) == displayVersion(result.LatestVersion):
		_, err := fmt.Fprintf(out, "installed lsfkit %s\n", displayVersion(result.LatestVersion))
		return err
	case result.Updated:
		_, err := fmt.Fprintf(out, "updated lsfkit from %s to %s\n", displayVersion(result.CurrentVersion), displayVersion(result.LatestVersion))
		return err
	case result.UpToDate:
		_, err := fmt.Fprintf(out, "lsfkit is up to date (%s)\n", displayVersion(result.CurrentVersion))
		return err
	case result.CheckOnly:
		_, err := fmt.Fprintf(out, "lsfkit %s is available (current %s)\n", displayVersion(result.LatestVersion), displayVersion(result.CurrentVersion))
		return err
	default:
		return nil
	}
}

func displayVersion(raw string) string {
	if len(raw) > 1 && (raw[0] == 'v' || raw[0] == 'V') && raw[1] >= '0' && raw[1] <= '9' {
		return raw[1:]
	}
	return raw
}
