/*
©AngelaMos | 2026
hide.go

The hide command: pack a payload into the envelope and embed it in a cover
*/

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/CarterPerez-dev/crypha/internal/engine"
	"github.com/CarterPerez-dev/crypha/internal/payload"
	"github.com/CarterPerez-dev/crypha/internal/report"
	"github.com/spf13/cobra"
)

var (
	errUnknownCipher   = errors.New("unknown cipher")
	errUnknownStrength = errors.New("unknown key-derivation strength")
)

func newHideCmd() *cobra.Command {
	var (
		format, technique string
		in, out           string
		message, file     string
		passphrase        string
		cipher, strength  string
		encrypt, compress bool
	)

	cmd := &cobra.Command{
		Use:   cmdHide + " --format F -i COVER -o OUT (-m MSG | -f FILE)",
		Short: "Hide a payload inside a cover file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := readPayloadSource(cmd, message, file)
			if err != nil {
				return err
			}

			pass, err := resolveHidePassphrase(passphrase, encrypt)
			if err != nil {
				return err
			}

			opts, err := buildOptions(pass, compress, cipher, strength)
			if err != nil {
				return err
			}

			cover, err := os.ReadFile(in)
			if err != nil {
				return err
			}

			var stego bytes.Buffer
			res, err := engine.Hide(engine.HideRequest{
				Format:    format,
				Technique: technique,
				Cover:     bytes.NewReader(cover),
				Payload:   data,
				Out:       &stego,
				Options:   opts,
			})
			if err != nil {
				return err
			}

			if err := writeOutputFile(out, stego.Bytes()); err != nil {
				return err
			}
			return report.HideSummary(cmd.OutOrStdout(), res, out, jsonEnabled(cmd))
		},
	}

	f := cmd.Flags()
	f.StringVar(&format, flagFormat, "", "carrier format: image, audio, qr, text, pdf")
	f.StringVarP(&in, flagIn, shortIn, "", "cover input path")
	f.StringVarP(&out, flagOut, shortOut, "", "stego output path")
	f.StringVarP(&message, flagMessage, shortMessage, "", "inline message payload")
	f.StringVarP(&file, flagFile, shortFile, "", "payload file path (- for stdin)")
	f.BoolVar(&encrypt, flagEncrypt, false, "encrypt the payload with a passphrase")
	f.StringVarP(&passphrase, flagPassphrase, shortPassphrase, "", "passphrase (prefer the prompt or "+envPassphrase+")")
	f.BoolVar(&compress, flagCompress, false, "compress the payload before hiding")
	f.StringVar(&cipher, flagCipher, string(payload.CipherChaCha20), "cipher: chacha20 or aes256gcm")
	f.StringVar(&strength, flagStrength, string(payload.StrengthDefault), "key-derivation strength: default or high")
	f.StringVar(&technique, flagTechnique, "", "pdf technique: attachment, metadata, or append")
	markRequired(cmd, flagFormat, flagIn, flagOut)

	return cmd
}

func buildOptions(pass []byte, compress bool, cipher, strength string) (payload.Options, error) {
	c := payload.Cipher(cipher)
	if c != payload.CipherChaCha20 && c != payload.CipherAES256GCM {
		return payload.Options{}, fmt.Errorf("%w: %q", errUnknownCipher, cipher)
	}
	s := payload.Strength(strength)
	if s != payload.StrengthDefault && s != payload.StrengthHigh {
		return payload.Options{}, fmt.Errorf("%w: %q", errUnknownStrength, strength)
	}
	return payload.Options{
		Passphrase: pass,
		Compress:   compress,
		Cipher:     c,
		Strength:   s,
	}, nil
}
