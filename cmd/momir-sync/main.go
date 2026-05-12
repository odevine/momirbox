package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"momirbox/internal/config"
	"momirbox/internal/converter"
	"momirbox/internal/mtgdb"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

const (
	MaxWorkers         = 5
	DitherSettingsPath = "./config/dither_settings.json"
)

type syncConfig struct {
	Mode         string
	TestCardName string
	ForceRebuild bool
	Quit         bool
}

type syncState int

const (
	stateUpdatingDB syncState = iota
	stateParsing
	stateDownloading
	stateDeploying
	stateTestingDither
	stateAwaitingNextCard
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
type deployStepMsg string
type parsedMsg struct{ queue []mtgdb.MissingFile }
type itemProcessedMsg struct {
	itemName string
	success  bool
}
type deployFinishedMsg struct{}
type ditherTestFinishedMsg struct {
	outputDir string
}

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
	deployStep     string

	spinner   spinner.Model
	progress  progress.Model
	viewport  viewport.Model
	help      help.Model
	textInput textinput.Model

	cardHistory   []string
	historyCursor int

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
	} else if cfg.Mode == "Test Dither Settings" {
		startState = stateTestingDither
	}

	ti := textinput.New()
	ti.Placeholder = "Black Lotus  —or—  Ancient Den|BRC|172"
	ti.Prompt = "› "
	ti.CharLimit = 100

	settings, _ := converter.LoadDitherSettings(DitherSettingsPath)

	return model{
		cfg:            cfg,
		state:          startState,
		ditherSettings: settings,
		noiseImg:       loadBlueNoise(),
		spinner:        s,
		progress:       progress.New(progress.WithDefaultGradient()),
		viewport:       viewport.New(80, 10),
		help:           help.New(),
		textInput:      ti,
		client:         &http.Client{Timeout: 15 * time.Second},
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (m *model) Init() tea.Cmd {
	if m.state == stateDeploying {
		return tea.Batch(m.spinner.Tick, deployCmd(m.ctx, m.cfg.Mode, m.cfg.ForceRebuild, m.program))
	} else if m.state == stateTestingDither {
		m.viewport.Style = viewportStyle
		if name := strings.TrimSpace(m.cfg.TestCardName); name != "" {
			m.pushHistory(name)
		}
		return tea.Batch(m.spinner.Tick, testDitherCmd(m.ctx, m.cfg.TestCardName, m.ditherSettings, m.noiseImg))
	}
	return tea.Batch(m.spinner.Tick, updateDatabaseCmd(m.ctx))
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.state == stateAwaitingNextCard {
			switch msg.String() {
			case "ctrl+c":
				m.cancel()
				m.cfg.Quit = true
				return m, tea.Quit
			case "esc":
				return m, tea.Quit
			case "up":
				if m.historyCursor > 0 {
					m.historyCursor--
					m.textInput.SetValue(m.cardHistory[m.historyCursor])
					m.textInput.CursorEnd()
				}
				return m, nil
			case "down":
				if m.historyCursor < len(m.cardHistory) {
					m.historyCursor++
					if m.historyCursor == len(m.cardHistory) {
						m.textInput.SetValue("")
					} else {
						m.textInput.SetValue(m.cardHistory[m.historyCursor])
						m.textInput.CursorEnd()
					}
				}
				return m, nil
			case "enter":
				name := strings.TrimSpace(m.textInput.Value())
				if name == "" {
					return m, nil
				}
				m.pushHistory(name)
				m.cfg.TestCardName = name
				m.textInput.Reset()
				m.textInput.Blur()
				m.state = stateTestingDither
				if reloaded, err := converter.LoadDitherSettings(DitherSettingsPath); err == nil {
					m.ditherSettings = reloaded
				} else {
					m.addLog(fmt.Sprintf("⚠ %s", errorStyle.Render(fmt.Sprintf("settings reload failed: %v", err))))
				}
				m.addLog(fmt.Sprintf("→ Testing %s...", logColorStyle.Render(name)))
				return m, tea.Batch(m.spinner.Tick, testDitherCmd(m.ctx, name, m.ditherSettings, m.noiseImg))
			}
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}
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
		m.textInput.Width = progWidth
		m.help.Width = msg.Width
		return m, nil

	case errMsg:
		if m.state == stateTestingDither {
			m.addLog(fmt.Sprintf("✗ %s", errorStyle.Render(msg.err.Error())))
			m.state = stateAwaitingNextCard
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, textinput.Blink
		}
		m.err = msg.err
		return m, nil

	case logLineMsg:
		m.addLog(string(msg))
		return m, nil

	case deployStepMsg:
		m.deployStep = string(msg)
		m.addLog(fmt.Sprintf("→ %s", logColorStyle.Render(string(msg))))
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
				return m, deployCmd(m.ctx, m.cfg.Mode, m.cfg.ForceRebuild, m.program)
			}
			m.state = stateDone
			return m, nil
		}
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
				return m, tea.Batch(progressCmd, deployCmd(m.ctx, m.cfg.Mode, m.cfg.ForceRebuild, m.program))
			}
			m.state = stateDone
			return m, progressCmd
		}
		return m, tea.Batch(progressCmd, m.fillWorkerPool())

	case deployFinishedMsg:
		m.state = stateDone
		return m, nil

	case ditherTestFinishedMsg:
		m.addLog(fmt.Sprintf("✓ Test complete! Image saved to: %s", successStyle.Render(msg.outputDir)))
		m.state = stateAwaitingNextCard
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, textinput.Blink

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

func (m *model) pushHistory(name string) {
	if n := len(m.cardHistory); n > 0 && m.cardHistory[n-1] == name {
		m.historyCursor = n
		return
	}
	m.cardHistory = append(m.cardHistory, name)
	m.historyCursor = len(m.cardHistory)
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

// withRegionBorders returns an RGBA copy of gray with a 2px rectangle drawn
// inside the inner edge of each region: red for Sauvola regions, blue for
// CLAHE regions. Used only by the test pipeline to make tunable areas easy
// to spot.
func withRegionBorders(gray *image.Gray, sauvola []converter.SauvolaRegion, clahe []converter.CLAHERegion) image.Image {
	bounds := gray.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			v := gray.GrayAt(x, y).Y
			rgba.SetRGBA(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	drawBorder := func(xs, xe, ys, ye float64, c color.RGBA) {
		xMin := int(xs * float64(width))
		xMax := int(xe * float64(width))
		yMin := int(ys * float64(height))
		yMax := int(ye * float64(height))
		if xMin < 0 {
			xMin = 0
		}
		if yMin < 0 {
			yMin = 0
		}
		if xMax > width {
			xMax = width
		}
		if yMax > height {
			yMax = height
		}
		if xMax <= xMin || yMax <= yMin {
			return
		}
		for x := xMin; x < xMax; x++ {
			for dy := 0; dy < 2; dy++ {
				if yMin+dy < yMax {
					rgba.SetRGBA(x, yMin+dy, c)
				}
				if yMax-1-dy >= yMin {
					rgba.SetRGBA(x, yMax-1-dy, c)
				}
			}
		}
		for y := yMin; y < yMax; y++ {
			for dx := 0; dx < 2; dx++ {
				if xMin+dx < xMax {
					rgba.SetRGBA(xMin+dx, y, c)
				}
				if xMax-1-dx >= xMin {
					rgba.SetRGBA(xMax-1-dx, y, c)
				}
			}
		}
	}
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 96, B: 255, A: 255}
	for _, r := range clahe {
		drawBorder(r.XStart, r.XEnd, r.YStart, r.YEnd, blue)
	}
	for _, r := range sauvola {
		drawBorder(r.XStart, r.XEnd, r.YStart, r.YEnd, red)
	}
	return rgba
}

func testDitherCmd(ctx context.Context, cardName string, settings converter.DitherSettings, noiseImg image.Image) tea.Cmd {
	return func() tea.Msg {
		// Accept either a fuzzy name ("Black Lotus") or a pinned printing
		// ("Ancient Den|BRC|172" — name is informational, lookup uses set+number).
		var apiURL string
		parts := strings.Split(cardName, "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if len(parts) >= 3 && parts[1] != "" && parts[2] != "" {
			apiURL = fmt.Sprintf("https://api.scryfall.com/cards/%s/%s",
				url.PathEscape(strings.ToLower(parts[1])), url.PathEscape(parts[2]))
		} else {
			apiURL = fmt.Sprintf("https://api.scryfall.com/cards/named?fuzzy=%s", url.QueryEscape(parts[0]))
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return errMsg{fmt.Errorf("failed to create request: %w", err)}
		}

		// Scryfall is strict about headers; use your config's User-Agent
		req.Header.Set("User-Agent", config.UserAgent)
		req.Header.Set("Accept", "application/json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return errMsg{fmt.Errorf("API request failed: %w", err)}
		}
		defer resp.Body.Close()

		// If it fails, read the actual error from Scryfall to display in the UI
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return errMsg{fmt.Errorf("Scryfall returned %d: %s", resp.StatusCode, string(bodyBytes))}
		}

		var sfData struct {
			Name      string `json:"name"`
			Frame     string `json:"frame"`
			ImageURIs struct {
				Large string `json:"large"`
			} `json:"image_uris"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&sfData); err != nil {
			return errMsg{fmt.Errorf("failed to decode JSON: %w", err)}
		}

		if sfData.ImageURIs.Large == "" {
			return errMsg{fmt.Errorf("no large image URI available for %s", sfData.Name)}
		}

		// 2. Download the card image
		imgReq, err := http.NewRequestWithContext(ctx, "GET", sfData.ImageURIs.Large, nil)
		if err != nil {
			return errMsg{err}
		}

		imgReq.Header.Set("User-Agent", config.UserAgent)
		imgReq.Header.Set("Accept", "image/jpeg")

		imgResp, err := client.Do(imgReq)
		if err != nil {
			return errMsg{err}
		}
		defer imgResp.Body.Close()

		srcImg, _, err := image.Decode(imgResp.Body)
		if err != nil {
			return errMsg{fmt.Errorf("failed to decode downloaded image: %w", err)}
		}

		outputDir := settings.TestOutputDir
		if outputDir == "" {
			outputDir = "."
		}
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return errMsg{fmt.Errorf("failed to create output dir: %w", err)}
		}

		baseName := mtgdb.SanitizeForFilename(sfData.Name)
		frame := converter.MapFrame(sfData.Frame)

		ditheredGray := converter.DitherImage(srcImg, config.PrinterWidth, settings, frame, noiseImg)

		filename := filepath.Join(outputDir, baseName+"_dither_test.png")
		outFile, err := os.Create(filename)
		if err != nil {
			return errMsg{fmt.Errorf("failed to create PNG file: %w", err)}
		}

		var encodeErr error
		if settings.TestShowRegionBorders {
			encodeErr = png.Encode(outFile, withRegionBorders(ditheredGray, settings.SauvolaRegions[frame], settings.CLAHERegions[frame]))
		} else {
			encodeErr = png.Encode(outFile, ditheredGray)
		}
		if encodeErr != nil {
			outFile.Close()
			return errMsg{fmt.Errorf("failed to encode PNG: %w", encodeErr)}
		}
		outFile.Close()

		return ditherTestFinishedMsg{outputDir: outputDir}
	}
}

func (m *model) addLog(entry string) {
	m.logs = append(m.logs, entry)
	if len(m.logs) > 100 {
		m.logs = m.logs[1:]
	}
	m.viewport.SetContent(strings.Join(m.logs, "\n"))

	if m.viewport.Height > m.viewport.Style.GetVerticalFrameSize() {
		m.viewport.GotoBottom()
	}
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
	case stateTestingDither:
		b.WriteString(fmt.Sprintf("%s %s", m.spinner.View(), infoStyle.Render(fmt.Sprintf("Fetching & Processing test card: %s...", m.cfg.TestCardName))))
		b.WriteString("\n" + m.viewport.View())
	case stateAwaitingNextCard:
		b.WriteString(infoStyle.Render("Test another card (enter to submit, esc for menu):"))
		b.WriteString("\n" + m.textInput.View())
		b.WriteString("\n" + m.viewport.View())
	case stateDeploying:
		stepLabel := m.deployStep
		if stepLabel == "" {
			stepLabel = "Preparing deployment..."
		}
		b.WriteString(fmt.Sprintf("%s %s", m.spinner.View(), infoStyle.Render(stepLabel)))
		b.WriteString("\n" + m.viewport.View())
	case stateDone:
		b.WriteString(successStyle.Render("✨ Operations Complete! Press 'esc' for Menu or 'q' to exit."))
		b.WriteString("\n" + m.viewport.View())
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

func deployCmd(ctx context.Context, mode string, forceRebuild bool, p *tea.Program) tea.Cmd {
	return func() tea.Msg {
		getEnv := func(k string) string { return strings.Trim(os.Getenv(k), "\"") }
		user, host, dest, appName := getEnv("PI_USER"), getEnv("PI_HOST"), getEnv("PI_DEST"), getEnv("APP_NAME")

		if user == "" || host == "" || dest == "" || appName == "" {
			return errMsg{fmt.Errorf("environment incomplete: check .env file")}
		}

		target := fmt.Sprintf("%s@%s:%s", user, host, dest)
		sshTarget := fmt.Sprintf("%s@%s", user, host)

		step := func(msg string) {
			if p != nil {
				p.Send(deployStepMsg(msg))
			}
		}

		runStreamingCmd := func(c *exec.Cmd) error {
			output, err := c.CombinedOutput()
			if err != nil {
				if p != nil {
					p.Send(logLineMsg(string(output)))
				}
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
			}
			return nil
		}

		if mode == "Build and Deploy Binary" || mode == "Full System Sync" {
			step(fmt.Sprintf("Building %s for linux/arm64...", appName))
			_ = os.MkdirAll("bin", 0755)
			build := exec.CommandContext(ctx, "go", "build", "-tags", "pi", "-o", "bin/"+appName, "./cmd/momir-box")
			build.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
			if err := runStreamingCmd(build); err != nil {
				return errMsg{fmt.Errorf("build failed: %w", err)}
			}

			step(fmt.Sprintf("Ensuring remote directory %s exists...", dest))
			_ = exec.CommandContext(ctx, "ssh", sshTarget, "mkdir -p "+dest).Run()

			step("Stopping momirbox service on device...")
			if err := runStreamingCmd(exec.CommandContext(ctx, "ssh", sshTarget, "sudo systemctl stop momirbox")); err != nil {
				return errMsg{fmt.Errorf("failed to stop service: %w", err)}
			}

			step(fmt.Sprintf("Uploading %s binary to device...", appName))
			if err := runStreamingCmd(exec.CommandContext(ctx, "scp", "bin/"+appName, target+"/")); err != nil {
				return errMsg{fmt.Errorf("scp failed: %w", err)}
			}

			step("Syncing assets/ to device...")
			if err := runStreamingCmd(exec.CommandContext(ctx, "rsync", "-avz", "assets/", target+"/assets/")); err != nil {
				return errMsg{fmt.Errorf("rsync failed: %w", err)}
			}

			step("Starting momirbox service on device...")
			if err := runStreamingCmd(exec.CommandContext(ctx, "ssh", sshTarget, "sudo systemctl start momirbox")); err != nil {
				return errMsg{fmt.Errorf("failed to start service: %w", err)}
			}
		}

		if mode == "Deploy Images" || mode == "Full System Sync" {
			if forceRebuild {
				step("Wiping data/images/ on device...")
				if err := runStreamingCmd(exec.CommandContext(ctx, "ssh", sshTarget, "rm -rf "+dest+"/data/images")); err != nil {
					return errMsg{fmt.Errorf("failed to wipe remote images: %w", err)}
				}
			}

			step("Ensuring remote data/images/ exists...")
			_ = exec.CommandContext(ctx, "ssh", sshTarget, "mkdir -p "+dest+"/data/images").Run()

			step("Syncing data/images/ to device...")
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
					huh.NewOption("Test Dither Settings", "Test Dither Settings"),
					huh.NewOption("Build and Deploy Binary", "Build and Deploy Binary"),
					huh.NewOption("Deploy Images", "Deploy Images"),
					huh.NewOption("Sync Creatures Locally", "Sync Creatures Locally"),
					huh.NewOption("Sync Tokens Locally", "Sync Tokens Locally"),
					huh.NewOption("Update DB", "Update DB"),
					huh.NewOption("Exit", "Quit"),
				).
				Value(&cfg.Mode),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Card Name").
				Description("Card name (fuzzy) or 'Name|SET|Number' for a specific printing").
				Value(&cfg.TestCardName),
		).WithHideFunc(func() bool {
			return cfg.Mode != "Test Dither Settings"
		}),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Force Rebuild?").
				Description("Wipe destination data before re-syncing (local for sync modes, remote images for deploy).").
				Value(&cfg.ForceRebuild),
		).WithHideFunc(func() bool {
			return cfg.Mode == "Build and Deploy Binary" || cfg.Mode == "Update DB" || cfg.Mode == "Test Dither Settings" || cfg.Mode == "Quit"
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
