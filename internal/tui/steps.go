/*
©AngelaMos | 2026
steps.go

Per-operation wizard step labels, current-step mapping, and contextual footer hints
*/

package tui

func (m Model) stepLabels() []string {
	switch m.op {
	case opReveal:
		return []string{"operation", "carrier", "stego", "reveal"}
	case opCapacity:
		return []string{"operation", "cover", "report"}
	default:
		labels := []string{"operation", "carrier"}
		if hasTechniques(m.format) {
			labels = append(labels, "technique")
		}
		return append(labels, "cover", "payload", "secure", "save", "embed")
	}
}

func (m Model) stepStages() []stage {
	switch m.op {
	case opReveal:
		return []stage{stageOperation, stageFormat, stageCover, stageRunning}
	case opCapacity:
		return []stage{stageOperation, stageCover, stageReview}
	default:
		stages := []stage{stageOperation, stageFormat}
		if hasTechniques(m.format) {
			stages = append(stages, stageTechnique)
		}
		return append(stages, stageCover, stagePayload, stageSecure, stageSave, stageReview)
	}
}

func (m Model) stepIndex() int {
	target := m.stage
	switch m.stage {
	case stagePassphrase:
		target = stageRunning
	case stageRunning, stageResult:
		if m.op == opReveal {
			target = stageRunning
		} else {
			target = stageReview
		}
	}
	for i, s := range m.stepStages() {
		if s == target {
			return i
		}
	}
	return 0
}

func (m Model) footerHints() []keyHint {
	switch m.stage {
	case stageOperation:
		return []keyHint{{"up/down", "move"}, {"enter", "select"}, {"q", "quit"}}
	case stageFormat, stageTechnique:
		return []keyHint{{"up/down", "move"}, {"enter", "select"}, {"esc", "back"}, {"q", "quit"}}
	case stageCover:
		return []keyHint{{"up/down", "browse"}, {"enter", "open or select"}, {"esc", "back"}}
	case stagePayload:
		if m.payloadMode == payloadFile {
			return []keyHint{{"up/down", "browse"}, {"enter", "choose"}, {"tab", "message"}, {"esc", "back"}}
		}
		return []keyHint{{"type", "message"}, {"enter", "continue"}, {"tab", "file"}, {"esc", "back"}}
	case stageSecure:
		return []keyHint{{"up/down", "field"}, {"space", "change"}, {"enter", "continue"}, {"esc", "back"}}
	case stageSave:
		return []keyHint{{"type", "path"}, {"enter", "continue"}, {"esc", "back"}}
	case stageReview:
		if m.op == opCapacity {
			return []keyHint{{"esc", "back"}, {"n", "new run"}, {"q", "quit"}}
		}
		return []keyHint{{"enter", "embed"}, {"esc", "back"}, {"q", "quit"}}
	case stagePassphrase:
		return []keyHint{{"enter", "unlock"}, {"esc", "cancel"}}
	case stageResult:
		if m.saving {
			return []keyHint{{"enter", "write"}, {"esc", "cancel"}}
		}
		if m.canSaveReveal() {
			return []keyHint{{"s", "save"}, {"n", "new run"}, {"q", "quit"}}
		}
		return []keyHint{{"n", "new run"}, {"q", "quit"}}
	case stageRunning:
		return []keyHint{{"working", "..."}}
	}
	return nil
}
