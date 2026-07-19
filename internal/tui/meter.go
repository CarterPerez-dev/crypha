/*
©AngelaMos | 2026
meter.go

The spectral capacity meter: payload-vs-carrier fill for hide, comparative table for capacity
*/

package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	fullPercent      = 100
	highWaterPercent = 80
	formatColumn     = 7
	unboundedTag     = "unbounded"
)

type capRow struct {
	format       string
	applicable   bool
	unbounded    bool
	capacity     int
	maxPlaintext int
	maxEncrypted int
	note         string
}

func renderHideMeter(payloadLen, envelope, capacity int, capErr error, barWidth int) string {
	if capErr != nil {
		return styleWarn.Render(reasonText(capErr))
	}
	if capacity >= math.MaxInt32 {
		bar := gradientText(strings.Repeat(blockFull, barWidth), unboundedStops, false)
		return lipgloss.JoinVertical(lipgloss.Left,
			bar+"  "+styleSuccess.Render(unboundedTag),
			"",
			kv("payload", fmt.Sprintf("%d B", payloadLen)),
			kv("envelope", fmt.Sprintf("%d B", envelope)),
			styleSuccess.Render("this carrier has effectively unbounded room; your payload always fits"),
		)
	}
	if capacity <= 0 {
		return styleWarn.Render("this cover is too small to hold a payload")
	}

	frac := float64(envelope) / float64(capacity)
	over := envelope > capacity
	pct := int(math.Round(math.Min(frac, 1) * fullPercent))

	bar := spectralBar(barWidth, frac, capacityStops)
	pctStyle := styleSuccess
	if over {
		pctStyle = styleError
	} else if pct >= highWaterPercent {
		pctStyle = styleWarn
	}

	lines := []string{
		bar + "  " + pctStyle.Render(fmt.Sprintf("%d%%", pct)),
		"",
		kv("payload", fmt.Sprintf("%d B", payloadLen)),
		kv("envelope", fmt.Sprintf("%d B", envelope)),
		kv("capacity", fmt.Sprintf("%d B", capacity)),
	}
	if over {
		lines = append(lines, "", styleError.Render(
			fmt.Sprintf("payload exceeds capacity by %d B: shorten it, enable compression, or pick a roomier carrier", envelope-capacity)))
	} else {
		lines = append(lines, "", styleSuccess.Render(
			fmt.Sprintf("fits with %d B of headroom", capacity-envelope)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func renderCapacityTable(rows []capRow, barWidth int) string {
	maxCap := 0
	for _, r := range rows {
		if r.applicable && !r.unbounded && r.capacity > maxCap {
			maxCap = r.capacity
		}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		name := styleValueBold.Width(formatColumn).Render(r.format)
		switch {
		case !r.applicable:
			out = append(out, name+styleHint.Render(r.note))
		case r.unbounded:
			bar := gradientText(strings.Repeat(blockFull, barWidth), unboundedStops, false)
			out = append(out, name+bar+"  "+styleHint.Render(unboundedTag))
		default:
			frac := 0.0
			if maxCap > 0 {
				frac = float64(r.capacity) / float64(maxCap)
			}
			bar := spectralBar(barWidth, frac, capacityStops)
			out = append(out, name+bar+"  "+capacityFacts(r))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

func capacityFacts(r capRow) string {
	plain := styleValue.Render(fmt.Sprintf("%d B", r.maxPlaintext)) + styleLabel.Render(" plain")
	var enc string
	if r.maxEncrypted <= 0 {
		enc = styleWarn.Render("encrypted does not fit")
	} else {
		enc = styleValue.Render(fmt.Sprintf("%d B", r.maxEncrypted)) + styleLabel.Render(" encrypted")
	}
	return plain + styleHelpSep.Render("  ·  ") + enc
}

func reasonText(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	return msg
}

func clampZero(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
