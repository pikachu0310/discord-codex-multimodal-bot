package chat

import (
	"strings"
	"sync"
)

type progressBuilder struct {
	model    string
	authorID string
	input    string
	steps    []string
	final    string
	done     bool
	verbose  bool
	mu       sync.Mutex
	OnUpdate func(progressSnapshot)
}

type progressSnapshot struct {
	AuthorID string
	Input   string
	Steps   []string
	Final   string
	Done    bool
	Verbose bool
}

const (
	progressInputLimit = 1500
	progressLogLimit   = 400
	messageChunkLimit  = 1900
)

func newProgress(model string) *progressBuilder {
	return &progressBuilder{
		model: model,
		steps: []string{},
	}
}

func (p *progressBuilder) WithInput(input string) *progressBuilder {
	p.input = strings.TrimSpace(input)
	return p
}

func (p *progressBuilder) WithAuthorID(authorID string) *progressBuilder {
	p.authorID = strings.TrimSpace(authorID)
	return p
}

func (p *progressBuilder) AddStep(step string) {
	step = strings.TrimSpace(step)
	if step == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, step)
}

func (p *progressBuilder) SetFinal(final string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.final = strings.TrimSpace(final)
	p.done = true
}

func (p *progressBuilder) SetVerbose(verbose bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.verbose = verbose
}

func (p *progressBuilder) Snapshot() progressSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	steps := append([]string(nil), p.steps...)
	return progressSnapshot{
		AuthorID: p.authorID,
		Input:   p.input,
		Steps:   steps,
		Final:   p.final,
		Done:    p.done,
		Verbose: p.verbose,
	}
}

func (p *progressBuilder) Render() string {
	snapshot := p.Snapshot()
	if snapshot.Done {
		return renderFinalCombined(snapshot)
	}
	return renderProgress(snapshot)
}

func runeLen(s string) int {
	return len([]rune(s))
}

func truncateWithEllipsis(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "..."
}

func renderProgress(snapshot progressSnapshot) string {
	logSection := buildLogSection(snapshot, progressLogLimit)
	inputSection := buildInputSection(snapshot.AuthorID, snapshot.Input, progressInputLimit)
	return joinSections(logSection, inputSection)
}

func renderInitial(snapshot progressSnapshot) string {
	return buildInputSection(snapshot.AuthorID, snapshot.Input, progressInputLimit)
}

func renderFinalCombined(snapshot progressSnapshot) string {
	inputSection := buildInputSection(snapshot.AuthorID, snapshot.Input, progressInputLimit)
	final := strings.TrimSpace(snapshot.Final)
	return joinBody(inputSection, final)
}

func buildLogSection(snapshot progressSnapshot, limit int) string {
	var lines []string
	for _, step := range snapshot.Steps {
		if strings.TrimSpace(step) == "" {
			continue
		}
		lines = append(lines, step)
	}
	if len(lines) == 0 {
		return ""
	}
	var logLine string
	if snapshot.Verbose {
		logLine = strings.Join(lines, "\n")
	} else {
		logLine = lines[len(lines)-1]
	}
	if limit > 0 && runeLen(logLine) > limit {
		logLine = truncateWithEllipsis(logLine, limit)
	}
	return logLine
}

func buildInputSection(authorID, input string, limit int) string {
	authorID = strings.TrimSpace(authorID)
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	if limit > 0 && runeLen(input) > limit {
		input = truncateWithEllipsis(input, limit)
	}
	mention := ""
	if authorID != "" {
		mention = "<@" + authorID + "> "
	}
	return mention + "「" + input + "」"
}

func joinSections(sections ...string) string {
	var cleaned []string
	for _, section := range sections {
		if strings.TrimSpace(section) == "" {
			continue
		}
		cleaned = append(cleaned, section)
	}
	return strings.Join(cleaned, "\n\n")
}

func joinBody(parts ...string) string {
	var cleaned []string
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "\n")
}
