/*
©AngelaMos | 2026
io.go

Payload source reading and revealed-output writing for the crypha CLI
*/

package cli

import (
	"errors"
	"io"
	"os"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	errNoStego         = errors.New("provide a stego file with -i or as an argument")
	errNoCover         = errors.New("provide a cover file with -i or as an argument")
	errNoPayloadSource = errors.New("provide a payload with -m or -f")
	errBothSources     = errors.New("use only one of -m or -f")
	errBinaryToTTY     = errors.New("payload is binary; write it to a file with -o")
	errAmbiguousPath   = errors.New("pass the file once, via -i or as an argument, not both")
)

func readPayloadSource(cmd *cobra.Command, message, file string) ([]byte, error) {
	switch {
	case message != "" && file != "":
		return nil, errBothSources
	case message != "":
		return []byte(message), nil
	case file == stdioPath:
		return io.ReadAll(cmd.InOrStdin())
	case file != "":
		return os.ReadFile(file)
	default:
		return nil, errNoPayloadSource
	}
}

func writeOutputFile(outPath string, data []byte) error {
	return os.WriteFile(outPath, data, outFilePerm)
}

func writeRevealed(cmd *cobra.Command, data []byte, outPath string) (string, error) {
	if outPath != "" {
		if err := os.WriteFile(outPath, data, outFilePerm); err != nil {
			return "", err
		}
		return outPath, nil
	}
	if stdoutIsTerminal() && !utf8.Valid(data) {
		return "", errBinaryToTTY
	}
	if _, err := cmd.OutOrStdout().Write(data); err != nil {
		return "", err
	}
	return stdoutName, nil
}

func stdoutIsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
