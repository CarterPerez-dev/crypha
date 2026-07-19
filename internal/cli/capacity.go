/*
©AngelaMos | 2026
capacity.go

The capacity command: report how much a cover can hold, per carrier
*/

package cli

import (
	"bytes"
	"os"

	"github.com/CarterPerez-dev/crypha/internal/engine"
	"github.com/CarterPerez-dev/crypha/internal/report"
	"github.com/spf13/cobra"
)

func newCapacityCmd() *cobra.Command {
	var format, in string

	cmd := &cobra.Command{
		Use:   cmdCapacity + " -i COVER [--format F]",
		Short: "Report how many bytes a cover can hide",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := pathFromFlagOrArg(in, args)
			if err != nil {
				return err
			}
			if path == "" {
				return errNoCover
			}
			cover, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			rows, err := capacityRows(format, cover)
			if err != nil {
				return err
			}
			return report.Capacity(cmd.OutOrStdout(), rows, jsonEnabled(cmd))
		},
	}

	f := cmd.Flags()
	f.StringVar(&format, flagFormat, "", "carrier format (all applicable formats if omitted)")
	f.StringVarP(&in, flagIn, shortIn, "", "cover input path (or pass as an argument)")

	return cmd
}

func capacityRows(format string, cover []byte) ([]engine.CapacityRow, error) {
	if format == "" {
		return engine.CapacityAll(cover), nil
	}
	if _, err := engine.ResolveCarrier(format, ""); err != nil {
		return nil, err
	}
	n, cerr := engine.Capacity(format, bytes.NewReader(cover))
	return []engine.CapacityRow{{Format: format, Capacity: n, Err: cerr}}, nil
}
