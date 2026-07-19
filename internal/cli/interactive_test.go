/*
©AngelaMos | 2026
interactive_test.go

Tests for passphrase prompting, reprompt loops, and output branches
*/

package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/CarterPerez-dev/crypha/internal/payload"
)

func stubTerminal(t *testing.T, inter bool, secrets ...string) {
	t.Helper()
	origInter, origRead := interactive, readSecret
	t.Cleanup(func() { interactive, readSecret = origInter, origRead })
	interactive = func() bool { return inter }
	idx := 0
	readSecret = func() ([]byte, error) {
		if idx >= len(secrets) {
			return nil, io.EOF
		}
		s := secrets[idx]
		idx++
		return []byte(s), nil
	}
}

func TestPromptForError(t *testing.T) {
	if got := promptForError(payload.ErrPassphraseRequired); got != promptEnterPassphrase {
		t.Errorf("passphrase-required prompt = %q", got)
	}
	if got := promptForError(payload.ErrDecrypt); got != promptRetryPassphrase {
		t.Errorf("decrypt prompt = %q", got)
	}
}

func TestInteractiveHidePromptRoundTrip(t *testing.T) {
	t.Setenv(envPassphrase, "")
	stubTerminal(t, true, "prompted-secret", "prompted-secret")
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	stego := filepath.Join(dir, "prompted.png")

	if _, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", stego, "-m", secret, "--encrypt"); err != nil {
		t.Fatalf("hide with prompt: %v", err)
	}
	out, _, err := run(t, "", "reveal", "-i", stego, "-k", "prompted-secret")
	if err != nil || out != secret {
		t.Fatalf("reveal = %q err %v", out, err)
	}
}

func TestInteractiveHidePromptMismatch(t *testing.T) {
	t.Setenv(envPassphrase, "")
	stubTerminal(t, true, "one", "two")
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	stego := filepath.Join(dir, "mismatch.png")

	_, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", stego, "-m", secret, "--encrypt")
	if !errors.Is(err, errPassphraseMismatch) {
		t.Fatalf("err = %v, want errPassphraseMismatch", err)
	}
}

func TestHideEncryptNonInteractiveNoKey(t *testing.T) {
	t.Setenv(envPassphrase, "")
	stubTerminal(t, false)
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	stego := filepath.Join(dir, "nokey.png")

	_, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", stego, "-m", secret, "--encrypt")
	if !errors.Is(err, errPassphraseNonInteractive) {
		t.Fatalf("err = %v, want errPassphraseNonInteractive", err)
	}
}

func TestRevealReprompt(t *testing.T) {
	t.Setenv(envPassphrase, "")
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	stego := filepath.Join(dir, "reprompt.png")

	if _, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", stego, "-m", secret, "-k", "rightpass"); err != nil {
		t.Fatalf("hide: %v", err)
	}

	stubTerminal(t, true, "wrongpass", "rightpass")
	out, _, err := run(t, "", "reveal", "-i", stego)
	if err != nil || out != secret {
		t.Fatalf("reveal after reprompt = %q err %v", out, err)
	}
}

func TestRevealRepromptExhausted(t *testing.T) {
	t.Setenv(envPassphrase, "")
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	stego := filepath.Join(dir, "exhausted.png")

	if _, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", stego, "-m", secret, "-k", "rightpass"); err != nil {
		t.Fatalf("hide: %v", err)
	}

	stubTerminal(t, true, "no", "no", "no")
	_, _, err := run(t, "", "reveal", "-i", stego)
	if !errors.Is(err, payload.ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt after exhausting attempts", err)
	}
}

func TestRevealToFile(t *testing.T) {
	dir := t.TempDir()
	cover := makeTextCover(t, dir)
	stego := filepath.Join(dir, "tofile.txt")
	if _, _, err := run(t, "", "hide", "--format", "text", "-i", cover, "-o", stego, "-m", secret); err != nil {
		t.Fatalf("hide: %v", err)
	}

	out := filepath.Join(dir, "revealed.txt")
	stdout, _, err := run(t, "", "reveal", "-i", stego, "-o", out)
	if err != nil {
		t.Fatalf("reveal -o: %v", err)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty when writing to a file, got %q", stdout)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read revealed file: %v", err)
	}
	if string(got) != secret {
		t.Errorf("revealed file = %q, want %q", got, secret)
	}
}

func TestBuildOptionsUnknownStrength(t *testing.T) {
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	out := filepath.Join(dir, "out.png")
	_, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", out, "-m", "x", "-k", "p", "--strength", "ultra")
	if err == nil || !errors.Is(err, errUnknownStrength) {
		t.Fatalf("err = %v, want errUnknownStrength", err)
	}
}

func TestCapacityUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	_, _, err := run(t, "", "capacity", "-i", cover, "--format", "bogus")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestMissingFileErrors(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")
	out := filepath.Join(dir, "out.png")
	cases := [][]string{
		{"hide", "--format", "image", "-i", missing, "-o", out, "-m", "x"},
		{"reveal", "-i", missing},
		{"capacity", "-i", missing},
	}
	for _, args := range cases {
		if _, _, err := run(t, "", args...); err == nil {
			t.Errorf("%v: expected error for missing file", args)
		}
	}
}

func TestRevealPromptReadError(t *testing.T) {
	t.Setenv(envPassphrase, "")
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	stego := filepath.Join(dir, "readerr.png")
	if _, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", stego, "-m", secret, "-k", "pw"); err != nil {
		t.Fatalf("hide: %v", err)
	}

	origInter, origRead := interactive, readSecret
	t.Cleanup(func() { interactive, readSecret = origInter, origRead })
	interactive = func() bool { return true }
	readSecret = func() ([]byte, error) { return nil, io.ErrUnexpectedEOF }

	if _, _, err := run(t, "", "reveal", "-i", stego); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v, want a terminal read error", err)
	}
}
