/*
©AngelaMos | 2026
tui.go

The tui command plus the terminal check that launches the wizard on a bare invocation
*/

package cli

import (
	"os"

	"github.com/CarterPerez-dev/crypha/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var launchInteractive = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func newTuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   cmdTUI,
		Short: "Launch the interactive terminal wizard",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return tui.Run()
		},
	}
}
