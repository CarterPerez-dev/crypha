/*
©AngelaMos | 2026
overhead_test.go

Known-answer test tying the engine overhead helper to the documented envelope sizes
*/

package engine

import "testing"

func TestOverheadKnownValues(t *testing.T) {
	if got := Overhead(false); got != 14 {
		t.Fatalf("Overhead(false) = %d, want 14", got)
	}
	if got := Overhead(true); got != 68 {
		t.Fatalf("Overhead(true) = %d, want 68", got)
	}
}
