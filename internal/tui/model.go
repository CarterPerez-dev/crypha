/*
©AngelaMos | 2026
model.go

The wizard model: state, flow definition, stage navigation, and engine-facing helpers
*/

package tui

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CarterPerez-dev/crypha/internal/config"
	"github.com/CarterPerez-dev/crypha/internal/engine"
	"github.com/CarterPerez-dev/crypha/internal/payload"
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	appMaxWidth      = 88
	appMinWidth      = 46
	meterBarWidth    = 44
	capBarWidth      = 24
	embedBarWidth    = 44
	filepickerHeight = 12
	messageWidth     = 58
	passFieldWidth   = 34
	outFieldWidth    = 50
	passLimit        = 256
	pathLimit        = 512
	maxRevealPrompts = 3
	animStep         = 0.06
	animInterval     = 45 * time.Millisecond
	outFilePerm      = 0o600
	inlineLabel      = "inline message"
)

var (
	errNeedPayload    = errors.New("enter a message, or press tab to choose a payload file")
	errEmptyFile      = errors.New("that file is empty; choose a file with contents")
	errNeedPassphrase = errors.New("encryption is on: enter a passphrase or turn encryption off")
	errNeedOutput     = errors.New("enter an output path")
	errPayloadTooBig  = errors.New("payload does not fit: go back and shorten it, enable compression, or pick a roomier carrier")
)

type stage int

const (
	stageOperation stage = iota
	stageFormat
	stageTechnique
	stageCover
	stagePayload
	stageSecure
	stageSave
	stageReview
	stagePassphrase
	stageRunning
	stageResult
)

type operation int

const (
	opHide operation = iota
	opReveal
	opCapacity
)

type payloadMode int

const (
	payloadMessage payloadMode = iota
	payloadFile
)

type Model struct {
	width    int
	height   int
	ready    bool
	quitting bool

	stage stage
	op    operation

	opPick   picker
	fmtPick  picker
	techPick picker

	files   filepicker.Model
	message textinput.Model
	secure  secureForm
	outPath textinput.Model
	revPass textinput.Model

	format       string
	technique    string
	coverPath    string
	coverBytes   []byte
	payloadBytes []byte
	payloadMode  payloadMode
	payloadLabel string
	pass         []byte

	capValue     int
	capErr       error
	capRows      []capRow
	envelopeSize int
	envelopeErr  error

	hideRes   engine.HideResult
	revealRes engine.RevealResult
	stego     []byte
	outputAt  string
	savedAt   string
	saving    bool

	passPrompts int
	engineDone  bool
	engineErr   error
	animFrac    float64

	err error
}

func New() Model {
	fp := filepicker.New()
	if wd, err := os.Getwd(); err == nil {
		fp.CurrentDirectory = wd
	}
	fp.ShowPermissions = false
	fp.AutoHeight = false
	fp.SetHeight(filepickerHeight)

	msg := textinput.New()
	msg.Prompt = ""
	msg.Placeholder = "type a secret message"
	msg.Width = messageWidth

	out := textinput.New()
	out.Prompt = ""
	out.CharLimit = pathLimit
	out.Width = outFieldWidth

	rev := textinput.New()
	rev.Prompt = ""
	rev.EchoMode = textinput.EchoPassword
	rev.CharLimit = passLimit
	rev.Width = passFieldWidth

	return Model{
		files:   fp,
		message: msg,
		outPath: out,
		revPass: rev,
		secure:  newSecureForm(),
		opPick:  newPicker(operationItems()),
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) flow() []stage {
	switch m.op {
	case opReveal:
		return []stage{stageOperation, stageFormat, stageCover, stageRunning, stageResult}
	case opCapacity:
		return []stage{stageOperation, stageCover, stageReview}
	default:
		steps := []stage{stageOperation, stageFormat}
		if hasTechniques(m.format) {
			steps = append(steps, stageTechnique)
		}
		return append(steps, stageCover, stagePayload, stageSecure, stageSave, stageReview, stageRunning, stageResult)
	}
}

func (m Model) adjacentStage(dir int) (stage, bool) {
	f := m.flow()
	for i, s := range f {
		if s != m.stage {
			continue
		}
		j := i + dir
		if j < 0 || j >= len(f) {
			return m.stage, false
		}
		return f[j], true
	}
	return m.stage, false
}

func (m Model) advance() (tea.Model, tea.Cmd) {
	next, ok := m.adjacentStage(1)
	if !ok {
		return m, nil
	}
	m.err = nil
	cmd := m.enter(next)
	return m, cmd
}

func (m Model) back() (tea.Model, tea.Cmd) {
	prev, ok := m.adjacentStage(-1)
	if !ok {
		return m, nil
	}
	m.err = nil
	cmd := m.enter(prev)
	return m, cmd
}

func (m *Model) enter(s stage) tea.Cmd {
	m.stage = s
	switch s {
	case stageFormat:
		m.fmtPick = newPicker(formatItems(m.op == opReveal))
	case stageTechnique:
		m.techPick = newPicker(techniqueItems(m.format))
	case stageCover:
		return m.files.Init()
	case stagePayload:
		m.payloadMode = payloadMessage
		return m.message.Focus()
	case stageSave:
		if strings.TrimSpace(m.outPath.Value()) == "" {
			m.outPath.SetValue(suggestOutput(m.coverPath, m.format))
		}
		return m.outPath.Focus()
	case stageReview:
		m.prepareReview()
	case stageRunning:
		return m.startRun()
	}
	return nil
}

func (m *Model) prepareReview() {
	switch m.op {
	case opHide:
		m.capValue, m.capErr = engine.Capacity(m.format, bytes.NewReader(m.coverBytes))
		m.envelopeSize, m.envelopeErr = engine.EnvelopeSize(m.payloadBytes, m.buildOptions())
	case opCapacity:
		m.capRows = buildCapRows(engine.CapacityAll(m.coverBytes))
	}
}

func (m *Model) startRun() tea.Cmd {
	m.animFrac = 0
	m.engineDone = false
	m.engineErr = nil
	switch m.op {
	case opReveal:
		return tea.Batch(revealCmd(m.format, m.coverBytes, m.revPassValue()), tick())
	default:
		req := engine.HideRequest{
			Format:    m.format,
			Technique: m.technique,
			Cover:     bytes.NewReader(m.coverBytes),
			Payload:   m.payloadBytes,
			Options:   m.buildOptions(),
		}
		return tea.Batch(hideCmd(req, m.outPath.Value()), tick())
	}
}

func (m Model) reset() (tea.Model, tea.Cmd) {
	next := New()
	next.width = m.width
	next.height = m.height
	next.ready = m.ready
	return next, textinput.Blink
}

func (m Model) buildOptions() payload.Options {
	return payload.Options{
		Passphrase: m.secure.passphrase(),
		Compress:   m.secure.compress,
		Cipher:     payload.Cipher(m.secure.cipherValue()),
		Strength:   payload.Strength(m.secure.strengthValue()),
	}
}

func (m Model) revPassValue() []byte {
	v := m.revPass.Value()
	if v == "" {
		return nil
	}
	return []byte(v)
}

func (m Model) encrypted() bool {
	return len(m.pass) > 0
}

func (m Model) hideFits() bool {
	if m.capErr != nil || m.envelopeErr != nil {
		return false
	}
	if m.capValue >= math.MaxInt32 {
		return true
	}
	return m.envelopeSize <= m.capValue
}

func hasTechniques(format string) bool {
	return len(engine.Techniques(format)) > 0
}

func suggestOutput(coverPath, format string) string {
	base := strings.TrimSuffix(filepath.Base(coverPath), filepath.Ext(coverPath))
	if base == "" || base == "." {
		base = config.BinaryName
	}
	return base + ".stego" + outputExt(format)
}

func (m Model) suggestReveal() string {
	base := strings.TrimSuffix(filepath.Base(m.coverPath), filepath.Ext(m.coverPath))
	if base == "" || base == "." {
		base = config.BinaryName
	}
	return base + ".revealed"
}

func outputExt(format string) string {
	switch strings.ToLower(config.FormatDetails[format].Output) {
	case "png":
		return ".png"
	case "wav":
		return ".wav"
	case "pdf":
		return ".pdf"
	case "text":
		return ".txt"
	default:
		return ".out"
	}
}

func buildCapRows(rows []engine.CapacityRow) []capRow {
	out := make([]capRow, 0, len(rows))
	for _, r := range rows {
		cr := capRow{format: r.Format}
		switch {
		case r.Err != nil:
			cr.note = reasonText(r.Err)
		case r.Capacity >= math.MaxInt32:
			cr.applicable = true
			cr.unbounded = true
		default:
			cr.applicable = true
			cr.capacity = r.Capacity
			cr.maxPlaintext = clampZero(r.Capacity - engine.Overhead(false))
			cr.maxEncrypted = clampZero(r.Capacity - engine.Overhead(true))
		}
		out = append(out, cr)
	}
	return out
}

func operationItems() []pickItem {
	return []pickItem{
		{title: "hide", desc: "embed an encrypted payload inside a cover file"},
		{title: "reveal", desc: "extract and decrypt a payload hidden in a stego file"},
		{title: "capacity", desc: "measure how many payload bytes each carrier can hold for a cover"},
	}
}

func formatItems(withAuto bool) []pickItem {
	items := make([]pickItem, 0)
	if withAuto {
		items = append(items, pickItem{title: "auto-detect", value: "", desc: "let crypha identify the carrier by inspecting the file"})
	}
	for _, fi := range engine.Catalog() {
		d := config.FormatDetails[fi.Name]
		desc := d.Blurb + "  ·  cover: " + d.CoverInput + "  ·  output: " + d.Output
		items = append(items, pickItem{title: fi.Name, value: fi.Name, desc: desc})
	}
	return items
}

func techniqueItems(format string) []pickItem {
	blurbs := map[string]string{
		"attachment": "embed the payload as a lossless PDF file attachment (default)",
		"metadata":   "stash the payload inside the PDF Info dictionary",
		"append":     "append the payload after the PDF end-of-file marker",
	}
	items := make([]pickItem, 0)
	for _, t := range engine.Techniques(format) {
		items = append(items, pickItem{title: t, value: t, desc: blurbs[t]})
	}
	return items
}
