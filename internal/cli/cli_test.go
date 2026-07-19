/*
©AngelaMos | 2026
cli_test.go

End-to-end tests that drive the crypha commands through the cobra tree
*/

package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CarterPerez-dev/crypha/internal/payload"
	goaudio "github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

const secret = "meet at dawn"

func run(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	root := newRootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errb.String(), err
}

func makeTextCover(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "cover.txt")
	if err := os.WriteFile(path, []byte("the quick brown fox jumps over the lazy dog"), 0o600); err != nil {
		t.Fatalf("write text cover: %v", err)
	}
	return path
}

func makeQRCover(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "cover.qrtext")
	if err := os.WriteFile(path, []byte("crypha qr cover"), 0o600); err != nil {
		t.Fatalf("write qr cover: %v", err)
	}
	return path
}

func makePNGCover(t *testing.T, dir string) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 96, 96))
	for y := 0; y < 96; y++ {
		for x := 0; x < 96; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: uint8(x ^ y), A: 255})
		}
	}
	path := filepath.Join(dir, "cover.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return path
}

func makeWAVCover(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "cover.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create wav: %v", err)
	}
	defer func() { _ = f.Close() }()
	enc := wav.NewEncoder(f, 44100, 16, 1, 1)
	data := make([]int, 16000)
	x := uint32(1)
	for i := range data {
		x = x*1664525 + 1013904223
		data[i] = int(int16(x >> 16))
	}
	buf := &goaudio.IntBuffer{
		Format:         &goaudio.Format{NumChannels: 1, SampleRate: 44100},
		Data:           data,
		SourceBitDepth: 16,
	}
	if err := enc.Write(buf); err != nil {
		t.Fatalf("write wav: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("close wav: %v", err)
	}
	return path
}

func makePDFCover(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "cover.pdf")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create pdf: %v", err)
	}
	defer func() { _ = f.Close() }()
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	if err := api.Create(nil, strings.NewReader(`{"pages":{"1":{"content":{}}}}`), f, conf); err != nil {
		t.Fatalf("create pdf content: %v", err)
	}
	return path
}

func TestHideRevealPerFormat(t *testing.T) {
	dir := t.TempDir()
	covers := map[string]string{
		"text":  makeTextCover(t, dir),
		"image": makePNGCover(t, dir),
		"audio": makeWAVCover(t, dir),
		"pdf":   makePDFCover(t, dir),
		"qr":    makeQRCover(t, dir),
	}
	for format, cover := range covers {
		t.Run(format, func(t *testing.T) {
			stego := filepath.Join(dir, format+".stego")
			if _, _, err := run(t, "", "hide", "--format", format, "-i", cover, "-o", stego, "-m", secret); err != nil {
				t.Fatalf("hide: %v", err)
			}
			forced, _, err := run(t, "", "reveal", "--format", format, "-i", stego)
			if err != nil {
				t.Fatalf("reveal --format: %v", err)
			}
			if forced != secret {
				t.Fatalf("reveal --format = %q, want %q", forced, secret)
			}
			auto, _, err := run(t, "", "reveal", "-i", stego)
			if err != nil {
				t.Fatalf("reveal auto-detect: %v", err)
			}
			if auto != secret {
				t.Fatalf("reveal auto-detect = %q, want %q", auto, secret)
			}
		})
	}
}

func TestPDFTechniques(t *testing.T) {
	dir := t.TempDir()
	cover := makePDFCover(t, dir)
	for _, tech := range []string{"attachment", "metadata", "append"} {
		t.Run(tech, func(t *testing.T) {
			stego := filepath.Join(dir, tech+".pdf")
			if _, _, err := run(t, "", "hide", "--format", "pdf", "--technique", tech, "-i", cover, "-o", stego, "-m", secret); err != nil {
				t.Fatalf("hide %s: %v", tech, err)
			}
			out, _, err := run(t, "", "reveal", "--format", "pdf", "-i", stego)
			if err != nil {
				t.Fatalf("reveal %s: %v", tech, err)
			}
			if out != secret {
				t.Fatalf("technique %s = %q, want %q", tech, out, secret)
			}
		})
	}
}

func TestRevealPositionalArg(t *testing.T) {
	dir := t.TempDir()
	cover := makeTextCover(t, dir)
	stego := filepath.Join(dir, "positional.txt")
	if _, _, err := run(t, "", "hide", "--format", "text", "-i", cover, "-o", stego, "-m", secret); err != nil {
		t.Fatalf("hide: %v", err)
	}
	out, _, err := run(t, "", "reveal", stego)
	if err != nil {
		t.Fatalf("reveal positional: %v", err)
	}
	if out != secret {
		t.Fatalf("reveal positional = %q, want %q", out, secret)
	}
}

func TestPayloadFromFileAndStdin(t *testing.T) {
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	payloadFile := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(payloadFile, []byte(secret), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	fileStego := filepath.Join(dir, "fromfile.png")
	if _, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", fileStego, "-f", payloadFile); err != nil {
		t.Fatalf("hide -f: %v", err)
	}
	out, _, err := run(t, "", "reveal", "-i", fileStego)
	if err != nil || out != secret {
		t.Fatalf("reveal from -f = %q err %v", out, err)
	}

	stdinStego := filepath.Join(dir, "fromstdin.png")
	if _, _, err := run(t, secret, "hide", "--format", "image", "-i", cover, "-o", stdinStego, "-f", stdioPath); err != nil {
		t.Fatalf("hide -f -: %v", err)
	}
	out, _, err = run(t, "", "reveal", "-i", stdinStego)
	if err != nil || out != secret {
		t.Fatalf("reveal from stdin = %q err %v", out, err)
	}
}

func TestEncryptedRoundTripCLI(t *testing.T) {
	t.Setenv(envPassphrase, "")
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	stego := filepath.Join(dir, "enc.png")
	if _, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", stego, "-m", secret, "--encrypt", "-k", "hunter2"); err != nil {
		t.Fatalf("hide encrypted: %v", err)
	}

	out, _, err := run(t, "", "reveal", "-i", stego, "-k", "hunter2")
	if err != nil || out != secret {
		t.Fatalf("reveal with key = %q err %v", out, err)
	}

	if _, _, err := run(t, "", "reveal", "-i", stego); !errors.Is(err, payload.ErrPassphraseRequired) {
		t.Fatalf("no-key reveal err = %v, want ErrPassphraseRequired", err)
	}
	if _, _, err := run(t, "", "reveal", "-i", stego, "-k", "wrong"); !errors.Is(err, payload.ErrDecrypt) {
		t.Fatalf("wrong-key reveal err = %v, want ErrDecrypt", err)
	}
}

func TestEnvPassphrase(t *testing.T) {
	t.Setenv(envPassphrase, "from-the-env")
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	stego := filepath.Join(dir, "env.png")
	if _, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", stego, "-m", secret, "--encrypt"); err != nil {
		t.Fatalf("hide with env: %v", err)
	}
	out, _, err := run(t, "", "reveal", "-i", stego)
	if err != nil || out != secret {
		t.Fatalf("reveal with env = %q err %v", out, err)
	}
}

func TestAES256GCMAndCompress(t *testing.T) {
	t.Setenv(envPassphrase, "")
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	stego := filepath.Join(dir, "aes.png")
	if _, _, err := run(t, "", "hide", "--format", "image", "-i", cover, "-o", stego, "-m", secret,
		"-k", "hunter2", "--cipher", "aes256gcm", "--compress", "--strength", "high"); err != nil {
		t.Fatalf("hide aes: %v", err)
	}
	out, _, err := run(t, "", "reveal", "-i", stego, "-k", "hunter2")
	if err != nil || out != secret {
		t.Fatalf("reveal aes = %q err %v", out, err)
	}
}

func TestFormatsCommand(t *testing.T) {
	out, _, err := run(t, "", "formats")
	if err != nil {
		t.Fatalf("formats: %v", err)
	}
	for _, f := range []string{"image", "audio", "qr", "text", "pdf"} {
		if !strings.Contains(out, f) {
			t.Errorf("formats missing %q", f)
		}
	}
	jsonOut, _, err := run(t, "", "formats", "--json")
	if err != nil {
		t.Fatalf("formats --json: %v", err)
	}
	if !json.Valid([]byte(jsonOut)) {
		t.Errorf("formats --json is not valid json: %s", jsonOut)
	}
}

func TestVersionCommand(t *testing.T) {
	out, _, err := run(t, "", "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "crypha") || !strings.Contains(out, "0.1.0") {
		t.Errorf("version = %q", out)
	}
}

func TestCapacityCommand(t *testing.T) {
	dir := t.TempDir()
	cover := makePNGCover(t, dir)

	all, _, err := run(t, "", "capacity", "-i", cover)
	if err != nil {
		t.Fatalf("capacity: %v", err)
	}
	if !strings.Contains(all, "image") {
		t.Errorf("capacity table missing image row:\n%s", all)
	}

	one, _, err := run(t, "", "capacity", "-i", cover, "--format", "image")
	if err != nil {
		t.Fatalf("capacity --format: %v", err)
	}
	if !strings.Contains(one, "image") {
		t.Errorf("single capacity missing image:\n%s", one)
	}

	jsonOut, _, err := run(t, "", "capacity", "-i", cover, "--json")
	if err != nil {
		t.Fatalf("capacity --json: %v", err)
	}
	if !json.Valid([]byte(jsonOut)) {
		t.Errorf("capacity --json invalid: %s", jsonOut)
	}
}

func TestErrorPaths(t *testing.T) {
	dir := t.TempDir()
	cover := makePNGCover(t, dir)
	out := filepath.Join(dir, "out.png")

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown format", []string{"hide", "--format", "bogus", "-i", cover, "-o", out, "-m", "x"}, "unknown carrier format"},
		{"unknown cipher", []string{"hide", "--format", "image", "-i", cover, "-o", out, "-m", "x", "-k", "p", "--cipher", "des"}, "unknown cipher"},
		{"both sources", []string{"hide", "--format", "image", "-i", cover, "-o", out, "-m", "x", "-f", cover}, "only one of"},
		{"technique on non-pdf", []string{"hide", "--format", "image", "-i", cover, "-o", out, "-m", "x", "--technique", "append"}, "technique only applies"},
		{"no payload source", []string{"hide", "--format", "image", "-i", cover, "-o", out}, "provide a payload"},
		{"reveal no stego", []string{"reveal"}, "provide a stego file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := run(t, "", tc.args...)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %q, want it to contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestHideMissingRequiredFlags(t *testing.T) {
	if _, _, err := run(t, "", "hide", "--format", "image", "-m", "x"); err == nil {
		t.Fatal("expected required-flag error for missing -i/-o")
	}
}

func TestRevealJSONToStdout(t *testing.T) {
	dir := t.TempDir()
	cover := makeTextCover(t, dir)
	stego := filepath.Join(dir, "json.txt")
	if _, _, err := run(t, "", "hide", "--format", "text", "-i", cover, "-o", stego, "-m", secret); err != nil {
		t.Fatalf("hide: %v", err)
	}

	stdout, _, err := run(t, "", "reveal", "-i", stego, "--json")
	if err != nil {
		t.Fatalf("reveal --json: %v", err)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("reveal --json stdout is not valid json: %s", stdout)
	}
	var obj struct {
		Format string `json:"format"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj.Format != "text" {
		t.Errorf("format = %q, want text", obj.Format)
	}
	decoded, err := base64.StdEncoding.DecodeString(obj.Data)
	if err != nil || string(decoded) != secret {
		t.Fatalf("decoded data = %q err %v, want %q", decoded, err, secret)
	}
}

func TestRevealAmbiguousPath(t *testing.T) {
	dir := t.TempDir()
	cover := makeTextCover(t, dir)
	stego := filepath.Join(dir, "amb.txt")
	if _, _, err := run(t, "", "hide", "--format", "text", "-i", cover, "-o", stego, "-m", secret); err != nil {
		t.Fatalf("hide: %v", err)
	}
	if _, _, err := run(t, "", "reveal", "-i", stego, stego); !errors.Is(err, errAmbiguousPath) {
		t.Fatalf("err = %v, want errAmbiguousPath", err)
	}
}
