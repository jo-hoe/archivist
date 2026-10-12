package cli

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jo-hoe/archivist/internal/config"
	"github.com/jo-hoe/archivist/internal/fileops"
	"github.com/jo-hoe/archivist/internal/pipeline"
)

func newTUICmd(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Sort and tag files in an interactive terminal UI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := configFromViper(v)
			if err := validateOrUsage(c); err != nil {
				return err
			}
			built, err := config.Build(cmd.Context(), c)
			if err != nil {
				return err
			}
			return runTUI(cmd.Context(), built, c)
		},
	}
}

// runTUI drives the Bubble Tea program: a preview phase (confirm/quit), then a
// progress phase that streams per-file results, then a summary.
func runTUI(ctx context.Context, built *config.Built, c config.Config) error {
	m := newTUIModel(ctx, built, c)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return err
	}
	// Surface a run-level error (e.g. cancelled context) after the UI exits.
	if fm, ok := final.(tuiModel); ok && fm.runErr != nil {
		return fm.runErr
	}
	return nil
}

// phase tracks which screen the TUI is showing.
type phase int

const (
	phasePreview phase = iota
	phaseRunning
	phaseDone
)

// resultMsg carries one processed file back to the Update loop.
type resultMsg struct {
	index int
	res   pipeline.Result
}

// doneMsg signals the pipeline finished.
type doneMsg struct {
	results []pipeline.Result
	err     error
}

type tuiModel struct {
	ctx     context.Context
	built   *config.Built
	cfg     config.Config
	entries []fileops.Entry

	phase    phase
	progress progress.Model
	results  []pipeline.Result
	runErr   error
	stream   chan resultMsg
	done     chan doneMsg
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func newTUIModel(ctx context.Context, built *config.Built, c config.Config) tuiModel {
	start := phasePreview
	if c.NoPreview {
		start = phaseRunning
	}
	return tuiModel{
		ctx:      ctx,
		built:    built,
		cfg:      c,
		entries:  built.Entries,
		phase:    start,
		progress: progress.New(progress.WithDefaultGradient()),
		stream:   make(chan resultMsg),
		done:     make(chan doneMsg, 1),
	}
}

func (m tuiModel) Init() tea.Cmd {
	if m.phase == phaseRunning {
		return tea.Batch(m.startPipeline(), waitForResult(m.stream, m.done))
	}
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case resultMsg:
		m.results = append(m.results, msg.res)
		cmds := []tea.Cmd{waitForResult(m.stream, m.done)}
		if total := len(m.entries); total > 0 {
			cmds = append(cmds, m.progress.SetPercent(float64(len(m.results))/float64(total)))
		}
		return m, tea.Batch(cmds...)
	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd
	case doneMsg:
		m.phase = phaseDone
		m.results = msg.results
		m.runErr = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m tuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "y", "enter":
		if m.phase == phasePreview {
			m.phase = phaseRunning
			return m, tea.Batch(m.startPipeline(), waitForResult(m.stream, m.done))
		}
	case "n":
		if m.phase == phasePreview {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m tuiModel) View() string {
	switch m.phase {
	case phasePreview:
		return m.previewView()
	case phaseRunning:
		return m.runningView()
	default:
		return m.doneView()
	}
}

func (m tuiModel) previewView() string {
	s := titleStyle.Render(fmt.Sprintf("archivist — %d file(s) to process", len(m.entries))) + "\n"
	s += dimStyle.Render(fmt.Sprintf("model=%s  tagging=%s  taxonomy=%s", m.cfg.Model.Name, taggingLabel(m.cfg), m.cfg.Taxonomy.Strategy)) + "\n\n"
	for _, e := range m.entries {
		s += "  • " + e.Rel + "\n"
	}
	s += "\n" + titleStyle.Render("Proceed? [y/N]") + "\n"
	return s
}

func (m tuiModel) runningView() string {
	s := titleStyle.Render(fmt.Sprintf("Processing %d/%d", len(m.results), len(m.entries))) + "\n\n"
	s += m.progress.View() + "\n\n"
	for _, r := range tail(m.results, 10) {
		s += "  " + resultLine(r) + "\n"
	}
	return s
}

func (m tuiModel) doneView() string {
	var ok, failed int
	for _, r := range m.results {
		if r.Err != nil {
			failed++
		} else {
			ok++
		}
	}
	s := titleStyle.Render("Done") + "\n\n"
	for _, r := range m.results {
		s += "  " + resultLine(r) + "\n"
	}
	verb := "processed"
	if m.cfg.DryRun {
		verb = "previewed (dry run)"
	}
	s += "\n" + fmt.Sprintf("%d %s, %d failed.\n", ok, verb, failed)
	return s
}

func resultLine(r pipeline.Result) string {
	if r.Err != nil {
		return errStyle.Render("✗ "+relOrBase(r.Source)) + ": " + r.Err.Error()
	}
	tags := "no tags"
	if len(r.Tags) > 0 {
		tags = "tags: " + join(r.Tags)
	}
	return okStyle.Render("✓ "+relOrBase(r.Source)) + " → " + r.Dest + dimStyle.Render(" ("+tags+")")
}

// startPipeline runs the pipeline in a goroutine, streaming results into the
// model's channels; the Update loop consumes them via waitForResult.
func (m tuiModel) startPipeline() tea.Cmd {
	return func() tea.Msg {
		go func() {
			results, err := m.built.Pipeline.Run(m.ctx, m.entries, pipeline.ProgressHook{
				OnFile: func(i int, r pipeline.Result) {
					m.stream <- resultMsg{index: i, res: r}
				},
			})
			m.done <- doneMsg{results: results, err: err}
		}()
		return nil
	}
}

// waitForResult blocks on the next streamed result or the done signal.
func waitForResult(stream chan resultMsg, done chan doneMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case r := <-stream:
			return r
		case d := <-done:
			return d
		}
	}
}

// tail returns the last n elements of s (or all if fewer).
func tail[T any](s []T, n int) []T {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
