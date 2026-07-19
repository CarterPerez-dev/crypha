/*
©AngelaMos | 2026
carrier_test.go

Registry behaviour tests using fake carriers
*/

package carrier

import (
	"bytes"
	"io"
	"testing"
)

type fakeCarrier struct {
	format string
	magic  byte
}

func (f fakeCarrier) Format() string { return f.format }

func (f fakeCarrier) Hide(_ io.Reader, _ []byte, _ io.Writer) error { return nil }

func (f fakeCarrier) Reveal(_ io.Reader) ([]byte, error) { return nil, nil }

func (f fakeCarrier) Capacity(_ io.Reader) (int, error) { return 0, nil }

func (f fakeCarrier) Sniff(stego io.ReadSeeker) bool {
	head := make([]byte, 1)
	if _, err := io.ReadFull(stego, head); err != nil {
		return false
	}
	return head[0] == f.magic
}

func TestRegisterGetFormats(t *testing.T) {
	registry = map[string]Carrier{}
	Register(fakeCarrier{format: "zeta", magic: 0x01})
	Register(fakeCarrier{format: "alpha", magic: 0x02})

	if _, ok := Get("alpha"); !ok {
		t.Fatal("expected alpha to be registered")
	}
	if _, ok := Get("missing"); ok {
		t.Fatal("did not expect missing to resolve")
	}

	got := Formats()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Errorf("Formats not sorted: %v", got)
	}
	if all := All(); len(all) != 2 || all[0].Format() != "alpha" {
		t.Errorf("All not sorted: %v", all)
	}
}

func TestDetect(t *testing.T) {
	registry = map[string]Carrier{}
	Register(fakeCarrier{format: "alpha", magic: 0x02})
	Register(fakeCarrier{format: "beta", magic: 0x03})

	stego := bytes.NewReader([]byte{0x03, 0xFF, 0xEE})
	c, ok := Detect(stego)
	if !ok || c.Format() != "beta" {
		t.Fatalf("Detect: got %v ok=%v want beta", c, ok)
	}

	none := bytes.NewReader([]byte{0x99})
	if _, ok := Detect(none); ok {
		t.Error("expected no match for unknown magic")
	}
}
