/*
©AngelaMos | 2026
carrier.go

The Carrier interface and self-registering registry that every format plugs into
*/

package carrier

import (
	"io"
	"sort"
)

type Carrier interface {
	Format() string
	Hide(cover io.Reader, payload []byte, out io.Writer) error
	Reveal(stego io.Reader) ([]byte, error)
	Capacity(cover io.Reader) (int, error)
	Sniff(stego io.ReadSeeker) bool
}

var registry = map[string]Carrier{}

func Register(c Carrier) {
	registry[c.Format()] = c
}

func Get(name string) (Carrier, bool) {
	c, ok := registry[name]
	return c, ok
}

func All() []Carrier {
	out := make([]Carrier, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Format() < out[j].Format()
	})
	return out
}

func Formats() []string {
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func Detect(stego io.ReadSeeker) (Carrier, bool) {
	for _, c := range All() {
		if _, err := stego.Seek(0, io.SeekStart); err != nil {
			return nil, false
		}
		if c.Sniff(stego) {
			if _, err := stego.Seek(0, io.SeekStart); err != nil {
				return nil, false
			}
			return c, true
		}
	}
	return nil, false
}
