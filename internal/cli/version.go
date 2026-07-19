/*
©AngelaMos | 2026
version.go

The version command: print the crypha build version
*/

package cli

import (
	"github.com/CarterPerez-dev/crypha/internal/report"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   cmdVersion,
		Short: "Print the crypha version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return report.Version(cmd.OutOrStdout(), jsonEnabled(cmd))
		},
	}
}
