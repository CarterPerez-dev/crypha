/*
©AngelaMos | 2026
chrome.go

Header wordmark, wizard stepper, footer keybinds, and shared panel primitives
*/

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	wordmarkRows = 3
	wordmarkGap  = "  "
	labelWidth   = 12
	tagline      = "multi-carrier steganography"
)

var wordmarkGlyphs = [][wordmarkRows]string{
	{"█▀▀", "█  ", "▀▀▀"},
	{"█▀▄", "█▀▄", "▀ ▀"},
	{"█ █", "▀█▀", " ▀ "},
	{"█▀▄", "█▀▀", "█  "},
	{"█ █", "█▀█", "▀ ▀"},
	{"▄▀▄", "█▀█", "▀ ▀"},
}

type keyHint struct {
	key  string
	desc string
}

func wordmarkLines() []string {
	lines := make([]string, wordmarkRows)
	for row := range wordmarkRows {
		parts := make([]string, len(wordmarkGlyphs))
		for i, g := range wordmarkGlyphs {
			parts[i] = g[row]
		}
		lines[row] = strings.Join(parts, wordmarkGap)
	}
	return lines
}

func renderHeader(width int) string {
	mark := gradientBlock(wordmarkLines(), brandStops, true)
	tag := styleTagline.Render(tagline)
	rule := gradientRule(width, brandStops)
	return lipgloss.JoinVertical(lipgloss.Left, mark, tag, "", rule)
}

func renderStepper(labels []string, current int) string {
	parts := make([]string, 0, len(labels)*2)
	for i, label := range labels {
		var token string
		switch {
		case i < current:
			token = styleStepDone.Render(label)
		case i == current:
			token = styleCursor.Render(accentBar) + styleStepCurrent.Render(label)
		default:
			token = styleStepFuture.Render(label)
		}
		parts = append(parts, token)
		if i < len(labels)-1 {
			parts = append(parts, styleHelpSep.Render(" "+connector+" "))
		}
	}
	return strings.Join(parts, "")
}

func renderFooter(hints []keyHint) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, styleHelpKey.Render(h.key)+" "+styleHelpDesc.Render(h.desc))
	}
	return strings.Join(parts, styleHelpSep.Render(" │ "))
}

func sectionTitle(text string) string {
	return stylePanelTitle.Render(accentBar) + " " + styleValueBold.Render(text)
}

func kv(label, value string) string {
	return styleLabel.Width(labelWidth).Render(label) + styleValue.Render(value)
}

func kvStyled(label, value string, valueStyle lipgloss.Style) string {
	return styleLabel.Width(labelWidth).Render(label) + valueStyle.Render(value)
}
