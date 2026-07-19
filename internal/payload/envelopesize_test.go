/*
©AngelaMos | 2026
envelopesize_test.go

Known-answer tests proving EnvelopeSize equals the real packed envelope length
*/

package payload

import (
	"bytes"
	"testing"
)

func TestEnvelopeSizeMatchesPack(t *testing.T) {
	data := bytes.Repeat([]byte("crypha steganography "), 8)
	cases := []Options{
		{},
		{Compress: true},
		{Passphrase: []byte("pw"), Cipher: CipherChaCha20, Strength: StrengthDefault},
		{Passphrase: []byte("pw"), Cipher: CipherAES256GCM, Strength: StrengthDefault, Compress: true},
	}
	for i, opts := range cases {
		env, err := Pack(data, opts)
		if err != nil {
			t.Fatalf("case %d Pack: %v", i, err)
		}
		size, err := EnvelopeSize(data, opts)
		if err != nil {
			t.Fatalf("case %d EnvelopeSize: %v", i, err)
		}
		if size != len(env) {
			t.Fatalf("case %d: EnvelopeSize=%d, len(Pack)=%d", i, size, len(env))
		}
	}
}

func TestEnvelopeSizeOverhead(t *testing.T) {
	data := []byte("hello")
	plain, _ := EnvelopeSize(data, Options{})
	if plain != len(data)+Overhead(false) {
		t.Fatalf("plaintext size = %d, want %d", plain, len(data)+Overhead(false))
	}
	enc, _ := EnvelopeSize(data, Options{Passphrase: []byte("x")})
	if enc != len(data)+Overhead(true) {
		t.Fatalf("encrypted size = %d, want %d", enc, len(data)+Overhead(true))
	}
}

func TestEnvelopeSizeCompressionShrinks(t *testing.T) {
	data := bytes.Repeat([]byte("A"), 1024)
	plain, _ := EnvelopeSize(data, Options{})
	comp, _ := EnvelopeSize(data, Options{Compress: true})
	if comp >= plain {
		t.Fatalf("compression did not shrink the envelope: comp=%d plain=%d", comp, plain)
	}
}
