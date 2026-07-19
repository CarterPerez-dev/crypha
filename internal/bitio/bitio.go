/*
©AngelaMos | 2026
bitio.go

MSB-first bit reader and writer for carriers that embed data bit by bit
*/

package bitio

import "io"

type Reader struct {
	in  []byte
	pos int
}

func NewReader(b []byte) *Reader {
	return &Reader{in: b}
}

func (r *Reader) ReadBit() (byte, error) {
	if r.pos >= len(r.in)*8 {
		return 0, io.EOF
	}
	byteIdx := r.pos / 8
	bitIdx := 7 - (r.pos % 8)
	bit := (r.in[byteIdx] >> bitIdx) & 1
	r.pos++
	return bit, nil
}

func (r *Reader) TotalBits() int {
	return len(r.in) * 8
}

func (r *Reader) Remaining() int {
	return len(r.in)*8 - r.pos
}

type Writer struct {
	out  []byte
	cur  byte
	fill int
}

func NewWriter() *Writer {
	return &Writer{}
}

func (w *Writer) WriteBit(bit byte) {
	w.cur = (w.cur << 1) | (bit & 1)
	w.fill++
	if w.fill == 8 {
		w.out = append(w.out, w.cur)
		w.cur = 0
		w.fill = 0
	}
}

func (w *Writer) Bytes() []byte {
	if w.fill == 0 {
		return w.out
	}
	return append(w.out, w.cur<<(8-w.fill))
}

func (w *Writer) BitsWritten() int {
	return len(w.out)*8 + w.fill
}
