/*
©AngelaMos | 2026
cli.go

Descriptive catalog metadata for each carrier, surfaced by the formats and help output
*/

package config

type FormatDetail struct {
	Blurb      string
	CoverInput string
	Output     string
	Notes      string
}

var FormatDetails = map[string]FormatDetail{
	"image": {
		Blurb:      "LSB of RGB pixel data",
		CoverInput: "PNG or 24-bit BMP",
		Output:     "PNG",
		Notes:      "alpha channel untouched; paletted and 16-bit covers are rejected",
	},
	"audio": {
		Blurb:      "LSB of 16-bit PCM samples",
		CoverInput: "16-bit PCM WAV or FLAC",
		Output:     "WAV",
		Notes:      "FLAC covers are decoded and re-emitted as WAV",
	},
	"qr": {
		Blurb:      "Reed-Solomon-correctable error injection",
		CoverInput: "UTF-8 text (the QR's visible content)",
		Output:     "PNG",
		Notes:      "capacity is tens of bytes, so an encrypted envelope will not fit",
	},
	"text": {
		Blurb:      "zero-width U+200B and U+2060 characters",
		CoverInput: "any UTF-8 text",
		Output:     "text",
		Notes:      "the payload is appended to the cover as invisible characters",
	},
	"pdf": {
		Blurb:      "embedded attachment, metadata, or append-after-EOF",
		CoverInput: "PDF",
		Output:     "PDF",
		Notes:      "default technique is a lossless embedded-file attachment",
	},
}
