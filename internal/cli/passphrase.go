/*
©AngelaMos | 2026
passphrase.go

Passphrase acquisition from flag, environment, or a no-echo terminal prompt
*/

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"
)

const (
	promptEnterPassphrase   = "Passphrase: "
	promptConfirmPassphrase = "Confirm passphrase: "
	promptRetryPassphrase   = "Wrong passphrase, try again: "
	maxPassphraseAttempts   = 3
)

var (
	errPassphraseNonInteractive = errors.New("encryption requested but no passphrase provided; pass -k or set " + envPassphrase)
	errPassphraseMismatch       = errors.New("passphrases did not match")
)

var (
	interactive = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
	readSecret  = func() ([]byte, error) { return term.ReadPassword(int(os.Stdin.Fd())) }
)

func passphraseFromFlagOrEnv(flagValue string) []byte {
	if flagValue != "" {
		return []byte(flagValue)
	}
	if env := os.Getenv(envPassphrase); env != "" {
		return []byte(env)
	}
	return nil
}

func resolveHidePassphrase(flagValue string, encryptWanted bool) ([]byte, error) {
	if p := passphraseFromFlagOrEnv(flagValue); p != nil {
		return p, nil
	}
	if !encryptWanted {
		return nil, nil
	}
	if !interactive() {
		return nil, errPassphraseNonInteractive
	}
	return promptPassphraseConfirmed()
}

func promptPassphraseConfirmed() ([]byte, error) {
	first, err := promptPassphrase(promptEnterPassphrase)
	if err != nil {
		return nil, err
	}
	second, err := promptPassphrase(promptConfirmPassphrase)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(first, second) {
		return nil, errPassphraseMismatch
	}
	return first, nil
}

func promptPassphrase(prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	pass, err := readSecret()
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}
	return pass, nil
}
