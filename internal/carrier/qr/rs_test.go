/*
©AngelaMos | 2026
rs_test.go

Field-table sanity plus Reed-Solomon encode/decode correctness under injected errors
*/

package qr

import (
	"bytes"
	"testing"
)

func rsRandom(n, seed int) []byte {
	b := make([]byte, n)
	x := uint32(seed)*2654435761 + 1
	for i := range b {
		x = x*1664525 + 1013904223
		b[i] = byte(x >> 24)
	}
	return b
}

func TestGFFieldTables(t *testing.T) {
	if gfExp[0] != 1 {
		t.Fatalf("gfExp[0]: got %d want 1", gfExp[0])
	}
	if gfExp[8] != 29 {
		t.Fatalf("gfExp[8]: got %d want 29 (primitive 0x11D)", gfExp[8])
	}
	if gfMul(2, 2) != 4 {
		t.Fatalf("gfMul(2,2): got %d want 4", gfMul(2, 2))
	}
	if gfPow(2, 8) != 29 {
		t.Fatalf("gfPow(2,8): got %d want 29", gfPow(2, 8))
	}
	for a := 1; a < gfFieldSize; a++ {
		if gfMul(byte(a), gfInv(byte(a))) != 1 {
			t.Fatalf("gfInv broken at %d", a)
		}
	}
	for a := 1; a < gfFieldSize; a++ {
		for b := 1; b < gfFieldSize; b++ {
			if gfMul(byte(a), byte(b)) != gfMul(byte(b), byte(a)) {
				t.Fatalf("gfMul not commutative at %d,%d", a, b)
			}
		}
	}
}

func TestGFPowEdges(t *testing.T) {
	if gfPow(gfGenerator, 0) != 1 {
		t.Fatalf("gfPow(2,0): got %d want 1", gfPow(gfGenerator, 0))
	}
	if gfPow(gfGenerator, gfOrder) != 1 {
		t.Fatalf("gfPow(2,255): got %d want 1", gfPow(gfGenerator, gfOrder))
	}
	if gfPow(0, 0) != 1 {
		t.Fatalf("gfPow(0,0): got %d want 1", gfPow(0, 0))
	}
	if gfPow(0, 5) != 0 {
		t.Fatalf("gfPow(0,5): got %d want 0", gfPow(0, 5))
	}
}

func TestRSGeneratorIsMonic(t *testing.T) {
	for _, nsym := range []int{7, 17, 22, 28} {
		gen := rsGenerator(nsym)
		if len(gen) != nsym+1 {
			t.Fatalf("generator degree: nsym=%d got len %d", nsym, len(gen))
		}
		if gen[0] != 1 {
			t.Fatalf("generator not monic: nsym=%d leading %d", nsym, gen[0])
		}
	}
}

func TestRSCleanRoundTrip(t *testing.T) {
	cases := []struct{ k, nsym int }{
		{9, 17},
		{13, 22},
		{16, 28},
		{15, 28},
	}
	for _, tc := range cases {
		data := rsRandom(tc.k, tc.k*tc.nsym)
		code := rsEncode(data, tc.nsym)
		if len(code) != tc.k+tc.nsym {
			t.Fatalf("encode length: got %d want %d", len(code), tc.k+tc.nsym)
		}
		if !bytes.Equal(code[:tc.k], data) {
			t.Fatal("encode is not systematic (data prefix altered)")
		}
		got, err := rsDecode(code, tc.nsym)
		if err != nil {
			t.Fatalf("decode clean codeword: %v", err)
		}
		if !bytes.Equal(got[:tc.k], data) {
			t.Fatal("clean decode did not return data")
		}
	}
}

func TestRSCorrectsUpToT(t *testing.T) {
	cases := []struct{ k, nsym int }{
		{9, 17},
		{13, 22},
		{16, 28},
	}
	for _, tc := range cases {
		n := tc.k + tc.nsym
		maxErr := tc.nsym / 2
		data := rsRandom(tc.k, tc.k+7*tc.nsym)
		clean := rsEncode(data, tc.nsym)
		for numErr := 1; numErr <= maxErr; numErr++ {
			corrupt := append([]byte(nil), clean...)
			offsets := rsRandom(numErr, numErr*97+tc.nsym)
			mags := rsRandom(numErr, numErr*131+tc.k)
			used := map[int]bool{}
			placed := 0
			for i := 0; placed < numErr; i++ {
				pos := int(offsets[placed%numErr]) % n
				pos = (pos + i) % n
				if used[pos] {
					continue
				}
				mag := mags[placed%numErr]
				if mag == 0 {
					mag = 1
				}
				corrupt[pos] ^= mag
				used[pos] = true
				placed++
			}
			got, err := rsDecode(corrupt, tc.nsym)
			if err != nil {
				t.Fatalf("k=%d nsym=%d numErr=%d: decode failed: %v", tc.k, tc.nsym, numErr, err)
			}
			if !bytes.Equal(got[:tc.k], data) {
				t.Fatalf("k=%d nsym=%d numErr=%d: did not recover data", tc.k, tc.nsym, numErr)
			}
		}
	}
}

func TestRSDoesNotFakeCorrectBeyondT(t *testing.T) {
	k, nsym := 13, 22
	n := k + nsym
	data := rsRandom(k, 999)
	clean := rsEncode(data, nsym)
	tooMany := nsym/2 + 1
	for seed := 0; seed < 16; seed++ {
		corrupt := append([]byte(nil), clean...)
		offsets := rsRandom(tooMany, seed*17+3)
		mags := rsRandom(tooMany, seed*29+5)
		used := map[int]bool{}
		placed := 0
		for i := 0; placed < tooMany; i++ {
			pos := (int(offsets[placed%tooMany]) + i) % n
			if used[pos] {
				continue
			}
			mag := mags[placed%tooMany]
			if mag == 0 {
				mag = 1
			}
			corrupt[pos] ^= mag
			used[pos] = true
			placed++
		}
		got, err := rsDecode(corrupt, nsym)
		if err == nil && bytes.Equal(got[:k], data) {
			t.Fatalf("seed %d: decoder silently recovered original from %d errors (> t)", seed, tooMany)
		}
	}
}

func TestGFPolyAddAsymmetric(t *testing.T) {
	want := []byte{1, 1, 0}
	if got := gfPolyAdd([]byte{1, 0, 1}, []byte{1, 1}); !bytes.Equal(got, want) {
		t.Fatalf("gfPolyAdd longer-first: got %v want %v", got, want)
	}
	if got := gfPolyAdd([]byte{1, 1}, []byte{1, 0, 1}); !bytes.Equal(got, want) {
		t.Fatalf("gfPolyAdd longer-second: got %v want %v", got, want)
	}
}

func TestRSRejectsMalformedBlocks(t *testing.T) {
	if _, err := rsDecode([]byte{1, 2, 3}, 0); err != ErrRSInput {
		t.Fatalf("nsym=0: got %v want ErrRSInput", err)
	}
	if _, err := rsDecode([]byte{1, 2, 3}, 3); err != ErrRSInput {
		t.Fatalf("recv==nsym: got %v want ErrRSInput", err)
	}
	if _, err := rsDecode([]byte{1, 2, 3}, 5); err != ErrRSInput {
		t.Fatalf("recv<nsym: got %v want ErrRSInput", err)
	}
}
