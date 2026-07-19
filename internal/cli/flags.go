/*
©AngelaMos | 2026
flags.go

Shared command and flag identifiers plus small helpers for the crypha cobra tree
*/

package cli

import "github.com/spf13/cobra"

const (
	cmdHide     = "hide"
	cmdReveal   = "reveal"
	cmdCapacity = "capacity"
	cmdFormats  = "formats"
	cmdVersion  = "version"
	cmdTUI      = "tui"

	flagJSON       = "json"
	flagFormat     = "format"
	flagIn         = "in"
	flagOut        = "out"
	flagMessage    = "message"
	flagFile       = "file"
	flagEncrypt    = "encrypt"
	flagPassphrase = "passphrase"
	flagCompress   = "compress"
	flagCipher     = "cipher"
	flagStrength   = "strength"
	flagTechnique  = "technique"

	shortIn         = "i"
	shortOut        = "o"
	shortMessage    = "m"
	shortFile       = "f"
	shortPassphrase = "k"

	envPassphrase = "CRYPHA_PASSPHRASE"

	stdioPath   = "-"
	stdoutName  = "(stdout)"
	outFilePerm = 0o600
)

func markRequired(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		_ = cmd.MarkFlagRequired(name)
	}
}

func jsonEnabled(cmd *cobra.Command) bool {
	on, _ := cmd.Flags().GetBool(flagJSON)
	return on
}

func pathFromFlagOrArg(flagValue string, args []string) (string, error) {
	if flagValue != "" && len(args) == 1 {
		return "", errAmbiguousPath
	}
	if flagValue != "" {
		return flagValue, nil
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return "", nil
}
