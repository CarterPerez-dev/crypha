/*
©AngelaMos | 2026
formats.go

The formats command: list every registered carrier and its capabilities
*/

package cli

import (
	"github.com/CarterPerez-dev/crypha/internal/report"
	"github.com/spf13/cobra"
)

func newFormatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   cmdFormats,
		Short: "List the available carrier formats",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return report.Formats(cmd.OutOrStdout(), jsonEnabled(cmd))
		},
	}
}
