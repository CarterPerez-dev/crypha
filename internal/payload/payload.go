/*
©AngelaMos | 2026
payload.go

Envelope types, constants, and errors shared across the crypha payload codec
*/

package payload

import "errors"

type Cipher string

const (
	CipherChaCha20  Cipher = "chacha20"
	CipherAES256GCM Cipher = "aes256gcm"
)

type Strength string

const (
	StrengthDefault Strength = "default"
	StrengthHigh    Strength = "high"
)

type Options struct {
	Passphrase []byte
	Compress   bool
	Cipher     Cipher
	Strength   Strength
}

const (
	currentVersion byte = 0x01

	flagEncrypted  byte = 1 << 0
	flagCompressed byte = 1 << 1

	cipherIDChaCha20  byte = 0x00
	cipherIDAES256GCM byte = 0x01

	saltLen    = 16
	nonceLen   = 12
	keyLen     = 32
	lenField   = 4
	crcField   = 4
	paramsLen  = 9
	versionLen = 1
	flagsLen   = 1
	cipherLen  = 1
	tagLen     = 16

	argonDefaultTime   uint32 = 3
	argonDefaultMemory uint32 = 64 * 1024
	argonHighTime      uint32 = 1
	argonHighMemory    uint32 = 2048 * 1024
	argonMaxMemory     uint32 = argonHighMemory
	argonThreads       uint8  = 4
)

var magic = [4]byte{0xC7, 0x1A, 0x9E, 0x5B}

var (
	ErrEmptyPayload       = errors.New("crypha: empty payload")
	ErrBadMagic           = errors.New("crypha: not a crypha payload (bad magic)")
	ErrUnsupportedVersion = errors.New("crypha: unsupported envelope version")
	ErrTruncated          = errors.New("crypha: truncated envelope")
	ErrBadParams          = errors.New("crypha: invalid key-derivation parameters")
	ErrChecksumMismatch   = errors.New("crypha: checksum mismatch")
	ErrPassphraseRequired = errors.New("crypha: payload is encrypted, passphrase required")
	ErrDecrypt            = errors.New("crypha: decryption failed (wrong passphrase or tampered data)")
	ErrUnknownCipher      = errors.New("crypha: unknown cipher")
)

func Overhead(encrypted bool) int {
	base := len(magic) + versionLen + flagsLen + lenField + crcField
	if !encrypted {
		return base
	}
	return base + cipherLen + paramsLen + saltLen + nonceLen + tagLen
}

func EnvelopeSize(data []byte, opts Options) (int, error) {
	body := data
	if opts.Compress {
		c, err := compress(data)
		if err != nil {
			return 0, err
		}
		body = c
	}
	return Overhead(len(opts.Passphrase) > 0) + len(body), nil
}
