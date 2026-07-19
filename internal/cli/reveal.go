/*
©AngelaMos | 2026
reveal.go

The reveal command: extract the envelope from a stego file and unpack the payload
*/

package cli

import (
	"errors"
	"os"

	"github.com/CarterPerez-dev/crypha/internal/engine"
	"github.com/CarterPerez-dev/crypha/internal/payload"
	"github.com/CarterPerez-dev/crypha/internal/report"
	"github.com/spf13/cobra"
)

func newRevealCmd() *cobra.Command {
	var format, in, out, passphrase string

	cmd := &cobra.Command{
		Use:   cmdReveal + " [-i] STEGO",
		Short: "Reveal a hidden payload from a stego file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := pathFromFlagOrArg(in, args)
			if err != nil {
				return err
			}
			if path == "" {
				return errNoStego
			}

			stego, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			res, err := revealWithPassphrase(format, stego, passphraseFromFlagOrEnv(passphrase))
			if err != nil {
				return err
			}

			if jsonEnabled(cmd) {
				if out != "" {
					if err := writeOutputFile(out, res.Data); err != nil {
						return err
					}
				}
				return report.RevealJSON(cmd.OutOrStdout(), res, out)
			}

			outPath, err := writeRevealed(cmd, res.Data, out)
			if err != nil {
				return err
			}
			return report.RevealStatus(cmd.ErrOrStderr(), res, outPath)
		},
	}

	f := cmd.Flags()
	f.StringVar(&format, flagFormat, "", "force a carrier format (auto-detect if omitted)")
	f.StringVarP(&in, flagIn, shortIn, "", "stego input path (or pass as an argument)")
	f.StringVarP(&out, flagOut, shortOut, "", "write the revealed payload here (default stdout)")
	f.StringVarP(&passphrase, flagPassphrase, shortPassphrase, "", "passphrase for an encrypted payload")

	return cmd
}

func revealWithPassphrase(format string, stego, pass []byte) (engine.RevealResult, error) {
	for attempt := 0; attempt <= maxPassphraseAttempts; attempt++ {
		res, err := engine.Reveal(engine.RevealRequest{Format: format, Stego: stego, Passphrase: pass})
		if err == nil {
			return res, nil
		}
		if !shouldReprompt(err, attempt) {
			return engine.RevealResult{}, err
		}
		entered, perr := promptPassphrase(promptForError(err))
		if perr != nil {
			return engine.RevealResult{}, perr
		}
		pass = entered
	}
	return engine.RevealResult{}, payload.ErrDecrypt
}

func shouldReprompt(err error, attempt int) bool {
	if !interactive() {
		return false
	}
	if errors.Is(err, payload.ErrPassphraseRequired) {
		return true
	}
	return errors.Is(err, payload.ErrDecrypt) && attempt < maxPassphraseAttempts
}

func promptForError(err error) string {
	if errors.Is(err, payload.ErrPassphraseRequired) {
		return promptEnterPassphrase
	}
	return promptRetryPassphrase
}
