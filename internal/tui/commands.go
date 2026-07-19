/*
©AngelaMos | 2026
commands.go

Bubbletea commands and messages for engine calls, file IO, and the embed animation
*/

package tui

import (
	"bytes"
	"os"
	"time"

	"github.com/CarterPerez-dev/crypha/internal/engine"
	tea "github.com/charmbracelet/bubbletea"
)

type fileLoadedMsg struct {
	origin stage
	path   string
	data   []byte
	err    error
}

type hideDoneMsg struct {
	res  engine.HideResult
	data []byte
	err  error
}

type revealDoneMsg struct {
	res engine.RevealResult
	err error
}

type savedMsg struct {
	path string
	err  error
}

type tickMsg time.Time

func loadFileCmd(origin stage, path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		return fileLoadedMsg{origin: origin, path: path, data: data, err: err}
	}
}

func hideCmd(req engine.HideRequest, outPath string) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		req.Out = &buf
		res, err := engine.Hide(req)
		if err != nil {
			return hideDoneMsg{err: err}
		}
		if err := os.WriteFile(outPath, buf.Bytes(), outFilePerm); err != nil {
			return hideDoneMsg{err: err}
		}
		return hideDoneMsg{res: res, data: buf.Bytes()}
	}
}

func revealCmd(format string, stego, pass []byte) tea.Cmd {
	return func() tea.Msg {
		res, err := engine.Reveal(engine.RevealRequest{Format: format, Stego: stego, Passphrase: pass})
		return revealDoneMsg{res: res, err: err}
	}
}

func saveCmd(path string, data []byte) tea.Cmd {
	return func() tea.Msg {
		err := os.WriteFile(path, data, outFilePerm)
		return savedMsg{path: path, err: err}
	}
}

func tick() tea.Cmd {
	return tea.Tick(animInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
