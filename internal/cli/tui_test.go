/*
©AngelaMos | 2026
tui_test.go

Tests for the tui command registration and the bare-invocation help fallback
*/

package cli

import (
	"strings"
	"testing"
)

func TestBareInvocationPrintsHelpWhenNonInteractive(t *testing.T) {
	old := launchInteractive
	launchInteractive = func() bool { return false }
	defer func() { launchInteractive = old }()

	out, _, err := run(t, "")
	if err != nil {
		t.Fatalf("bare invocation errored: %v", err)
	}
	if !strings.Contains(out, cmdTUI) {
		t.Fatalf("help output missing the tui command:\n%s", out)
	}
	if !strings.Contains(out, "Available Commands") {
		t.Fatalf("bare invocation did not print help:\n%s", out)
	}
}

func TestTuiCommandRegistered(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Name() == cmdTUI {
			return
		}
	}
	t.Fatalf("tui command not registered on root")
}
