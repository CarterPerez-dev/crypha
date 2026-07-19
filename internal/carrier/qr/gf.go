/*
©AngelaMos | 2026
gf.go

Arithmetic over GF(2^8) under the QR primitive polynomial for Reed-Solomon coding
*/

package qr

const (
	gfOrder     = 255
	gfFieldSize = 256
	gfPrimitive = 0x11D
	gfGenerator = 2
	gfHighBit   = 0x100
)

var (
	gfExp [gfOrder * 2]byte
	gfLog [gfFieldSize]byte
)

func init() {
	x := 1
	for i := 0; i < gfOrder; i++ {
		gfExp[i] = byte(x)
		gfLog[x] = byte(i)
		x <<= 1
		if x&gfHighBit != 0 {
			x ^= gfPrimitive
		}
	}
	for i := gfOrder; i < gfOrder*2; i++ {
		gfExp[i] = gfExp[i-gfOrder]
	}
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

func gfInv(a byte) byte {
	return gfExp[gfOrder-int(gfLog[a])]
}

func gfPow(base byte, exp int) byte {
	if base == 0 {
		if exp == 0 {
			return 1
		}
		return 0
	}
	return gfExp[(int(gfLog[base])*exp)%gfOrder]
}
