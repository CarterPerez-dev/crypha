/*
©AngelaMos | 2026
bitio_test.go

Round-trip and boundary tests for the MSB-first bit reader and writer
*/

package bitio

import (
	"bytes"
	"io"
	"testing"
)

func TestRoundTripByteAligned(t *testing.T) {
	cases := [][]byte{
		{},
		{0x00},
		{0xFF},
		{0xA5, 0x5A, 0x00, 0xFF, 0x0F, 0xF0},
		[]byte("crypha carries the secret"),
	}
	for _, in := range cases {
		r := NewReader(in)
		w := NewWriter()
		for {
			bit, err := r.ReadBit()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("ReadBit: %v", err)
			}
			w.WriteBit(bit)
		}
		if got := w.Bytes(); !bytes.Equal(got, in) {
			t.Errorf("round-trip mismatch: got %x want %x", got, in)
		}
	}
}

func TestMSBFirstOrder(t *testing.T) {
	r := NewReader([]byte{0b10000001})
	want := []byte{1, 0, 0, 0, 0, 0, 0, 1}
	for i, wbit := range want {
		got, err := r.ReadBit()
		if err != nil {
			t.Fatalf("bit %d: %v", i, err)
		}
		if got != wbit {
			t.Errorf("bit %d: got %d want %d", i, got, wbit)
		}
	}
}

func TestReaderCounters(t *testing.T) {
	r := NewReader([]byte{0x00, 0x00})
	if r.TotalBits() != 16 {
		t.Fatalf("TotalBits: got %d want 16", r.TotalBits())
	}
	if _, err := r.ReadBit(); err != nil {
		t.Fatal(err)
	}
	if r.Remaining() != 15 {
		t.Errorf("Remaining: got %d want 15", r.Remaining())
	}
}

func TestWriterPadsPartialByte(t *testing.T) {
	w := NewWriter()
	w.WriteBit(1)
	w.WriteBit(1)
	w.WriteBit(1)
	if w.BitsWritten() != 3 {
		t.Fatalf("BitsWritten: got %d want 3", w.BitsWritten())
	}
	got := w.Bytes()
	if len(got) != 1 || got[0] != 0b11100000 {
		t.Errorf("partial byte: got %08b want 11100000", got[0])
	}
}
