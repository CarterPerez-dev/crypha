/*
©AngelaMos | 2026
root.go

Cobra root command and CLI entry wiring for crypha
*/

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/CarterPerez-dev/crypha/internal/config"
	"github.com/CarterPerez-dev/crypha/internal/tui"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           config.BinaryName,
		Short:         config.ShortDescription,
		Long:          config.LongDescription,
		Version:       config.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate(config.BinaryName + " {{.Version}}\n")
	root.PersistentFlags().Bool(flagJSON, false, "emit machine-readable JSON")
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && launchInteractive() {
			return tui.Run()
		}
		return cmd.Help()
	}
	root.AddCommand(
		newHideCmd(),
		newRevealCmd(),
		newCapacityCmd(),
		newFormatsCmd(),
		newVersionCmd(),
		newTuiCmd(),
	)
	return root
}

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		msg := err.Error()
		if !strings.HasPrefix(msg, config.BinaryName) {
			msg = config.BinaryName + ": " + msg
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}
