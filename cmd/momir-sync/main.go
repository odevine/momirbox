package main

import (
	"bufio"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"momirbox/internal/config"
	"momirbox/internal/converter"
	"momirbox/internal/mtgdb"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

const MaxWorkers = 5

type syncConfig struct {
	Mode         string
	ForceRebuild bool
	Quit         bool
}

type syncState int

const (
	stateUpdatingDB syncState = iota
	stateParsing
	stateDownloading
	stateDeploying
	stateDone
)

var (
	appStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	titleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).MarginBottom(1)
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	subTextStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	logColorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	viewportStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false).BorderForeground(lipgloss.Color("240")).MarginTop(1)
)

type keyMap struct {
	Quit  key.Binding
	Pause key.Binding
	Back  key.Binding
}

var keys = keyMap{
	Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Pause: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pause")),
	Back:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "menu")),
}

func (k keyMap) ShortHelp() []key.Binding  { return []key.Binding{k.Pause, k.Back, k.Quit} }
func (k keyMap) FullHelp() [][]key.Binding { return [][]key.Binding{{k.Pause, k.Back, k.Quit}} }

type errMsg struct{ err error }
type dbUpdatedMsg struct{}
type logLineMsg string
type parsedMsg struct{ queue []mtgdb.MissingFile }
type itemProcessedMsg struct {
	itemName string
	success  bool
}
type deployFinishedMsg struct{}

type model struct {
	cfg            syncConfig
	state          syncState
	err            error
	queue          []mtgdb.MissingFile
	totalItems     int
	doneItems      int
	activeTasks    int
	logs           []string
	isPaused       bool
	syncStartTime  time.Time
	ditherSettings converter.DitherSettings
	noiseImg       image.Image

	spinner  spinner.Model
	progress progress.Model
	viewport viewport.Model
	help     help.Model

	client  *http.Client
	ctx     context.Context
	cancel  context.CancelFunc
	program *tea.Program
}

func loadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "=") && !strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line, "=", 2)
			key := parts[0]
			value := strings.Trim(parts[1], "\"")
			os.Setenv(key, value)
		}
	}
}

func loadBlueNoise() image.Image {
	f, err := os.Open("assets/bluenoise.png")
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

func initialModel(cfg syncConfig) model {
	ctx, cancel := context.WithCancel(context.Background())
	s := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	startState := stateUpdatingDB
	if cfg.Mode == "Build and Deploy Binary" || cfg.Mode == "Deploy Images" {
		startState = stateDeploying
	}

	return model{
		cfg:   cfg,
		state: startState,
		ditherSettings: converter.DitherSettings{
			Brightness: 0,
			Contrast:   1.0,
			Method:     converter.MethodFloydSteinberg,
		},
		noiseImg: loadBlueNoise(),
		spinner:  s,
		progress: progress.New(progress.WithDefaultGradient()),
		viewport: viewport.New(80, 10),
		help:     help.New(),
		client:   &http.Client{Timeout: 15 * time.Second},
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (m *model) Init() tea.Cmd {
	if m.state == stateDeploying {
		return tea.Batch(m.spinner.Tick, deployCmd(m.ctx, m.cfg.Mode, m.program))
	}
	return tea.Batch(m.spinner.Tick, updateDatabaseCmd(m.ctx))
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			m.cancel()
			m.cfg.Quit = true
			return m, tea.Quit
		case key.Matches(msg, keys.Back):
			if m.state == stateDone || m.err != nil {
				return m, tea.Quit
			}
		case key.Matches(msg, keys.Pause):
			m.isPaused = !m.isPaused
			if !m.isPaused && m.state == stateDownloading {
				return m, m.fillWorkerPool()
			}
		}

	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()

		progWidth := msg.Width - h - 4
		vpWidth := msg.Width - h
		vpHeight := msg.Height - v - 14

		if progWidth < 0 {
			progWidth = 0
		}
		if vpWidth < 0 {
			vpWidth = 0
		}
		if vpHeight < 0 {
			vpHeight = 0
		}

		m.progress.Width = progWidth
		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
		m.help.Width = msg.Width
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case logLineMsg:
		m.addLog(string(msg))
		return m, nil

	case dbUpdatedMsg:
		if m.cfg.Mode == "Update DB" {
			m.state = stateDone
			return m, nil
		}
		m.state = stateParsing
		return m, parseFilesCmd(m.ctx, m.cfg)

	case parsedMsg:
		m.queue = msg.queue
		m.totalItems = len(m.queue)
		if m.totalItems == 0 {
			if m.cfg.Mode == "Full System Sync" {
				m.state = stateDeploying
				m.viewport.Style = viewportStyle
				return m, deployCmd(m.ctx, m.cfg.Mode, m.program)
			}
			m.state = stateDone
			return m, nil
		}
		// Skip preview entirely, jump straight to downloading
		m.state = stateDownloading
		m.syncStartTime = time.Now()
		m.viewport.Style = viewportStyle
		return m, m.fillWorkerPool()

	case itemProcessedMsg:
		m.doneItems++
		m.activeTasks--
		if msg.success {
			m.addLog(fmt.Sprintf("✓ %s", logColorStyle.Render(msg.itemName)))
		} else {
			m.addLog(fmt.Sprintf("✗ %s", errorStyle.Render(msg.itemName)))
		}

		progressCmd := m.progress.SetPercent(float64(m.doneItems) / float64(m.totalItems))

		if m.doneItems >= m.totalItems {
			if m.cfg.Mode == "Full System Sync" {
				m.state = stateDeploying
				return m, tea.Batch(progressCmd, deployCmd(m.ctx, m.cfg.Mode, m.program))
			}
			m.state = stateDone
			return m, progressCmd
		}
		return m, tea.Batch(progressCmd, m.fillWorkerPool())

	case deployFinishedMsg:
		m.state = stateDone
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		m.progress = newModel.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m *model) fillWorkerPool() tea.Cmd {
	if m.isPaused || m.ctx.Err() != nil {
		return nil
	}
	var cmds []tea.Cmd
	for m.activeTasks < MaxWorkers && (m.doneItems+m.activeTasks) < m.totalItems {
		item := m.queue[m.doneItems+m.activeTasks]
		m.activeTasks++
		cmds = append(cmds, downloadCmd(m.client, item, m.ditherSettings, m.noiseImg))
	}
	return tea.Batch(cmds...)
}

func downloadCmd(client *http.Client, item mtgdb.MissingFile, settings converter.DitherSettings, noise image.Image) tea.Cmd {
	return func() tea.Msg {
		success := mtgdb.DownloadAndConvert(client, item, settings, noise)
		return itemProcessedMsg{itemName: item.Name, success: success}
	}
}

func (m *model) addLog(entry string) {
	m.logs = append(m.logs, entry)
	if len(m.logs) > 100 {
		m.logs = m.logs[1:]
	}
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
}

func (m *model) getETA() string {
	if m.doneItems == 0 {
		return "Calculating..."
	}
	elapsed := time.Since(m.syncStartTime)
	itemsPerSecond := float64(m.doneItems) / elapsed.Seconds()
	if itemsPerSecond == 0 {
		return "--"
	}
	remainingItems := float64(m.totalItems - m.doneItems)
	remainingSeconds := remainingItems / itemsPerSecond
	return (time.Duration(remainingSeconds) * time.Second).Round(time.Second).String()
}

func (m *model) View() string {
	if m.err != nil {
		return appStyle.Render(fmt.Sprintf("%s\n\n%s", errorStyle.Render(fmt.Sprintf("Fatal Error: %v", m.err)), m.help.View(keys)))
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("🔮 MomirBox Project Manager"))
	b.WriteString("\n\n")

	switch m.state {
	case stateUpdatingDB:
		b.WriteString(fmt.Sprintf("%s %s", m.spinner.View(), infoStyle.Render("Updating MTGJSON Database...")))
	case stateParsing:
		b.WriteString(fmt.Sprintf("%s %s", m.spinner.View(), infoStyle.Render("Scanning local filesystem...")))
	case stateDownloading:
		b.WriteString(m.progress.View() + "\n\n")
		stateLabel := "Downloading"
		if m.isPaused {
			stateLabel = errorStyle.Render("PAUSED")
		}
		countStr := fmt.Sprintf("%d/%d", m.doneItems, m.totalItems)
		etaStr := fmt.Sprintf("ETC: %s", m.getETA())
		statusLine := fmt.Sprintf("%s %s %s • %s", m.spinner.View(), stateLabel, subTextStyle.Render(countStr), infoStyle.Render(etaStr))
		b.WriteString(statusLine + "\n")
		b.WriteString(m.viewport.View())
	case stateDeploying:
		b.WriteString(fmt.Sprintf("%s %s", m.spinner.View(), infoStyle.Render("Running Deployment Tasks...")))
		b.WriteString("\n" + m.viewport.View())
	case stateDone:
		b.WriteString(successStyle.Render("✨ Operations Complete! Press 'esc' for Menu or 'q' to exit."))
	}

	b.WriteString("\n\n")
	b.WriteString(m.help.View(keys))
	return appStyle.Render(b.String())
}

func updateDatabaseCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if err := mtgdb.UpdateDatabase(ctx); err != nil {
			return errMsg{err}
		}
		return dbUpdatedMsg{}
	}
}

func parseFilesCmd(ctx context.Context, cfg syncConfig) tea.Cmd {
	return func() tea.Msg {
		if cfg.ForceRebuild {
			if strings.Contains(cfg.Mode, "Creatures") || cfg.Mode == "Full System Sync" {
				_ = os.RemoveAll(config.CreaturesDir)
			}
			if strings.Contains(cfg.Mode, "Tokens") || cfg.Mode == "Full System Sync" {
				_ = os.RemoveAll(config.TokensDir)
			}
		}
		var queue []mtgdb.MissingFile
		if strings.Contains(cfg.Mode, "Creatures") || cfg.Mode == "Full System Sync" {
			c, _ := mtgdb.GetMissingCreatures(ctx)
			queue = append(queue, c...)
		}
		if strings.Contains(cfg.Mode, "Tokens") || cfg.Mode == "Full System Sync" {
			t, _ := mtgdb.GetMissingTokens(ctx)
			queue = append(queue, t...)
		}
		return parsedMsg{queue: queue}
	}
}

func deployCmd(ctx context.Context, mode string, p *tea.Program) tea.Cmd {
	return func() tea.Msg {
		getEnv := func(k string) string { return strings.Trim(os.Getenv(k), "\"") }
		user, host, dest, appName := getEnv("PI_USER"), getEnv("PI_HOST"), getEnv("PI_DEST"), getEnv("APP_NAME")

		if user == "" || host == "" || dest == "" || appName == "" {
			return errMsg{fmt.Errorf("environment incomplete: check .env file")}
		}

		target := fmt.Sprintf("%s@%s:%s", user, host, dest)
		sshTarget := fmt.Sprintf("%s@%s", user, host)

		runStreamingCmd := func(c *exec.Cmd) error {
			// Combine Stdout and Stderr so we see compiler/ssh errors
			output, err := c.CombinedOutput()
			if err != nil {
				if p != nil {
					p.Send(logLineMsg(string(output))) // Send the full error text to the UI logs
				}
				// FIX: Wrap the generic exit error with the actual string output from the command
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
			}
			return nil
		}

		if mode == "Build and Deploy Binary" || mode == "Full System Sync" {
			_ = os.MkdirAll("bin", 0755)
			build := exec.CommandContext(ctx, "go", "build", "-tags", "pi", "-o", "bin/"+appName, "./cmd/momir-box")
			build.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
			if err := runStreamingCmd(build); err != nil {
				return errMsg{fmt.Errorf("build failed: %w", err)}
			}

			_ = exec.CommandContext(ctx, "ssh", sshTarget, "mkdir -p "+dest).Run()

			// 1. Stop the service before copying the new binary
			if err := runStreamingCmd(exec.CommandContext(ctx, "ssh", sshTarget, "sudo systemctl stop momirbox")); err != nil {
				return errMsg{fmt.Errorf("failed to stop service: %w", err)}
			}

			// 2. Upload the new binary (now that the file is unlocked)
			if err := runStreamingCmd(exec.CommandContext(ctx, "scp", "bin/"+appName, target+"/")); err != nil {
				return errMsg{fmt.Errorf("scp failed: %w", err)}
			}

			if err := runStreamingCmd(exec.CommandContext(ctx, "rsync", "-avz", "assets/", target+"/assets/")); err != nil {
				return errMsg{fmt.Errorf("rsync failed: %w", err)}
			}

			// 3. Start the service back up with the fresh binary
			if err := runStreamingCmd(exec.CommandContext(ctx, "ssh", sshTarget, "sudo systemctl start momirbox")); err != nil {
				return errMsg{fmt.Errorf("failed to start service: %w", err)}
			}
		}

		if mode == "Deploy Images" || mode == "Full System Sync" {
			_ = exec.CommandContext(ctx, "ssh", sshTarget, "mkdir -p "+dest+"/data/images").Run()
			syncCmd := exec.CommandContext(ctx, "rsync", "-avz", "--progress", "--delete-during",
				"--include=*/", "--include=*.bin", "--exclude=*",
				"data/images/", target+"/data/images/")
			if err := runStreamingCmd(syncCmd); err != nil {
				return errMsg{err}
			}
		}

		return deployFinishedMsg{}
	}
}

func runPreFlight() (syncConfig, error) {
	var cfg syncConfig
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Action").
				Options(
					huh.NewOption("Full System Sync", "Full System Sync"),
					huh.NewOption("Build and Deploy Binary", "Build and Deploy Binary"),
					huh.NewOption("Deploy Images", "Deploy Images"),
					huh.NewOption("Sync Creatures Locally", "Sync Creatures"),
					huh.NewOption("Sync Tokens Locally", "Sync Tokens"),
					huh.NewOption("Update DB", "Update DB"),
					huh.NewOption("Exit", "Quit"),
				).
				Value(&cfg.Mode),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Force Rebuild?").
				Description("Wipe local data and re-dither everything.").
				Value(&cfg.ForceRebuild),
		).WithHideFunc(func() bool {
			return cfg.Mode == "Build and Deploy Binary" || cfg.Mode == "Deploy Images" || cfg.Mode == "Update DB" || cfg.Mode == "Quit"
		}),
	).WithTheme(huh.ThemeDracula())

	err := f.Run()
	if cfg.Mode == "Quit" {
		cfg.Quit = true
	}
	return cfg, err
}

func main() {
	loadEnv()

	for {
		cfg, err := runPreFlight()
		if err != nil || cfg.Quit {
			os.Exit(0)
		}

		m := initialModel(cfg)
		p := tea.NewProgram(&m, tea.WithAltScreen())
		m.program = p

		finalModel, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if mod, ok := finalModel.(*model); ok && mod.cfg.Quit {
			os.Exit(0)
		}
	}
}
