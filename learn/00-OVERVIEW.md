<!-- ©AngelaMos | 2026 -->
<!-- 00-OVERVIEW.md -->

# crypha: Overview

## What This Is

A multi-format steganography tool written in Go. You hand it a message or a file, and it seals that payload in a passphrase-encrypted, compressed, integrity-checked envelope, then hides the envelope inside an ordinary-looking carrier: the low bits of an image or an audio file, the Reed-Solomon slack of a QR code, zero-width characters in a block of text, or the structure of a PDF. Point `reveal` at the result and it auto-detects the carrier and hands the message back. It ships as a single static, dependency-free binary and drives from either a scriptable CLI or a guided terminal wizard.

The point of the project is to understand, by building it, the honest exchange between two adversaries. Steganography hides that a message exists; steganalysis is the statistical hunt that finds it anyway. crypha implements five very different hiding channels, each with its own capacity, fragility, and detection story, and the `learn/` track walks the analysis that breaks each one. Nothing here is a stub. Every carrier round-trips text and random-binary payloads under test, and the QR carrier is differentially tested against reference encoders and decoders.

## Why This Matters

Cryptography and steganography answer two different questions, and people constantly conflate them. Encryption makes a message unreadable. Steganography makes it unnoticeable. Encryption on its own still announces that a secret exists, and an opaque high-entropy blob is itself a signal, the exact thing a data-loss-prevention scanner, an intrusion-detection rule, or a border inspection is trained to flag. Steganography removes the signal by making the carrier look like a holiday photo, a voice memo, a PDF invoice, or a QR code on a poster.

The technique is not academic. It shows up in real intrusions on both sides of the line.

- **Witchetty, 2022.** The espionage group concealed a backdoor inside a bitmap of an old Windows logo hosted on a public cloud service, so the payload arrived on the target looking like an ordinary image download rather than malware (Symantec). The image still rendered as a logo. The code rode in the bits underneath.
- **The Stegano exploit kit, 2016.** Malicious script was hidden in the alpha channel of PNG banner ads served to millions of visitors on mainstream sites (ESET). A tiny per-pixel change to transparency, invisible on the page, carried the redirect logic.
- **Invoke-PSImage, 2017.** An open-source tool that packs a full PowerShell script into the two least-significant bits of the RGB channels of a PNG. It turned image LSB steganography into a point-and-click red-team primitive, and it is the direct ancestor of crypha's `image` carrier.
- **Stegoloader / Gatak, 2015.** This malware family pulled its own components out of images fetched at runtime, keeping the obvious executable code off disk where a scanner would look for it (Dell SecureWorks).

Defenders answer with steganalysis. The chi-square attack, RS analysis, and sample-pair analysis each estimate how much of an image has been touched, without ever needing the key. crypha exists to teach both moves. It encrypts first, so a discovered payload is still unreadable, then hides the ciphertext, and then tells you honestly how each carrier gets caught.

**Real-world scenarios where this applies:**
- **Covert-channel research.** Understanding how a payload survives, or fails to survive, a copy, a re-encode, a normalization pass, or a PDF save-as.
- **Blue-team detection.** Building the intuition to spot LSB embedding, zero-width runes, and appended-after-EOF data before an exfiltration tool uses them against you.
- **Learning applied cryptography.** The envelope is a complete, correct AEAD construction: Argon2id key derivation, ChaCha20-Poly1305, associated-data binding, and fail-closed verification, in about two hundred readable lines.

## What You'll Learn

**Security concepts:**
- **Steganography versus cryptography, and why you want both.** Concealment and secrecy are independent properties. crypha layers them: encrypt for secrecy, hide for concealment.
- **Five hiding channels and their trade-offs.** LSB in pixels and PCM samples, Reed-Solomon error injection in QR codes, zero-width Unicode in text, and three structural techniques in PDF, each with a different capacity, robustness, and detection profile.
- **Steganalysis, honestly.** How a defender detects each carrier: the chi-square and RS attacks on LSB, a one-pass rune scan on zero-width text, `strings | tail` on an appended PDF payload.
- **AEAD done correctly.** Why the header is authenticated as associated data, why a flipped byte must fail to open rather than decrypt to garbage, and why compress-then-encrypt is safe for an offline file tool when it would be dangerous for a network protocol.

**Technical skills:**
- **A plugin architecture in Go.** One `Carrier` interface, a self-registering registry via blank imports, and an engine that dispatches through it without knowing any carrier's internals.
- **Reimplementing a spec.** The QR carrier rebuilds module placement, mask reversal, block de-interleaving, and Reed-Solomon decode from ISO/IEC 18004, because the available Go library exposes only a finished bitmap.
- **Bit-level I/O and format quirks.** MSB-first bit packing, the mandatory NRGBA conversion that stops Go's PNG encoder from corrupting your low bits, and the WAV encoder's mandatory seek-back on close.
- **One engine, two frontends.** A shared brain that a cobra CLI and a bubbletea wizard both drive as thin, interchangeable faces, so the terminal UI holds zero carrier logic.

**Tools and techniques:**
- **`skip2/go-qrcode`** to generate a clean QR matrix, used as a generator and a differential-test oracle, never at runtime for decode.
- **`golang.org/x/crypto`** for Argon2id and ChaCha20-Poly1305, plus `crypto/aes` for the AES-256-GCM alternate.
- **`go-audio/wav`** for PCM read and write, and **`mewkiz/flac`** to decode a FLAC cover to samples.
- **`pdfcpu`** for lossless attachment and metadata techniques, and plain `io` for the append-after-EOF technique.

## Prerequisites

You do not need prior steganography experience. You do need some comfort with the following.

**Required knowledge:**
- **Go basics.** Structs, interfaces, slices, and errors. If you can read a method on an interface value, you can read this code.
- **Bytes and bits.** What a byte is, what "the least-significant bit" means, and that a `uint32` is four big-endian bytes on the wire.
- **What encryption is, roughly.** That a key turns readable data into unreadable data and back. The envelope chapter in [01-CONCEPTS.md](./01-CONCEPTS.md) explains the rest.

**Tools you'll need:**
- **A Go toolchain**, 1.25 or newer, only if you build from source. The prebuilt binary needs nothing. The `install.sh` script fetches a toolchain for you if one is missing and you asked to build.
- **Nothing else to run it.** No API keys, no network, no services. crypha is a one-shot offline file tool.

**Helpful but not required:**
- **ImageMagick and ffmpeg**, to synthesize throwaway covers for experimenting. [DEMO.md](../DEMO.md) shows the exact commands.
- A skim of how QR codes are structured, if you want the showpiece in [03-IMPLEMENTATION.md](./03-IMPLEMENTATION.md) to land faster.

## Quick Start

```bash
# Install (grabs a prebuilt binary, no Go needed):
curl -fsSL https://angelamos.com/crypha/install.sh | bash
# or, with a Go toolchain:
go install github.com/CarterPerez-dev/crypha/cmd/crypha@latest

# Launch the guided wizard:
crypha

# Ask how much a cover can hold:
crypha capacity -i photo.png

# Hide a message in an image, then read it back:
crypha hide -i photo.png -o secret.png --format image -m "meet at noon"
crypha reveal secret.png
```

Expected output: `capacity` prints a per-carrier table with an exact envelope byte count for the cover you gave it. `hide` prints a short receipt (output file, format, payload bytes, envelope bytes, whether it was encrypted and compressed). `reveal` needs no `--format`; it detects the carrier itself, writes the recovered message to stdout, and prints a one-line status to stderr like `revealed 11 bytes via image -> (stdout)`.

Add a passphrase and `reveal` asks for it before it decrypts:

```bash
crypha hide -i photo.png -o secret.png --format image -m "coordinates inside" --encrypt --compress
crypha reveal secret.png     # prompts for the passphrase, then decrypts
```

A passphrase from any source, the `-k` flag, the `CRYPHA_PASSPHRASE` environment variable, or the no-echo prompt, always means the payload is encrypted. crypha never silently writes plaintext when you asked for a key.

## Project Structure

```
steganography-multi-tool/
├── cmd/crypha/            # tiny main: it only calls cli.Execute()
├── internal/
│   ├── cli/               # cobra: hide reveal capacity formats version tui, plus secure passphrase input
│   ├── tui/               # the bubbletea wizard, a pure view over the engine
│   ├── engine/            # the shared brain both frontends call
│   ├── carrier/           # the Carrier interface + Register/Get/All/Detect registry
│   │   ├── all/           # blank-import aggregator that registers every carrier
│   │   └── image/ audio/ qr/ text/ pdf/
│   ├── payload/           # the encrypted envelope: Argon2id, AEAD, flate, CRC32, framing
│   ├── bitio/             # MSB-first BitReader / BitWriter
│   ├── config/            # every constant: magic bytes, KDF params, the format catalog
│   └── report/            # human tables and --json rendering
├── learn/                 # this teaching track
├── install.sh             # the one-shot curl-able installer
├── .goreleaser.yaml       # cross-platform release binaries
└── justfile               # every recipe
```

The single most important thing to understand first is the seam in `internal/engine/engine.go`. Both frontends call `engine.Hide`, `engine.Reveal`, and `engine.Capacity`; the engine wraps the payload in an envelope and dispatches to a carrier through the registry. Everything in `internal/carrier` exists to store and retrieve opaque bytes, and everything in `internal/payload` exists to build those bytes. Neither knows about the other.

## Next Steps

1. **Understand the ideas.** Read [01-CONCEPTS.md](./01-CONCEPTS.md) for steganography versus cryptography, each of the five hiding channels, the AEAD envelope, and the steganalysis that breaks each carrier, grounded in real incidents.
2. **See the design.** Read [02-ARCHITECTURE.md](./02-ARCHITECTURE.md) for the one-engine-two-frontends design, the self-registering carrier registry, the exact envelope byte layout, and how auto-detect avoids mistaking a QR-PNG for an ordinary image.
3. **Walk the code.** Read [03-IMPLEMENTATION.md](./03-IMPLEMENTATION.md) to trace a payload from message to hidden bytes, with the QR-from-ISO/IEC-18004 Reed-Solomon injection as the showpiece.
4. **Extend it.** Read [04-CHALLENGES.md](./04-CHALLENGES.md) for projects from a new carrier to variable-density LSB modes to stronger steganalysis resistance.

## Common Issues

**`hide --format image` refuses the cover**
```
crypha: image cover must be an 8-bit PNG or a 24-bit BMP
```
Solution: the image carrier refuses paletted and 16-bit PNGs on purpose, because editing palette indices or high bytes would visibly wreck the cover. ImageMagick defaults to a 16-bit PNG for synthetic gradients; force 8-bit truecolor with `-depth 8 PNG24:cover.png`. A normal photo saved as PNG or a 24-bit BMP just works.

**`reveal` says it found nothing**
```
crypha: no crypha payload detected; pass a format to force a carrier
```
Solution: the file either does not contain a crypha payload, or the carrier was destroyed. LSB carriers survive a byte-for-byte copy but not a re-encode; if you re-saved the stego PNG as JPEG, or the WAV as MP3, the payload is gone. If you know the format, pass `--format` to skip detection and get a specific error.

**`capacity` on a QR cover says "does not fit" for encryption**
```
qr   52   38   does not fit
```
Solution: this is correct, not a bug. A QR code holds only tens of bytes in its correctable-error budget, and an encrypted envelope adds 68 bytes of overhead. QR is a plaintext-only carrier by design, and `capacity` tells you so up front.

## Related Projects

If you found this interesting, look at:
- **metadata-scrubber-tool**: the inverse instinct, stripping the hidden metadata out of a file instead of adding to it.
- **binary-analysis-tool**: static analysis of a binary's structure, the same "read a file format precisely" muscle applied to executables.
- **nadezhda** (security-news-scraper): the same single-static-binary, one-engine-two-frontends shape, applied to security-news intelligence.
