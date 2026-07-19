/*
©AngelaMos | 2026
envelope_test.go

Round-trip, integrity, and hardening tests for the payload envelope
*/

package payload

import (
	"bytes"
	"errors"
	"testing"
)

var secret = []byte("the eagle lands at midnight -- coordinates 41.40N 2.17E")

func TestRoundTrip(t *testing.T) {
	pass := []byte("correct horse battery staple")
	cases := []struct {
		name string
		opts Options
	}{
		{"plain", Options{}},
		{"compressed", Options{Compress: true}},
		{"encrypted-chacha", Options{Passphrase: pass}},
		{"encrypted-aes", Options{Passphrase: pass, Cipher: CipherAES256GCM}},
		{"encrypted-compressed", Options{Passphrase: pass, Compress: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, err := Pack(secret, tc.opts)
			if err != nil {
				t.Fatalf("Pack: %v", err)
			}
			got, err := Unpack(env, tc.opts.Passphrase)
			if err != nil {
				t.Fatalf("Unpack: %v", err)
			}
			if !bytes.Equal(got, secret) {
				t.Errorf("round-trip mismatch: got %q", got)
			}
		})
	}
}

func TestWrongPassphraseFails(t *testing.T) {
	env, err := Pack(secret, Options{Passphrase: []byte("right")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Unpack(env, []byte("wrong")); !errors.Is(err, ErrDecrypt) {
		t.Errorf("got %v want ErrDecrypt", err)
	}
}

func TestMissingPassphraseFails(t *testing.T) {
	env, err := Pack(secret, Options{Passphrase: []byte("right")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Unpack(env, nil); !errors.Is(err, ErrPassphraseRequired) {
		t.Errorf("got %v want ErrPassphraseRequired", err)
	}
}

func TestTamperDetected(t *testing.T) {
	env, err := Pack(secret, Options{Passphrase: []byte("right")})
	if err != nil {
		t.Fatal(err)
	}
	env[len(env)-6] ^= 0xFF // flip a byte inside the ciphertext body
	if _, err := Unpack(env, []byte("right")); err == nil {
		t.Error("expected an error on tampered envelope")
	}
}

func TestChecksumMismatch(t *testing.T) {
	env, err := Pack(secret, Options{})
	if err != nil {
		t.Fatal(err)
	}
	env[len(env)-5] ^= 0xFF // flip a body byte on the plaintext path
	if _, err := Unpack(env, nil); !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("got %v want ErrChecksumMismatch", err)
	}
}

func TestBadMagic(t *testing.T) {
	env, _ := Pack(secret, Options{})
	env[0] ^= 0xFF
	if _, err := Unpack(env, nil); !errors.Is(err, ErrBadMagic) {
		t.Errorf("got %v want ErrBadMagic", err)
	}
}

func TestVersionReject(t *testing.T) {
	env, _ := Pack(secret, Options{})
	env[len(magic)] = 0x7F
	if _, err := Unpack(env, nil); !errors.Is(err, ErrUnsupportedVersion) {
		t.Errorf("got %v want ErrUnsupportedVersion", err)
	}
}

func TestTruncated(t *testing.T) {
	env, _ := Pack(secret, Options{})
	if _, err := Unpack(env[:5], nil); !errors.Is(err, ErrTruncated) {
		t.Errorf("got %v want ErrTruncated", err)
	}
}

func TestEmptyPayload(t *testing.T) {
	if _, err := Pack(nil, Options{}); !errors.Is(err, ErrEmptyPayload) {
		t.Errorf("got %v want ErrEmptyPayload", err)
	}
}

func TestHostileParamsRejected(t *testing.T) {
	env, err := Pack(secret, Options{Passphrase: []byte("right")})
	if err != nil {
		t.Fatal(err)
	}
	// zero out the Argon2 time field (offset magic+ver+flags+cipherID)
	timeOff := len(magic) + 1 + 1 + 1
	for i := 0; i < 4; i++ {
		env[timeOff+i] = 0
	}
	if _, err := Unpack(env, []byte("right")); !errors.Is(err, ErrBadParams) {
		t.Errorf("got %v want ErrBadParams", err)
	}
}

func TestValidateAndIsEncrypted(t *testing.T) {
	plain, _ := Pack(secret, Options{})
	enc, _ := Pack(secret, Options{Passphrase: []byte("k")})

	if err := Validate(plain); err != nil {
		t.Errorf("Validate(plain): %v", err)
	}
	if err := Validate([]byte("garbage")); err == nil {
		t.Error("Validate(garbage) should fail")
	}
	if e, _ := IsEncrypted(plain); e {
		t.Error("plain reported encrypted")
	}
	if e, _ := IsEncrypted(enc); !e {
		t.Error("encrypted reported plain")
	}
}

func TestStrengthParams(t *testing.T) {
	if p := paramsForStrength(StrengthHigh); p.memory != argonHighMemory || p.time != argonHighTime {
		t.Errorf("high params wrong: %+v", p)
	}
	if p := paramsForStrength(StrengthDefault); p.memory != argonDefaultMemory {
		t.Errorf("default params wrong: %+v", p)
	}
}

func TestOverheadMatchesEnvelope(t *testing.T) {
	plain := []byte("a modest secret")

	clear, err := Pack(plain, Options{})
	if err != nil {
		t.Fatalf("pack plaintext: %v", err)
	}
	if got := len(clear) - len(plain); got != Overhead(false) {
		t.Errorf("plaintext overhead = %d, Overhead(false) = %d", got, Overhead(false))
	}

	sealed, err := Pack(plain, Options{Passphrase: []byte("pw"), Cipher: CipherChaCha20})
	if err != nil {
		t.Fatalf("pack encrypted: %v", err)
	}
	if got := len(sealed) - len(plain); got != Overhead(true) {
		t.Errorf("encrypted overhead = %d, Overhead(true) = %d", got, Overhead(true))
	}
}
