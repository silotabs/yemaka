package tui

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"yemaka/internal/agent"
	"yemaka/internal/diagnostics"
	"yemaka/internal/skills"
)

const (
	tabChat = iota
	tabCommand
	tabMemory
	tabDocs
	tabExtensions
	tabJobs
	tabHealth
	tabLearning
	tabHelp
)

var bubbleTabs = []string{"Chat", "Command", "Memory", "Docs", "Extensions", "Jobs", "Health", "Learning", "Help"}

type bubbleLine struct {
	Role string
	Text string
}

type bubbleModel struct {
	runner *Runner
	ctx    context.Context

	width  int
	height int
	tab    int

	input    textinput.Model
	viewport viewport.Model

	lines    []bubbleLine
	activity []string
	status   diagnostics.Report
	err      string

	busy           bool
	stream         <-chan tea.Msg
	cancel         context.CancelFunc
	replyIdx       int
	conversationID string
}

type bubbleStatusMsg struct {
	report diagnostics.Report
}

type bubbleCommandMsg struct {
	command string
	output  string
	err     error
}

type bubbleTokenMsg struct {
	token string
}

type bubbleDoneMsg struct {
	conversationID string
	model          string
	err            error
}

func (r *Runner) runBubble(ctx context.Context, input io.Reader, output io.Writer) error {
	model := newBubbleModel(ctx, r)
	program := tea.NewProgram(model, tea.WithInput(input), tea.WithOutput(output), tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return err
	}
	if model, ok := finalModel.(bubbleModel); ok && strings.TrimSpace(model.conversationID) != "" {
		r.activeConversationID = model.conversationID
	}
	r.printConversationResume(output)
	return nil
}

func newBubbleModel(ctx context.Context, runner *Runner) bubbleModel {
	input := textinput.New()
	input.Placeholder = "Ask Yemaka or type /help"
	input.Prompt = "> "
	input.CharLimit = 4000
	input.Focus()

	vp := viewport.New(80, 20)
	model := bubbleModel{
		runner:   runner,
		ctx:      ctx,
		input:    input,
		viewport: vp,
		lines: []bubbleLine{
			{Role: "system", Text: "Yemaka TUI is attached to the same core as Desktop and Web, created by SiloTabs https://silotabs.com."},
			{Role: "system", Text: "Enter sends an ask. Use /commands for core actions. Tab changes panels."},
		},
	}
	model.syncViewport()
	return model
}

func (m bubbleModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.refreshStatusCmd())
}

func (m bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.stopActive()
			return m, tea.Quit
		case "esc":
			if m.busy {
				m.stopActive()
				m.addActivity("run cancelled")
				m.busy = false
				return m, nil
			}
			m.err = ""
		case "tab":
			m.tab = (m.tab + 1) % len(bubbleTabs)
			m.input.Placeholder = m.placeholder()
			m.syncViewport()
		case "shift+tab":
			m.tab = (m.tab + len(bubbleTabs) - 1) % len(bubbleTabs)
			m.input.Placeholder = m.placeholder()
			m.syncViewport()
		case "ctrl+r":
			cmds = append(cmds, m.refreshStatusCmd())
		case "ctrl+l":
			m.lines = nil
			m.syncViewport()
		case "enter":
			if !m.busy {
				cmd := m.submit()
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)
		}
	case bubbleStatusMsg:
		m.status = msg.report
	case bubbleCommandMsg:
		m.busy = false
		m.err = ""
		if msg.err != nil {
			m.err = msg.err.Error()
			m.addLine("error", msg.err.Error())
		} else {
			m.conversationID = m.runner.activeConversationID
			m.addLine("tool", strings.TrimSpace(msg.output))
			m.addActivity(msg.command)
		}
	case bubbleTokenMsg:
		if m.replyIdx >= 0 && m.replyIdx < len(m.lines) {
			m.lines[m.replyIdx].Text += msg.token
			m.syncViewport()
		}
		if m.stream != nil {
			cmds = append(cmds, waitBubbleStream(m.stream))
		}
	case bubbleDoneMsg:
		m.busy = false
		m.stream = nil
		m.stopActive()
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			m.err = msg.err.Error()
			m.addLine("error", msg.err.Error())
		}
		if msg.conversationID != "" {
			m.conversationID = msg.conversationID
			m.runner.activeConversationID = msg.conversationID
			m.addActivity("conversation " + msg.conversationID)
		}
		if msg.model != "" {
			m.addActivity("model " + msg.model)
		}
	case bubbleLine:
		m.addLine(msg.Role, msg.Text)
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)
	return m, tea.Batch(cmds...)
}

func (m bubbleModel) View() string {
	if m.width <= 0 {
		return "Yemaka TUI\n"
	}
	header := m.headerView()
	sidebar := m.sidebarView()
	main := m.mainView()
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
	prompt := promptStyle.Width(max(20, m.width-2)).Render(m.input.View())
	footer := footerStyle.Width(max(20, m.width-2)).Render("enter send  tab panels  ctrl+r refresh  esc stop  ctrl+l clear  /quit exits")
	if m.err != "" {
		footer = errorStyle.Width(max(20, m.width-2)).Render(m.err)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, prompt, footer)
}

func (m *bubbleModel) resize() {
	sidebarW := sidebarWidth(m.width)
	bodyH := max(8, m.height-6)
	mainW := max(30, m.width-sidebarW-4)
	m.viewport.Width = mainW - 2
	m.viewport.Height = bodyH - 4
	m.input.Width = max(20, m.width-8)
	m.syncViewport()
}

func (m bubbleModel) headerView() string {
	model := strings.TrimSpace(m.status.SelectedModel)
	if model == "" {
		model = "not loaded"
	}
	modelReady := "not ready"
	if m.status.ModelReady {
		modelReady = "ready"
	}
	policy := policyMode(m.runner.deps.Config)
	internet := "net off"
	if m.runner.deps.Config != nil && m.runner.deps.Config.Internet.Enabled {
		internet = "net " + m.runner.deps.Config.Internet.DefaultMode
	}
	title := fmt.Sprintf("Yemaka TUI  %s  %s  %s  %s", modelReady, model, policy, internet)
	if m.busy {
		title += "  running"
	}
	return headerStyle.Width(max(20, m.width-2)).Render(title)
}

func (m bubbleModel) sidebarView() string {
	width := sidebarWidth(m.width)
	lines := []string{}
	for i, tab := range bubbleTabs {
		style := tabStyle
		if i == m.tab {
			style = activeTabStyle
		}
		lines = append(lines, style.Width(width-2).Render(tab))
	}
	lines = append(lines, "")
	lines = append(lines, metaStyle.Width(width-2).Render("Conversation"))
	if strings.TrimSpace(m.conversationID) == "" {
		lines = append(lines, sidebarText("new", width-2))
	} else {
		lines = append(lines, sidebarText(m.conversationID, width-2))
	}
	lines = append(lines, "")
	lines = append(lines, metaStyle.Width(width-2).Render("Profile"))
	lines = append(lines, sidebarText(m.status.ProfilePath, width-2))
	lines = append(lines, "")
	lines = append(lines, metaStyle.Width(width-2).Render("Activity"))
	for _, item := range take(m.activity, 6) {
		lines = append(lines, sidebarText(item, width-2))
	}
	return sidebarStyle.Width(width).Height(max(8, m.height-6)).Render(strings.Join(lines, "\n"))
}

func (m bubbleModel) mainView() string {
	title := bubbleTabs[m.tab]
	box := mainStyle.Width(max(30, m.width-sidebarWidth(m.width)-4)).Height(max(8, m.height-6)).Render(
		panelTitleStyle.Render(title) + "\n" + m.viewport.View(),
	)
	return box
}

func (m *bubbleModel) submit() tea.Cmd {
	raw := strings.TrimSpace(m.input.Value())
	if raw == "" {
		raw = m.defaultCommand()
		if raw == "" {
			return nil
		}
	}
	m.input.SetValue("")
	if raw == "/" {
		m.addLine("tool", helpPanel())
		return nil
	}
	if raw == "quit" || raw == "/q" || raw == "/quit" {
		return tea.Quit
	}
	if strings.HasPrefix(raw, "/") {
		command := strings.TrimSpace(strings.TrimPrefix(raw, "/"))
		if command == "q" || command == "quit" {
			return tea.Quit
		}
		m.addLine("user", "/"+command)
		m.busy = true
		return m.runCommandCmd(command)
	}
	switch m.tab {
	case tabCommand:
		m.addLine("user", "/"+raw)
		m.busy = true
		return m.runCommandCmd(raw)
	case tabMemory:
		command := "memory " + raw
		m.addLine("user", "/"+command)
		m.busy = true
		return m.runCommandCmd(command)
	case tabDocs:
		command := "docs " + raw
		m.addLine("user", "/"+command)
		m.busy = true
		return m.runCommandCmd(command)
	default:
		m.addLine("user", raw)
		m.addLine("assistant", "")
		m.replyIdx = len(m.lines) - 1
		m.busy = true
		ch := make(chan tea.Msg, 64)
		m.stream = ch
		runCtx, cancel := context.WithCancel(m.ctx)
		m.cancel = cancel
		go m.streamAsk(runCtx, raw, m.conversationID, ch)
		return waitBubbleStream(ch)
	}
}

func (m bubbleModel) defaultCommand() string {
	switch m.tab {
	case tabExtensions:
		return "extension list"
	case tabJobs:
		return "job status"
	case tabHealth:
		return "heartbeat status"
	case tabLearning:
		return "learn report"
	case tabCommand:
		return "status"
	default:
		return ""
	}
}

func (m bubbleModel) runCommandCmd(command string) tea.Cmd {
	return func() tea.Msg {
		var output bytes.Buffer
		reader := bufio.NewReader(strings.NewReader(""))
		err := m.runner.runCommand(m.ctx, reader, &output, command)
		return bubbleCommandMsg{command: command, output: output.String(), err: err}
	}
}

func (m bubbleModel) streamAsk(ctx context.Context, content string, conversationID string, ch chan<- tea.Msg) {
	defer close(ch)
	selectedSkill, hasSkill := m.runner.deps.Skills.Select(content, availableSkillTools())
	skillInstructions := ""
	if hasSkill {
		skillInstructions = skills.PromptBlock(selectedSkill)
	}
	service := &agent.Service{
		Router:          m.runner.deps.Router,
		Runtime:         m.runner.deps.Runtime,
		RuntimeFactory:  m.runner.deps.RuntimeFactory,
		Memory:          m.runner.deps.Memory,
		CloudFallback:   tuiCloudFallback(m.runner.deps.Config, m.runner.deps.Cloud),
		ToolExecutor:    tuiToolExecutor(m.runner.deps),
		Retriever:       tuiRetriever(m.runner.deps),
		CapabilityGap:   agent.NewCapabilityGapRouter(m.runner.deps.Config, m.runner.extensionStore()),
		ReplayStore:     tuiReplayStore(m.runner.deps),
		ModelProfileDir: modelProfileDir(m.runner.deps),
		PromptOptions:   agent.PromptOptionsFromConfig(m.runner.deps.Config),
		PolicyMode:      policyMode(m.runner.deps.Config),
		InternetTools:   agent.InternetToolLoopEnabled(m.runner.deps.Config, policyMode(m.runner.deps.Config)),
		InternetSearch:  agent.InternetSearchLoopEnabled(m.runner.deps.Config, policyMode(m.runner.deps.Config)),
	}
	var final bubbleDoneMsg
	err := service.Ask(ctx, agent.AskInput{
		Content:            content,
		ConversationID:     conversationID,
		SkillName:          selectedSkill.Name,
		SkillVersion:       selectedSkill.Version,
		SkillRequiredTools: selectedSkill.RequiredTools,
		SkillInstructions:  skillInstructions,
	}, func(event agent.Event) error {
		switch event.Type {
		case agent.EventModelSelected:
			final.model = event.Data["model"]
		case agent.EventModelToken:
			ch <- bubbleTokenMsg{token: event.Token}
		case agent.EventWorkspaceUsed:
			if event.Data["sources"] != "" {
				ch <- bubbleLine{Role: "context", Text: event.Data["source_kind"] + ": " + event.Data["sources"]}
			}
		case agent.EventSkillSelected:
			ch <- bubbleLine{Role: "skill", Text: event.Data["name"] + "@" + event.Data["version"]}
		case agent.EventToolCompleted:
			ch <- bubbleLine{Role: "tool", Text: event.Data["tool_name"] + " " + event.Data["status"]}
		case agent.EventAgentCompleted:
			final.conversationID = event.Data["conversation_id"]
		case agent.EventAgentError:
			return errors.New(event.Message)
		}
		return nil
	})
	final.err = err
	ch <- final
}

func (m bubbleModel) refreshStatusCmd() tea.Cmd {
	return func() tea.Msg {
		return bubbleStatusMsg{report: diagnostics.Run(m.ctx, m.runner.deps.Config, m.runner.deps.Profile, m.runner.deps.Memory, m.runner.deps.Runtime)}
	}
}

func waitBubbleStream(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return bubbleDoneMsg{}
		}
		return msg
	}
}

func (m *bubbleModel) stopActive() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func (m *bubbleModel) addLine(role string, text string) {
	text = strings.TrimSpace(text)
	if text == "" && role != "assistant" {
		return
	}
	m.lines = append(m.lines, bubbleLine{Role: role, Text: text})
	m.syncViewport()
}

func (m *bubbleModel) addActivity(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	m.activity = append([]string{text}, m.activity...)
	if len(m.activity) > 20 {
		m.activity = m.activity[:20]
	}
}

func (m *bubbleModel) syncViewport() {
	if m.tab == tabHelp {
		m.viewport.SetContent(helpPanel())
	} else {
		m.viewport.SetContent(renderLines(m.lines, max(30, m.viewport.Width)))
	}
	m.viewport.GotoBottom()
}

func (m bubbleModel) placeholder() string {
	switch m.tab {
	case tabCommand:
		return "status, extension list, job status, policy status..."
	case tabMemory:
		return "Search memory..."
	case tabDocs:
		return "Search ingested docs..."
	case tabExtensions:
		return "Enter for extension list or type /extension failures"
	case tabJobs:
		return "Enter for job status or type /job list"
	case tabHealth:
		return "Enter for heartbeat status"
	case tabLearning:
		return "Enter for learn report"
	case tabHelp:
		return "Use /help or switch tabs"
	default:
		return "Ask Yemaka..."
	}
}

func renderLines(lines []bubbleLine, width int) string {
	if len(lines) == 0 {
		return ""
	}
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		style := assistantStyle
		label := line.Role
		switch line.Role {
		case "user":
			style = userStyle
		case "system", "context", "skill":
			style = systemStyle
		case "tool":
			style = toolStyle
		case "error":
			style = inlineErrorStyle
		}
		text := style.Width(width).Render(label + ": " + line.Text)
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

func helpPanel() string {
	return strings.Join([]string{
		"Yemaka full-screen TUI",
		"",
		"Enter sends an agent ask from Chat.",
		"Commands start with / and call the same Go core used by Desktop and Web.",
		"",
		"Useful commands:",
		"  /status",
		"  /memory project preference",
		"  /docs model routing",
		"  /rag embeddings status",
		"  /rag embeddings on qwen3-embedding:0.6b",
		"  /rag embeddings index",
		"  /capability propose build a small CSV tool",
		"  /capability generate build a small CSV tool --yes --name csv_tool",
		"  /capability generate build a small CSV tool --yes --name csv_tool --run --input-json {\"task\":\"normalize rows\"}",
		"  /capability generate monitor a page --yes --job --every 1h --job-input-json {\"url\":\"https://example.com\"}",
		"  /extension list",
		"  /extension failures",
		"  /job status",
		"  /heartbeat status",
		"  /internet search-provider status",
		"  /internet search-provider searxng https://search.example/search --enable-internet",
		"  /internet search SearXNG --yes",
		"  /policy status",
		"  /learn report",
		"",
		"Keys:",
		"  tab / shift+tab  switch panels",
		"  ctrl+r           refresh status",
		"  ctrl+l           clear transcript",
		"  esc              cancel active run",
		"  /q or /quit      exit fullscreen TUI",
	}, "\n")
}

func sidebarWidth(total int) int {
	if total < 80 {
		return 18
	}
	return 24
}

func sidebarText(input string, width int) string {
	input = strings.Join(strings.Fields(input), " ")
	if len(input) > width && width > 3 {
		input = input[:width-3] + "..."
	}
	return subtleStyle.Width(width).Render(input)
}

func take(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("29")).
			Bold(true).
			Padding(0, 1)

	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("238")).
			Padding(1, 1)

	mainStyle = lipgloss.NewStyle().
			Padding(1, 1)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("29")).
			MarginBottom(1)

	tabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("29")).
			Bold(true).
			Padding(0, 1)

	metaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("178")).
			Bold(true)

	subtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("24")).
			Padding(0, 1)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("151")).
			Padding(0, 1)

	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("178")).
			Padding(0, 1)

	inlineErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("203")).
				Padding(0, 1)

	promptStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Padding(0, 1)
)
