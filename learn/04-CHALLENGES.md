<!-- ©AngelaMos | 2026 -->
<!-- 04-CHALLENGES.md -->

# crypha: Challenges

The best way to understand a hiding technique is to extend it, and the best way to understand a hiding technique's weakness is to build the tool that detects it. This chapter is a graded set of projects. Each one names the files and functions you would touch and the test that would prove you finished. They are ordered roughly by effort. Nothing here is a hint at incomplete work; the tool is complete, and these are the doors it deliberately leaves open.

Before you start: `just test` runs the suite, `just test-race` runs it under the race detector, and `just lint` runs the linter pinned to a Go 1.25 toolchain. Every challenge below should end with a green suite and a test that would have failed before your change.

## Warm-ups

**Add a new cipher to the envelope.** The envelope already carries a one-byte cipher identifier, and the AEAD is selected by `newAEAD` and `aeadByID` in `internal/payload`. Add a third AEAD (XChaCha20-Poly1305 is the natural choice; its 24-byte nonce removes any lingering nonce-collision worry) behind a new `--cipher` value. The interesting constraint is the nonce size: `Pack` already writes `aead.NonceSize()` bytes, so the encode side is nonce-agnostic, but `parse` reads a fixed 12-byte nonce field. Decide whether to widen the field (a version bump) or store the nonce length. This teaches you why the header is versioned. Prove it with a round-trip test and a known-answer test that a wrong key fails to open.

**Make the QR quiet zone and module size configurable.** `renderPNG` in `matrix.go` hard-codes an 8-pixel module and a 4-module quiet zone through `modulePixels` and `quietZoneModules`. Move them behind flags, then make `readGrid` tolerant of any module size it can infer rather than assuming 8. The test that matters: a stego produced at module size 4 must still round-trip through `Reveal`, and must still scan under the `gozxing` oracle in the QR test suite.

**Add a capacity-safety flag.** The QR carrier injects only half the correctable budget (`injectionSafetyRatio = 2` in `blocks.go`). Expose that ratio as a `--qr-margin` option so a user can trade robustness for capacity. Then measure the trade honestly: write a test that injects at ratio 1 (the full Reed-Solomon limit) and confirm whether `gozxing` still recovers the cover. You will learn where the real decode margin lives.

## Intermediate

**Build a variable-density LSB mode.** Both the image and audio carriers embed one bit per channel or sample. Two bits per sample doubles capacity and is the technique Invoke-PSImage actually used. Add a `--bits` option (1 or 2) to the image carrier. The work is in `pixOffset` and the read/write loops in `image.go`: instead of masking the single low bit, mask the low two bits and pack two payload bits per channel. The lesson is in the follow-up challenge, because 2-bit embedding is markedly easier to detect.

**Write a chi-square detector against crypha's own output.** This is the single most valuable challenge in the file, because it makes the concepts chapter concrete. Implement the Westfeld-Pfitzmann chi-square attack as a small command or test: for a suspect PNG, build the histogram of the RGB values, measure how close each pair of values that differ only in their low bit (2k and 2k+1) is to being equally frequent, and compute a chi-square statistic. Run it against a clean cover and against a crypha 1-bit stego and a crypha 2-bit stego of the same image. You should see the 2-bit stego light up clearly, the 1-bit stego light up faintly, and the clean cover stay quiet. You will have built the detector that breaks your own tool, which is the entire point of the project.

**Randomize the embedding path from the key.** crypha's image carrier fills channel LSBs sequentially, which is exactly what the chi-square attack keys on. Derive a permutation of the channel-slot order from the passphrase (or a stored seed) and embed along that permutation instead of in raster order. `Reveal` reconstructs the same permutation and reads in the same order. Then run your chi-square detector again: sequential detection should collapse, because the attack assumes contiguous embedding. This connects steganography and cryptography in one change, since the hiding order now depends on the key.

**Add the Tags-block alternate for zero-width text.** The text carrier uses U+200B and U+2060. The Unicode Tags block (U+E0000 to U+E007F) is also invisible and maps directly onto ASCII, so it can be more compact for text-heavy payloads, at the cost of a glaring byte signature (`F3 A0 8x`). Add it behind a flag in `text.go`, keeping the two-rune scheme the default. The teaching value is in documenting, in a test comment or the learn track, exactly why the default is the safer alphabet.

## Advanced

**Implement native FLAC output.** Today a FLAC cover is decoded and re-emitted as WAV, because the only Go FLAC encoder emits frames that strict parsers reject and has a panic path in `frame.Hash`. Implement `--flac-native` properly: encode back to FLAC, recover defensively from the known panic, and verify that crypha-in to crypha-out round-trips even if universal player compatibility does not hold. The honest deliverable is a test that proves the round-trip and a documented limit on which players accept the output. This is a real, unsolved-in-Go problem, not a toy.

**Implement the invisible-text PDF technique.** The PDF carrier offers attachment, metadata, and append. The fourth technique from the research, text drawn in render mode 3 (neither filled nor stroked, so invisible), is deferred because pdfcpu has no write-page-content API and it requires surgery on the PDF's internal object table that breaks between library versions. Building it means learning the PDF content-stream model deeply. Success is a stego PDF whose invisible text carries the envelope, survives a round-trip, and does not render anything a human sees.

**Push the QR carrier past version 10, or below level H.** The carrier fixes error-correction level H and supports versions 1 to 10, which bounds capacity at tens of bytes. Extend `versionTable` in `blocks.go` with the block structures for versions 11 and up (transcribed carefully from the ISO tables), or generalize `parseFormat` and the injection logic to accept levels Q, M, and L. Lower levels have fewer EC codewords per block, so `t` shrinks and capacity with it, which is a counter-intuitive result worth confirming with a test. Every addition must pass both differential oracles: your clean matrix must match `skip2`, and your stego must still scan under `gozxing`.

**Build adaptive, edge-aware embedding.** Uniform LSB embedding in a smooth region (a clear sky, a sustained tone) is where steganalysis is strongest, because the local statistics are predictable. Adaptive embedding hides only in high-variance regions, edges, texture, transients, where a one-bit change is statistically invisible. Compute a local variance map of the cover, embed only where variance exceeds a threshold, and store the threshold so `Reveal` walks the same regions. Then measure the result with your chi-square detector. This is the frontier of practical image steganography and a genuine research direction.

## A capstone: the full adversary loop

If you want one project that ties the whole tool together, build the complete loop crypha models: hide a payload, run a battery of your own detectors against the stego (chi-square, RS analysis, a zero-width rune scan, a `strings | tail` check on PDFs), score how detectable each carrier is, and then improve the weakest carrier until your own detectors miss it. When you can no longer detect your own embedding, you will have learned more about steganography than any tutorial teaches, because you will have played both sides of the exchange the tool exists to teach.

## Where to go next

You have read the whole track. Re-read [01-CONCEPTS.md](./01-CONCEPTS.md) with the code in [03-IMPLEMENTATION.md](./03-IMPLEMENTATION.md) fresh in mind; the theory reads differently once you have seen the nine lines of `injectFramed` that make it real. Then pick one challenge above and make the test suite fail, then pass.
