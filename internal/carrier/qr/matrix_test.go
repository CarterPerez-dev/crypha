/*
©AngelaMos | 2026
matrix_test.go

Unit coverage for QR geometry: mask formulas, version sizing, and capacity edge cases
*/

package qr

import (
	"bytes"
	"strings"
	"testing"
)

func TestMaskFormulas(t *testing.T) {
	cases := []struct {
		maskID   int
		row, col int
		want     bool
	}{
		{0, 0, 0, true}, {0, 0, 1, false},
		{1, 0, 3, true}, {1, 1, 3, false},
		{2, 4, 0, true}, {2, 4, 1, false},
		{3, 0, 0, true}, {3, 0, 1, false},
		{4, 0, 0, true}, {4, 2, 0, false},
		{5, 0, 0, true}, {5, 1, 1, false},
		{6, 1, 1, true}, {6, 1, 5, false},
		{7, 0, 0, true}, {7, 0, 1, false},
	}
	for _, tc := range cases {
		if got := maskBit(tc.maskID, tc.row, tc.col); got != tc.want {
			t.Fatalf("maskBit(%d,%d,%d): got %v want %v", tc.maskID, tc.row, tc.col, got, tc.want)
		}
	}
	if maskBit(99, 0, 0) {
		t.Fatal("maskBit with out-of-range id should be false")
	}
}

func TestFunctionMapCountMatchesDataModules(t *testing.T) {
	for version := minSupportedVersion; version <= maxSupportedVersion; version++ {
		isFunc := functionModules(version)
		size := symbolSize(version)
		funcCount := 0
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if isFunc[y][x] {
					funcCount++
				}
			}
		}
		if nonFunc := size*size - funcCount; nonFunc != versionTable[version].dataModules() {
			t.Fatalf("v%d: non-function modules %d, want dataModules %d", version, nonFunc, versionTable[version].dataModules())
		}
	}
}

func TestVersionForSize(t *testing.T) {
	cases := []struct {
		size    int
		version int
		ok      bool
	}{
		{21, 1, true},
		{25, 2, true},
		{57, 10, true},
		{17, 0, false},
		{23, 0, false},
		{61, 0, false},
	}
	for _, tc := range cases {
		version, ok := versionForSize(tc.size)
		if ok != tc.ok || version != tc.version {
			t.Fatalf("versionForSize(%d): got (%d,%v) want (%d,%v)", tc.size, version, ok, tc.version, tc.ok)
		}
	}
}

func TestCapacityBytesFloorsAtZero(t *testing.T) {
	tiny := qrVersion{version: 0, groups: []blockGroup{{count: 1, total: 9, data: 8}}}
	if got := tiny.capacityBytes(); got != 0 {
		t.Fatalf("capacityBytes for a sub-frame budget: got %d want 0", got)
	}
}

func TestSupportedVersionCapacities(t *testing.T) {
	want := map[int]int{
		1: 0, 2: 3, 3: 6, 4: 12, 5: 16,
		6: 24, 7: 26, 8: 32, 9: 44, 10: 52,
	}
	for version := minSupportedVersion; version <= maxSupportedVersion; version++ {
		if got := versionTable[version].capacityBytes(); got != want[version] {
			t.Fatalf("v%d capacity: got %d want %d", version, got, want[version])
		}
	}
}

func TestRoundTripAcrossCovers(t *testing.T) {
	covers := []string{
		"crypha",
		"https://angelamos.com/x",
		"TEST 123",
		"The quick brown fox jumps",
		"0123456789",
	}
	secret := []byte("covert channel")
	for _, cover := range covers {
		var stego bytes.Buffer
		if err := (qrCarrier{}).Hide(strings.NewReader(cover), secret, &stego); err != nil {
			t.Fatalf("Hide cover %q: %v", cover, err)
		}
		got, err := (qrCarrier{}).Reveal(bytes.NewReader(stego.Bytes()))
		if err != nil {
			t.Fatalf("Reveal cover %q: %v", cover, err)
		}
		if !bytes.Equal(got, secret) {
			t.Fatalf("cover %q: round-trip mismatch", cover)
		}
	}
}
