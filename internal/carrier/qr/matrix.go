/*
©AngelaMos | 2026
matrix.go

QR module geometry from ISO/IEC 18004: function map, format info, masks, placement, rendering
*/

package qr

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/bits"
)

const (
	quietZoneModules   = 4
	modulePixels       = 8
	darkThreshold      = 128
	formatBitCount     = 15
	formatDataBits     = 5
	formatGenPoly      = 0x537
	formatMaskPoly     = 0x5412
	formatMaxDistance  = 3
	formatCodeCount    = 32
	alignmentRadius    = 2
	timingLine         = 6
	finderBlockEdge    = 8
	finderPatternSize  = 7
	versionInfoOrigin  = 11
	versionInfoModules = 18
	versionInfoBand    = 3
	minVersionWithInfo = 7
	darkLight          = 0xFF
	darkDark           = 0x00
)

var formatBitCells = [formatBitCount]point{
	{8, 0}, {8, 1}, {8, 2}, {8, 3}, {8, 4}, {8, 5},
	{8, 7}, {8, 8}, {7, 8},
	{5, 8}, {4, 8}, {3, 8}, {2, 8}, {1, 8}, {0, 8},
}

var alignmentCenters = map[int][]int{
	1:  {},
	2:  {6, 18},
	3:  {6, 22},
	4:  {6, 26},
	5:  {6, 30},
	6:  {6, 34},
	7:  {6, 22, 38},
	8:  {6, 24, 42},
	9:  {6, 26, 46},
	10: {6, 28, 50},
}

type matrix struct {
	size int
	grid [][]bool
}

type point struct {
	x, y int
}

func newMatrix(size int) matrix {
	grid := make([][]bool, size)
	for i := range grid {
		grid[i] = make([]bool, size)
	}
	return matrix{size: size, grid: grid}
}

func (m matrix) clone() matrix {
	out := newMatrix(m.size)
	for y := range m.grid {
		copy(out.grid[y], m.grid[y])
	}
	return out
}

func functionModules(version int) [][]bool {
	size := symbolSize(version)
	isFunc := make([][]bool, size)
	for i := range isFunc {
		isFunc[i] = make([]bool, size)
	}
	mark := func(x, y int) {
		if x >= 0 && x < size && y >= 0 && y < size {
			isFunc[y][x] = true
		}
	}

	for y := 0; y <= finderBlockEdge; y++ {
		for x := 0; x <= finderBlockEdge; x++ {
			mark(x, y)
		}
	}
	for y := 0; y <= finderBlockEdge; y++ {
		for x := size - finderBlockEdge; x < size; x++ {
			mark(x, y)
		}
	}
	for y := size - finderBlockEdge; y < size; y++ {
		for x := 0; x <= finderBlockEdge; x++ {
			mark(x, y)
		}
	}

	for i := 0; i < size; i++ {
		mark(timingLine, i)
		mark(i, timingLine)
	}

	centers := alignmentCenters[version]
	for _, cx := range centers {
		for _, cy := range centers {
			if inFinderBlock(cx, cy, size) {
				continue
			}
			for dy := -alignmentRadius; dy <= alignmentRadius; dy++ {
				for dx := -alignmentRadius; dx <= alignmentRadius; dx++ {
					mark(cx+dx, cy+dy)
				}
			}
		}
	}

	if version >= minVersionWithInfo {
		for i := 0; i < versionInfoModules; i++ {
			mark(i/versionInfoBand, size-versionInfoOrigin+i%versionInfoBand)
			mark(size-versionInfoOrigin+i%versionInfoBand, i/versionInfoBand)
		}
	}

	return isFunc
}

func inFinderBlock(cx, cy, size int) bool {
	span := finderPatternSize + 1
	inTopLeft := cx < span && cy < span
	inTopRight := cx >= size-span && cy < span
	inBottomLeft := cx < span && cy >= size-span
	return inTopLeft || inTopRight || inBottomLeft
}

func maskBit(maskID, row, col int) bool {
	switch maskID {
	case 0:
		return (row+col)%2 == 0
	case 1:
		return row%2 == 0
	case 2:
		return col%3 == 0
	case 3:
		return (row+col)%3 == 0
	case 4:
		return (row/2+col/3)%2 == 0
	case 5:
		return (row*col)%2+(row*col)%3 == 0
	case 6:
		return ((row*col)%2+(row*col)%3)%2 == 0
	case 7:
		return ((row+col)%2+(row*col)%3)%2 == 0
	}
	return false
}

func formatCode(dataBits int) int {
	remainder := dataBits << (formatBitCount - formatDataBits)
	for i := formatBitCount - 1; i >= formatBitCount-formatDataBits; i-- {
		if remainder&(1<<i) != 0 {
			remainder ^= formatGenPoly << (i - (formatBitCount - formatDataBits))
		}
	}
	return ((dataBits << (formatBitCount - formatDataBits)) | remainder) ^ formatMaskPoly
}

func readFormatBits(m matrix) int {
	value := 0
	for i, cell := range formatBitCells {
		if m.grid[cell.y][cell.x] {
			value |= 1 << i
		}
	}
	return value
}

func parseFormat(m matrix) (maskID, level int, ok bool) {
	stored := readFormatBits(m)
	bestDist := formatBitCount + 1
	bestData := -1
	for d := 0; d < formatCodeCount; d++ {
		dist := bits.OnesCount(uint(stored ^ formatCode(d)))
		if dist < bestDist {
			bestDist = dist
			bestData = d
		}
	}
	if bestData < 0 || bestDist > formatMaxDistance {
		return 0, 0, false
	}
	return bestData & 0x7, bestData >> 3, true
}

func placementOrder(version int, isFunc [][]bool) []point {
	size := symbolSize(version)
	v, _ := lookupVersion(version)
	count := v.dataModules()
	order := make([]point, 0, count)

	xOffset := 1
	dirUp := true
	x := size - 2
	y := size - 1

	for i := 0; i < count; i++ {
		order = append(order, point{x: x + xOffset, y: y})
		if i == count-1 {
			break
		}
		for {
			if xOffset == 1 {
				xOffset = 0
			} else {
				xOffset = 1
				if dirUp {
					if y > 0 {
						y--
					} else {
						dirUp = false
						x -= 2
					}
				} else {
					if y < size-1 {
						y++
					} else {
						dirUp = true
						x -= 2
					}
				}
			}
			if x == timingLine-1 {
				x--
			}
			if x < 0 {
				return order
			}
			if !isFunc[y][x+xOffset] {
				break
			}
		}
	}
	return order
}

func readSerial(m matrix, order []point, maskID, totalCodewords int) []byte {
	out := make([]byte, totalCodewords)
	for c := 0; c < totalCodewords; c++ {
		var b byte
		for bit := 0; bit < bitsPerCodeword; bit++ {
			p := order[c*bitsPerCodeword+bit]
			v := m.grid[p.y][p.x]
			if maskBit(maskID, p.y, p.x) {
				v = !v
			}
			b <<= 1
			if v {
				b |= 1
			}
		}
		out[c] = b
	}
	return out
}

func writeSerial(m matrix, order []point, maskID int, serial []byte) {
	for c := 0; c < len(serial); c++ {
		b := serial[c]
		for bit := 0; bit < bitsPerCodeword; bit++ {
			p := order[c*bitsPerCodeword+bit]
			v := (b>>(7-bit))&1 == 1
			if maskBit(maskID, p.y, p.x) {
				v = !v
			}
			m.grid[p.y][p.x] = v
		}
	}
}

func renderPNG(m matrix, out io.Writer) error {
	dim := (m.size + 2*quietZoneModules) * modulePixels
	img := image.NewGray(image.Rect(0, 0, dim, dim))
	for i := range img.Pix {
		img.Pix[i] = darkLight
	}
	for y := 0; y < m.size; y++ {
		for x := 0; x < m.size; x++ {
			if !m.grid[y][x] {
				continue
			}
			px := (x + quietZoneModules) * modulePixels
			py := (y + quietZoneModules) * modulePixels
			for dy := 0; dy < modulePixels; dy++ {
				for dx := 0; dx < modulePixels; dx++ {
					img.SetGray(px+dx, py+dy, color.Gray{Y: darkDark})
				}
			}
		}
	}
	return png.Encode(out, img)
}

func readGrid(r io.Reader) (matrix, int, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return matrix{}, 0, fmt.Errorf("crypha/qr: decode stego: %w", err)
	}
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	if width != height || width%modulePixels != 0 {
		return matrix{}, 0, ErrNotQR
	}
	across := width / modulePixels
	size := across - 2*quietZoneModules
	version, ok := versionForSize(size)
	if !ok {
		return matrix{}, 0, ErrNotQR
	}
	m := newMatrix(size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			cx := b.Min.X + (quietZoneModules+x)*modulePixels + modulePixels/2
			cy := b.Min.Y + (quietZoneModules+y)*modulePixels + modulePixels/2
			gray := color.GrayModel.Convert(img.At(cx, cy)).(color.Gray)
			m.grid[y][x] = gray.Y < darkThreshold
		}
	}
	return m, version, nil
}
