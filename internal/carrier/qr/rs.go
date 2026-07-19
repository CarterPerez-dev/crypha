/*
©AngelaMos | 2026
rs.go

Systematic Reed-Solomon over GF(2^8): encode plus a syndrome-Berlekamp-Massey decoder
*/

package qr

import "errors"

var (
	ErrRSInput         = errors.New("crypha/qr: invalid reed-solomon block")
	ErrRSUncorrectable = errors.New("crypha/qr: reed-solomon block is uncorrectable")
)

func gfPolyEval(p []byte, x byte) byte {
	y := p[0]
	for i := 1; i < len(p); i++ {
		y = gfMul(y, x) ^ p[i]
	}
	return y
}

func gfPolyScale(p []byte, s byte) []byte {
	out := make([]byte, len(p))
	for i := range p {
		out[i] = gfMul(p[i], s)
	}
	return out
}

func gfPolyAdd(a, b []byte) []byte {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	out := make([]byte, n)
	for i := range a {
		out[i+n-len(a)] = a[i]
	}
	for i := range b {
		out[i+n-len(b)] ^= b[i]
	}
	return out
}

func gfPolyMul(a, b []byte) []byte {
	out := make([]byte, len(a)+len(b)-1)
	for i := range a {
		for j := range b {
			out[i+j] ^= gfMul(a[i], b[j])
		}
	}
	return out
}

func rsGenerator(nsym int) []byte {
	g := []byte{1}
	for i := 0; i < nsym; i++ {
		g = gfPolyMul(g, []byte{1, gfPow(gfGenerator, i)})
	}
	return g
}

func rsEncode(data []byte, nsym int) []byte {
	gen := rsGenerator(nsym)
	out := make([]byte, len(data)+nsym)
	copy(out, data)
	for i := 0; i < len(data); i++ {
		coef := out[i]
		if coef != 0 {
			for j := 1; j < len(gen); j++ {
				out[i+j] ^= gfMul(gen[j], coef)
			}
		}
	}
	copy(out, data)
	return out
}

func rsSyndromes(recv []byte, nsym int) []byte {
	s := make([]byte, nsym)
	for i := 0; i < nsym; i++ {
		s[i] = gfPolyEval(recv, gfPow(gfGenerator, i))
	}
	return s
}

func rsAllZero(s []byte) bool {
	for _, v := range s {
		if v != 0 {
			return false
		}
	}
	return true
}

func rsErrorLocator(synd []byte) []byte {
	errLoc := []byte{1}
	oldLoc := []byte{1}
	for i := 0; i < len(synd); i++ {
		delta := synd[i]
		for j := 1; j < len(errLoc); j++ {
			delta ^= gfMul(errLoc[len(errLoc)-1-j], synd[i-j])
		}
		oldLoc = append(oldLoc, 0)
		if delta != 0 {
			if len(oldLoc) > len(errLoc) {
				newLoc := gfPolyScale(oldLoc, delta)
				oldLoc = gfPolyScale(errLoc, gfInv(delta))
				errLoc = newLoc
			}
			errLoc = gfPolyAdd(errLoc, gfPolyScale(oldLoc, delta))
		}
	}
	for len(errLoc) > 0 && errLoc[0] == 0 {
		errLoc = errLoc[1:]
	}
	return errLoc
}

func rsErrorPositions(errLoc []byte, n int) ([]int, bool) {
	numErr := len(errLoc) - 1
	positions := make([]int, 0, numErr)
	for p := 0; p < n; p++ {
		locator := gfPow(gfGenerator, (n-1-p)%gfOrder)
		if gfPolyEval(errLoc, gfInv(locator)) == 0 {
			positions = append(positions, p)
		}
	}
	if len(positions) != numErr {
		return nil, false
	}
	return positions, true
}

func rsSolveMagnitudes(locators, synd []byte) ([]byte, bool) {
	e := len(locators)
	matrix := make([][]byte, e)
	for row := 0; row < e; row++ {
		matrix[row] = make([]byte, e+1)
		for col := 0; col < e; col++ {
			matrix[row][col] = gfPow(locators[col], row)
		}
		matrix[row][e] = synd[row]
	}

	for col := 0; col < e; col++ {
		pivot := -1
		for row := col; row < e; row++ {
			if matrix[row][col] != 0 {
				pivot = row
				break
			}
		}
		if pivot < 0 {
			return nil, false
		}
		matrix[col], matrix[pivot] = matrix[pivot], matrix[col]
		inv := gfInv(matrix[col][col])
		for k := col; k <= e; k++ {
			matrix[col][k] = gfMul(matrix[col][k], inv)
		}
		for row := 0; row < e; row++ {
			if row != col && matrix[row][col] != 0 {
				factor := matrix[row][col]
				for k := col; k <= e; k++ {
					matrix[row][k] ^= gfMul(factor, matrix[col][k])
				}
			}
		}
	}

	magnitudes := make([]byte, e)
	for row := 0; row < e; row++ {
		magnitudes[row] = matrix[row][e]
	}
	for j := 0; j < len(synd); j++ {
		var acc byte
		for m := 0; m < e; m++ {
			acc ^= gfMul(magnitudes[m], gfPow(locators[m], j))
		}
		if acc != synd[j] {
			return nil, false
		}
	}
	return magnitudes, true
}

func rsDecode(recv []byte, nsym int) ([]byte, error) {
	if nsym <= 0 || len(recv) <= nsym || len(recv) > gfOrder {
		return nil, ErrRSInput
	}
	synd := rsSyndromes(recv, nsym)
	if rsAllZero(synd) {
		return append([]byte(nil), recv...), nil
	}
	errLoc := rsErrorLocator(synd)
	numErr := len(errLoc) - 1
	if numErr <= 0 || numErr > nsym/2 {
		return nil, ErrRSUncorrectable
	}
	positions, ok := rsErrorPositions(errLoc, len(recv))
	if !ok {
		return nil, ErrRSUncorrectable
	}
	locators := make([]byte, numErr)
	for m, p := range positions {
		locators[m] = gfPow(gfGenerator, (len(recv)-1-p)%gfOrder)
	}
	magnitudes, ok := rsSolveMagnitudes(locators, synd)
	if !ok {
		return nil, ErrRSUncorrectable
	}
	corrected := append([]byte(nil), recv...)
	for m, p := range positions {
		corrected[p] ^= magnitudes[m]
	}
	return corrected, nil
}
