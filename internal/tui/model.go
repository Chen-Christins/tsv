package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"tsv/internal/svn"
)

const pageSize = 100

type loaded struct {
	client  svn.Client
	info    svn.Info
	commits []svn.Commit
	err     error
}
type moreLoaded struct {
	generation int
	commits    []svn.Commit
	err        error
}
type positionLoaded struct {
	generation int
	info       svn.Info
	err        error
}
type preview struct {
	id       int
	text     string
	err      error
	revision int
}
type Model struct {
	client                                        svn.Client
	info                                          svn.Info
	commits                                       []svn.Commit
	selected                                      int // zero is the synthetic working-tree row
	width, height                                 int
	view                                          viewport.Model
	detail, focusDiff, loading, paging, exhausted bool
	err                                           error
	request, generation                           int
	cancel                                        context.CancelFunc
	now                                           time.Time
	locatingBase                                  bool
	positionLoading                               bool
	positionError                                 error
	detailCache                                   map[int]string
}

func New(client svn.Client) Model {
	return Model{client: client, view: viewport.New(40, 10), loading: true, now: time.Now(), detailCache: make(map[int]string)}
}
func (m Model) Init() tea.Cmd { return m.load() }
func (m Model) load() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		client := m.client
		info, err := client.Info(ctx)
		if err != nil {
			return loaded{err: err}
		}
		if info.Root != "" {
			client.Dir = info.Root
			info, err = client.Info(ctx)
		}
		if err != nil {
			return loaded{err: err}
		}
		return loaded{client: client, info: info}
	}
}
func (m Model) loadPosition() tea.Cmd {
	client, info, generation := m.client, m.info, m.generation
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		info, err := client.Position(ctx, info)
		return positionLoaded{generation, info, err}
	}
}
func (m Model) loadHistory() tea.Cmd {
	client, generation := m.client, m.generation
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		commits, err := client.Log(ctx, 0, pageSize)
		return moreLoaded{generation, commits, err}
	}
}
func (m *Model) size() {
	w := m.width
	if m.detail {
		w = m.width - m.leftWidth() - 1
	}
	m.view.Width, m.view.Height = max(1, w), max(1, m.height-4)
}
func (m Model) leftWidth() int {
	if !m.detail {
		return m.width
	}
	return m.width / 2
}
func (m *Model) selectPreview() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	m.request++
	m.detail, m.focusDiff = true, true
	m.size()
	m.view.GotoTop()
	m.view.SetContent("Loading detail...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	m.cancel = cancel
	id, selected, client := m.request, m.selected, m.client
	var commit svn.Commit
	if selected > 0 {
		commit = m.commits[selected-1]
		if text, ok := m.detailCache[commit.Revision]; ok {
			cancel()
			m.view.SetContent(text)
			return nil
		}
	}
	return func() tea.Msg {
		defer cancel()
		var text string
		var err error
		if selected == 0 {
			text, err = client.WorkingDetail(ctx)
		} else {
			text, err = client.CommitDetail(ctx, commit)
		}
		return preview{id: id, text: text, err: err, revision: commit.Revision}
	}
}
func (m *Model) more() tea.Cmd {
	if m.loading || m.paging || m.exhausted || len(m.commits) == 0 {
		return nil
	}
	m.paging = true
	client, before, generation := m.client, m.commits[len(m.commits)-1].Revision, m.generation
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		commits, err := client.Log(ctx, before, pageSize)
		return moreLoaded{generation, commits, err}
	}
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.size()
	case loaded:
		m.loading, m.err = false, msg.err
		if msg.info.Root != "" {
			m.client, m.info = msg.client, msg.info
		}
		if msg.err == nil {
			m.client, m.info, m.commits = msg.client, msg.info, msg.commits
			m.selected = 0
			m.now = time.Now()
			m.exhausted = false
			m.positionLoading, m.positionError = false, nil
			m.locatingBase = false
			m.paging = true
			return m, m.loadHistory()
		}
		m.locatingBase = false
	case positionLoaded:
		if msg.generation != m.generation {
			return m, nil
		}
		m.positionLoading, m.positionError = false, msg.err
		if msg.err == nil {
			m.info = msg.info
		}
		if m.locatingBase {
			if msg.err != nil {
				m.locatingBase = false
			} else {
				return m, m.locateBase()
			}
		}
	case moreLoaded:
		if msg.generation != m.generation {
			return m, nil
		}
		m.paging, m.err = false, msg.err
		if msg.err == nil {
			m.commits = append(m.commits, msg.commits...)
			m.exhausted = len(msg.commits) < pageSize
		}
		if m.locatingBase {
			if msg.err != nil {
				m.locatingBase = false
				m.positionLoading, m.positionError = false, nil
			} else {
				return m, m.locateBase()
			}
		}
	case preview:
		if msg.id != m.request {
			return m, nil
		}
		text := msg.text
		if msg.err != nil {
			text += "\nSVN error:\n" + msg.err.Error()
		}
		m.view.SetContent(colorDiff(safeText(text)))
		if msg.err == nil && msg.revision > 0 {
			if len(m.detailCache) >= 20 {
				m.detailCache = make(map[int]string)
			}
			m.detailCache[msg.revision] = colorDiff(safeText(text))
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case "q", "esc":
			if m.detail {
				if m.cancel != nil {
					m.cancel()
				}
				m.request++
				m.detail, m.focusDiff = false, false
				m.size()
			} else {
				return m, tea.Quit
			}
		case "tab":
			if m.detail {
				m.focusDiff = !m.focusDiff
			}
		case "enter":
			if !m.loading {
				return m, m.selectPreview()
			}
		case "H":
			if !m.loading {
				m.locatingBase = false
				m.focusDiff = false
				if len(m.commits) > 0 {
					m.selected = 1
				}
			}
		case "B":
			if !m.loading {
				m.focusDiff = false
				m.locatingBase = true
				return m, m.locateBase()
			}
		case "V":
			if !m.loading && !m.positionLoading {
				m.positionLoading = true
				m.positionError = nil
				return m, m.loadPosition()
			}
		case "r":
			if !m.loading {
				if m.cancel != nil {
					m.cancel()
				}
				m.request++
				m.generation++
				m.loading = true
				m.paging = false
				m.locatingBase = false
				m.positionLoading, m.positionError = false, nil
				m.detailCache = make(map[int]string)
				m.detail, m.focusDiff = false, false
				m.size()
				return m, m.load()
			}
		default:
			m.locatingBase = false
			if m.focusDiff {
				var cmd tea.Cmd
				m.view, cmd = m.view.Update(msg)
				return m, cmd
			}
			if m.loading {
				return m, nil
			}
			switch msg.String() {
			case "up", "k":
				m.selected--
			case "down", "j":
				m.selected++
			case "pgup":
				m.selected -= m.view.Height
			case "pgdown":
				m.selected += m.view.Height
			case "home", "g":
				m.selected = 0
			case "end", "G":
				m.selected = len(m.commits)
			}
			m.selected = max(0, min(len(m.commits), m.selected))
			if m.selected >= len(m.commits)-10 {
				return m, m.more()
			}
		}
	}
	return m, nil
}

func (m *Model) locateBase() tea.Cmd {
	if m.positionLoading {
		return nil
	}
	for i, c := range m.commits {
		if c.Revision == m.info.BaseCommit {
			m.selected = i + 1
			m.locatingBase = false
			return nil
		}
	}
	if m.info.BaseCommit == 0 || m.exhausted {
		m.locatingBase = false
		return nil
	}
	return m.more()
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading terminal..."
	}
	if m.width < 60 || m.height < 12 {
		return "Terminal too small (minimum 60 x 12).\nResize terminal, or press q to quit."
	}
	h, w := m.view.Height, m.leftWidth()
	rows := make([]string, 0, h)
	start := max(0, m.selected-h+1)
	for i := start; i <= len(m.commits) && len(rows) < h; i++ {
		date, author, subject := m.now.Format("2006-01-02 15:04 -0700"), "Not Committed Yet", "Unstaged changes"
		revision := ""
		if i > 0 {
			c := m.commits[i-1]
			date = c.Date.Local().Format("2006-01-02 15:04 -0700")
			author = c.Author
			revision = fmt.Sprintf("[r%d]", c.Revision)
			if i == 1 {
				revision = "[HEAD] " + revision
			}
			if c.Revision == m.info.BaseCommit {
				label := "[ROOT BASE]"
				if m.info.MinRevision != m.info.MaxRevision {
					label = "[ROOT BASE]"
				}
				revision = label + " " + revision
			}
			subject = strings.Split(c.Message, "\n")[0]
		}
		line := historyRow(date, author, revision, subject, i == 0, i == m.selected, w)
		rows = append(rows, line)
	}
	list := lipgloss.NewStyle().Width(w).Height(h).Render(strings.Join(rows, "\n"))
	body := list
	if m.detail {
		body = lipgloss.JoinHorizontal(lipgloss.Top, list, lipgloss.NewStyle().Foreground(mutedColor).Height(h).Render(strings.TrimSuffix(strings.Repeat("│\n", h), "\n")), m.view.View())
	}
	status := fmt.Sprintf("main | %d commits | row %d", len(m.commits), m.selected+1)
	if m.info.Revision != "" {
		status += " | root BASE r" + m.info.Revision
	}
	if m.info.MinRevision != m.info.MaxRevision {
		status += fmt.Sprintf(" | mixed r%d:r%d", m.info.MinRevision, m.info.MaxRevision)
	}
	if m.locatingBase {
		status += " | locating BASE..."
	}
	if m.positionLoading {
		status += " | checking BASE..."
	}
	if m.positionError != nil {
		status += " | BASE unavailable: " + safeText(strings.ReplaceAll(m.positionError.Error(), "\n", " "))
	}
	if m.detail {
		if m.focusDiff {
			status += " | focus: detail"
		} else {
			status += " | focus: history"
		}
	}
	if m.loading {
		status = "Loading SVN history..."
	} else if m.paging {
		if len(m.commits) == 0 {
			status += " | loading history..."
		} else {
			status += " | loading older commits..."
		}
	}
	if m.err != nil {
		status = "SVN error: " + safeText(strings.ReplaceAll(m.err.Error(), "\n", " "))
	}
	return bar("tsv | "+safeText(m.client.Dir), m.width, false) + "\n" + body + "\n" + bar(status, m.width, m.err != nil) + "\n" + lipgloss.NewStyle().Foreground(mutedColor).Render(ansi.Truncate("↑↓/jk move  Enter open  Tab focus  H HEAD  B BASE  V versions  r refresh  q close/quit", m.width, "…"))
}
func safeText(s string) string {
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if r < 32 && r != '\n' || r >= 127 && r < 160 {
			return -1
		}
		return r
	}, s)
}
