/*
©AngelaMos | 2026
config.go

Central constants for crypha so no magic numbers or strings live elsewhere
*/

package config

const (
	BinaryName       = "crypha"
	Version          = "0.1.0"
	ShortDescription = "Multi-format steganography for images, audio, QR, text, and PDFs"
	LongDescription  = "crypha hides an encrypted payload inside five carrier types " +
		"(image, audio, QR, zero-width text, PDF) and extracts it back, " +
		"from a scriptable CLI or an interactive TUI."
)
