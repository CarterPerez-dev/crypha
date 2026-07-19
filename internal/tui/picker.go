/*
©AngelaMos | 2026
picker.go

A compact keyboard-driven selection list with an inline blurb for the highlighted item
*/

package tui

import (
	"github.com/charmbracelet/lipgloss"
)

type pickItem struct {
	title string
	desc  string
	value string
}

type picker struct {
	items  []pickItem
	cursor int
}

func newPicker(items []pickItem) picker {
	return picker{items: items}
}

func (p *picker) up() {
	if len(p.items) == 0 {
		return
	}
	p.cursor = (p.cursor - 1 + len(p.items)) % len(p.items)
}

func (p *picker) down() {
	if len(p.items) == 0 {
		return
	}
	p.cursor = (p.cursor + 1) % len(p.items)
}

func (p picker) selected() pickItem {
	if len(p.items) == 0 {
		return pickItem{}
	}
	return p.items[p.cursor]
}

func (p picker) view(width int) string {
	rows := make([]string, 0, len(p.items))
	for i, it := range p.items {
		if i == p.cursor {
			rows = append(rows, styleCursor.Render(accentBar)+" "+styleSelected.Render(it.title))
		} else {
			rows = append(rows, "  "+styleUnselected.Render(it.title))
		}
	}
	list := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if sel := p.selected(); sel.desc != "" {
		blurb := styleHint.Width(width).Render(sel.desc)
		return lipgloss.JoinVertical(lipgloss.Left, list, "", blurb)
	}
	return list
}
