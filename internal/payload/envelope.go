/*
©AngelaMos | 2026
envelope.go

Pack and unpack the crypha payload envelope

Layout: magic(4) ver(1) flags(1) [cipher(1) params(9) salt(16) nonce(12) if encrypted]
        len(4) body(N) crc32(4). The header up to nonce is the AEAD additional data.
*/

package payload

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"hash/crc32"
)

func Pack(data []byte, opts Options) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyPayload
	}

	var flags byte
	body := data

	if opts.Compress {
		c, err := compress(body)
		if err != nil {
			return nil, err
		}
		body = c
		flags |= flagCompressed
	}

	header := new(bytes.Buffer)
	header.Write(magic[:])
	header.WriteByte(currentVersion)

	if len(opts.Passphrase) > 0 {
		flags |= flagEncrypted
		header.WriteByte(flags)

		salt := make([]byte, saltLen)
		if _, err := rand.Read(salt); err != nil {
			return nil, err
		}
		params := paramsForStrength(opts.Strength)
		key := deriveKey(opts.Passphrase, salt, params)
		aead, cipherID, err := newAEAD(opts.Cipher, key)
		if err != nil {
			return nil, err
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return nil, err
		}

		header.WriteByte(cipherID)
		var pbuf [paramsLen]byte
		binary.BigEndian.PutUint32(pbuf[0:4], params.time)
		binary.BigEndian.PutUint32(pbuf[4:8], params.memory)
		pbuf[8] = params.threads
		header.Write(pbuf[:])
		header.Write(salt)
		header.Write(nonce)

		body = aead.Seal(nil, nonce, body, header.Bytes())
	} else {
		header.WriteByte(flags)
	}

	out := new(bytes.Buffer)
	out.Write(header.Bytes())
	var meta [lenField]byte
	binary.BigEndian.PutUint32(meta[:], uint32(len(body)))
	out.Write(meta[:])
	out.Write(body)
	binary.BigEndian.PutUint32(meta[:], crc32.ChecksumIEEE(body))
	out.Write(meta[:])

	return out.Bytes(), nil
}

type parsed struct {
	flags    byte
	aad      []byte
	cipherID byte
	params   kdfParams
	salt     []byte
	nonce    []byte
	body     []byte
}

func parse(env []byte) (*parsed, error) {
	off := 0
	remaining := func(n int) bool { return off+n <= len(env) }

	if !remaining(len(magic)) || !bytes.Equal(env[0:len(magic)], magic[:]) {
		return nil, ErrBadMagic
	}
	off = len(magic)

	if !remaining(1) {
		return nil, ErrTruncated
	}
	if env[off] != currentVersion {
		return nil, ErrUnsupportedVersion
	}
	off++

	if !remaining(1) {
		return nil, ErrTruncated
	}
	p := &parsed{flags: env[off]}
	off++

	if p.flags&flagEncrypted != 0 {
		if !remaining(1 + paramsLen + saltLen + nonceLen) {
			return nil, ErrTruncated
		}
		p.cipherID = env[off]
		off++
		p.params = kdfParams{
			time:    binary.BigEndian.Uint32(env[off : off+4]),
			memory:  binary.BigEndian.Uint32(env[off+4 : off+8]),
			threads: env[off+8],
		}
		off += paramsLen
		if !p.params.valid() {
			return nil, ErrBadParams
		}
		p.salt = env[off : off+saltLen]
		off += saltLen
		p.nonce = env[off : off+nonceLen]
		off += nonceLen
		p.aad = env[0:off]
	}

	if !remaining(lenField) {
		return nil, ErrTruncated
	}
	bodyLen := int(binary.BigEndian.Uint32(env[off : off+lenField]))
	off += lenField
	if bodyLen < 0 || !remaining(bodyLen) {
		return nil, ErrTruncated
	}
	p.body = env[off : off+bodyLen]
	off += bodyLen

	if !remaining(crcField) {
		return nil, ErrTruncated
	}
	if crc32.ChecksumIEEE(p.body) != binary.BigEndian.Uint32(env[off:off+crcField]) {
		return nil, ErrChecksumMismatch
	}

	return p, nil
}

func Unpack(env, passphrase []byte) ([]byte, error) {
	p, err := parse(env)
	if err != nil {
		return nil, err
	}

	body := p.body
	if p.flags&flagEncrypted != 0 {
		if len(passphrase) == 0 {
			return nil, ErrPassphraseRequired
		}
		key := deriveKey(passphrase, p.salt, p.params)
		aead, err := aeadByID(p.cipherID, key)
		if err != nil {
			return nil, err
		}
		plain, err := aead.Open(nil, p.nonce, body, p.aad)
		if err != nil {
			return nil, ErrDecrypt
		}
		body = plain
	}

	if p.flags&flagCompressed != 0 {
		return decompress(body)
	}
	return body, nil
}

func Validate(env []byte) error {
	_, err := parse(env)
	return err
}

func IsEncrypted(env []byte) (bool, error) {
	p, err := parse(env)
	if err != nil {
		return false, err
	}
	return p.flags&flagEncrypted != 0, nil
}
