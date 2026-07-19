/*
©AngelaMos | 2026
crypto.go

Argon2id key derivation and AEAD selection for the payload envelope
*/

package payload

import (
	"crypto/aes"
	"crypto/cipher"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

type kdfParams struct {
	time    uint32
	memory  uint32
	threads uint8
}

func paramsForStrength(s Strength) kdfParams {
	if s == StrengthHigh {
		return kdfParams{time: argonHighTime, memory: argonHighMemory, threads: argonThreads}
	}
	return kdfParams{time: argonDefaultTime, memory: argonDefaultMemory, threads: argonThreads}
}

func (p kdfParams) valid() bool {
	if p.threads == 0 || p.time == 0 {
		return false
	}
	if p.memory > argonMaxMemory {
		return false
	}
	return p.memory >= 8*uint32(p.threads)
}

func deriveKey(passphrase, salt []byte, p kdfParams) []byte {
	return argon2.IDKey(passphrase, salt, p.time, p.memory, p.threads, keyLen)
}

func newAEAD(c Cipher, key []byte) (cipher.AEAD, byte, error) {
	switch c {
	case CipherAES256GCM:
		aead, err := gcm(key)
		return aead, cipherIDAES256GCM, err
	case CipherChaCha20, "":
		aead, err := chacha20poly1305.New(key)
		return aead, cipherIDChaCha20, err
	default:
		return nil, 0, ErrUnknownCipher
	}
}

func aeadByID(id byte, key []byte) (cipher.AEAD, error) {
	switch id {
	case cipherIDAES256GCM:
		return gcm(key)
	case cipherIDChaCha20:
		return chacha20poly1305.New(key)
	default:
		return nil, ErrUnknownCipher
	}
}

func gcm(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
