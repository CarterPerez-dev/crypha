/*
©AngelaMos | 2026
theme.go

Spectral cybercore palette, HCL gradient engine, and lipgloss styles for the TUI
*/

package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	hexVioletLight = "#A78BFA"
	hexViolet      = "#8B5CF6"
	hexVioletDeep  = "#6D4AFF"

	hexBlue       = "#4457E8"
	hexBlueBright = "#6E86FF"
	hexBlueDim    = "#5A63A8"

	hexRed = "#CE2417"

	hexBright = "#E9E6F5"
	hexNormal = "#C4C0D6"
	hexDim    = "#8B86A6"
	hexFaint  = "#46425F"

	hexFrame = "#39346B"
	hexInk   = "#0C0A16"
)

const (
	blockFull  = "█"
	blockEmpty = "░"
	ruleGlyph  = "━"
	accentBar  = "▌"
	connector  = "─"
)

var (
	brandStops     = mustStops(hexVioletLight, hexViolet, hexVioletDeep, hexBlue)
	capacityStops  = mustStops(hexBlue, hexVioletLight, hexViolet, hexVioletDeep)
	unboundedStops = mustStops(hexBlue, hexViolet)
)

var (
	styleTagline = lipgloss.NewStyle().Foreground(lipgloss.Color(hexDim)).Italic(true)

	styleLabel     = lipgloss.NewStyle().Foreground(lipgloss.Color(hexBlueDim))
	styleValue     = lipgloss.NewStyle().Foreground(lipgloss.Color(hexBright))
	styleValueBold = lipgloss.NewStyle().Foreground(lipgloss.Color(hexBright)).Bold(true)

	stylePanelTitle = lipgloss.NewStyle().Foreground(lipgloss.Color(hexViolet)).Bold(true)
	styleCursor     = lipgloss.NewStyle().Foreground(lipgloss.Color(hexRed)).Bold(true)

	styleStepCurrent = lipgloss.NewStyle().Foreground(lipgloss.Color(hexBright)).Bold(true)
	styleStepDone    = lipgloss.NewStyle().Foreground(lipgloss.Color(hexBlue))
	styleStepFuture  = lipgloss.NewStyle().Foreground(lipgloss.Color(hexFaint))

	styleSelected   = lipgloss.NewStyle().Foreground(lipgloss.Color(hexBright)).Bold(true)
	styleUnselected = lipgloss.NewStyle().Foreground(lipgloss.Color(hexNormal))
	styleHint       = lipgloss.NewStyle().Foreground(lipgloss.Color(hexDim))

	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color(hexRed)).Bold(true)
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color(hexBlueBright)).Bold(true)
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color(hexRed))

	styleHelpKey  = lipgloss.NewStyle().Foreground(lipgloss.Color(hexBlue)).Bold(true)
	styleHelpDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(hexDim))
	styleHelpSep  = lipgloss.NewStyle().Foreground(lipgloss.Color(hexFaint))

	styleTrack = lipgloss.NewStyle().Foreground(lipgloss.Color(hexFaint))
)

const (
	framePadX        = 3
	framePadY        = 1
	frameBorder      = 1
	horizontalGutter = 2
)

func frameStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(hexFrame)).
		Padding(framePadY, framePadX).
		Width(width)
}

func badge(text, hex string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(hexInk)).
		Background(lipgloss.Color(hex)).
		Bold(true).
		Padding(0, 1).
		Render(text)
}

func mustStops(hexes ...string) []colorful.Color {
	stops := make([]colorful.Color, len(hexes))
	for i, h := range hexes {
		c, err := colorful.Hex(h)
		if err != nil {
			panic("tui: invalid palette hex " + h)
		}
		stops[i] = c
	}
	return stops
}

func ramp(stops []colorful.Color, n int) []lipgloss.Color {
	out := make([]lipgloss.Color, n)
	if n == 0 {
		return out
	}
	if n == 1 || len(stops) == 1 {
		for i := range out {
			out[i] = lipgloss.Color(stops[0].Hex())
		}
		return out
	}
	segments := len(stops) - 1
	for i := range out {
		x := float64(i) / float64(n-1) * float64(segments)
		idx := int(x)
		if idx >= segments {
			idx = segments - 1
		}
		local := x - float64(idx)
		out[i] = lipgloss.Color(stops[idx].BlendHcl(stops[idx+1], local).Clamped().Hex())
	}
	return out
}

func gradientText(text string, stops []colorful.Color, bold bool) string {
	runes := []rune(text)
	cols := ramp(stops, len(runes))
	var b strings.Builder
	for i, r := range runes {
		s := lipgloss.NewStyle().Foreground(cols[i])
		if bold {
			s = s.Bold(true)
		}
		b.WriteString(s.Render(string(r)))
	}
	return b.String()
}

func gradientBlock(lines []string, stops []colorful.Color, bold bool) string {
	width := 0
	for _, ln := range lines {
		if w := len([]rune(ln)); w > width {
			width = w
		}
	}
	cols := ramp(stops, width)
	out := make([]string, len(lines))
	for li, ln := range lines {
		var b strings.Builder
		for i, r := range []rune(ln) {
			s := lipgloss.NewStyle().Foreground(cols[i])
			if bold {
				s = s.Bold(true)
			}
			b.WriteString(s.Render(string(r)))
		}
		out[li] = b.String()
	}
	return strings.Join(out, "\n")
}

func gradientRule(width int, stops []colorful.Color) string {
	if width <= 0 {
		return ""
	}
	return gradientText(strings.Repeat(ruleGlyph, width), stops, false)
}

func spectralBar(width int, frac float64, stops []colorful.Color) string {
	if width <= 0 {
		return ""
	}
	over := frac > 1
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(math.Round(frac * float64(width)))
	if filled > width {
		filled = width
	}
	cols := ramp(capacityStops, width)
	if stops != nil {
		cols = ramp(stops, width)
	}
	var b strings.Builder
	for i := range width {
		if i < filled {
			col := cols[i]
			if over {
				col = lipgloss.Color(hexRed)
			}
			b.WriteString(lipgloss.NewStyle().Foreground(col).Render(blockFull))
		} else {
			b.WriteString(styleTrack.Render(blockEmpty))
		}
	}
	return b.String()
}
