package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"aicp/internal/diffutil"
	"aicp/internal/ops"
	"aicp/internal/store"
	"aicp/internal/web"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	selStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("61"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	faintStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	borderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238"))
	barStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	cyanStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	addStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	modStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	delStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	headStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	sepLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmGoto
	confirmGotoPurge
	confirmDrop
	confirmReset
)

type screen int

const (
	screenMain screen = iota
	screenSetInput
	screenConfirm
)

type EntryRow struct {
	M   *store.Manifest
	Amd ops.AddModDel
}

type Summary struct {
	Branch    string
	Size      int64
	LatestID  int
	Dirty     bool
	Counts    ops.AddModDel
	NoSession bool
}

type Model struct {
	dir          string
	rows         []EntryRow
	sum          Summary
	cursor       int
	screen       screen
	ti           textinput.Model
	vp           viewport.Model
	previewPrev  bool
	previewKey   string
	status       string
	isErr        bool
	width        int
	height       int
	loaded       bool
	confirmKind  confirmKind
	confirmMsg   string
	pendingID    int
	dashAddr     string
	showHelp     bool
}

func New(dir string) Model {
	ti := textinput.New()
	ti.Placeholder = "checkpoint message (empty = automatic)"
	ti.Prompt = "> "
	return Model{dir: dir, ti: ti, previewPrev: true}
}

func (m Model) Init() tea.Cmd {
	return loadBundle(m.dir)
}

type bundleMsg struct {
	rows []EntryRow
	sum  Summary
	err  error
}

type previewMsg struct {
	key     string
	content string
}

type opMsg struct {
	text string
	err  error
}

type dashboardMsg struct {
	addr string
	err  error
}

func loadBundle(dir string) tea.Cmd {
	return func() tea.Msg {
		ms, lerr := ops.List(dir)
		sum := Summary{Branch: ops.CurrentBranch(dir)}
		if lerr != nil {
			if strings.Contains(lerr.Error(), "no checkpoint session") {
				sum.NoSession = true
				return bundleMsg{sum: sum}
			}
			return bundleMsg{err: lerr}
		}
		rows := make([]EntryRow, 0, len(ms))
		for i, m := range ms {
			var amd ops.AddModDel
			if i > 0 {
				amd = ops.Classify(ops.ManifestMap(ms[i-1].Files), ops.ManifestMap(m.Files))
			}
			rows = append(rows, EntryRow{M: m, Amd: amd})
		}
		if st, serr := ops.StoreInfo(dir); serr == nil {
			sum.Size = st.SizeBytes()
		}
		if len(ms) > 0 {
			latest, amd, serr := ops.Status(dir)
			if serr == nil && latest != nil {
				sum.LatestID = latest.ID
				sum.Counts = amd
				sum.Dirty = !amd.Empty()
			}
		}
		return bundleMsg{rows: rows, sum: sum}
	}
}

func (m Model) fetchPreview() tea.Cmd {
	if len(m.rows) == 0 || m.cursor >= len(m.rows) {
		return nil
	}
	sel := m.rows[m.cursor].M.ID
	mode := "prev"
	if !m.previewPrev {
		mode = "working"
	}
	key := fmt.Sprintf("%d:%s", sel, mode)
	return func() tea.Msg {
		var diffs []diffutil.FileDiff
		var err error
		switch {
		case mode == "working":
			diffs, err = ops.DiffWorking(m.dir, sel)
		case sel == 0:
			diffs, err = ops.DiffFromEmpty(m.dir, 0)
		default:
			diffs, err = ops.DiffCheckpoints(m.dir, sel-1, sel)
		}
		if err != nil {
			return previewMsg{key: key, content: "error: " + err.Error()}
		}
		content := diffutil.StatLine(diffs) + "\n\n" + formatDiffs(diffs)
		if len(content) > 300000 {
			content = content[:300000] + "\n...(truncated)"
		}
		return previewMsg{key: key, content: content}
	}
}

func openDashboardCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		port := resolveDashboardPort(dir)
		addr, err := web.Serve(dir, port)
		return dashboardMsg{addr: addr, err: err}
	}
}

func resolveDashboardPort(dir string) int {
	for _, p := range []string{filepath.Join(dir, ".env"), ".env"} {
		loadEnvFile(p)
	}
	raw := strings.TrimSpace(os.Getenv("AICP_DASHBOARD_PORT"))
	if raw == "" {
		return 0
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0
	}
	return port
}

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.Trim(strings.TrimSpace(kv[1]), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
}

func runOpCmd(dir string, f func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		msg, err := f()
		return opMsg{text: msg, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.vp.Width = msg.Width - 4
		return m, nil

	case bundleMsg:
		m.loaded = true
		if msg.err != nil {
			m.status, m.isErr = msg.err.Error(), true
			return m, nil
		}
		m.rows = msg.rows
		m.sum = msg.sum
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		cmds := []tea.Cmd{m.fetchPreview()}
		return m, tea.Batch(cmds...)

	case previewMsg:
		want := m.previewKeyFor()
		if msg.key == want {
			m.vp.SetContent(msg.content)
			m.vp.GotoTop()
		}
		return m, nil

	case dashboardMsg:
		if msg.err != nil {
			m.status, m.isErr = msg.err.Error(), true
			return m, nil
		}
		m.dashAddr = msg.addr
		m.status, m.isErr = fmt.Sprintf("Dashboard opened at http://%s", msg.addr), false
		return m, nil

	case opMsg:
		if msg.err != nil {
			m.status, m.isErr = msg.err.Error(), true
		} else {
			m.status, m.isErr = msg.text, false
		}
		return m, loadBundle(m.dir)

	case tea.KeyMsg:
		switch m.screen {
		case screenSetInput:
			switch msg.String() {
			case "esc":
				m.screen = screenMain
				return m, nil
			case "enter":
				val := strings.TrimSpace(m.ti.Value())
				m.screen = screenMain
				return m, runOpCmd(m.dir, func() (string, error) {
					man, amd, err := ops.Set(m.dir, val)
					if err != nil {
						return "", err
					}
					return fmt.Sprintf("Checkpoint #%d saved: %s (+%d ~%d -%d)",
						man.ID, man.Message, len(amd.Added), len(amd.Modified), len(amd.Deleted)), nil
				})
			}
			var cmd tea.Cmd
			m.ti, cmd = m.ti.Update(msg)
			return m, cmd

		case screenConfirm:
			switch msg.String() {
			case "y", "Y":
				kind, id := m.confirmKind, m.pendingID
				m.screen = screenMain
				switch kind {
				case confirmGoto:
					return m, runOpCmd(m.dir, func() (string, error) {
						safety, _, target, err := ops.GotoEx(m.dir, id, false)
						if err != nil {
							return "", err
						}
						return fmt.Sprintf("Restored to #%d %s. Safety #%d kept.",
							target.ID, target.Message, safety.ID), nil
					})
				case confirmGotoPurge:
					return m, runOpCmd(m.dir, func() (string, error) {
						safety, _, target, err := ops.GotoEx(m.dir, id, true)
						if err != nil {
							return "", err
						}
						return fmt.Sprintf("Restored to #%d and purged newer checkpoints. Safety #%d kept.", target.ID, safety.ID), nil
					})
				case confirmDrop:
					return m, runOpCmd(m.dir, func() (string, error) {
						dropped, err := ops.DropLatest(m.dir, -1)
						if err != nil {
							return "", err
						}
						return fmt.Sprintf("Checkpoint #%d dropped.", dropped.ID), nil
					})
				case confirmReset:
					return m, runOpCmd(m.dir, func() (string, error) {
						size, err := ops.Reset(m.dir)
						if err != nil {
							return "", err
						}
						return fmt.Sprintf("All checkpoints deleted (%.1f KB freed).", float64(size)/1024), nil
					})
				}
				return m, nil
			case "n", "N", "esc":
				m.screen = screenMain
				return m, nil
			}
			return m, nil

		case screenMain:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "?":
				m.showHelp = !m.showHelp
				return m, nil
			case "esc":
				if m.showHelp {
					m.showHelp = false
					return m, nil
				}
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
					m.previewKey = ""
					return m, m.fetchPreview()
				}
			case "down", "j":
				if m.cursor < len(m.rows)-1 {
					m.cursor++
					m.previewKey = ""
					return m, m.fetchPreview()
				}
			case "tab":
				m.previewPrev = !m.previewPrev
				m.previewKey = ""
				return m, m.fetchPreview()
			case "r":
				return m, loadBundle(m.dir)
			case "n":
				if m.sum.NoSession {
					m.status, m.isErr = "No session yet. Press s to start one.", true
					return m, nil
				}
				m.ti.SetValue("")
				m.ti.Focus()
				m.screen = screenSetInput
				return m, textinput.Blink
			case "s":
				return m, runOpCmd(m.dir, func() (string, error) {
					man, err := ops.Start(m.dir)
					if err != nil {
						return "", err
					}
					return fmt.Sprintf("Baseline #%d dibuat (%d file).", man.ID, len(man.Files)), nil
				})
			case "g":
				if !m.canSelectTarget() {
					return m, nil
				}
				id := m.rows[m.cursor].M.ID
				m.pendingID, m.confirmKind = id, confirmGoto
				m.confirmMsg = fmt.Sprintf("Restore working tree ke #%d? Safety snapshot otomatis dibuat. y/n", id)
				m.screen = screenConfirm
				return m, nil
			case "G":
				if !m.canSelectTarget() {
					return m, nil
				}
				id := m.rows[m.cursor].M.ID
				m.pendingID, m.confirmKind = id, confirmGotoPurge
				m.confirmMsg = fmt.Sprintf("Restore to #%d and DELETE every newer checkpoint? The safety snapshot is still kept. y/n", id)
				m.screen = screenConfirm
				return m, nil
			case "d":
				if m.sum.NoSession || len(m.rows) == 0 {
					return m, nil
				}
				latest := m.rows[len(m.rows)-1].M
				m.pendingID, m.confirmKind = latest.ID, confirmDrop
				m.confirmMsg = fmt.Sprintf("Drop latest checkpoint #%d (%s)? The working tree will not change. y/n", latest.ID, latest.Message)
				m.screen = screenConfirm
				return m, nil
			case "D":
				if m.sum.NoSession || len(m.rows) == 0 {
					return m, nil
				}
				m.confirmKind = confirmReset
				m.confirmMsg = "Delete ALL checkpoints? History is gone; the working tree stays untouched. y/n"
				m.screen = screenConfirm
				return m, nil
			case "v":
				if m.dashAddr != "" {
					m.status, m.isErr = fmt.Sprintf("Dashboard sudah jalan di http://%s", m.dashAddr), false
					return m, nil
				}
				return m, openDashboardCmd(m.dir)
			case "pgup", "ctrl+u":
				m.vp.HalfPageUp()
				return m, nil
			case "pgdown", "ctrl+d":
				m.vp.HalfPageDown()
				return m, nil
			}
		}
	}
	return m, nil
}

func (m Model) canSelectTarget() bool {
	if m.sum.NoSession || len(m.rows) == 0 || m.cursor >= len(m.rows) {
		return false
	}
	return true
}

func (m Model) previewKeyFor() string {
	if len(m.rows) == 0 || m.cursor >= len(m.rows) {
		return ""
	}
	mode := "prev"
	if !m.previewPrev {
		mode = "working"
	}
	return fmt.Sprintf("%d:%s", m.rows[m.cursor].M.ID, mode)
}

func formatDiffs(diffs []diffutil.FileDiff) string {
	var sb strings.Builder
	for _, d := range diffs {
		sb.WriteString(fmt.Sprintf("[%s] %s", d.Status, d.Path))
		if d.Binary {
			sb.WriteString(" (binary)\n")
			continue
		}
		sb.WriteString("\n")
		if d.Patch != "" {
			sb.WriteString(d.Patch)
			if !strings.HasSuffix(d.Patch, "\n") {
				sb.WriteString("\n")
			}
		}
	}
	if sb.Len() == 0 {
		return "(no differences)"
	}
	return sb.String()
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	w := m.width
	var b strings.Builder

	head := titleStyle.Render("aicp") +
		dimStyle.Render("  "+filepath.Base(m.dir)) +
		dimStyle.Render("  branch: ") + cyanStyle.Render(orDash(m.sum.Branch))
	if m.sum.Size > 0 {
		head += dimStyle.Render(fmt.Sprintf("  store %.1f MB", float64(m.sum.Size)/(1024*1024)))
	}
	b.WriteString(head + "\n")

	tableH := clamp(len(m.rows)+4, 6, maxInt(6, m.height*45/100))
	b.WriteString(borderStyle.Width(w - 2).Render(m.renderTable(w-6, tableH)) + "\n")

	previewTitle := "preview: off"
	previewBody := ""
	if m.showHelp {
		previewTitle = "help  (? or esc to close)"
	} else if len(m.rows) > 0 {
		if m.previewPrev {
			previewTitle = "preview: changes in selected checkpoint (tab: vs working)"
		} else {
			previewTitle = "preview: selected vs working tree (tab: vs previous)"
		}
	}
	localVP := m.vp
	localVP.Width = w - 6
	localVP.Height = clamp(m.height-tableH-9, 3, 60)
	if m.showHelp {
		lines := strings.Split(aicpHelpText, "\n")
		if len(lines) > localVP.Height {
			lines = lines[:localVP.Height]
		}
		previewBody = strings.Join(lines, "\n")
	} else {
		previewBody = localVP.View()
	}
	b.WriteString(borderStyle.Width(w - 2).Render(previewTitle+"\n"+previewBody) + "\n")

	help := "↑/↓ select · tab preview mode · n set · g goto · G goto+purge · d drop · R reset-all · s start · v view · ? help · q quit"
	if m.screen == screenSetInput {
		help = "checkpoint message: " + m.ti.View() + "   (enter save · esc cancel)"
	} else if m.screen == screenConfirm {
		help = errStyle.Render(m.confirmMsg + "   (y = yes · n = no)")
	}
	b.WriteString(barStyle.Render(truncRunes(help, w)))

	if m.status != "" {
		st := okStyle.Render(truncRunes(m.status, w))
		if m.isErr {
			st = errStyle.Render(truncRunes(m.status, w))
		}
		b.WriteString("\n" + st)
	}
	return b.String()
}

func (m Model) renderTable(width, _ int) string {
	msgW := width - 5 - 17 - 11 - 12 - 18 - 10
	if msgW < 10 {
		msgW = 10
	}

	header := headStyle.Render(
		padRight("ID", 5) + padRight("WHEN", 18) + padRight("BRANCH", 12) +
			padRight("TAGS", 13) + padRight("MESSAGE", msgW+1) + "CHANGES")
	lines := []string{header, sepLineStyle.Render(strings.Repeat("─", maxInt(width, 20)))}

	if !m.loaded {
		return strings.Join(append(lines, dimStyle.Render("loading...")), "\n")
	}
	if m.sum.NoSession {
		return strings.Join(append(lines,
			dimStyle.Render("No checkpoint session yet."),
			dimStyle.Render("Press s to start one (captures baseline #0).")), "\n")
	}
	if len(m.rows) == 0 {
		return strings.Join(append(lines, dimStyle.Render("Tidak ada checkpoint.")), "\n")
	}

	latestID := m.rows[len(m.rows)-1].M.ID
	for i, row := range m.rows {
		tags := ""
		if row.M.Auto {
			tags += modStyle.Render("auto ")
		}
		if row.M.ID == latestID {
			tags += cyanStyle.Render("latest")
		}
		changes := faintStyle.Render("-")
		if i > 0 {
			changes = fmt.Sprintf("%s %s %s",
				addStyle.Render(fmt.Sprintf("+%d", len(row.Amd.Added))),
				modStyle.Render(fmt.Sprintf("~%d", len(row.Amd.Modified))),
				delStyle.Render(fmt.Sprintf("-%d", len(row.Amd.Deleted))))
		}
		line := fmt.Sprintf("#%-4d%s%s%s%s",
			row.M.ID,
			padRight(row.M.Time.Format("01-02 15:04"), 18),
			padRight(truncRunes(branchOrDash(row.M.Branch), 11), 12),
			padRight(tags, 13),
			truncRunes(row.M.Message, msgW)+"  "+changes)
		if i == m.cursor {
			line = selStyle.Render(padTo(line, width))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func branchOrDash(b string) string {
	if b == "" {
		return "-"
	}
	return b
}

const aicpHelpText = `NAVIGATION
  ↑/k, ↓/j        select a checkpoint
  tab             switch preview mode (per-CP ↔ vs working tree)
  pgup/pgdn       scroll preview
  r               reload data

OPERATIONS
  s               start a checkpoint session (baseline #0)
  n               checkpoint the current working tree
  g               goto the selected checkpoint (auto safety snapshot)
  G               goto + purge: also delete every newer checkpoint
  d               drop the latest checkpoint only (LIFO)
  R               reset: delete ALL checkpoints
  v               open the web dashboard in a browser

SAFETY NOTES
  · goto always creates an "auto" safety snapshot first.
  · set/goto only append; nothing is ever overwritten.
  · drop & reset erase history only; working files are never touched.
  · --purge keeps the safety snapshot, so the discarded state stays
    recoverable until you run drop/reset.

MISC
  ?               show/hide this help
  q / ctrl+c      quit`

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func padRight(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func padTo(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		if n <= 1 {
			return string(r[:n])
		}
		return string(r[:n-1]) + "…"
	}
	return s
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Run(dir string) error {
	p := tea.NewProgram(New(dir), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
