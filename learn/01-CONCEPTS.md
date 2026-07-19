<!-- ©AngelaMos | 2026 -->
<!-- 01-CONCEPTS.md -->

# crypha: Concepts

This chapter is the theory crypha is built on. It explains why concealment and secrecy are different problems, how each of the five carriers hides bytes, what the encrypted envelope does, and how a defender catches each carrier anyway. Every claim here is exercised by the code you will read in [03-IMPLEMENTATION.md](./03-IMPLEMENTATION.md).

## Steganography is not cryptography

These two words get used interchangeably and they should not be. They solve different problems and they compose.

- **Cryptography provides secrecy.** It transforms a message so that without the key, the content is unreadable. It does not hide that a message exists. A PGP block, an encrypted zip, a TLS session: all of them loudly announce "there is a secret here," they just deny you the content.
- **Steganography provides concealment.** It hides that a message exists at all, by embedding it in something that looks unremarkable. Classic steganography does not, by itself, protect the content: if you find the hiding scheme, you read the message.

The failure mode of each is the strength of the other. Encryption's opaque blob is a signal, and signals get flagged. A data-loss-prevention system does not need to break your encryption to stop an exfiltration; it just needs to notice a 40 KB high-entropy attachment leaving the network and quarantine it. Steganography defeats that by never producing a suspicious object in the first place. The carrier is a photo, a song, an invoice.

crypha layers both, in this order:

```
message ──> [ compress ] ──> [ encrypt ] ──> envelope bytes ──> [ hide in carrier ] ──> stego file
```

Encrypt for secrecy, then hide for concealment. If the concealment fails and someone extracts the envelope, they still hold ciphertext they cannot read. If the encryption were somehow broken, they would still have had to notice the payload was there at all. The two layers cover each other's weaknesses. This is the whole design thesis of the tool.

## The carrier idea

A **carrier** is a file format with room to store bytes that a casual observer, and ideally an automated one, will not notice. crypha treats a carrier as a black box with four operations: hide bytes in a cover, reveal bytes from a stego file, report how many bytes a given cover can hold, and sniff whether a file even looks like this kind of carrier. The bytes it stores are always the opaque envelope; the carrier never knows or cares what is inside.

There are two broad families of carrier, and crypha implements both:

- **Substitution carriers** overwrite low-importance bits of existing data. The image and audio carriers replace least-significant bits. The QR carrier substitutes whole codewords within an error-correction budget. These change the cover's data slightly and are detectable by statistics.
- **Additive / structural carriers** append data the format ignores, or use fields the format tolerates. The zero-width text carrier appends invisible runes. The PDF carriers attach a file, write custom metadata keys, or append bytes after the end-of-file marker. These do not alter the visible content at all, but they are trivially found by anyone who looks at the raw bytes.

Neither family is "better." They trade capacity, robustness, and stealth against each other differently, which is exactly why a teaching tool implements five.

## Channel one and two: least-significant-bit substitution

An 8-bit color channel stores a value from 0 to 255. Changing its lowest bit changes the value by at most 1, a difference no eye can see in a photograph. A 16-bit audio sample sits on a scale of roughly 98 dB of dynamic range; flipping its lowest bit moves it by about -96 dBFS, below the noise floor of any real listening environment. So both images and audio have a spare bit per sample that you can overwrite with payload, one payload bit at a time.

```
original R channel   1 0 1 1 0 0 1 [1]     payload bit = 0
after embedding      1 0 1 1 0 0 1 [0]     value changed 179 -> 178, invisible
                                  ^ least-significant bit carries one payload bit
```

Capacity is straightforward. An image holds `width x height x 3 / 8` bytes, three channels of RGB per pixel at one bit each, eight bits to a byte. The alpha channel is deliberately never touched, because a pixel at 254/255 alpha instead of 255 shows as a faint fringe on a non-white background and is a direct steganalysis tell. A 640x480 image gives `640 x 480 x 3 / 8 = 115,200` channel-bits worth of bytes; crypha reserves a four-byte length prefix and reports **115,196** usable envelope bytes. Audio holds `samples x channels / 8`; two seconds of 44.1 kHz mono holds **11,021**.

The catch, and it is a big one, is fragility. LSB steganography survives a byte-for-byte copy and nothing else. Re-encode the PNG as JPEG and the discrete-cosine quantization discards exactly the low-order spatial detail your payload lived in. Re-encode the WAV as MP3 and psychoacoustic compression throws away the inaudible bits, including yours. This is not a bug in crypha; it is the defining property of substitution in a lossless-only channel. crypha refuses JPEG covers outright for this reason.

## Channel three: Reed-Solomon error injection in QR codes

This is the showpiece, and it is the one people get wrong. A QR code is not just a bitmap of a URL. It is a systematic error-correcting code: the data is stored in **data codewords**, and alongside them the encoder computes **error-correction codewords** so a scanner can still read the code when part of it is dirty, torn, or obscured by a logo. At error-correction level H, a QR code can lose about 30% of its codewords and still decode.

The naive, wrong idea is "hide data in the error-correction bits." The correct idea inverts it. You leave the error-correction codewords untouched and you deliberately **corrupt the data codewords**, staying within the code's correction budget. Reed-Solomon over the field GF(2^8) can correct up to `t = floor((n - k) / 2)` unknown-location errors per block, where `n` is the total codewords in a block and `k` is the data codewords. So:

1. Encode the benign cover content (say, a URL) into a clean QR code.
2. Inject up to `t` codeword-errors per block into the data region. The pattern of errors is your payload.
3. An ordinary scanner's Reed-Solomon decoder corrects those errors, recovers the original URL, and shows the user a perfectly normal link. It never knows anything was there.
4. crypha reads the same code, re-derives what the clean data should have been by RS-decoding each block, diffs the corrected data against the received data, and reads the injected error pattern back as the payload.

```
clean data block      D0 D1 D2 D3 ...          EC codewords intact
inject payload        D0 D1^p0 D2 D3^p1 ...     up to t corrupted codewords per block
phone scanner         RS-corrects -> D0 D1 D2 D3, reads the URL, sees nothing
crypha                RS-corrects -> D0 D1 D2 D3, XOR-diffs -> p0 p1, reads the payload
```

The receiver needs only the code and the version, not a separate copy of the cover; the correction machinery reconstructs the clean data for it. This is "blind" extraction.

The honest cost is capacity. Each corrupted codeword carries one payload byte, and crypha stays at half the theoretical budget for decode margin, so a small QR holds tens of bytes. The 28-character cover `https://angelamos.com/crypha` gives an envelope capacity of **52 bytes**, which is **38 bytes** of plaintext after framing and **does not fit** an encrypted envelope at all. The widely-quoted 7/15/25/30% figures for L/M/Q/H are the error-correction fraction of total codewords, not payload size. Do not confuse them. QR is a plaintext-only carrier, and the tool says so.

## Channel four: zero-width Unicode text

Unicode contains characters that render to nothing. crypha uses exactly two:

```
U+200B  ZERO WIDTH SPACE  ->  bit 0
U+2060  WORD JOINER       ->  bit 1
```

Both are format characters with the `Default_Ignorable` property, both take zero display width, and neither causes any shaping or joining behavior, which rules out characters like U+200D (the zero-width joiner) that visibly fuse emoji. Eight of these runes encode one payload byte. crypha appends a framed run of them after the visible cover text, so `The quick brown fox.` still reads as `The quick brown fox.` while carrying an invisible payload behind it.

A persistent myth says Unicode normalization strips these characters. It does not. None of the four normalization forms, NFC, NFD, NFKC, NFKD, has a decomposition mapping for U+200B or U+2060, so all four leave them exactly in place. The myth comes from misreading two unrelated algorithms: IDNA2003 maps zero-width joiners to nothing, but only inside domain-name labels, not message text; and the UTS #39 confusable-skeleton algorithm removes ignorable characters, but that is an identifier-security check, not a text pipeline. A normal copy-paste, a database round-trip, or a chat message preserves the payload.

The trade-off here is the opposite of QR. Capacity is effectively unbounded and the visible text is byte-for-byte identical, but detection is trivial. This is concealment from a human skim, not a covert channel against a machine.

## Channel five: PDF structure

A PDF is a container with slack in several places, and crypha uses three, offered as `--technique`:

- **attachment** (default): PDF supports embedded file attachments as a first-class feature. crypha attaches the envelope as a lossless embedded file. It is robust, it survives a normal save in Acrobat or Preview, and it is the correct default for a teaching carrier because the round-trip is clean and easy to verify. Its stealth is the weakest of the three: the attachment shows in the panel and in `pdfinfo`.
- **metadata**: the PDF Info dictionary tolerates custom keys, and the spec requires readers to ignore keys they do not recognize. crypha stores a base64 payload across custom keys. It survives controlled delivery but is fragile against an "optimize" or "linearize" pass that drops non-standard keys.
- **append**: the spec says a reader seeks back to the last cross-reference table, so anything after the final `%%EOF` marker with no valid xref is ignored. crypha appends the framed envelope there. It is the purest "zero change to the document structure" option and survives naive copies, but any full save-as rewrite discards it.

All three are structural, not statistical, so none of them changes a single rendered pixel of the document. All three are found instantly by anyone who looks at the raw bytes.

## The encrypted envelope

Whatever the carrier, every payload is packed into one versioned envelope first. The carrier only ever stores these opaque bytes; all cryptography, compression, integrity, and versioning live in one place. This is the layout, with the exact overhead:

```
plaintext    magic(4) ver(1) flags(1) │ len(4) body(N) │ crc32(4)                              +14 bytes

encrypted    magic(4) ver(1) flags(1) cipher(1) params(9) salt(16) nonce(12) │ len(4) body(N+16) │ crc32(4)
             └───────────────── authenticated as AEAD associated data ───────┘                          +68 bytes
```

Three ideas do the work here.

**Argon2id for the key.** A passphrase is not a key. Argon2id (RFC 9106) is a memory-hard key-derivation function: it turns a passphrase plus a random 16-byte salt into a 32-byte key while deliberately burning memory and time, so an attacker guessing passphrases pays that cost per guess. crypha's default is the RFC's laptop-safe profile (64 MiB, three passes); `--strength high` uses the 2 GiB profile. The exact parameters used are written into the envelope, so `reveal` reproduces the same key without you re-declaring the profile.

**Authenticated encryption for secrecy and integrity together.** crypha defaults to ChaCha20-Poly1305 (RFC 8439), with AES-256-GCM behind `--cipher aes256gcm`. ChaCha20 is the default deliberately: it is constant-time in software on any CPU, while Go's AES-GCM is only constant-time with AES-NI hardware, and a portable file tool cannot assume the target has it. Both are AEAD constructions, meaning they produce a 16-byte authentication tag alongside the ciphertext. If any ciphertext byte is flipped, `Open` returns an error, not garbage.

**The header is authenticated as associated data.** This is the subtle, important part. The entire header up to and including the nonce is passed to the AEAD as "additional authenticated data." It is not encrypted, but it is covered by the tag. So an attacker cannot flip the "encrypted" flag, downgrade the cipher, or tamper with the Argon2id parameters without breaking authentication. An unknown version is rejected loudly rather than guessed. A tampered byte or a wrong passphrase fails closed.

One design question worth naming: crypha compresses **before** it encrypts. Compressing after encryption is pointless (ciphertext does not compress), but compressing before encryption is what enabled the CRIME and BREACH attacks on TLS. Those attacks require an adaptive network attacker who can inject chosen plaintext and watch the compressed-then-encrypted length across many requests. A one-shot offline file tool has no such oracle: there is no attacker in the loop injecting guesses. So compress-then-encrypt is safe here, and it means the ciphertext, and therefore the required carrier capacity, is smaller.

## Steganalysis: how each carrier gets caught

A teaching tool that only showed you how to hide would be lying by omission. Here is how a defender breaks each carrier. crypha's threat model assumes the defender is competent, which is exactly why the payload is always encrypted first.

**Image and audio LSB are caught by statistics.** LSB embedding leaves a faint, measurable fingerprint even though it is invisible. The classic attacks:
- **The chi-square attack** (Westfeld and Pfitzmann, 1999) exploits that sequential LSB embedding drives pairs of values, like 178 and 179, toward equal frequency. It measures how close each such pair is to a 50/50 split and flags images where they are suspiciously balanced.
- **RS analysis** (Fridrich, Goljan, and Du, 2001) flips LSBs in groups and watches how a "smoothness" measure of the image responds. Clean and embedded images respond differently, and the method estimates the embedded payload fraction, not just its presence.
- **Sample-pair analysis** (Dumitrescu, Wu, and Wang, 2002) does something similar with a precise statistical model of sample pairs.
- **StegExpose** (Boehm, 2014) fuses these into one detector for PNG and BMP.

The practical defense against all of them is to embed sparsely and randomly rather than filling every low bit sequentially, which is why crypha defaults to a single bit per channel and treats a 2-bit mode as a documented extension, not a default.

**Zero-width text is caught in one pass.** There is no statistical subtlety. `unicode.Is(unicode.Cf, r)` over the runes, or `grep -P '\xe2\x80\x8b'` over the bytes, or simply noticing that the visible character count is far below the total rune count, finds every hidden byte. There is no deniability against an automated scan, which is again why the payload is encrypted.

**QR injection hides from a scanner, not from an analyst.** A phone scanner genuinely cannot see the payload; its Reed-Solomon decoder erases it as noise. But an analyst who suspects a QR code and re-encodes the visible content into a clean QR can diff it against the suspect image, the same operation crypha's `reveal` performs, and recover the injected pattern. The hiding is robust against the intended consumer and transparent to a motivated investigator.

**PDF structural tricks are caught by reading the file.** `pdfinfo` and the attachment panel expose an attachment. `exiftool` dumps custom metadata keys. `strings file.pdf | tail` reveals bytes appended after `%%EOF`. None of these techniques is statistically covert; they hide from a person who opens the document, not from a person who inspects it.

The honest summary: crypha's carriers are teaching-grade concealment, and the encryption is production-grade secrecy. Against a competent inspector the concealment will often fail, and when it does, the encryption is what still protects the message. That layering is the point.

## Where to go next

[02-ARCHITECTURE.md](./02-ARCHITECTURE.md) turns these ideas into structure: the `Carrier` interface, the self-registering registry, the exact envelope encoding, and the auto-detect logic that keeps a QR-PNG from being mistaken for an ordinary image.
