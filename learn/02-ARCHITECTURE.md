<!-- ©AngelaMos | 2026 -->
<!-- 02-ARCHITECTURE.md -->

# crypha: Architecture

This chapter is the shape of the code: how the pieces are separated, why they are separated that way, and how a request flows from a keystroke to a hidden file. The guiding rule is a strict separation of concerns. Carriers know nothing about cryptography. The envelope knows nothing about carriers. The frontends know nothing about either. One engine in the middle wires them together.

## One engine, two frontends

crypha has two ways to drive it: a scriptable cobra CLI and a guided bubbletea wizard. A tempting but wrong design would let each frontend do its own hiding. That path duplicates the important logic, and the two copies drift. crypha instead puts every operation behind one package, `internal/engine`, and makes both frontends thin faces over it.

```
        cobra CLI ───┐
                     ├──>  engine  ──>  carrier registry  ──>  image  audio  qr  text  pdf
     bubbletea TUI ──┘         │
                               └──>  payload envelope (Argon2id · AEAD · flate · CRC32)  ──>  bitio
```

The engine exposes exactly the verbs the tool has: `Hide`, `Reveal`, `Capacity`, `CapacityAll`, `Catalog`, plus two preflight helpers, `Overhead` and `EnvelopeSize`, that let the wizard show an exact capacity meter before it commits. The cobra commands in `internal/cli` parse flags and call these. The bubbletea wizard in `internal/tui` collects the same inputs through a series of steps and calls the same functions. The TUI holds zero carrier logic; when it needs the exact envelope size for its live meter, it asks `engine.EnvelopeSize`, it does not reach into a carrier. Both auditors of this project confirmed the wizard is a pure view. That is the design working.

The payoff is that a bug fixed in the engine is fixed in both frontends at once, and a new carrier appears in both the CLI and the wizard without either frontend changing.

## The Carrier interface

Every hiding channel implements one small interface, in `internal/carrier/carrier.go`:

```go
type Carrier interface {
    Format() string
    Hide(cover io.Reader, payload []byte, out io.Writer) error
    Reveal(stego io.Reader) ([]byte, error)
    Capacity(cover io.Reader) (int, error)
    Sniff(stego io.ReadSeeker) bool
}
```

Four verbs and a name. `Hide` reads a cover and writes a stego file carrying the given bytes. `Reveal` reads a stego file and returns the bytes. `Capacity` reports how many bytes a cover can hold. `Sniff` is a cheap "does this file even look like my format" check used by auto-detect. Notice what is absent: the interface takes and returns `[]byte`, never a message, never a passphrase, never an `Options`. A carrier is handed the finished envelope and stores it. It does not know whether those bytes are encrypted, compressed, or plaintext, and it does not care. That ignorance is the separation of concerns made concrete.

Each carrier lives in its own package, `image`, `audio`, `qr`, `text`, `pdf`, and depends only on `internal/carrier` and standard or third-party libraries. No carrier imports another. You can read, test, or replace any one of them in isolation.

## The self-registering registry

Carriers register themselves. Each carrier package has an `init` function that calls `carrier.Register` with an instance of its type:

```go
func init() {
    carrier.Register(qrCarrier{})
}
```

The registry is a plain map from format name to `Carrier`, with `Register`, `Get`, `All`, `Formats`, and `Detect` helpers. The trick that makes registration fire is the blank-import aggregator `internal/carrier/all/all.go`, which imports every carrier package purely for its side effect:

```go
import (
    _ "github.com/CarterPerez-dev/crypha/internal/carrier/image"
    _ "github.com/CarterPerez-dev/crypha/internal/carrier/audio"
    // ... qr, text, pdf
)
```

The engine imports `all` once, also with a blank import. That single import chain runs every carrier's `init`, populating the registry before `main` starts. To add a sixth carrier you write its package with an `init` that registers it, add one line to `all.go`, and the engine, both frontends, `formats`, `capacity`, and auto-detect all pick it up with no further wiring. This is the plugin pattern that Go's `image` and `database/sql` packages use, applied here.

`All` returns the carriers sorted by name, so any iteration over them, capacity tables, auto-detect order, format listings, is deterministic. Determinism matters for the tests and for reproducible output.

## The engine as dispatcher

`internal/engine/engine.go` is the seam. Its job on `Hide` is three steps:

```go
func Hide(req HideRequest) (HideResult, error) {
    c, err := ResolveCarrier(req.Format, req.Technique)   // pick the carrier
    env, err := payload.Pack(req.Payload, req.Options)     // build the envelope
    err = c.Hide(req.Cover, env, req.Out)                  // store the opaque bytes
    return HideResult{ /* receipt */ }, nil
}
```

`ResolveCarrier` is where the one special case lives. Four of the five carriers are resolved by name straight out of the registry. PDF is different because it has techniques: `hide --format pdf --technique attachment|metadata|append`. So `ResolveCarrier` constructs a PDF carrier via `pdf.New(technique)` when the format is `pdf`, and rejects a `--technique` flag on any non-PDF format. This keeps the technique concept contained to the one carrier that has it, rather than polluting the interface for the other four.

`Reveal` runs the reverse: locate the carrier and extract the envelope, check whether the envelope is encrypted, demand a passphrase if it is and none was given, then `payload.Unpack`. `Capacity` and `CapacityAll` just call the carriers' `Capacity` and format the results into rows. The engine never touches bits or ciphers directly; it orchestrates the two subsystems that do.

## The envelope encoding

The envelope is built and parsed in `internal/payload/envelope.go` by `Pack`, `parse`, and `Unpack`. This is the exact byte layout, and the overhead is exact, not approximate:

```
plaintext    magic(4) ver(1) flags(1) │ len(4) body(N) │ crc32(4)                              14 bytes overhead

encrypted    magic(4) ver(1) flags(1) cipher(1) params(9) salt(16) nonce(12) │ len(4) body(N+16) │ crc32(4)
             └───────────────── AAD: authenticated, not encrypted ───────────┘                        68 bytes overhead
```

- **magic (4)** is a fixed constant, so `reveal` and auto-detect can reject random bytes cheaply before doing real work.
- **ver (1)** starts at 1. `parse` rejects any other version loudly rather than guessing at a layout it does not understand.
- **flags (1)** carries two bits today: encrypted and compressed, with room to grow.
- When encrypted, the header continues with **cipher (1)** identifying ChaCha20-Poly1305 or AES-256-GCM, **params (9)** holding the Argon2id time, memory, and threads, **salt (16)** for the key derivation, and **nonce (12)** for the AEAD.
- **len (4)** is the big-endian length of the body that follows.
- **body (N)** is the payload after the optional compress and encrypt steps. When encrypted it includes the 16-byte authentication tag.
- **crc32 (4)** covers the body. On the plaintext path it is the only integrity check and is what lets auto-detect confirm "this is a real crypha payload." On the encrypted path the AEAD tag already guarantees integrity, so the CRC is a cheap post-decrypt sanity check.

You can verify these numbers yourself. `crypha capacity -i cover.png --format image` on a 640x480 cover reports an envelope capacity of 115196, a max plaintext of 115182 (14 less), and a max encrypted of 115128 (68 less). The framing overhead is not a rounded estimate; it is these two constants.

The order of operations is the crux of correctness. On hide: compress if asked, then encrypt if asked, then frame. On reveal: unframe, then decrypt, then decompress. `Pack` writes the header first specifically so it can pass the header bytes to `aead.Seal` as associated data before it appends the ciphertext. `parse` records that same header slice as `aad` so `Unpack` can pass it to `aead.Open`. That is how tampering with the version or cipher choice breaks authentication.

## Bit I/O and the carrier plumbing

The substitution carriers need to read and write individual bits, not bytes, and they must agree on bit order. `internal/bitio` provides an MSB-first `BitReader` and `BitWriter`: the high bit of a byte is emitted first. As long as hide and reveal use the same order, any consistent choice works; crypha fixes MSB-first everywhere so a carrier written today and read tomorrow agree.

`internal/config` holds every constant the tool uses: the magic bytes, the KDF parameter profiles, the format catalog and its descriptions, the cipher identifiers. Nothing magic is hard-coded at a call site. `internal/report` renders results two ways, as human-readable aligned tables and, under a global `--json` flag, as machine-readable JSON where `reveal` base64-encodes the payload so binary data survives a pipe.

## Auto-detect, and the shadowing trap

The most interesting piece of the architecture is how `reveal` works with no `--format`. Naively you would sniff each carrier and use the first that says yes. That is exactly what the registry's `carrier.Detect` does, and it is not good enough, because of a shadowing problem:

**a QR stego is a PNG.** If auto-detect sniffs the image carrier first and the image carrier says "yes, I can read LSBs out of this PNG," it will happily extract 115 KB of noise and hand it back, shadowing the QR carrier that actually owns the bytes. Sniffing alone cannot tell the two apart, because the image carrier genuinely can read bits from any PNG; they just are not a valid payload.

The engine solves this in `detect` by adding a validation step. For each carrier in deterministic order it sniffs, then actually reveals, then validates the extracted envelope against the payload format:

```go
func detect(stego []byte) (carrier.Carrier, []byte, error) {
    for _, c := range carrier.All() {
        if !c.Sniff(bytes.NewReader(stego)) {
            continue
        }
        env, err := c.Reveal(bytes.NewReader(stego))
        if err != nil {
            continue
        }
        if payload.Validate(env) == nil {   // does it have our magic + a good CRC?
            return c, env, nil
        }
    }
    return nil, nil, ErrUndetected
}
```

A carrier only wins if the bytes it extracts carry the crypha magic and pass the CRC check. When the image carrier reads noise out of a QR-PNG, `payload.Validate` fails on the magic, the loop moves on, and the QR carrier, which extracts a real framed payload, wins. The correctness auditor for this stage confirmed the behavior empirically: across hundreds of QR stegos, none was shadowed by the image carrier, and across hundreds of random files, none produced a false positive. The `Sniff` step stays fast and envelope-agnostic; the validation step is what makes detection correct.

This is why `reveal` can auto-detect safely and why the DEMO can say, of a QR stego, "it must not be mistaken for an ordinary image carrier. It is not."

## The full data flow

Putting it together, here is one message from `crypha hide` to a file and back:

```
hide -m "meet at noon" --format image --encrypt
  │
  ├─ cli parses flags, resolves the passphrase (‑k / env / no-echo prompt)
  ├─ engine.Hide
  │     ├─ ResolveCarrier("image", "")            -> image carrier
  │     ├─ payload.Pack(msg, {encrypt, compress})
  │     │     compress > Argon2id(salt) > ChaCha20-Poly1305.Seal(header as AAD) > frame
  │     └─ image.Hide(cover, envelope, out)
  │           toNRGBA(cover) > write length prefix + envelope bits into RGB LSBs > png.Encode
  └─ receipt: image, 12 bytes payload, 92 bytes envelope, encrypted, compressed

reveal secret.png
  │
  ├─ engine.Reveal
  │     ├─ detect: sniff each carrier, reveal, payload.Validate -> image wins
  │     ├─ payload.IsEncrypted(env) -> true, so demand a passphrase
  │     └─ payload.Unpack(env, passphrase)
  │           unframe > Argon2id(stored salt+params) > AEAD.Open(header as AAD) > decompress
  └─ "meet at noon" to stdout, status to stderr
```

Every arrow crosses exactly one seam, and each seam is a package boundary you can test on its own. The next chapter walks the code that lives inside these boxes, with the QR carrier as the showpiece.

## Where to go next

[03-IMPLEMENTATION.md](./03-IMPLEMENTATION.md) is the code walkthrough: the NRGBA trap in the image carrier, the WAV encoder's seek-back, and the full QR-from-ISO/IEC-18004 injection and Reed-Solomon decode, function by function.
