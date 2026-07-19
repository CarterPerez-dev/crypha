/*
©AngelaMos | 2026
blocks.go

Version and error-correction block tables (level H) plus codeword interleaving from ISO/IEC 18004
*/

package qr

const (
	ecLevelHigh          = 2
	framePrefixBytes     = 4
	bitsPerCodeword      = 8
	injectionSafetyRatio = 2
	minSupportedVersion  = 1
	maxSupportedVersion  = 10
	baseSymbolSize       = 21
	versionSizeStep      = 4
)

type blockGroup struct {
	count int
	total int
	data  int
}

type qrVersion struct {
	version       int
	remainderBits int
	groups        []blockGroup
}

var versionTable = map[int]qrVersion{
	1:  {1, 0, []blockGroup{{1, 26, 9}}},
	2:  {2, 7, []blockGroup{{1, 44, 16}}},
	3:  {3, 7, []blockGroup{{2, 35, 13}}},
	4:  {4, 7, []blockGroup{{4, 25, 9}}},
	5:  {5, 7, []blockGroup{{2, 33, 11}, {2, 34, 12}}},
	6:  {6, 7, []blockGroup{{4, 43, 15}}},
	7:  {7, 0, []blockGroup{{4, 39, 13}, {1, 40, 14}}},
	8:  {8, 0, []blockGroup{{4, 40, 14}, {2, 41, 15}}},
	9:  {9, 0, []blockGroup{{4, 36, 12}, {4, 37, 13}}},
	10: {10, 0, []blockGroup{{6, 43, 15}, {2, 44, 16}}},
}

func lookupVersion(version int) (qrVersion, bool) {
	v, ok := versionTable[version]
	return v, ok
}

func symbolSize(version int) int {
	return baseSymbolSize + (version-1)*versionSizeStep
}

func versionForSize(size int) (int, bool) {
	if size < baseSymbolSize || (size-baseSymbolSize)%versionSizeStep != 0 {
		return 0, false
	}
	version := (size-baseSymbolSize)/versionSizeStep + 1
	if version < minSupportedVersion || version > maxSupportedVersion {
		return 0, false
	}
	return version, true
}

func (v qrVersion) numBlocks() int {
	n := 0
	for _, g := range v.groups {
		n += g.count
	}
	return n
}

func (v qrVersion) totalCodewords() int {
	n := 0
	for _, g := range v.groups {
		n += g.count * g.total
	}
	return n
}

func (v qrVersion) ecPerBlock() int {
	return v.groups[0].total - v.groups[0].data
}

func (v qrVersion) correctable() int {
	return v.ecPerBlock() / 2
}

func (v qrVersion) injectPerBlock() int {
	return v.correctable() / injectionSafetyRatio
}

func (v qrVersion) dataModules() int {
	return v.totalCodewords()*bitsPerCodeword + v.remainderBits
}

func (v qrVersion) capacityBytes() int {
	usable := v.numBlocks()*v.injectPerBlock() - framePrefixBytes
	if usable < 0 {
		return 0
	}
	return usable
}

func (v qrVersion) blockLayout() (dataLens, ecLens []int) {
	for _, g := range v.groups {
		for i := 0; i < g.count; i++ {
			dataLens = append(dataLens, g.data)
			ecLens = append(ecLens, g.total-g.data)
		}
	}
	return dataLens, ecLens
}

func (v qrVersion) interleave(dataBlocks, ecBlocks [][]byte) []byte {
	out := make([]byte, 0, v.totalCodewords())
	maxData := 0
	for _, b := range dataBlocks {
		if len(b) > maxData {
			maxData = len(b)
		}
	}
	for i := 0; i < maxData; i++ {
		for _, b := range dataBlocks {
			if i < len(b) {
				out = append(out, b[i])
			}
		}
	}
	maxEC := 0
	for _, b := range ecBlocks {
		if len(b) > maxEC {
			maxEC = len(b)
		}
	}
	for i := 0; i < maxEC; i++ {
		for _, b := range ecBlocks {
			if i < len(b) {
				out = append(out, b[i])
			}
		}
	}
	return out
}

func (v qrVersion) deinterleave(serial []byte) (dataBlocks, ecBlocks [][]byte, ok bool) {
	if len(serial) < v.totalCodewords() {
		return nil, nil, false
	}
	dataLens, ecLens := v.blockLayout()
	nb := len(dataLens)
	dataBlocks = make([][]byte, nb)
	ecBlocks = make([][]byte, nb)
	for b := 0; b < nb; b++ {
		dataBlocks[b] = make([]byte, dataLens[b])
		ecBlocks[b] = make([]byte, ecLens[b])
	}

	pos := 0
	maxData := 0
	for _, d := range dataLens {
		if d > maxData {
			maxData = d
		}
	}
	for i := 0; i < maxData; i++ {
		for b := 0; b < nb; b++ {
			if i < dataLens[b] {
				dataBlocks[b][i] = serial[pos]
				pos++
			}
		}
	}
	maxEC := 0
	for _, e := range ecLens {
		if e > maxEC {
			maxEC = e
		}
	}
	for i := 0; i < maxEC; i++ {
		for b := 0; b < nb; b++ {
			if i < ecLens[b] {
				ecBlocks[b][i] = serial[pos]
				pos++
			}
		}
	}
	return dataBlocks, ecBlocks, true
}
