<!-- ©AngelaMos | 2026 -->
<!-- 03-IMPLEMENTATION.md -->

# crypha: Implementation

This chapter walks the code. It starts with the envelope every carrier shares, moves through the three simpler carriers to build intuition, and then spends most of its length on the showpiece: the QR carrier, which reimplements QR module geometry and Reed-Solomon decoding from ISO/IEC 18004 because no Go library exposes what the covert channel needs. Function names are given so you can find each piece; there are no line-number references, since those rot the moment the file changes.

## The envelope: `payload.Pack` and `payload.Unpack`

Everything hidden by any carrier is first built by `Pack` in `internal/payload/envelope.go`. `Pack` takes the raw payload and an `Options` struct and returns the framed envelope bytes. The body of the function is the order of operations from the concepts chapter, made literal:

```go
if opts.Compress {
    body, _ = compress(body)          // flate, sets flagCompressed
    flags |= flagCompressed
}
header.Write(magic[:])
header.WriteByte(currentVersion)
if len(opts.Passphrase) > 0 {
    flags |= flagEncrypted
    header.WriteByte(flags)
    // salt, params, cipher id, nonce all written into the header ...
    body = aead.Seal(nil, nonce, body, header.Bytes())   // header IS the AAD
} else {
    header.WriteByte(flags)
}
```

The single most important line is `aead.Seal(nil, nonce, body, header.Bytes())`. The fourth argument is the associated data. By passing the entire header, magic, version, flags, cipher id, Argon2id parameters, salt, and nonce, `Pack` binds all of that metadata to the ciphertext's authentication tag. Nothing in the header is secret, but nothing in the header can be changed without invalidating the tag.

`parse` reads the layout back and, crucially, records the exact header slice it consumed as `p.aad`. `Unpack` then calls `aead.Open(nil, p.nonce, body, p.aad)` with that same slice. If an attacker flipped the encrypted flag, downgraded the cipher, or edited the KDF parameters, the `aad` no longer matches what was sealed and `Open` returns an error. crypha maps that to `ErrDecrypt`, "decryption failed (wrong passphrase or tampered data)," and exits non-zero. It never returns partially-decrypted garbage.

The Argon2id parameters travel in the envelope precisely so `reveal` needs no flags. `parse` reads `params` into a `kdfParams` struct, `Unpack` calls `deriveKey(passphrase, p.salt, p.params)`, and the exact same key falls out. `parse` also validates the parameters before use (`p.params.valid()`), because `argon2.IDKey` panics rather than errors on a zero time cost or an impossible memory-to-threads ratio, and a hostile envelope must not be able to trigger that panic.

## The image carrier: the NRGBA trap

`internal/carrier/image/image.go` is short, and almost all of its subtlety is in one helper, `toNRGBA`. The naive version of an LSB image carrier decodes the PNG, edits the low bits, and re-encodes. It produces corrupt output, and the reason is a genuine Go gotcha.

A standard 24-bit truecolor PNG decodes to `*image.RGBA`, which stores color with **premultiplied** alpha. When you hand an `*image.RGBA` to `png.Encode`, the encoder runs an alpha-unmultiply pass that does arithmetic on your pixel values, and that arithmetic rewrites exactly the low bits you just carefully set. Your payload is destroyed on the way out. A color-type-6 PNG decodes to `*image.NRGBA` (non-premultiplied), and for that type `png.Encode` copies the pixel bytes directly with no color math, so the low bits survive.

The fix is to always convert the cover to `*image.NRGBA` before touching it:

```go
func toNRGBA(src stdimage.Image) *stdimage.NRGBA {
    if n, ok := src.(*stdimage.NRGBA); ok && n.Rect.Min == (stdimage.Point{}) {
        return n
    }
    dst := stdimage.NewNRGBA(stdimage.Rect(0, 0, b.Dx(), b.Dy()))
    draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
    return dst
}
```

For an opaque source, every pixel at alpha 255, the RGBA-to-NRGBA conversion is exactly lossless, so this costs nothing but a copy. `Hide` then walks payload bits into the low bit of each channel with `pixOffset`, which is where the alpha channel gets protected:

```go
func pixOffset(slot int) int {
    return bytesPerPixel*(slot/channelsPerPixel) + (slot % channelsPerPixel)
}
```

The NRGBA pixel buffer is laid out `[R G B A R G B A ...]`, four bytes per pixel. By mapping slot `n` to pixel `n/3` and channel `n%3`, `pixOffset` visits only R, G, and B and steps over every A. The alpha byte is never written, because a pixel at alpha 254 instead of 255 shows as a faint fringe on a composited background and is a direct steganalysis tell.

Before any of this, `rejectLossy` refuses covers that cannot round-trip: `*image.Paletted` (editing a palette index jumps to a whole different color) and the 16-bit types like `*image.RGBA64` (eight bytes per pixel, a different layout). crypha would rather refuse a cover than silently mangle it, which is why the DEMO forces `-depth 8 PNG24:` for its synthetic gradient. A `frame` helper prefixes the payload with a big-endian `uint32` length so `Reveal` reads the length first, then exactly that many payload bytes, and stops.

## The zero-width text carrier: exact-length framing

`internal/carrier/text/text.go` maps bits to the two invisible runes and back. `Hide` builds a frame of `[magic][length][payload]`, walks it bit by bit with the shared `bitio.NewReader`, appends `zeroRune` (U+200B) for a 0 and `oneRune` (U+2060) for a 1, and writes the visible cover followed by the invisible run. Reading is the mirror: `extractBits` ranges over the runes and collects a 0 or 1 for each of the two carrier characters, ignoring everything else.

The interesting defensive detail is `parseFrame`. Because this carrier can be stacked (a stego text can itself be used as a cover), and because a scanner walks every possible magic offset in `findFrame`, the parser must not accept a frame whose declared length disagrees with the bits actually present:

```go
remaining := len(bits) - payloadStart
if uint64(length)*bitsPerByte != uint64(remaining) {
    return nil, false        // declared length must consume the run exactly
}
```

Requiring the payload to consume the run to its exact end, rather than merely fit inside it, is what stops a spurious earlier magic match from shadowing the real frame. This exactness was a finding fixed during the carrier's audit, and it is the difference between a robust parser and one that occasionally reads the wrong bytes.

## The audio carrier: WAV, and the mandatory Close

`internal/carrier/audio` decodes 16-bit PCM samples with `go-audio/wav`, sets the low bit of each sample the same way the image carrier sets a channel LSB, and re-encodes. The one non-obvious requirement is that the WAV encoder's `Close` is mandatory, not optional cleanup: WAV writes a data-chunk size field in its header that is only known once all samples are written, so `Close` seeks back to the start and patches it. crypha writes to an in-memory `writeseeker` so the encoder can perform that seek even when the ultimate destination is a plain `io.Writer` that cannot seek. A FLAC cover is decoded to the same PCM samples with `mewkiz/flac` and then re-emitted as WAV; native FLAC output is deferred because the only Go FLAC encoder emits frames that strict parsers reject.

## The showpiece: QR Reed-Solomon error injection

Now the main event. The QR carrier hides a payload as deliberate, correctable errors in a QR code's data codewords, so an ordinary phone scanner reads the visible cover and silently repairs the injected errors away, while crypha reads them back as the secret. This is the highest-effort carrier in the project, because the Go ecosystem gives you almost nothing to work with.

### The problem: `skip2` only hands you a finished bitmap

`github.com/skip2/go-qrcode` can generate a QR code and, via `NewWithForcedVersion`, pin its version and error-correction level. But its public surface exposes only `Bitmap() [][]bool`: the final, masked, fully-assembled matrix with function patterns baked in. The codewords, the block structure, and the chosen data mask are all unexported. To inject errors into specific data codewords and later read them back, crypha has to take that finished bitmap apart and reconstruct everything the library hid, from the spec. That reconstruction lives across `qr.go`, `matrix.go`, `blocks.go`, `gf.go`, and `rs.go`. crypha fixes the error-correction level at H and supports versions 1 through 10.

Here is the whole `Hide` pipeline, then each stage.

```go
code, version, _ := selectVersion(string(coverText), len(payload))  // 1. clean QR
clean, _ := matrixFromBitmap(code.Bitmap(), version)                // 2. bitmap -> matrix
maskID, level, ok := parseFormat(clean)                             // 3. read format info
isFunc := functionModules(version)                                  // 4. function map
order := placementOrder(version, isFunc)                            // 5. zigzag order
serial := readSerial(clean, order, maskID, spec.totalCodewords())   //    unmask + read codewords
dataBlocks, ecBlocks, _ := spec.deinterleave(serial)               // 6. split into blocks
framed := frame(payload)
injectFramed(dataBlocks, spec, framed)                              // 7. inject errors
stego := clean.clone()
writeSerial(stego, order, maskID, spec.interleave(dataBlocks, ecBlocks))  // 8. re-lay + write
renderPNG(stego, out)                                              //    encode PNG
```

### Stage 1: a clean cover matrix

`selectVersion` asks `skip2` for the smallest supported version (1 to 10) whose H-level capacity holds the payload, and whose cover text fits. It disables the library's quiet-zone border so crypha controls the geometry. `matrixFromBitmap` copies the returned `[][]bool` into crypha's `matrix` type, checking that the dimensions match the version's expected `symbolSize` (21 modules for version 1, growing by 4 per version).

### Stage 3: reading the format information

The format information is a 15-bit field stored twice in the code, encoding the mask id and error-correction level under a BCH error-correcting code. `parseFormat` reads the 15 module cells listed in `formatBitCells`, then recovers the intended value the way a real decoder does, by nearest codeword:

```go
for d := 0; d < formatCodeCount; d++ {
    dist := bits.OnesCount(uint(stored ^ formatCode(d)))
    if dist < bestDist { bestDist, bestData = dist, d }
}
if bestData < 0 || bestDist > formatMaxDistance { return 0, 0, false }
```

`formatCode` recomputes the BCH encoding of each of the 32 possible format values (including the mandatory mask `0x5412`), and the loop picks whichever is closest in Hamming distance to what was read, rejecting anything more than the code's 3-bit correction radius away. crypha then insists the level is H; any other level means this is not a crypha QR.

### Stage 4: the function map

Payload can only ride in data modules, never in the fixed patterns a scanner uses to orient itself. `functionModules` builds a boolean grid marking every non-data module for the given version: the three finder patterns and their separators, the two timing lines, the alignment patterns (looked up per version in `alignmentCenters`, skipping any that collide with a finder block), and, for version 7 and up, the two version-information blocks. This mask is what the placement walk consults to know which cells to skip.

One subtlety worth calling out: the finder-plus-separator block is 8 modules across, not the 7 of the finder pattern alone. Getting that edge off by one shifts the entire codeword placement and produces garbage. That exact off-by-one, the top-right and bottom-left finder blocks being marked 8 wide rather than 9 or 7, was one of two real bugs the differential tests caught during development. The test that catches it is described at the end of this section.

### Stage 5: the zigzag placement and mask reversal

QR modules are filled in a serpentine order: two columns at a time, starting from the bottom-right, snaking up then down, skipping function modules and the vertical timing line. `placementOrder` reproduces that walk exactly, appending each data-module coordinate to an ordered slice until it has visited every data module the version has:

```go
x := size - 2; y := size - 1; dirUp := true; xOffset := 1
for i := 0; i < count; i++ {
    order = append(order, point{x: x + xOffset, y: y})
    // toggle between the two columns, step up or down, bounce at the edges,
    // hop over the timing column, and skip any function module
}
```

With that ordering in hand, `readSerial` walks it one codeword (8 modules) at a time and reads the bits, but with a mask reversal baked in. A QR code XORs a checkerboard-like mask over its data region so no large blank areas confuse a scanner. To recover the true codeword bits you must XOR the same mask back off:

```go
v := m.grid[p.y][p.x]
if maskBit(maskID, p.y, p.x) {
    v = !v            // undo the data mask
}
```

`maskBit` implements all eight ISO mask formulas. Skipping this step is not a subtle error; it turns every codeword into noise, and an early version of the carrier that omitted it produced a meaningless module diff. `writeSerial` is the exact inverse, re-applying the mask as it lays codewords back down.

### Stage 6: de-interleaving into blocks

A QR code does not store its blocks end to end; it interleaves their codewords so a physical smudge damages a little of each block rather than destroying one entirely. `blocks.go` holds the per-version, level-H block tables (`versionTable`), and `deinterleave` reverses the interleave to recover the individual data and error-correction blocks, using `blockLayout` to know each block's data and EC lengths. `interleave` is its inverse for the write path. These tables are transcribed from the ISO block-structure tables; they are the least glamorous and most error-prone part of the whole carrier, which is why the differential test against `skip2`'s clean bitmap matters so much.

### Stage 7: injecting the payload

This is the actual hiding, and it is nine lines. `injectFramed` distributes the framed payload bytes across the blocks round-robin, XORing each payload byte into one data codeword:

```go
for t := 0; t < len(framed); t++ {
    block := t % nb          // spread across blocks round-robin
    slot := t / nb           // next free slot within the block
    if slot >= inject || slot >= len(dataBlocks[block]) {
        return ErrPayloadTooLarge
    }
    dataBlocks[block][slot] ^= framed[t]
}
```

The budget is `inject := spec.injectPerBlock()`, which is `correctable() / 2`, which is `(ecPerBlock / 2) / 2`. The first halving is the Reed-Solomon limit itself, `t = floor((n - k) / 2)` correctable errors per block. The second halving is a deliberate safety margin (`injectionSafetyRatio = 2`): crypha injects only half the errors the code can correct, so the visible content still decodes with comfortable margin on a real scanner. This is exactly why capacity is honest and small. A version-10 code has 8 blocks, 28 EC codewords per block, so `correctable` is 14 and `injectPerBlock` is 7; `8 x 7` is 56 codewords, minus the 4-byte frame prefix, gives the **52**-byte envelope capacity you see in `capacity`.

Because a corrupted data codeword differs from the clean one, and the clean one is recoverable by Reed-Solomon, XOR is the perfect channel: `stego = clean XOR payload`, so `payload = clean XOR stego`, and the receiver recovers `clean` for free by decoding.

### Stage 8: writing it back out

`interleave` re-serializes the now-corrupted data blocks with the untouched EC blocks, `writeSerial` lays them back into a clone of the clean matrix (re-applying the mask), and `renderPNG` scales each module to an 8x8 pixel block, adds a 4-module quiet zone, and encodes a grayscale PNG. The result scans as the cover on any phone.

### Reveal: correcting the errors to read them

`Reveal` reverses the geometry, exactly the same `parseFormat` to `deinterleave` chain, and then `extractFramed` does the cryptographically-satisfying part: it recovers each block's clean data by Reed-Solomon decoding, and XOR-diffs it against what was received.

```go
recv := append(append([]byte(nil), dataBlocks[b]...), ecBlocks[b]...)
corrected, err := rsDecode(recv, ec)          // repair the injected errors
clean[b] = corrected[:len(dataBlocks[b])]
// ... then framed[t] = clean[block][slot] ^ dataBlocks[block][slot]
```

`rsDecode`, in `rs.go`, is a from-scratch systematic Reed-Solomon decoder over GF(2^8). It is worth understanding because it is the machinery a phone scanner runs too:

1. **Syndromes** (`rsSyndromes`). Evaluate the received block as a polynomial at successive powers of the field generator. If every syndrome is zero, there are no errors and the block is returned as-is.
2. **Error locator** (`rsErrorLocator`). The Berlekamp-Massey algorithm builds the error-locator polynomial from the syndromes. Its degree is the number of errors; if that exceeds the correctable budget, the block is declared uncorrectable.
3. **Error positions** (`rsErrorPositions`). A Chien search evaluates the locator at every position to find the roots, which point at the corrupted codewords. If the root count does not match the locator degree, it fails cleanly.
4. **Error magnitudes** (`rsSolveMagnitudes`). With positions known, crypha solves a small Vandermonde linear system by Gaussian elimination over the field to find each error's value, then verifies the solution reproduces the syndromes before trusting it.

The field arithmetic underneath, in `gf.go`, is the standard log/exp-table trick: `init` walks the powers of the generator under the QR primitive polynomial `0x11D` to fill `gfExp` and `gfLog`, so multiplication becomes an addition of logarithms and inversion becomes a subtraction. This is the same GF(2^8) every QR implementation uses.

The Chien search hides the second of the two real bugs the differential tests caught: the mapping from a root of the locator polynomial back to a codeword index depends on a position convention, and getting that convention backward finds the right number of errors at the wrong places. The fix is the `(len(recv)-1-p)` term you see in both `rsErrorPositions` and the magnitude setup; it aligns the locator's root positions with the codeword ordering the encoder used.

### The two oracles that keep it honest

crypha reimplemented a spec, and a spec reimplementation is exactly where silent, plausible-looking bugs breed. The defense is differential testing against two independent reference implementations, and neither is used at runtime:

- **Generation oracle: `skip2/go-qrcode`.** For a given cover and version, crypha's own clean-matrix reconstruction must match `skip2`'s `Bitmap()` bit for bit. This validates placement, masking, and block handling all at once. The 8-wide finder-block off-by-one failed this test loudly.
- **Scan oracle: `makiuchi-d/gozxing`.** After injection, the stego PNG must still decode, via a completely independent QR decoder, back to the original cover content. This proves the injection stayed inside the correction budget and the code genuinely self-heals. If crypha ever injected one error too many, `gozxing` would fail to recover the cover and the test would catch it.

Both oracles are test-only dependencies. At runtime crypha does its own generation introspection and its own Reed-Solomon decode; it never calls `gozxing` to read a code. That independence is the whole point: a bug in crypha's decoder cannot hide behind the same bug in its encoder, because the encoder is `skip2` and the scan check is `gozxing`.

## Where to go next

[04-CHALLENGES.md](./04-CHALLENGES.md) turns the reader loose: add a sixth carrier, build a variable-density LSB mode, implement a chi-square detector against crypha's own output, or push the QR carrier past version 10.
