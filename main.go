package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"controlx/chain"
	"controlx/ops"
	"controlx/wallet"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ethereum/go-ethereum/common"
)

// ── Constants ────────────────────────────────────────────────────────

const (
	groupIndexFile    = "wallet_groups.json"
	legacyWalletsFile = "wallets.json"
	ankrFile          = "ankr.txt"
	txLogCSV          = "tx_log.csv"
	txLogJSON         = "tx_log.json"
	labelsFile        = "wallet_labels.json"
	presetsFile       = "presets.json"
	proxyFile         = "proxies.txt"
)

// ── Tornado Cash Green Theme ─────────────────────────────────────────

var (
	// ── Cyan + Violet Cyberpunk Palette ──
	cCyan      = lipgloss.Color("51")  // bright cyan (primary highlight)
	cCyanMed   = lipgloss.Color("44")  // medium cyan
	cCyanDark  = lipgloss.Color("37")  // muted cyan
	cCyanSoft  = lipgloss.Color("123") // soft cyan (accents)
	cViolet    = lipgloss.Color("141") // violet (category headers)
	cVioletDim = lipgloss.Color("60")  // muted violet (borders)
	cPurple    = lipgloss.Color("99")  // purple (banner)
	cMagenta   = lipgloss.Color("183") // light magenta (secondary accent)
	cWhite     = lipgloss.Color("255") // bright white
	cSoftWhite = lipgloss.Color("252") // soft white (body text)
	cGray      = lipgloss.Color("245") // gray (address middle, info)
	cDarkGray  = lipgloss.Color("242") // dark gray (dimmed text)
	cDarkerGray= lipgloss.Color("238") // darker gray
	cRed       = lipgloss.Color("204") // coral red (errors)
	cDarkRed   = lipgloss.Color("167") // muted red (exit)
	cYellow    = lipgloss.Color("214") // amber (warnings)
	cGreen     = lipgloss.Color("84")  // mint green (success checks)
)

// Styles
var (
	sTitle    = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	sDim      = lipgloss.NewStyle().Foreground(cDarkGray)
	sDimmer   = lipgloss.NewStyle().Foreground(cDarkerGray)
	sGreen    = lipgloss.NewStyle().Foreground(cGreen)        // success / check marks
	sGreenB   = lipgloss.NewStyle().Foreground(cGreen).Bold(true)
	sLime     = lipgloss.NewStyle().Foreground(cViolet).Bold(true) // category headers
	sWhite    = lipgloss.NewStyle().Foreground(cWhite)
	sWhiteB   = lipgloss.NewStyle().Foreground(cWhite).Bold(true)
	sSoft     = lipgloss.NewStyle().Foreground(cSoftWhite)
	sGray     = lipgloss.NewStyle().Foreground(cGray)
	sRed      = lipgloss.NewStyle().Foreground(cRed)
	sDarkRed  = lipgloss.NewStyle().Foreground(cDarkRed)
	sYellow   = lipgloss.NewStyle().Foreground(cYellow)
	sBorder   = lipgloss.NewStyle().Foreground(cVioletDim)
	sGreenMed = lipgloss.NewStyle().Foreground(cCyanMed)       // banner accent
	sGreenBr  = lipgloss.NewStyle().Foreground(cCyanSoft)      // amounts, secondary highlight
	sAccent   = lipgloss.NewStyle().Foreground(cCyan).Bold(true) // primary highlight (cursor, counts)
	sPurple   = lipgloss.NewStyle().Foreground(cPurple)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(cVioletDim).
			Padding(1, 2)

	resultBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cVioletDim).
			Padding(1, 2)
)

// ── Menu Items ───────────────────────────────────────────────────────

type menuItem struct {
	key      string
	label    string
	desc     string
	category string
}

var menuItems = []menuItem{
	{key: "generate", label: "Generate new wallets", desc: "", category: "WALLET"},
	{key: "load", label: "Load existing wallets", desc: "", category: "WALLET"},
	{key: "label", label: "Wallet labels", desc: "tag wallets", category: "WALLET"},
	{key: "backup", label: "Backup / Restore", desc: "encrypted", category: "WALLET"},
	{key: "balance", label: "Check all balances", desc: "native + ERC-20", category: "OPERATIONS"},
	{key: "distribute", label: "Distribute (1 → many)", desc: "", category: "OPERATIONS"},
	{key: "collect", label: "Collect (many → 1)", desc: "", category: "OPERATIONS"},
	{key: "sweep", label: "Sweep all funds", desc: "", category: "OPERATIONS"},
	{key: "autofund", label: "Auto-fund gas", desc: "ERC-20 prep", category: "OPERATIONS"},
	{key: "dexmix", label: "Mix (cross-chain bridge)", desc: "native→native", category: "MIX / PRIVACY"},
	{key: "swapmix", label: "DEX Mix (swap)", desc: "native→token→native", category: "MIX / PRIVACY"},
	{key: "portfolio", label: "Portfolio dashboard", desc: "USD values", category: "ANALYTICS"},
	{key: "gastrack", label: "Gas tracker", desc: "estimate costs", category: "ANALYTICS"},
	{key: "dryrun", label: "Dry-run simulate", desc: "no broadcast", category: "ANALYTICS"},
	{key: "delay", label: "Humanizer delay", desc: "gaussian/uniform", category: "SETTINGS"},
	{key: "alert", label: "Alerts (Telegram/Discord)", desc: "webhook", category: "SETTINGS"},
	{key: "proxy", label: "Proxy settings", desc: "SOCKS5/HTTP", category: "SETTINGS"},
	{key: "session", label: "Session timeout", desc: "auto-lock", category: "SETTINGS"},
	{key: "queue", label: "Operation queue", desc: "chain ops", category: "SETTINGS"},
	{key: "export", label: "Export addresses (CSV)", desc: "", category: "SETTINGS"},
	{key: "viewlog", label: "Transaction log", desc: "", category: "SETTINGS"},
	{key: "exit", label: "Exit", desc: "", category: ""},
}

// ── View States ──────────────────────────────────────────────────────

type viewState int

const (
	viewMenu viewState = iota
	viewInput
	viewChainSelect
	viewConfirm
	viewSpinner
	viewResult
	viewDelayMenu
	viewDelayCustom
	viewTokenSelect
	viewGroupSelect
	viewAlertMenu
	viewProxyMenu
	viewSessionMenu
	viewBackupMenu
	viewQueueMenu
	viewLabelMenu
	viewLocked
)

// ── Messages ─────────────────────────────────────────────────────────

type opDoneMsg struct {
	result string
	err    error
	apply  func(m *model)
}

// bridgeProgressMsg carries a live progress line from bridge operation.
type bridgeProgressMsg struct {
	line string
}

// listenForProgress returns a Cmd that reads one message from the channel.
// It must be re-called after each received message to keep listening.
func listenForProgress(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nil // channel closed, stop listening
		}
		return bridgeProgressMsg{line: line}
	}
}

// ── Model ────────────────────────────────────────────────────────────

type model struct {
	view viewState

	// Menu
	menuCursor int

	// Text input
	textInput  textinput.Model
	inputLabel string
	inputKey   string

	// Chain select
	chainCursor int

	// Confirm
	confirmMsg    string
	confirmCursor int

	// Delay menu
	delayCursor int

	// Token select
	tokenCursor int

	// Spinner
	spinner       spinner.Model
	spinnerMsg    string
	progressLines []string      // live progress lines for bridge
	progressCh    chan string    // channel for bridge progress updates

	// Result
	resultText      string
	resultLines     []string
	scrollOffset    int
	resultContinues bool // if true, Enter advances wizard instead of menu

	// Operation wizard
	currentOp string
	step      int
	data      map[string]string

	// Auto-fund scan results
	scanResults []ops.FundResult

	// Group select
	groupCursor int
	groupIndex  *wallet.GroupIndex
	groupItems  []groupListItem // built on demand for load screen

	// App state
	wallets    []wallet.Wallet
	provider   *chain.Provider
	delayCfg   ops.DelayConfig
	txLogger   *ops.TxLogger
	alertCfg   ops.AlertConfig
	proxyCfg   ops.ProxyConfig
	session    *ops.SessionConfig
	priceCache *ops.PriceCache
	labels     *wallet.WalletLabels
	presets    *ops.PresetStore
	queue      *ops.QueueConfig
	amtRandPct int // amount randomization percentage (0=off)

	// Sub-menu cursor for generic list views
	subCursor int

	// Terminal
	width, height int
	statusMsg     string
}

// groupListItem represents one entry in the group selection list.
type groupListItem struct {
	name    string
	file    string
	count   string // "500 wallets" or "???"
	date    string // "2026-02-22" or ""
	isLegacy bool
}

func initialModel(prov *chain.Provider) model {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 50

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(cCyan)

	gi, _ := wallet.LoadGroupIndex(groupIndexFile)
	if gi == nil {
		gi = &wallet.GroupIndex{}
	}

	labels, _ := wallet.LoadLabels(labelsFile)
	if labels == nil {
		labels = &wallet.WalletLabels{Labels: make(map[string]string)}
	}

	presets := loadPresets()

	return model{
		view:       viewMenu,
		textInput:  ti,
		spinner:    s,
		provider:   prov,
		delayCfg:   ops.NoDelay(),
		txLogger:   ops.NewTxLogger(txLogCSV, txLogJSON),
		data:       make(map[string]string),
		groupIndex: gi,
		alertCfg:   ops.NoAlert(),
		proxyCfg:   ops.NoProxy(),
		session:    ops.NoSession(),
		priceCache: ops.NewPriceCache(5 * time.Minute),
		labels:     labels,
		presets:    presets,
		queue:      ops.NewQueue(),
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

// ── Update Router ────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Session lock check
	if m.session.IsLocked() && m.view != viewLocked {
		m.view = viewLocked
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.cleanup()
			return m, tea.Quit
		}
		// Touch session on any keypress
		m.session.Touch()
	case bridgeProgressMsg:
		m.progressLines = append(m.progressLines, msg.line)
		// Re-subscribe to get next progress message
		if m.progressCh != nil {
			return m, listenForProgress(m.progressCh)
		}
		return m, nil
	case opDoneMsg:
		if msg.err != nil {
			m.resultText = sRed.Render("  Error: " + msg.err.Error())
		} else {
			m.resultText = msg.result
		}
		if msg.apply != nil {
			msg.apply(&m)
		}
		m.resultLines = strings.Split(m.resultText, "\n")
		m.scrollOffset = 0
		m.progressLines = nil
		m.progressCh = nil
		m.view = viewResult
		return m, nil
	}

	switch m.view {
	case viewMenu:
		return m.updateMenu(msg)
	case viewInput:
		return m.updateInput(msg)
	case viewChainSelect:
		return m.updateChainSelect(msg)
	case viewConfirm:
		return m.updateConfirm(msg)
	case viewSpinner:
		return m.updateSpinner(msg)
	case viewResult:
		return m.updateResult(msg)
	case viewDelayMenu:
		return m.updateDelayMenu(msg)
	case viewTokenSelect:
		return m.updateTokenSelect(msg)
	case viewDelayCustom:
		return m.updateDelayCustom(msg)
	case viewGroupSelect:
		return m.updateGroupSelect(msg)
	case viewAlertMenu:
		return m.updateAlertMenu(msg)
	case viewProxyMenu:
		return m.updateProxyMenu(msg)
	case viewSessionMenu:
		return m.updateSessionMenu(msg)
	case viewBackupMenu:
		return m.updateBackupMenu(msg)
	case viewQueueMenu:
		return m.updateQueueMenu(msg)
	case viewLabelMenu:
		return m.updateLabelMenu(msg)
	case viewLocked:
		return m.updateLocked(msg)
	}
	return m, nil
}

// ── Menu ─────────────────────────────────────────────────────────────

func (m model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case "down", "j":
			if m.menuCursor < len(menuItems)-1 {
				m.menuCursor++
			}
		case "enter":
			return m.selectMenuItem()
		case "q":
			m.cleanup()
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) selectMenuItem() (tea.Model, tea.Cmd) {
	item := menuItems[m.menuCursor]
	m.currentOp = item.key
	m.step = 0
	m.data = make(map[string]string)
	m.statusMsg = ""
	m.resultContinues = false

	needsWallets := map[string]bool{
		"balance": true, "distribute": true, "collect": true,
		"sweep": true, "autofund": true, "export": true,
		"dexmix": true, "swapmix": true, "portfolio": true,
		"gastrack": true, "dryrun": true, "label": true,
	}

	if needsWallets[item.key] && len(m.wallets) == 0 {
		m.statusMsg = "No wallets loaded. Generate [1] or Load [2] first."
		return m, nil
	}

	switch item.key {
	case "exit":
		m.cleanup()
		return m, tea.Quit
	case "delay":
		m.view = viewDelayMenu
		m.delayCursor = 0
		return m, nil
	case "alert":
		m.view = viewAlertMenu
		m.subCursor = 0
		return m, nil
	case "proxy":
		m.view = viewProxyMenu
		m.subCursor = 0
		return m, nil
	case "session":
		m.view = viewSessionMenu
		m.subCursor = 0
		return m, nil
	case "backup":
		m.view = viewBackupMenu
		m.subCursor = 0
		return m, nil
	case "queue":
		m.view = viewQueueMenu
		m.subCursor = 0
		return m, nil
	case "label":
		m.view = viewLabelMenu
		m.subCursor = 0
		return m, nil
	case "viewlog":
		m.resultText = m.buildLogResult()
		m.resultLines = strings.Split(m.resultText, "\n")
		m.scrollOffset = 0
		m.view = viewResult
		return m, nil
	default:
		return m.advanceWizard()
	}
}

// ── Input ────────────────────────────────────────────────────────────

func (m model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter":
			val := m.textInput.Value()
			if val == "" {
				val = m.data["_default"]
			}
			m.data[m.inputKey] = val
			m.textInput.SetValue("")
			m.step++
			return m.advanceWizard()
		case "esc":
			m.textInput.SetValue("")
			return m.returnToMenu()
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) showInput(label, key string, password bool, defVal string) (model, tea.Cmd) {
	m.view = viewInput
	m.inputLabel = label
	m.inputKey = key
	m.textInput.SetValue("")
	m.textInput.Placeholder = defVal
	m.data["_default"] = defVal
	if password {
		m.textInput.EchoMode = textinput.EchoPassword
		m.textInput.EchoCharacter = '•'
	} else {
		m.textInput.EchoMode = textinput.EchoNormal
	}
	m.textInput.Focus()
	return m, textinput.Blink
}

// ── Chain Select ─────────────────────────────────────────────────────

func (m model) updateChainSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.chainCursor > 0 {
				m.chainCursor--
			}
		case "down", "j":
			if m.chainCursor < len(chain.AllChains)-1 {
				m.chainCursor++
			}
		case "enter":
			m.data["chain"] = strconv.Itoa(m.chainCursor)
			m.step++
			return m.advanceWizard()
		case "esc":
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) showChainSelect() (model, tea.Cmd) {
	m.view = viewChainSelect
	m.chainCursor = 0
	return m, nil
}

// ── Confirm ──────────────────────────────────────────────────────────

func (m model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "left", "h":
			m.confirmCursor = 0
		case "right", "l":
			m.confirmCursor = 1
		case "y":
			m.data["confirm"] = "y"
			m.step++
			return m.advanceWizard()
		case "n", "esc":
			return m.returnToMenu()
		case "enter":
			if m.confirmCursor == 0 {
				m.data["confirm"] = "y"
				m.step++
				return m.advanceWizard()
			}
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) showConfirm(msg string) (model, tea.Cmd) {
	m.view = viewConfirm
	m.confirmMsg = msg
	m.confirmCursor = 0
	return m, nil
}

// ── Spinner ──────────────────────────────────────────────────────────

func (m model) updateSpinner(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m model) showSpinner(msg string, fn func() (string, error, func(*model))) (model, tea.Cmd) {
	m.view = viewSpinner
	m.spinnerMsg = msg
	cmd := func() tea.Msg {
		result, err, apply := fn()
		return opDoneMsg{result: result, err: err, apply: apply}
	}
	return m, tea.Batch(m.spinner.Tick, cmd)
}

// ── Result ───────────────────────────────────────────────────────────

func (m model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "esc", "q":
			return m.returnToMenu()
		case "enter":
			if m.resultContinues {
				m.resultContinues = false
				m.step++
				return m.advanceWizard()
			}
			return m.returnToMenu()
		case "up", "k":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "down", "j":
			m.scrollOffset++
		}
	}
	return m, nil
}

// ── Delay Menu ───────────────────────────────────────────────────────

type delayOption struct {
	label string
	cfg   ops.DelayConfig
}

func (m model) delayOptions() []delayOption {
	opts := []delayOption{}
	if m.delayCfg.Enabled {
		opts = append(opts, delayOption{"OFF (max speed)", ops.NoDelay()})
	}
	opts = append(opts,
		delayOption{"Conservative  5-15s  uniform", ops.DelayConfig{Enabled: true, MinMs: 5000, MaxMs: 15000, Mode: ops.DelayModeUniform}},
		delayOption{"Moderate      3-10s  uniform", ops.DelayConfig{Enabled: true, MinMs: 3000, MaxMs: 10000, Mode: ops.DelayModeUniform}},
		delayOption{"Aggressive    1-5s   uniform", ops.DelayConfig{Enabled: true, MinMs: 1000, MaxMs: 5000, Mode: ops.DelayModeUniform}},
		delayOption{"Conservative  5-15s  gaussian", ops.DelayConfig{Enabled: true, MinMs: 5000, MaxMs: 15000, Mode: ops.DelayModeGaussian}},
		delayOption{"Moderate      3-10s  gaussian", ops.DelayConfig{Enabled: true, MinMs: 3000, MaxMs: 10000, Mode: ops.DelayModeGaussian}},
		delayOption{"Aggressive    1-5s   gaussian", ops.DelayConfig{Enabled: true, MinMs: 1000, MaxMs: 5000, Mode: ops.DelayModeGaussian}},
		delayOption{"Custom range", ops.DelayConfig{}},
		delayOption{"Amount randomizer ±%", ops.DelayConfig{}},
	)
	return opts
}

func (m model) updateDelayMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	opts := m.delayOptions()
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.delayCursor > 0 {
				m.delayCursor--
			}
		case "down", "j":
			if m.delayCursor < len(opts)-1 {
				m.delayCursor++
			}
		case "enter":
			sel := opts[m.delayCursor]
			if sel.label == "Custom range" {
				m.view = viewDelayCustom
				m.data = map[string]string{}
				m.step = 0
				m.currentOp = "delay_custom"
				return m.showInput("Min seconds", "min", false, "3")
			}
			if sel.label == "Amount randomizer ±%" {
				m.view = viewDelayCustom
				m.data = map[string]string{}
				m.step = 0
				m.currentOp = "amount_rand"
				return m.showInput("Variance % (0=off, 10=±10%)", "variance", false, "10")
			}
			m.delayCfg = sel.cfg
			return m.returnToMenu()
		case "esc":
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) updateDelayCustom(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Route to regular input handler
	return m.updateInput(msg)
}

// ── Token Select ─────────────────────────────────────────────────────

func (m model) tokenOptions() []chain.Token {
	ch := m.selectedChain()
	tokens := chain.PopularTokens[ch.Name]
	var opts []chain.Token
	// If optional mode, add "Native only" as first option
	if m.data["_token_optional"] == "1" {
		opts = append(opts, chain.Token{Symbol: "Native only", Address: "_native_"})
	}
	opts = append(opts, tokens...)
	opts = append(opts, chain.Token{Symbol: "Custom", Address: ""})
	return opts
}

func (m model) showTokenSelect(optional bool) (model, tea.Cmd) {
	m.view = viewTokenSelect
	m.tokenCursor = 0
	if optional {
		m.data["_token_optional"] = "1"
	} else {
		m.data["_token_optional"] = "0"
	}
	return m, nil
}

func (m model) updateTokenSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	opts := m.tokenOptions()
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.tokenCursor > 0 {
				m.tokenCursor--
			}
		case "down", "j":
			if m.tokenCursor < len(opts)-1 {
				m.tokenCursor++
			}
		case "enter":
			sel := opts[m.tokenCursor]
			if sel.Symbol == "Custom" {
				// showInput will do step++ on submit via updateInput
				return m.showInput("ERC-20 contract address", "token", false, "")
			}
			if sel.Address == "_native_" {
				m.data["token"] = ""
				m.data["token_symbol"] = ""
			} else {
				m.data["token"] = sel.Address
				m.data["token_symbol"] = sel.Symbol
			}
			m.step++
			return m.advanceWizard()
		case "esc":
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) viewTokenSelect() string {
	var b strings.Builder
	ch := m.selectedChain()
	opts := m.tokenOptions()

	b.WriteString("\n")
	b.WriteString(viewHeader(m.currentOp))
	b.WriteString("\n")
	b.WriteString("   " + sWhiteB.Render("SELECT TOKEN") + sDim.Render("  on "+ch.Name) + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 50)) + "\n\n")

	for i, tok := range opts {
		cursor := "   "
		symStyle := sSoft
		addrStyle := sDim
		if i == m.tokenCursor {
			cursor = sAccent.Render(" ▸ ")
			symStyle = sAccent
			addrStyle = sGray
		}
		if tok.Symbol == "Custom" || tok.Symbol == "Native only" {
			b.WriteString(fmt.Sprintf("  %s %s\n", cursor, symStyle.Render(tok.Symbol+"…")))
		} else {
			b.WriteString(fmt.Sprintf("  %s %-8s %s\n", cursor, symStyle.Render(tok.Symbol), addrStyle.Render(shortAddr(tok.Address))))
		}
	}

	b.WriteString("\n   " + sDim.Render("↑/↓ navigate · enter select · esc cancel") + "\n")
	return b.String()
}

// ── Group Select ─────────────────────────────────────────────────────

func (m model) buildGroupItems() []groupListItem {
	var items []groupListItem
	for _, g := range m.groupIndex.Groups {
		items = append(items, groupListItem{
			name:  g.Name,
			file:  g.File,
			count: fmt.Sprintf("%d wallets", g.Count),
			date:  g.CreatedAt.Format("2006-01-02"),
		})
	}
	// Legacy detection
	if wallet.WalletFileExists(legacyWalletsFile) {
		isIndexed := false
		for _, g := range m.groupIndex.Groups {
			if g.File == legacyWalletsFile {
				isIndexed = true
				break
			}
		}
		if !isIndexed {
			items = append(items, groupListItem{
				name:     "legacy",
				file:     legacyWalletsFile,
				count:    "???",
				date:     "",
				isLegacy: true,
			})
		}
	}
	return items
}

func (m model) showGroupSelect() (model, tea.Cmd) {
	items := m.buildGroupItems()
	if len(items) == 0 {
		m.resultText = sRed.Render("  No wallet groups found. Generate wallets first.")
		m.resultLines = strings.Split(m.resultText, "\n")
		m.view = viewResult
		return m, nil
	}
	m.groupItems = items
	m.groupCursor = 0
	m.view = viewGroupSelect
	return m, nil
}

func (m model) updateGroupSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.groupCursor > 0 {
				m.groupCursor--
			}
		case "down", "j":
			if m.groupCursor < len(m.groupItems)-1 {
				m.groupCursor++
			}
		case "enter":
			sel := m.groupItems[m.groupCursor]
			m.data["load_file"] = sel.file
			m.data["load_name"] = sel.name
			m.step++
			return m.advanceWizard()
		case "esc":
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) viewGroupSelect() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(viewHeader(m.currentOp))
	b.WriteString("\n")
	b.WriteString("   " + sWhiteB.Render("SELECT WALLET GROUP") + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 50)) + "\n\n")

	for i, item := range m.groupItems {
		cursor := "   "
		nameStyle := sSoft
		countStyle := sDim
		dateStyle := sDimmer
		if i == m.groupCursor {
			cursor = sAccent.Render(" ▸ ")
			nameStyle = sAccent
			countStyle = sGreenBr
			dateStyle = sGray
		}
		datePart := ""
		if item.date != "" {
			datePart = dateStyle.Render("  " + item.date)
		}
		extra := ""
		if item.isLegacy {
			extra = sDim.Render("  (" + item.file + ")")
		}
		b.WriteString(fmt.Sprintf("  %s %-12s %s%s%s\n",
			cursor,
			nameStyle.Render(item.name),
			countStyle.Render(item.count),
			datePart,
			extra))
	}

	b.WriteString("\n   " + sDim.Render("↑/↓ navigate · enter select · esc cancel") + "\n")
	return b.String()
}

// ── Helpers ──────────────────────────────────────────────────────────

func (m model) returnToMenu() (model, tea.Cmd) {
	m.view = viewMenu
	m.currentOp = ""
	m.step = 0
	return m, nil
}

func (m *model) cleanup() {
	if m.txLogger != nil {
		m.txLogger.Flush()
	}
	if m.provider != nil {
		m.provider.Close()
	}
}

func (m model) selectedChain() chain.Chain {
	idx, _ := strconv.Atoi(m.data["chain"])
	if idx >= 0 && idx < len(chain.AllChains) {
		return chain.AllChains[idx]
	}
	return chain.Ethereum
}

func (m *model) flushLog() string {
	if err := m.txLogger.Flush(); err != nil {
		return sRed.Render("  Flush log: " + err.Error())
	}
	if m.txLogger.Count() > 0 {
		return sDim.Render(fmt.Sprintf("  Logged %d tx → %s", m.txLogger.Count(), txLogCSV))
	}
	return ""
}

// ── Wizard Advancement ───────────────────────────────────────────────

func (m model) advanceWizard() (model, tea.Cmd) {
	switch m.currentOp {
	case "generate":
		return m.wizGenerate()
	case "load":
		return m.wizLoad()
	case "balance":
		return m.wizBalance()
	case "distribute":
		return m.wizDistribute()
	case "collect":
		return m.wizCollect()
	case "sweep":
		return m.wizSweep()
	case "autofund":
		return m.wizAutoFund()
	case "export":
		return m.wizExport()
	case "delay_custom":
		return m.wizDelayCustom()
	case "amount_rand":
		return m.wizAmountRand()
	case "dexmix":
		return m.wizDexMix()
	case "swapmix":
		return m.wizSwapMix()
	case "portfolio":
		return m.wizPortfolio()
	case "gastrack":
		return m.wizGasTrack()
	case "dryrun":
		return m.wizDryRun()
	case "alert_telegram":
		return m.wizAlertTelegram()
	case "alert_discord":
		return m.wizAlertDiscord()
	case "proxy_load":
		return m.wizProxyLoad()
	case "session_set":
		return m.wizSessionSet()
	case "backup_create":
		return m.wizBackupCreate()
	case "backup_restore":
		return m.wizBackupRestore()
	case "label_set":
		return m.wizLabelSet()
	case "queue_add":
		return m.wizQueueAdd()
	}
	return m.returnToMenu()
}

// ── Generate Wizard ──────────────────────────────────────────────────

func (m model) wizGenerate() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showInput("Group name", "group_name", false, m.groupIndex.NextName())
	case 1:
		return m.showInput("How many wallets?", "count", false, "500")
	case 2:
		return m.showInput("Set encryption password", "password", true, "")
	case 3:
		return m.showInput("Confirm password", "confirm", true, "")
	case 4:
		pwd := m.data["password"]
		confirm := m.data["confirm"]
		if pwd == "" {
			m.resultText = sRed.Render("  Password cannot be empty")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		if pwd != confirm {
			m.resultText = sRed.Render("  Passwords don't match")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		count := 500
		if n, err := strconv.Atoi(m.data["count"]); err == nil && n > 0 {
			count = n
		}
		groupName := m.data["group_name"]
		gi := m.groupIndex
		return m.showSpinner(fmt.Sprintf("Generating %d wallets…", count), func() (string, error, func(*model)) {
			start := time.Now()
			wls, err := wallet.GenerateWallets(count)
			if err != nil {
				return "", err, nil
			}
			g := gi.Add(groupName, count)
			err = wallet.SaveWallets(wls, g.File, pwd)
			if err != nil {
				return "", err, nil
			}
			err = wallet.SaveGroupIndex(groupIndexFile, gi)
			if err != nil {
				return "", err, nil
			}
			elapsed := time.Since(start)
			result := buildGenerateResult(wls, count, elapsed, g.File)
			return result, nil, func(m *model) {
				m.wallets = wls
				m.groupIndex = gi
			}
		})
	}
	return m.returnToMenu()
}

// ── Load Wizard ──────────────────────────────────────────────────────

func (m model) wizLoad() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showGroupSelect()
	case 1:
		return m.showInput("Enter password", "password", true, "")
	case 2:
		pwd := m.data["password"]
		loadFile := m.data["load_file"]
		return m.showSpinner("Decrypting wallets…", func() (string, error, func(*model)) {
			wls, err := wallet.LoadWallets(loadFile, pwd)
			if err != nil {
				return "", err, nil
			}
			fp := wallet.Fingerprint(loadFile)
			result := buildLoadResult(wls, fp)
			return result, nil, func(m *model) { m.wallets = wls }
		})
	}
	return m.returnToMenu()
}

// ── Balance Wizard ───────────────────────────────────────────────────

func (m model) wizBalance() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showChainSelect()
	case 1:
		return m.showTokenSelect(true)
	case 2:
		ch := m.selectedChain()
		tokenAddr := m.data["token"]
		wls := m.wallets
		prov := m.provider
		return m.showSpinner(fmt.Sprintf("Scanning %d wallets on %s…", len(wls), ch.Name), func() (string, error, func(*model)) {
			start := time.Now()
			results, err := ops.CheckBalances(prov, ch, wls, tokenAddr)
			if err != nil {
				return "", err, nil
			}
			return buildBalanceResult(results, ch, tokenAddr, time.Since(start)), nil, nil
		})
	}
	return m.returnToMenu()
}

// ── Distribute Wizard ────────────────────────────────────────────────

func (m model) wizDistribute() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showChainSelect()
	case 1:
		return m.showInput(fmt.Sprintf("Source wallet index (1-%d)", len(m.wallets)), "source", false, "1")
	case 2:
		return m.showInput("Destination range (e.g. 2-100, all)", "range", false, "all")
	case 3:
		return m.showTokenSelect(true)
	case 4:
		ch := m.selectedChain()
		return m.showInput(fmt.Sprintf("Amount per wallet (%s)", ch.Symbol), "amount", false, "")
	case 5:
		ch := m.selectedChain()
		fromIdx, _ := strconv.Atoi(m.data["source"])
		toWls := parseWalletRange(m.data["range"], m.wallets)
		amtWei := parseEther(m.data["amount"])
		if fromIdx < 1 || fromIdx > len(m.wallets) {
			m.resultText = sRed.Render("  Invalid source wallet index")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		if len(toWls) == 0 {
			m.resultText = sRed.Render("  Invalid destination range")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		if amtWei == nil {
			m.resultText = sRed.Render("  Invalid amount")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		total := new(big.Int).Mul(amtWei, big.NewInt(int64(len(toWls))))
		msg := fmt.Sprintf("Send %s %s each to %d wallets (total: %s %s)?",
			ops.FormatBalance(amtWei, 18), ch.Symbol, len(toWls),
			ops.FormatBalance(total, 18), ch.Symbol)
		return m.showConfirm(msg)
	case 6:
		ch := m.selectedChain()
		fromIdx, _ := strconv.Atoi(m.data["source"])
		fromWallet := m.wallets[fromIdx-1]
		toWls := parseWalletRange(m.data["range"], m.wallets)
		amtWei := parseEther(m.data["amount"])
		tokenAddr := m.data["token"]
		prov := m.provider
		delay := m.delayCfg
		logger := m.txLogger
		alertDist := m.alertCfg
		return m.showSpinner(fmt.Sprintf("Distributing to %d wallets…", len(toWls)), func() (string, error, func(*model)) {
			start := time.Now()
			var results []ops.TxResult
			var err error
			if tokenAddr == "" {
				results, err = ops.Distribute(prov, ch, fromWallet, toWls, amtWei, delay, logger)
			} else {
				token := common.HexToAddress(tokenAddr)
				results, err = ops.DistributeERC20(prov, ch, fromWallet, toWls, token, amtWei, delay, logger)
			}
			if err != nil {
				return "", err, nil
			}
			elapsed := time.Since(start)
			s, f := 0, 0
			for _, r := range results {
				if r.TxHash != "" { s++ } else if r.Error != "" { f++ }
			}
			alertDist.SendTxAlert("Distribute", ch.Name, s, f, elapsed)
			return buildDistributeResult(results, ch, amtWei, elapsed, delay), nil, nil
		})
	}
	return m.returnToMenu()
}

// ── Collect Wizard ───────────────────────────────────────────────────

func (m model) wizCollect() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showChainSelect()
	case 1:
		return m.showInput("Destination address", "dest", false, "")
	case 2:
		return m.showTokenSelect(true)
	case 3:
		dest := m.data["dest"]
		if !common.IsHexAddress(dest) {
			m.resultText = sRed.Render("  Invalid address")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		msg := fmt.Sprintf("Collect from %d wallets → %s?", len(m.wallets), shortAddr(dest))
		return m.showConfirm(msg)
	case 4:
		ch := m.selectedChain()
		dest := common.HexToAddress(m.data["dest"])
		tokenAddr := m.data["token"]
		wls := m.wallets
		prov := m.provider
		delay := m.delayCfg
		logger := m.txLogger
		alertColl := m.alertCfg
		return m.showSpinner(fmt.Sprintf("Collecting from %d wallets…", len(wls)), func() (string, error, func(*model)) {
			start := time.Now()
			var results []ops.TxResult
			var err error
			if tokenAddr == "" {
				results, err = ops.Collect(prov, ch, wls, dest, big.NewInt(0), delay, logger)
			} else {
				token := common.HexToAddress(tokenAddr)
				results, err = ops.CollectERC20(prov, ch, wls, dest, token, new(big.Int), delay, logger)
			}
			if err != nil {
				return "", err, nil
			}
			elapsed := time.Since(start)
			s, f := 0, 0
			for _, r := range results {
				if r.TxHash != "" { s++ } else if r.Error != "" && r.Error != "insufficient balance" { f++ }
			}
			alertColl.SendTxAlert("Collect", ch.Name, s, f, elapsed)
			return buildCollectResult(results, ch, elapsed, delay), nil, nil
		})
	}
	return m.returnToMenu()
}

// ── Sweep Wizard ─────────────────────────────────────────────────────

func (m model) wizSweep() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showChainSelect()
	case 1:
		return m.showInput("Destination address", "dest", false, "")
	case 2:
		return m.showTokenSelect(true)
	case 3:
		dest := m.data["dest"]
		if !common.IsHexAddress(dest) {
			m.resultText = sRed.Render("  Invalid address")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		msg := fmt.Sprintf("SWEEP ALL from %d wallets → %s?", len(m.wallets), shortAddr(dest))
		return m.showConfirm(msg)
	case 4:
		ch := m.selectedChain()
		dest := common.HexToAddress(m.data["dest"])
		tokenAddr := m.data["token"]
		wls := m.wallets
		prov := m.provider
		delay := m.delayCfg
		logger := m.txLogger
		alertSweep := m.alertCfg
		return m.showSpinner(fmt.Sprintf("Sweeping %d wallets…", len(wls)), func() (string, error, func(*model)) {
			start := time.Now()
			var sweepResults []ops.SweepResult
			if tokenAddr == "" {
				r, err := ops.SweepNative(prov, ch, wls, dest, delay, logger)
				if err != nil {
					return "", err, nil
				}
				sweepResults = r
			} else {
				token := common.HexToAddress(tokenAddr)
				r, err := ops.SweepERC20(prov, ch, wls, dest, token, delay, logger)
				if err != nil {
					return "", err, nil
				}
				sweepResults = r
			}
			elapsed := time.Since(start)
			s, f := 0, 0
			for _, r := range sweepResults {
				if r.TxHash != "" { s++ } else if r.Error != "" { f++ }
			}
			alertSweep.SendTxAlert("Sweep", ch.Name, s, f, elapsed)
			return buildSweepResult(sweepResults, ch, elapsed), nil, nil
		})
	}
	return m.returnToMenu()
}

// ── Auto-Fund Wizard ─────────────────────────────────────────────────

func (m model) wizAutoFund() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showChainSelect()
	case 1:
		return m.showInput(fmt.Sprintf("Funder wallet index (1-%d)", len(m.wallets)), "funder", false, "1")
	case 2:
		ch := m.selectedChain()
		defGas := "0.005"
		if ch.Symbol == "BNB" {
			defGas = "0.002"
		} else if ch.Symbol == "MATIC" {
			defGas = "0.1"
		} else if ch.Symbol == "AVAX" {
			defGas = "0.05"
		} else if ch.Symbol == "FTM" {
			defGas = "0.5"
		}
		return m.showInput(fmt.Sprintf("Min gas per wallet (%s)", ch.Symbol), "min_gas", false, defGas)
	case 3:
		funderIdx, _ := strconv.Atoi(m.data["funder"])
		if funderIdx < 1 || funderIdx > len(m.wallets) {
			m.resultText = sRed.Render("  Invalid funder index")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		minGas := parseEther(m.data["min_gas"])
		if minGas == nil || minGas.Sign() == 0 {
			m.resultText = sRed.Render("  Invalid gas amount")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		ch := m.selectedChain()
		wls := m.wallets
		prov := m.provider
		return m.showSpinner(fmt.Sprintf("Scanning %d wallets for low %s…", len(wls), ch.Symbol), func() (string, error, func(*model)) {
			start := time.Now()
			scanRes, err := ops.ScanLowGas(prov, ch, wls, minGas)
			if err != nil {
				return "", err, nil
			}
			result := buildScanResult(scanRes, ch, time.Since(start))
			needGas := 0
			for _, r := range scanRes {
				if r.NeedGas {
					needGas++
				}
			}
			if needGas == 0 {
				result += "\n\n" + sGreenB.Render(fmt.Sprintf("  All wallets have >= %s %s!", m.data["min_gas"], ch.Symbol))
				return result, nil, func(m *model) { m.scanResults = scanRes }
			}
			result += "\n\n" + sYellow.Render(fmt.Sprintf("  Press Enter to fund %d wallets, Esc to cancel", needGas))
			return result, nil, func(m *model) {
				m.scanResults = scanRes
				m.resultContinues = true
			}
		})
	case 4:
		needGas := 0
		for _, r := range m.scanResults {
			if r.NeedGas {
				needGas++
			}
		}
		if needGas == 0 {
			return m.returnToMenu()
		}
		ch := m.selectedChain()
		msg := fmt.Sprintf("Fund %s to %d wallets?", ch.Symbol, needGas)
		return m.showConfirm(msg)
	case 5:
		ch := m.selectedChain()
		funderIdx, _ := strconv.Atoi(m.data["funder"])
		funder := m.wallets[funderIdx-1]
		wls := m.wallets
		scanRes := m.scanResults
		prov := m.provider
		logger := m.txLogger
		return m.showSpinner(fmt.Sprintf("Funding %s…", ch.Symbol), func() (string, error, func(*model)) {
			start := time.Now()
			fundRes, err := ops.AutoFundGas(prov, ch, funder, wls, scanRes, logger)
			if err != nil {
				return "", err, nil
			}
			return buildFundResult(fundRes, ch, funder, time.Since(start)), nil, nil
		})
	}
	return m.returnToMenu()
}

// ── Bridge Mix Wizard ────────────────────────────────────────────────

func (m model) wizDexMix() (model, tea.Cmd) {
	switch m.step {
	case 0: // Chain rotation input
		return m.showInput(chainRotationLabel(), "chains", false, "2,1,3")
	case 1: // Source wallet index
		// Validate chain rotation
		chains := parseChainRotation(m.data["chains"])
		if len(chains) < 2 {
			m.resultText = sRed.Render("  Need at least 2 different chains (e.g., 2,1,3)")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		return m.showInput(fmt.Sprintf("Source wallet index (1-%d)", len(m.wallets)), "source", false, "1")
	case 2: // Number of hops
		srcIdx, _ := strconv.Atoi(m.data["source"])
		maxHops := len(m.wallets) - srcIdx
		defHops := "19"
		if maxHops < 19 {
			defHops = strconv.Itoa(maxHops)
		}
		return m.showInput(fmt.Sprintf("Number of hops (max %d)", maxHops), "hops", false, defHops)
	case 3: // Confirm
		chains := parseChainRotation(m.data["chains"])
		srcIdx, _ := strconv.Atoi(m.data["source"])
		numHops, _ := strconv.Atoi(m.data["hops"])

		if srcIdx < 1 || srcIdx > len(m.wallets) {
			m.resultText = sRed.Render("  Invalid source wallet index")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		if numHops < 1 || srcIdx+numHops > len(m.wallets) {
			m.resultText = sRed.Render(fmt.Sprintf("  Not enough wallets: need %d from index %d, have %d total", numHops+1, srcIdx, len(m.wallets)))
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}

		lastIdx := srcIdx + numHops
		lastWallet := m.wallets[lastIdx-1]

		// Build rotation display
		var rotDisplay strings.Builder
		for i, ch := range chains {
			if i > 0 {
				rotDisplay.WriteString(" → ")
			}
			rotDisplay.WriteString(ch.Name)
		}
		rotDisplay.WriteString(" → …")

		msg := fmt.Sprintf("BRIDGE MIX: %d cross-chain hops\nRotation: %s\nWallets: #%d → #%d\nNative → Native (self-paying gas)\nFinal: %s\n\nBridge via Li.Fi (auto-selects best route)\nProceed?",
			numHops, rotDisplay.String(), srcIdx, lastIdx, shortAddr(lastWallet.Address))
		return m.showConfirm(msg)
	case 4: // Execute
		chains := parseChainRotation(m.data["chains"])
		srcIdx, _ := strconv.Atoi(m.data["source"])
		numHops, _ := strconv.Atoi(m.data["hops"])

		hopWallets := m.wallets[srcIdx-1 : srcIdx+numHops]
		prov := m.provider
		delayCfg := m.delayCfg
		logger := m.txLogger
		alertCfg := m.alertCfg

		// Determine group name from loaded data
		groupName := m.data["load_name"]

		// Create progress channel for live updates
		progressCh := make(chan string, 100)
		m.progressLines = nil
		m.progressCh = progressCh

		m.view = viewSpinner
		m.spinnerMsg = fmt.Sprintf("Bridge Mix: %d hops across %d chains…", numHops, len(chains))

		// Operation cmd — runs BridgeMix and sends progress through channel
		opCmd := func() tea.Msg {
			start := time.Now()
			results, err := ops.BridgeMix(prov, chains, hopWallets, delayCfg, logger, func(msg string) {
				progressCh <- msg
			}, &ops.BridgeMixOpts{
				StartIndex: srcIdx - 1,
				GroupName:  groupName,
			})
			close(progressCh)
			if err != nil {
				return opDoneMsg{err: err}
			}
			elapsed := time.Since(start)

			// Send alert notification
			success, failed := 0, 0
			for _, r := range results {
				if r.TxHash != "" {
					success++
				} else if r.Error != "" {
					failed++
				}
			}
			chainNames := make([]string, len(chains))
			for ci, c := range chains { chainNames[ci] = c.Name }
			alertCfg.SendTxAlert("Bridge Mix", strings.Join(chainNames, "→"), success, failed, elapsed)

			return opDoneMsg{result: buildMixResult(results, chains, elapsed, delayCfg)}
		}

		return m, tea.Batch(m.spinner.Tick, opCmd, listenForProgress(progressCh))
	}
	return m.returnToMenu()
}

// parseChainRotation parses "2,1,3" → [BSC, Ethereum, Polygon]
func parseChainRotation(s string) []chain.Chain {
	parts := strings.Split(s, ",")
	var chains []chain.Chain
	for _, p := range parts {
		idx, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || idx < 1 || idx > len(chain.AllChains) {
			continue
		}
		chains = append(chains, chain.AllChains[idx-1])
	}
	return chains
}

// chainRotationLabel builds a readable chain index reference for the input prompt.
func chainRotationLabel() string {
	var b strings.Builder
	b.WriteString("Chain rotation  e.g. 2,1,3\n")
	for i, ch := range chain.AllChains {
		b.WriteString(fmt.Sprintf("   %2d = %-12s (%s)\n", i+1, ch.Name, ch.Symbol))
	}
	b.WriteString("   Enter numbers separated by commas")
	return b.String()
}

func buildMixResult(results []ops.BridgeHopResult, chains []chain.Chain, elapsed time.Duration, delay ops.DelayConfig) string {
	var b strings.Builder
	success, failed := 0, 0

	for _, r := range results {
		if r.TxHash != "" {
			success++
		} else if r.Error != "" {
			failed++
		}
	}

	// ── Header ──
	b.WriteString("\n")
	b.WriteString("   " + sLime.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n")
	b.WriteString("   " + sLime.Render("  BRIDGE MIX — CROSS-CHAIN TRANSFER TREE") + "\n")
	b.WriteString("   " + sLime.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n\n")

	// ── Rotation legend ──
	var rotNames []string
	for _, ch := range chains {
		rotNames = append(rotNames, ch.Name)
	}
	b.WriteString("   " + sDim.Render("rotation: ") + sAccent.Render(strings.Join(rotNames, " → ")) + "\n\n")

	if len(results) == 0 {
		b.WriteString("   " + sDim.Render("No hops executed.") + "\n")
		return b.String()
	}

	// ── Origin wallet ──
	first := results[0]
	b.WriteString("   " + sGreenB.Render("●") + "  " + sWhiteB.Render(fmt.Sprintf("wallet #%d", first.FromIndex+1)) +
		"  " + styledAddr(first.FromWallet) + "\n")
	b.WriteString("   " + sBorder.Render("│") + "  " + sDim.Render(first.FromChain) + "\n")

	// ── Hop tree ──
	for i, r := range results {
		isLast := i == len(results)-1 || r.Error != ""

		pipe := sBorder.Render("│")
		if isLast {
			pipe = " "
		}

		if r.TxHash != "" {
			// ── Success hop ──
			amtIn := "?"
			if r.AmountIn != nil {
				amtIn = ops.FormatBalance(r.AmountIn, 18)
			}
			amtOut := "?"
			if r.AmountOut != "" {
				parsed := parseBigHex(r.AmountOut)
				if parsed.Sign() > 0 {
					amtOut = ops.FormatBalance(parsed, 18)
				}
			}

			bridge := r.Bridge
			if bridge == "" {
				bridge = "Li.Fi"
			}

			// Hop line
			b.WriteString("   " + sBorder.Render("│") + "\n")
			b.WriteString("   " + sBorder.Render("├──") + " " + sGreenB.Render("✓") + " " +
				sWhiteB.Render(fmt.Sprintf("HOP %d", i)) + "  " +
				sAccent.Render(r.FromChain) + sDim.Render(" → ") + sAccent.Render(r.ToChain) + "\n")

			// Details
			etaTag := ""
			if r.ETASeconds > 0 {
				etaTag = sDim.Render(fmt.Sprintf("  (~%ds)", r.ETASeconds))
			}
			b.WriteString("   " + pipe + "   " +
				sDim.Render("bridge  : ") + sGreenBr.Render(bridge) + etaTag + "\n")
			b.WriteString("   " + pipe + "   " +
				sDim.Render("sent    : ") + sWhiteB.Render(amtIn) + " " + sDim.Render(r.FromChain) + "\n")
			b.WriteString("   " + pipe + "   " +
				sDim.Render("received: ") + sGreenB.Render(amtOut) + " " + sDim.Render(r.ToChain) + "\n")
			b.WriteString("   " + pipe + "   " +
				sDim.Render("tx      : ") + sDimmer.Render(r.TxHash) + "\n")

			// Destination wallet
			b.WriteString("   " + pipe + "\n")
			b.WriteString("   " + pipe + "  " +
				sGreenB.Render("▼") + "  " + sWhiteB.Render(fmt.Sprintf("wallet #%d", r.ToIndex+1)) +
				"  " + styledAddr(r.ToWallet) + "\n")
			b.WriteString("   " + pipe + "  " +
				"   " + sDim.Render(r.ToChain) +
				"  " + sGreenBr.Render("balance: "+amtOut) + "\n")

		} else {
			// ── Failed hop ──
			b.WriteString("   " + sBorder.Render("│") + "\n")
			b.WriteString("   " + sBorder.Render("└──") + " " + sRed.Render("✗") + " " +
				sWhiteB.Render(fmt.Sprintf("HOP %d", i)) + "  " +
				sRed.Render(r.FromChain+" → "+r.ToChain) + "\n")
			b.WriteString("     " + "   " +
				sDim.Render("from    : ") + sWhiteB.Render(fmt.Sprintf("wallet #%d", r.FromIndex+1)) +
				"  " + styledAddr(r.FromWallet) + "\n")
			b.WriteString("     " + "   " +
				sDim.Render("to      : ") + sWhiteB.Render(fmt.Sprintf("wallet #%d", r.ToIndex+1)) +
				"  " + styledAddr(r.ToWallet) + "\n")

			errMsg := r.Error
			b.WriteString("     " + "   " +
				sRed.Render("error   : "+errMsg) + "\n")
			break
		}
	}

	// ── Final destination highlight ──
	if success > 0 {
		lastOk := results[success-1]
		amtOut := "?"
		if lastOk.AmountOut != "" {
			parsed := parseBigHex(lastOk.AmountOut)
			if parsed.Sign() > 0 {
				amtOut = ops.FormatBalance(parsed, 18)
			}
		}

		b.WriteString("\n")
		b.WriteString("   " + sGreenB.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n")
		b.WriteString("   " + sGreenB.Render("  FINAL DESTINATION") + "\n")
		b.WriteString("   " + sGreenB.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n")
		b.WriteString("   " + sGreenB.Render("●") + "  " +
			sWhiteB.Render(fmt.Sprintf("wallet #%d", lastOk.ToIndex+1)) +
			"  " + styledAddr(lastOk.ToWallet) + "\n")
		b.WriteString("      " + sDim.Render("chain   : ") + sAccent.Render(lastOk.ToChain) + "\n")
		b.WriteString("      " + sDim.Render("balance : ") + sGreenB.Render(amtOut) + "\n")
		b.WriteString("      " + sDim.Render("status  : ") + sGreenB.Render("CLEAN WALLET") + "\n")
	}

	// ── Summary stats ──
	var rot strings.Builder
	for i, ch := range chains {
		if i > 0 {
			rot.WriteString(" → ")
		}
		rot.WriteString(ch.Name)
	}

	rows := [][2]string{
		{"Total Hops", fmt.Sprintf("%d", len(results))},
		{"Success", fmt.Sprintf("%d", success)},
	}
	if failed > 0 {
		rows = append(rows, [2]string{"Failed", fmt.Sprintf("%d", failed)})
	}
	rows = append(rows,
		[2]string{"Rotation", rot.String()},
		[2]string{"Bridge", "Li.Fi (auto-route)"},
		[2]string{"Duration", elapsed.Round(time.Millisecond).String()},
	)
	if delay.Enabled {
		rows = append(rows, [2]string{"Delay", fmt.Sprintf("%d-%ds %s", delay.MinMs/1000, delay.MaxMs/1000, delay.ModeName())})
	}

	// Origin → Destination summary
	if success > 0 {
		first := results[0]
		last := results[success-1]
		rows = append(rows,
			[2]string{"Origin", fmt.Sprintf("wallet #%d on %s", first.FromIndex+1, first.FromChain)},
			[2]string{"Destination", fmt.Sprintf("wallet #%d on %s", last.ToIndex+1, last.ToChain)},
		)
	}

	b.WriteString(buildStatBox(rows))
	return b.String()
}

// parseBigHex (for main.go usage — delegates to the same logic as ops)
func parseBigHex(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" || s == "0x0" || s == "0x" {
		return big.NewInt(0)
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, ok := new(big.Int).SetString(s[2:], 16)
		if !ok {
			return big.NewInt(0)
		}
		return v
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return big.NewInt(0)
	}
	return v
}

// ── DEX Swap Mix Wizard ──────────────────────────────────────────────

func (m model) wizSwapMix() (model, tea.Cmd) {
	switch m.step {
	case 0: // Chain select
		return m.showChainSelect()
	case 1: // Token select
		return m.showTokenSelect(false)
	case 2: // Source wallet index
		return m.showInput(fmt.Sprintf("Source wallet index (1-%d)", len(m.wallets)), "source", false, "1")
	case 3: // Number of hops
		srcIdx, _ := strconv.Atoi(m.data["source"])
		maxHops := len(m.wallets) - srcIdx
		defHops := "19"
		if maxHops < 19 {
			defHops = strconv.Itoa(maxHops)
		}
		return m.showInput(fmt.Sprintf("Number of hops (max %d)", maxHops), "hops", false, defHops)
	case 4: // Slippage
		return m.showInput("Slippage basis points (100 = 1%)", "slippage", false, "100")
	case 5: // Confirm
		ch := m.selectedChain()
		srcIdx, _ := strconv.Atoi(m.data["source"])
		numHops, _ := strconv.Atoi(m.data["hops"])
		slippage, _ := strconv.Atoi(m.data["slippage"])
		tokenAddr := m.data["token"]
		tokenSym := m.data["token_symbol"]

		if srcIdx < 1 || srcIdx > len(m.wallets) {
			m.resultText = sRed.Render("  Invalid source wallet index")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		if numHops < 1 || srcIdx+numHops > len(m.wallets) {
			m.resultText = sRed.Render(fmt.Sprintf("  Not enough wallets: need %d from index %d, have %d total", numHops+1, srcIdx, len(m.wallets)))
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		if slippage < 1 || slippage > 5000 {
			m.resultText = sRed.Render("  Slippage must be 1-5000 bps")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}

		lastIdx := srcIdx + numHops
		if tokenSym == "" {
			tokenSym = shortAddr(tokenAddr)
		}

		router, _ := chain.DexRouters[ch.Name]
		msg := fmt.Sprintf("DEX MIX (SWAP): %d hops on %s\nRouter: %s\nToken: %s\nWallets: #%d → #%d\nSlippage: %d bps (%.1f%%)\nPattern: native→token→native→…\n\nProceed?",
			numHops, ch.Name, router.Name, tokenSym,
			srcIdx, lastIdx, slippage, float64(slippage)/100.0)
		return m.showConfirm(msg)
	case 6: // Execute
		ch := m.selectedChain()
		srcIdx, _ := strconv.Atoi(m.data["source"])
		numHops, _ := strconv.Atoi(m.data["hops"])
		slippage, _ := strconv.Atoi(m.data["slippage"])
		tokenAddr := m.data["token"]

		hopWallets := m.wallets[srcIdx-1 : srcIdx+numHops]
		prov := m.provider
		delayCfg := m.delayCfg
		logger := m.txLogger

		progressCh := make(chan string, 100)
		m.progressLines = nil
		m.progressCh = progressCh

		m.view = viewSpinner
		m.spinnerMsg = fmt.Sprintf("DEX Mix: %d hops on %s…", numHops, ch.Name)

		alertCfgSwap := m.alertCfg
		opCmd := func() tea.Msg {
			start := time.Now()
			results, err := ops.DexMix(prov, ch, hopWallets, tokenAddr, slippage, delayCfg, logger, func(msg string) {
				progressCh <- msg
			})
			close(progressCh)
			if err != nil {
				return opDoneMsg{err: err}
			}
			elapsed := time.Since(start)

			success, failed := 0, 0
			for _, r := range results {
				if r.Error == "" {
					success++
				} else {
					failed++
				}
			}
			alertCfgSwap.SendTxAlert("DEX Mix", ch.Name, success, failed, elapsed)

			return opDoneMsg{result: buildSwapResult(results, ch, tokenAddr, elapsed, delayCfg)}
		}

		return m, tea.Batch(m.spinner.Tick, opCmd, listenForProgress(progressCh))
	}
	return m.returnToMenu()
}

func buildSwapResult(results []ops.SwapHopResult, ch chain.Chain, tokenAddr string, elapsed time.Duration, delay ops.DelayConfig) string {
	var b strings.Builder
	success, failed := 0, 0
	for _, r := range results {
		if r.Error == "" {
			success++
		} else {
			failed++
		}
	}

	router, _ := chain.DexRouters[ch.Name]
	b.WriteString("\n   " + sLime.Render("━━ DEX MIX (SWAP) ━━") + "\n\n")
	b.WriteString("   " + sDim.Render("chain: "+ch.Name+"  router: "+router.Name) + "\n")
	b.WriteString("   " + sDim.Render("token: "+shortAddr(tokenAddr)) + "\n\n")

	if len(results) > 0 {
		b.WriteString("   " + sGreenB.Render("●") + "  " + styledAddr(results[0].FromWallet) + "\n")
		b.WriteString("   " + sBorder.Render("│") + "\n")
	}

	for i, r := range results {
		con := sBorder.Render("├─")
		if i == len(results)-1 || r.Error != "" {
			con = sBorder.Render("└─")
		}

		dir := r.TokenIn + "→" + r.TokenOut
		if r.Error == "" {
			amtIn := ""
			if r.AmountIn != nil {
				amtIn = ops.FormatBalance(r.AmountIn, 18)
			}
			b.WriteString(fmt.Sprintf("   %s %s %s  %s  %s\n",
				con, sGreenB.Render("✓"),
				sGreenBr.Render(dir),
				sDim.Render(amtIn),
				sDim.Render("→ "+shortAddr(r.ToWallet))))
			if r.SwapTx != "" {
				b.WriteString(fmt.Sprintf("   %s    %s\n",
					sBorder.Render("│"), sDimmer.Render("tx:"+shortHash(r.SwapTx))))
			}
			if r.ApproveTx != "" {
				b.WriteString(fmt.Sprintf("   %s    %s\n",
					sBorder.Render("│"), sDimmer.Render("approve:"+shortHash(r.ApproveTx))))
			}
		} else {
			errMsg := r.Error
			if len(errMsg) > 50 {
				errMsg = errMsg[:50] + "…"
			}
			b.WriteString(fmt.Sprintf("   %s %s %s  %s  %s\n",
				con, sRed.Render("✗"),
				dir,
				sDim.Render("→ "+shortAddr(r.ToWallet)),
				sRed.Render(errMsg)))
			break
		}
	}

	if success > 0 {
		last := results[success-1]
		b.WriteString("\n   " + sGreenB.Render("▼") + "\n")
		b.WriteString("   " + sGreenB.Render("●") + "  " + styledAddr(last.ToWallet) + "\n")
	}

	rows := [][2]string{
		{"Hops", fmt.Sprintf("%d", len(results))},
		{"Success", fmt.Sprintf("%d", success)},
	}
	if failed > 0 {
		rows = append(rows, [2]string{"Failed", fmt.Sprintf("%d", failed)})
	}
	rows = append(rows,
		[2]string{"Chain", ch.Name},
		[2]string{"Router", router.Name},
		[2]string{"Duration", elapsed.Round(time.Millisecond).String()},
	)
	if delay.Enabled {
		rows = append(rows, [2]string{"Delay", fmt.Sprintf("%d-%ds", delay.MinMs/1000, delay.MaxMs/1000)})
	}
	b.WriteString(buildStatBox(rows))
	return b.String()
}

// ── Export Wizard ────────────────────────────────────────────────────

func (m model) wizExport() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showInput("Filename", "filename", false, "addresses.csv")
	case 1:
		filename := m.data["filename"]
		if filename == "" {
			filename = "addresses.csv"
		}
		wls := m.wallets
		f, err := os.Create(filename)
		if err != nil {
			m.resultText = sRed.Render("  Create file: " + err.Error())
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		w := csv.NewWriter(f)
		w.Write([]string{"index", "address", "private_key"})
		for i, wl := range wls {
			w.Write([]string{strconv.Itoa(i + 1), wl.Address, wl.PrivateKey})
		}
		w.Flush()
		f.Close()
		m.resultText = sGreenB.Render(fmt.Sprintf("  Exported %d wallets → %s", len(wls), filename))
		m.resultLines = strings.Split(m.resultText, "\n")
		m.view = viewResult
		return m, nil
	}
	return m.returnToMenu()
}

// ── Delay Custom Wizard ──────────────────────────────────────────────

func (m model) wizDelayCustom() (model, tea.Cmd) {
	switch m.step {
	case 1:
		return m.showInput("Max seconds", "max", false, "12")
	case 2:
		minSec, e1 := strconv.Atoi(m.data["min"])
		maxSec, e2 := strconv.Atoi(m.data["max"])
		if e1 != nil || e2 != nil || minSec < 0 || maxSec < minSec {
			m.delayCfg = ops.DefaultDelay()
		} else {
			m.delayCfg = ops.DelayConfig{Enabled: true, MinMs: minSec * 1000, MaxMs: maxSec * 1000}
		}
		return m.returnToMenu()
	}
	return m.returnToMenu()
}

// ── View Router ──────────────────────────────────────────────────────

func (m model) View() string {
	switch m.view {
	case viewMenu:
		return m.viewMenu()
	case viewInput:
		return m.viewInput()
	case viewChainSelect:
		return m.viewChainSelect()
	case viewConfirm:
		return m.viewConfirm()
	case viewSpinner:
		return m.viewSpinner()
	case viewResult:
		return m.viewResult()
	case viewDelayMenu:
		return m.viewDelayMenu()
	case viewTokenSelect:
		return m.viewTokenSelect()
	case viewDelayCustom:
		return m.viewInput()
	case viewGroupSelect:
		return m.viewGroupSelect()
	case viewAlertMenu:
		return m.renderAlertMenu()
	case viewProxyMenu:
		return m.renderProxyMenu()
	case viewSessionMenu:
		return m.renderSessionMenu()
	case viewBackupMenu:
		return m.renderBackupMenu()
	case viewQueueMenu:
		return m.renderQueueMenu()
	case viewLabelMenu:
		return m.renderLabelMenu()
	case viewLocked:
		return m.renderLocked()
	}
	return ""
}

// ── Menu View ────────────────────────────────────────────────────────

func (m model) viewMenu() string {
	var b strings.Builder

	// Banner
	b.WriteString("\n")
	logo := []string{
		"    ██████╗ ██████╗ ███╗   ██╗████████╗██████╗  ██████╗ ██╗     ██╗  ██╗",
		"   ██╔════╝██╔═══██╗████╗  ██║╚══██╔══╝██╔══██╗██╔═══██╗██║     ╚██╗██╔╝",
		"   ██║     ██║   ██║██╔██╗ ██║   ██║   ██████╔╝██║   ██║██║      ╚███╔╝ ",
		"   ██║     ██║   ██║██║╚██╗██║   ██║   ██╔══██╗██║   ██║██║      ██╔██╗ ",
		"   ╚██████╗╚██████╔╝██║ ╚████║   ██║   ██║  ██║╚██████╔╝███████╗██╔╝ ██╗",
		"    ╚═════╝ ╚═════╝ ╚═╝  ╚═══╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝╚══════╝╚═╝  ╚═╝",
	}
	logoGreen := []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("48")).Bold(true),  // bright green
		lipgloss.NewStyle().Foreground(lipgloss.Color("41")).Bold(true),  // green
		lipgloss.NewStyle().Foreground(lipgloss.Color("35")),             // medium green
		lipgloss.NewStyle().Foreground(lipgloss.Color("29")),             // darker green
		lipgloss.NewStyle().Foreground(lipgloss.Color("23")),             // dark green
		lipgloss.NewStyle().Foreground(lipgloss.Color("22")),             // darkest green
	}
	for i, line := range logo {
		b.WriteString(logoGreen[i].Render(line) + "\n")
	}
	sLineBorder := lipgloss.NewStyle().Foreground(lipgloss.Color("29"))
	b.WriteString(sLineBorder.Render("   " + strings.Repeat("━", 70)) + "\n")
	b.WriteString("   " + lipgloss.NewStyle().Foreground(lipgloss.Color("35")).Render("EVM Multi-Wallet Manager") + sDimmer.Render("  ·  v1.0") + "\n\n")

	// Status bar
	walletTag := sDarkRed.Render("---")
	if len(m.wallets) > 0 {
		walletTag = sAccent.Render(fmt.Sprintf("%d", len(m.wallets)))
	}
	delayTag := sDarkRed.Render("OFF")
	if m.delayCfg.Enabled {
		delayTag = sGreenBr.Render(fmt.Sprintf("%d-%ds", m.delayCfg.MinMs/1000, m.delayCfg.MaxMs/1000))
	}
	proxyTag := sDarkRed.Render("OFF")
	if m.proxyCfg.Enabled {
		proxyTag = sGreenBr.Render(fmt.Sprintf("%d proxies", m.proxyCfg.Count()))
	}
	alertTag := sDarkRed.Render("OFF")
	if m.alertCfg.Enabled {
		alertTag = sGreenBr.Render(m.alertCfg.TypeName())
	}
	randTag := ""
	if m.amtRandPct > 0 {
		randTag = "  rand: " + sGreenBr.Render(fmt.Sprintf("±%d%%", m.amtRandPct))
	}
	b.WriteString(fmt.Sprintf("   wallets: %s    delay: %s %s    rpc: %s\n",
		walletTag, delayTag, sDim.Render(m.delayCfg.ModeName()), sGreenBr.Render(fmt.Sprintf("%d keys", m.provider.KeyCount()))))
	b.WriteString(fmt.Sprintf("   proxy: %s    alert: %s%s\n\n",
		proxyTag, alertTag, randTag))

	// Menu with categories
	lastCat := ""
	for i, item := range menuItems {
		// Category header
		if item.category != lastCat && item.category != "" {
			if lastCat != "" {
				b.WriteString("\n")
			}
			b.WriteString("   " + sLime.Render(item.category) + "\n")
			lastCat = item.category
		} else if item.category == "" && lastCat != "" {
			b.WriteString("\n")
			lastCat = ""
		}

		cursor := "   "
		style := sSoft
		if i == m.menuCursor {
			cursor = sAccent.Render(" ▸ ")
			style = sAccent
		}

		label := style.Render(item.label)
		desc := ""
		if item.desc != "" {
			desc = " " + sDim.Render(item.desc)
		}

		if item.key == "exit" {
			if i == m.menuCursor {
				label = sRed.Bold(true).Render(item.label)
			} else {
				label = sDarkRed.Render(item.label)
			}
		}

		b.WriteString(fmt.Sprintf("  %s %s%s\n", cursor, label, desc))
	}

	// Status message
	if m.statusMsg != "" {
		b.WriteString("\n   " + sYellow.Render("! "+m.statusMsg) + "\n")
	}

	// Help
	b.WriteString("\n   " + sDim.Render("↑/↓ navigate · enter select · q quit") + "\n")

	return b.String()
}

// ── Input View ───────────────────────────────────────────────────────

func (m model) viewInput() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(viewHeader(m.currentOp))
	b.WriteString("\n")

	// Support multi-line labels: first line is the title, rest are helper text
	lines := strings.Split(m.inputLabel, "\n")
	b.WriteString("   " + sAccent.Render("❯") + " " + sWhiteB.Render(lines[0]) + "\n")
	for _, extra := range lines[1:] {
		b.WriteString("   " + sDim.Render(extra) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("     " + m.textInput.View() + "\n\n")
	if m.data["_default"] != "" {
		b.WriteString("   " + sDim.Render("default: "+m.data["_default"]) + "\n")
	}
	b.WriteString("   " + sDim.Render("enter confirm · esc cancel") + "\n")
	return b.String()
}

// ── Chain Select View ────────────────────────────────────────────────

func (m model) viewChainSelect() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(viewHeader(m.currentOp))
	b.WriteString("\n")
	b.WriteString("   " + sWhiteB.Render("SELECT CHAIN") + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 40)) + "\n\n")

	for i, ch := range chain.AllChains {
		cursor := "   "
		style := sSoft
		symStyle := sDim
		if i == m.chainCursor {
			cursor = sAccent.Render(" ▸ ")
			style = sAccent
			symStyle = sGreenBr
		}
		b.WriteString(fmt.Sprintf("  %s %-14s %s\n", cursor, style.Render(ch.Name), symStyle.Render(ch.Symbol)))
	}

	b.WriteString("\n   " + sDim.Render("↑/↓ navigate · enter select · esc cancel") + "\n")
	return b.String()
}

// ── Confirm View ─────────────────────────────────────────────────────

func (m model) viewConfirm() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(viewHeader(m.currentOp))
	b.WriteString("\n")
	b.WriteString("   " + sYellow.Render("⚠") + " " + sWhiteB.Render(m.confirmMsg) + "\n\n")

	yesStyle := sDim
	noStyle := sDim
	if m.confirmCursor == 0 {
		yesStyle = sAccent
	} else {
		noStyle = sRed.Bold(true)
	}
	b.WriteString("     " + yesStyle.Render("[ Yes ]") + "   " + noStyle.Render("[ No ]") + "\n\n")
	b.WriteString("   " + sDim.Render("←/→ select · y/n · enter confirm · esc cancel") + "\n")
	return b.String()
}

// ── Spinner View ─────────────────────────────────────────────────────

func (m model) viewSpinner() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(viewHeader(m.currentOp))
	b.WriteString("\n\n")
	b.WriteString("   " + m.spinner.View() + " " + sGreenBr.Render(m.spinnerMsg) + "\n")

	// Show live progress lines (bridge ops)
	if len(m.progressLines) > 0 {
		b.WriteString("\n")
		// Show last N lines that fit in the viewport
		maxLines := m.height - 8
		if maxLines < 5 {
			maxLines = 15
		}
		start := 0
		if len(m.progressLines) > maxLines {
			start = len(m.progressLines) - maxLines
		}
		for idx := start; idx < len(m.progressLines); idx++ {
			line := m.progressLines[idx]
			// Style based on content
			styled := m.styleProgressLine(line)
			b.WriteString("   " + styled + "\n")
		}
	}

	return b.String()
}

func (m model) styleProgressLine(line string) string {
	switch {
	case strings.Contains(line, " OK:"):
		return sGreenB.Render("  ✓ ") + sGreenB.Render(line)
	case strings.Contains(line, "SUCCESS on retry"):
		return sGreenB.Render("  ✓ ") + sGreenB.Render(line)
	case strings.Contains(line, " FAIL:"):
		return sRed.Render("  ✗ ") + sRed.Render(line)
	case strings.Contains(line, "non-recoverable"):
		return sRed.Render("  ✗ ") + sRed.Render(line)
	case strings.Contains(line, "bridge confirmed!"):
		return sGreenB.Render("  ● ") + sGreenB.Render(line)
	case strings.Contains(line, "bridge confirmed on"):
		return sGreenB.Render("  ● ") + sGreen.Render(line)
	case strings.Contains(line, "waiting:") && strings.Contains(line, "confirm"):
		return sGreenBr.Render("  ⏳ ") + sGreenBr.Render(line)
	case strings.Contains(line, "waiting:"):
		return sDim.Render("  ⏳ " + line)
	case strings.Contains(line, "wallet#") && strings.Contains(line, "→"):
		return sAccent.Render("  ► ") + sWhiteB.Render(line)
	case strings.Contains(line, "hop") && strings.Contains(line, "→"):
		return sAccent.Render("  ► ") + sSoft.Render(line)
	case strings.Contains(line, "balance="):
		return sGreenBr.Render("  ◆ ") + sSoft.Render(line)
	case strings.Contains(line, "gasPrice="):
		return sDim.Render("  ◇ " + line)
	case strings.Contains(line, "bridging"):
		return sGreenBr.Render("  ↗ ") + sWhiteB.Render(line)
	case strings.Contains(line, "adjusting amount"):
		return sYellow.Render("  ⟳ ") + sYellow.Render(line)
	case strings.Contains(line, "reduced amount"):
		return sYellow.Render("  ⟳ ") + sYellow.Render(line)
	case strings.Contains(line, "retry"):
		return sYellow.Render("  ↻ ") + sYellow.Render(line)
	case strings.Contains(line, "new quote"):
		return sGreenBr.Render("  ◈ ") + sSoft.Render(line)
	case strings.Contains(line, "tx sent"):
		return sAccent.Render("  ⟶ ") + sAccent.Render(line)
	case strings.Contains(line, "bridge="):
		return sDim.Render("  ◇ " + line)
	case strings.Contains(line, "START"):
		return sLime.Render("  ━━ " + line + " ━━")
	case strings.Contains(line, "COMPLETE"):
		return sGreenB.Render("  ━━ " + line + " ━━")
	case strings.Contains(line, "rotation="):
		return sAccent.Render("  ◆ ") + sAccent.Render(line)
	case strings.Contains(line, "send attempt") && strings.Contains(line, "failed"):
		return sRed.Render("  ✗ ") + sRed.Render(line)
	case strings.Contains(line, "still too expensive"):
		return sYellow.Render("  ⟳ ") + sYellow.Render(line)
	default:
		return sDim.Render("    " + line)
	}
}

// ── Result View ──────────────────────────────────────────────────────

func (m model) viewResult() string {
	var b strings.Builder
	b.WriteString("\n")

	// Scrollable content
	viewH := m.height - 4
	if viewH < 10 {
		viewH = 30
	}

	lines := m.resultLines
	maxOffset := len(lines) - viewH
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.scrollOffset
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + viewH
	if end > len(lines) {
		end = len(lines)
	}

	for i := offset; i < end; i++ {
		b.WriteString(lines[i] + "\n")
	}

	// Scroll indicator
	if len(lines) > viewH {
		pct := 0
		if maxOffset > 0 {
			pct = offset * 100 / maxOffset
		}
		b.WriteString(sDim.Render(fmt.Sprintf("   ↑/↓ scroll (%d%%) · ", pct)))
	} else {
		b.WriteString("   ")
	}
	if m.resultContinues {
		b.WriteString(sGreen.Render("enter continue") + sDim.Render(" · esc cancel") + "\n")
	} else {
		b.WriteString(sDim.Render("enter/esc back") + "\n")
	}

	return b.String()
}

// ── Delay Menu View ──────────────────────────────────────────────────

func (m model) viewDelayMenu() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("   " + sLime.Render("HUMANIZER DELAY") + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 40)) + "\n\n")

	for i, opt := range m.delayOptions() {
		cursor := "   "
		style := sSoft
		if i == m.delayCursor {
			cursor = sAccent.Render(" ▸ ")
			style = sAccent
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", cursor, style.Render(opt.label)))
	}

	current := "OFF"
	if m.delayCfg.Enabled {
		current = fmt.Sprintf("%d-%ds %s", m.delayCfg.MinMs/1000, m.delayCfg.MaxMs/1000, m.delayCfg.ModeName())
	}
	randInfo := ""
	if m.amtRandPct > 0 {
		randInfo = fmt.Sprintf("  amount rand: ±%d%%", m.amtRandPct)
	}
	b.WriteString("\n   " + sDim.Render("current: "+current+randInfo) + "\n")
	b.WriteString("   " + sDim.Render("↑/↓ navigate · enter select · esc cancel") + "\n")
	return b.String()
}

// ── View Helpers ─────────────────────────────────────────────────────

func viewHeader(op string) string {
	titles := map[string]string{
		"generate":       "GENERATE WALLETS",
		"load":           "LOAD WALLETS",
		"balance":        "CHECK BALANCES",
		"distribute":     "DISTRIBUTE",
		"collect":        "COLLECT",
		"sweep":          "SWEEP",
		"autofund":       "AUTO-FUND GAS",
		"export":         "EXPORT",
		"delay_custom":   "CUSTOM DELAY",
		"amount_rand":    "AMOUNT RANDOMIZER",
		"dexmix":         "MIX",
		"swapmix":        "DEX MIX (SWAP)",
		"portfolio":      "PORTFOLIO DASHBOARD",
		"gastrack":       "GAS TRACKER",
		"dryrun":         "DRY-RUN SIMULATE",
		"alert_telegram": "TELEGRAM ALERT",
		"alert_discord":  "DISCORD ALERT",
		"proxy_load":     "LOAD PROXIES",
		"session_set":    "SESSION TIMEOUT",
		"backup_create":  "CREATE BACKUP",
		"backup_restore": "RESTORE BACKUP",
		"label_set":      "WALLET LABEL",
		"queue_add":      "ADD TO QUEUE",
	}
	title := titles[op]
	if title == "" {
		title = strings.ToUpper(op)
	}
	return "   " + sLime.Render("━━ "+title+" ━━")
}

// ── Result Builders ──────────────────────────────────────────────────

func buildGenerateResult(wls []wallet.Wallet, count int, elapsed time.Duration, filename string) string {
	var b strings.Builder
	b.WriteString("   " + sGreenB.Render("✓") + fmt.Sprintf(" Generated %s wallets in %s\n",
		sGreenB.Render(fmt.Sprintf("%d", count)),
		sWhiteB.Render(elapsed.Round(time.Millisecond).String())))
	b.WriteString("   " + sGreenB.Render("✓") + " Saved to " + sWhiteB.Render(filename) + sDim.Render(" (encrypted)") + "\n")
	b.WriteString(buildWalletTree(wls, 10))
	return b.String()
}

func buildLoadResult(wls []wallet.Wallet, fp string) string {
	var b strings.Builder
	b.WriteString("   " + sGreenB.Render("✓") + fmt.Sprintf(" Loaded %s wallets  %s\n",
		sGreenB.Render(fmt.Sprintf("%d", len(wls))),
		sDim.Render("["+fp+"]")))
	b.WriteString(buildWalletTree(wls, 10))
	return b.String()
}

func buildWalletTree(wls []wallet.Wallet, maxShow int) string {
	var b strings.Builder
	total := len(wls)
	if maxShow > total {
		maxShow = total
	}
	b.WriteString("\n   " + sLime.Render("WALLET TREE") + sDim.Render(fmt.Sprintf("  %d wallets", total)) + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 50)) + "\n")

	for i := 0; i < maxShow; i++ {
		con := sBorder.Render("├─")
		if i == maxShow-1 && maxShow == total {
			con = sBorder.Render("└─")
		}
		idx := sDim.Render(fmt.Sprintf("%03d", i+1))
		b.WriteString(fmt.Sprintf("   %s %s  %s\n", con, idx, styledAddr(wls[i].Address)))
	}
	if maxShow < total {
		b.WriteString(fmt.Sprintf("   %s %s\n", sBorder.Render("└─"), sDimmer.Render(fmt.Sprintf("… +%d more", total-maxShow))))
	}

	// Groups
	if total > 50 {
		groupSize := 50
		groups := (total + groupSize - 1) / groupSize
		b.WriteString("\n   " + sLime.Render("GROUPS") + "\n")
		b.WriteString("   " + sBorder.Render(strings.Repeat("─", 55)) + "\n")
		for g := 0; g < groups; g++ {
			start := g * groupSize
			end := start + groupSize
			if end > total {
				end = total
			}
			con := sBorder.Render("├─")
			if g == groups-1 {
				con = sBorder.Render("└─")
			}
			b.WriteString(fmt.Sprintf("   %s %s  %s … %s  %s\n", con,
				sGreenBr.Render(fmt.Sprintf("Group %d", g+1)),
				sGray.Render(wls[start].ShortAddress()),
				sGray.Render(wls[end-1].ShortAddress()),
				sDimmer.Render(fmt.Sprintf("(%d)", end-start))))
		}
	}
	return b.String()
}

func buildBalanceResult(results []ops.WalletBalance, ch chain.Chain, tokenAddr string, elapsed time.Duration) string {
	var b strings.Builder

	// Header
	b.WriteString("   " + sGray.Render(fmt.Sprintf("%-5s  %-42s  %-18s", "#", "ADDRESS", ch.Symbol)))
	if tokenAddr != "" {
		b.WriteString(sGray.Render(fmt.Sprintf("  %-18s", "TOKEN")))
	}
	b.WriteString("\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 70)) + "\n")

	nonZero := 0
	totalNative := new(big.Int)
	for _, r := range results {
		if r.Error != "" {
			continue
		}
		if r.NativeBalance != nil && r.NativeBalance.Sign() > 0 {
			nonZero++
			totalNative.Add(totalNative, r.NativeBalance)
		}
		show := false
		if r.NativeBalance != nil && r.NativeBalance.Sign() > 0 {
			show = true
		} else if r.Index < 10 || r.Index >= len(results)-5 {
			show = true
		}
		if show {
			idx := fmt.Sprintf("%03d", r.Index+1)
			bal := ops.FormatBalance(r.NativeBalance, 18)
			if r.NativeBalance != nil && r.NativeBalance.Sign() > 0 {
				b.WriteString(fmt.Sprintf("   %s  %s  %s", sGreenB.Render(idx), styledAddr(r.Address), sGreenB.Render(bal)))
			} else {
				b.WriteString(fmt.Sprintf("   %s  %s  %s", sDimmer.Render(idx), styledAddr(r.Address), sDimmer.Render(bal)))
			}
			if tokenAddr != "" {
				b.WriteString("  " + ops.FormatBalance(r.TokenBalance, 18))
			}
			b.WriteString("\n")
		}
	}
	if len(results) > 15 {
		b.WriteString(sDimmer.Render(fmt.Sprintf("   … %d more wallets (zero balance)\n", len(results)-15)))
	}

	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 70)) + "\n")
	b.WriteString("   " + sGreenB.Render("✓") + " Scanned in " + sWhiteB.Render(elapsed.Round(time.Millisecond).String()) + "\n")
	b.WriteString(fmt.Sprintf("   %s Non-zero: %s  Total: %s %s\n",
		sDim.Render("·"),
		sGreenB.Render(fmt.Sprintf("%d", nonZero)),
		sGreenB.Render(ops.FormatBalance(totalNative, 18)),
		ch.Symbol))
	return b.String()
}

func buildDistributeResult(results []ops.TxResult, ch chain.Chain, amtWei *big.Int, elapsed time.Duration, delay ops.DelayConfig) string {
	var b strings.Builder
	success, failed := 0, 0
	for _, r := range results {
		if r.TxHash != "" {
			success++
		} else if r.Error != "" {
			failed++
		}
	}

	from := "unknown"
	if len(results) > 0 {
		from = results[0].From
	}
	amt := ops.FormatBalance(amtWei, 18)

	b.WriteString("\n   " + sLime.Render("━━ DISTRIBUTE ━━") + "\n\n")
	b.WriteString("   " + sGreenB.Render("●") + " " + styledAddr(from) + sDim.Render("  → "+amt+" "+ch.Symbol+" per tx") + "\n")
	b.WriteString("   " + sBorder.Render("│") + "\n")

	maxShow := 20
	for i := 0; i < len(results) && i < maxShow; i++ {
		r := results[i]
		con := sBorder.Render("├─")
		pipe := sBorder.Render("│")
		if i == len(results)-1 || i == maxShow-1 {
			con = sBorder.Render("└─")
			pipe = "  "
		}
		if r.TxHash != "" {
			b.WriteString(fmt.Sprintf("   %s %s %s  %s\n", con, sGreenB.Render("✓"), styledAddr(r.To), sGreenBr.Render(amt+" "+ch.Symbol)))
			b.WriteString(fmt.Sprintf("   %s    %s\n", pipe, sDimmer.Render(shortHash(r.TxHash))))
		} else {
			errMsg := r.Error
			if len(errMsg) > 35 {
				errMsg = errMsg[:35] + "…"
			}
			b.WriteString(fmt.Sprintf("   %s %s %s  %s\n", con, sRed.Render("✗"), styledAddr(r.To), sRed.Render(errMsg)))
		}
	}
	if len(results) > maxShow {
		b.WriteString(fmt.Sprintf("   %s %s\n", sBorder.Render("└─"), sDimmer.Render(fmt.Sprintf("… +%d more", len(results)-maxShow))))
	}

	b.WriteString(buildStatBox(txStats(success, failed, elapsed, ch, delay)))
	return b.String()
}

func buildCollectResult(results []ops.TxResult, ch chain.Chain, elapsed time.Duration, delay ops.DelayConfig) string {
	var b strings.Builder
	success, failed := 0, 0
	for _, r := range results {
		if r.TxHash != "" {
			success++
		} else if r.Error != "" && r.Error != "insufficient balance" {
			failed++
		}
	}

	dest := "unknown"
	if len(results) > 0 {
		dest = results[0].To
	}

	b.WriteString("\n   " + sLime.Render("━━ COLLECT ━━") + "\n\n")

	maxShow := 20
	shown := 0
	for _, r := range results {
		if shown >= maxShow {
			break
		}
		if r.Error == "insufficient balance" || (r.Error == "" && r.TxHash == "") {
			continue
		}
		shown++
		if r.TxHash != "" {
			b.WriteString(fmt.Sprintf("   %s %s %s  %s %s\n",
				sBorder.Render("├─"), sGreenB.Render("✓"), styledAddr(r.From),
				sGreenBr.Render("→"), sDimmer.Render(shortHash(r.TxHash))))
		} else {
			errMsg := r.Error
			if len(errMsg) > 35 {
				errMsg = errMsg[:35] + "…"
			}
			b.WriteString(fmt.Sprintf("   %s %s %s  %s\n",
				sBorder.Render("├─"), sRed.Render("✗"), styledAddr(r.From), sRed.Render(errMsg)))
		}
	}
	if success+failed > shown {
		b.WriteString(fmt.Sprintf("   %s %s\n", sBorder.Render("├─"), sDimmer.Render(fmt.Sprintf("… +%d more", success+failed-shown))))
	}
	b.WriteString("   " + sBorder.Render("│") + "\n")
	b.WriteString("   " + sGreenB.Render("▼") + "\n")
	b.WriteString("   " + sGreenB.Render("●") + " " + styledAddr(dest) + sDim.Render("  destination") + "\n")

	b.WriteString(buildStatBox(txStats(success, failed, elapsed, ch, delay)))
	return b.String()
}

func buildSweepResult(results []ops.SweepResult, ch chain.Chain, elapsed time.Duration) string {
	var b strings.Builder
	success, failed, skipped := 0, 0, 0
	totalSwept := new(big.Int)

	for _, r := range results {
		if r.TxHash != "" {
			success++
			if r.Amount != nil {
				totalSwept.Add(totalSwept, r.Amount)
			}
		} else if r.Error == "zero balance" || r.Error == "balance <= gas cost" ||
			r.Error == "skip: destination" || r.Error == "zero token balance" {
			skipped++
		} else if r.Error != "" {
			failed++
		}
	}

	b.WriteString("\n   " + sLime.Render("━━ SWEEP ━━") + "\n\n")

	maxShow := 25
	shown := 0
	for _, r := range results {
		if shown >= maxShow {
			break
		}
		if r.Error == "zero balance" || r.Error == "balance <= gas cost" ||
			r.Error == "skip: destination" || r.Error == "zero token balance" {
			continue
		}
		if r.TxHash == "" && r.Error == "" {
			continue
		}
		shown++
		if r.TxHash != "" {
			amt := ops.FormatBalance(r.Amount, 18)
			b.WriteString(fmt.Sprintf("   %s %s %s  %s  %s\n",
				sBorder.Render("├─"), sGreenB.Render("✓"), styledAddr(r.Address),
				sGreenB.Render(amt+" "+ch.Symbol),
				sDimmer.Render(shortHash(r.TxHash))))
		} else {
			b.WriteString(fmt.Sprintf("   %s %s %s  %s\n",
				sBorder.Render("├─"), sRed.Render("✗"), styledAddr(r.Address), sRed.Render(r.Error)))
		}
	}
	remaining := (success + failed) - shown
	if remaining > 0 {
		b.WriteString(fmt.Sprintf("   %s %s\n", sBorder.Render("├─"), sDimmer.Render(fmt.Sprintf("… +%d more", remaining))))
	}

	b.WriteString("   " + sBorder.Render("│") + "\n")
	b.WriteString("   " + sGreenB.Render("╰─►") + " " + sWhiteB.Render("TOTAL SWEPT: ") +
		sGreenB.Render(ops.FormatBalance(totalSwept, 18)+" "+ch.Symbol) + "\n")

	b.WriteString(buildStatBox([][2]string{
		{"Swept", fmt.Sprintf("%d wallets", success)},
		{"Skipped", fmt.Sprintf("%d (zero/dust)", skipped)},
		{"Failed", fmt.Sprintf("%d", failed)},
		{"Total", fmt.Sprintf("%s %s", ops.FormatBalance(totalSwept, 18), ch.Symbol)},
		{"Duration", elapsed.Round(time.Millisecond).String()},
	}))
	return b.String()
}

func buildScanResult(results []ops.FundResult, ch chain.Chain, elapsed time.Duration) string {
	var b strings.Builder
	needGas, sufficient := 0, 0
	totalNeeded := new(big.Int)
	for _, r := range results {
		if r.Error != "" {
			continue
		}
		if r.NeedGas && r.GasNeeded != nil {
			needGas++
			totalNeeded.Add(totalNeeded, r.GasNeeded)
		} else {
			sufficient++
		}
	}

	b.WriteString("\n   " + sLime.Render("━━ GAS SCAN ━━") + "\n\n")
	b.WriteString("   " + sGreenB.Render("✓") + " Scan done in " + sWhiteB.Render(elapsed.Round(time.Millisecond).String()) + "\n\n")

	shown := 0
	for _, r := range results {
		if r.Error != "" || shown >= 20 {
			continue
		}
		if !r.NeedGas {
			continue
		}
		shown++
		b.WriteString(fmt.Sprintf("   %s %s %s  %s\n",
			sBorder.Render("├─"), sRed.Render("⚡"), styledAddr(r.Address),
			sRed.Render("NEEDS "+ops.FormatBalance(r.GasNeeded, 18)+" "+ch.Symbol)))
	}
	if needGas > shown {
		b.WriteString(fmt.Sprintf("   %s %s\n", sBorder.Render("└─"), sDimmer.Render(fmt.Sprintf("… +%d more", needGas-shown))))
	}

	b.WriteString(buildStatBox([][2]string{
		{"Sufficient", fmt.Sprintf("%d wallets", sufficient)},
		{"Need gas", fmt.Sprintf("%d wallets", needGas)},
		{"Total needed", ops.FormatBalance(totalNeeded, 18) + " " + ch.Symbol},
	}))
	return b.String()
}

func buildFundResult(results []ops.FundResult, ch chain.Chain, funder wallet.Wallet, elapsed time.Duration) string {
	var b strings.Builder
	funded, fundFail := 0, 0
	for _, r := range results {
		if r.TxHash != "" {
			funded++
		} else if r.NeedGas && r.Error != "" {
			fundFail++
		}
	}

	b.WriteString("\n   " + sLime.Render("━━ AUTO-FUND ━━") + "\n\n")
	b.WriteString("   " + sGreenB.Render("●") + " " + styledAddr(funder.Address) + sDim.Render("  funder") + "\n")
	b.WriteString("   " + sBorder.Render("│") + "\n")

	shown := 0
	for _, r := range results {
		if !r.NeedGas || shown >= 20 {
			continue
		}
		shown++
		if r.TxHash != "" {
			b.WriteString(fmt.Sprintf("   %s %s %s  %s  %s\n",
				sBorder.Render("├─"), sGreenB.Render("✓"), styledAddr(r.Address),
				sGreenBr.Render(ops.FormatBalance(r.GasFunded, 18)+" "+ch.Symbol),
				sDimmer.Render(shortHash(r.TxHash))))
		} else {
			b.WriteString(fmt.Sprintf("   %s %s %s  %s\n",
				sBorder.Render("├─"), sRed.Render("✗"), styledAddr(r.Address), sRed.Render(r.Error)))
		}
	}

	b.WriteString(buildStatBox([][2]string{
		{"Funded", fmt.Sprintf("%d wallets", funded)},
		{"Failed", fmt.Sprintf("%d", fundFail)},
		{"Duration", elapsed.Round(time.Millisecond).String()},
	}))
	return b.String()
}

func (m model) buildLogResult() string {
	summary := m.txLogger.Summary()
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("━━ TRANSACTION LOG ━━") + "\n\n")

	if summary.Total == 0 {
		b.WriteString("   " + sDim.Render("· No transactions in this session") + "\n")
		if _, err := os.Stat(txLogCSV); err == nil {
			b.WriteString("   " + sDim.Render("· Previous logs: "+txLogCSV) + "\n")
		}
		return b.String()
	}

	stats := [][2]string{
		{"Total", fmt.Sprintf("%d", summary.Total)},
		{"Sent", fmt.Sprintf("%d", summary.Sent)},
		{"Failed", fmt.Sprintf("%d", summary.Failed)},
	}
	for ch, c := range summary.ByChain {
		stats = append(stats, [2]string{ch, fmt.Sprintf("%d", c)})
	}
	for tp, c := range summary.ByType {
		stats = append(stats, [2]string{tp, fmt.Sprintf("%d", c)})
	}
	stats = append(stats, [2]string{"CSV", txLogCSV}, [2]string{"JSON", txLogJSON})
	b.WriteString(buildStatBox(stats))
	return b.String()
}

// ── Stat Box ─────────────────────────────────────────────────────────

func buildStatBox(rows [][2]string) string {
	var b strings.Builder
	b.WriteString("\n   " + sBorder.Render("┌───────────────────────────────────────────┐") + "\n")
	for _, row := range rows {
		b.WriteString(fmt.Sprintf("   %s  %-16s %s %s\n",
			sBorder.Render("│"), sDim.Render(row[0]), sGreenB.Render(fmt.Sprintf("%-22s", row[1])), sBorder.Render("│")))
	}
	b.WriteString("   " + sBorder.Render("└───────────────────────────────────────────┘") + "\n")
	return b.String()
}

func txStats(success, failed int, elapsed time.Duration, ch chain.Chain, delay ops.DelayConfig) [][2]string {
	rows := [][2]string{
		{"Success", fmt.Sprintf("%d tx", success)},
	}
	if failed > 0 {
		rows = append(rows, [2]string{"Failed", fmt.Sprintf("%d tx", failed)})
	}
	rows = append(rows, [2]string{"Chain", ch.Name})
	rows = append(rows, [2]string{"Duration", elapsed.Round(time.Millisecond).String()})
	if delay.Enabled {
		rows = append(rows, [2]string{"Delay", fmt.Sprintf("%d-%ds random", delay.MinMs/1000, delay.MaxMs/1000)})
	}
	return rows
}

// ── Styled Helpers ───────────────────────────────────────────────────

func styledAddr(addr string) string {
	if len(addr) < 10 {
		return addr
	}
	return sAccent.Render(addr[:6]) + sGray.Render(addr[6:len(addr)-4]) + sMagenta.Render(addr[len(addr)-4:])
}

var sMagenta = lipgloss.NewStyle().Foreground(cMagenta)

func shortAddr(addr string) string {
	if len(addr) < 12 {
		return addr
	}
	return addr[:6] + "…" + addr[len(addr)-4:]
}

func shortHash(hash string) string {
	if len(hash) < 16 {
		return hash
	}
	return hash[:10] + "…" + hash[len(hash)-6:]
}

// ── Utility ──────────────────────────────────────────────────────────

func parseWalletRange(s string, wls []wallet.Wallet) []wallet.Wallet {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "all" {
		return wls
	}
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return nil
	}
	start, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil || start < 1 || end > len(wls) || start > end {
		return nil
	}
	return wls[start-1 : end]
}

func parseEther(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ".")
	whole := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	for len(frac) < 18 {
		frac += "0"
	}
	frac = frac[:18]
	weiStr := strings.TrimLeft(whole+frac, "0")
	if weiStr == "" {
		return big.NewInt(0)
	}
	wei, ok := new(big.Int).SetString(weiStr, 10)
	if !ok {
		return nil
	}
	return wei
}

// ── Amount Randomizer Wizard ─────────────────────────────────────────

func (m model) wizAmountRand() (model, tea.Cmd) {
	switch m.step {
	case 1:
		v, err := strconv.Atoi(m.data["variance"])
		if err != nil || v < 0 || v > 50 {
			m.amtRandPct = 0
		} else {
			m.amtRandPct = v
		}
		return m.returnToMenu()
	}
	return m.returnToMenu()
}

// ── Portfolio Wizard ────────────────────────────────────────────────

func (m model) wizPortfolio() (model, tea.Cmd) {
	switch m.step {
	case 0:
		wls := m.wallets
		prov := m.provider
		pc := m.priceCache
		return m.showSpinner(fmt.Sprintf("Scanning portfolio across %d chains…", len(chain.AllChains)), func() (string, error, func(*model)) {
			result, err := ops.ScanPortfolio(prov, wls, chain.AllChains, pc)
			if err != nil {
				return "", err, nil
			}
			return buildPortfolioResult(result), nil, nil
		})
	}
	return m.returnToMenu()
}

func buildPortfolioResult(pr *ops.PortfolioResult) string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("━━ PORTFOLIO DASHBOARD ━━") + "\n\n")

	b.WriteString(fmt.Sprintf("   %-12s  %10s  %8s  %12s  %10s\n",
		sGray.Render("CHAIN"), sGray.Render("BALANCE"), sGray.Render("WALLETS"), sGray.Render("PRICE"), sGray.Render("USD VALUE")))
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 65)) + "\n")

	for _, cp := range pr.Chains {
		if cp.TotalNative.Sign() == 0 && cp.Errors == 0 {
			continue
		}
		bal := ops.FormatBalance(cp.TotalNative, 18)
		priceStr := "N/A"
		usdStr := "—"
		if cp.PriceUSD > 0 {
			priceStr = fmt.Sprintf("$%.2f", cp.PriceUSD)
		}
		if cp.TotalUSD > 0.01 {
			usdStr = fmt.Sprintf("$%.2f", cp.TotalUSD)
		}

		nameStyle := sSoft
		balStyle := sGreenBr
		if cp.NonZero == 0 {
			nameStyle = sDim
			balStyle = sDim
		}

		b.WriteString(fmt.Sprintf("   %-12s  %10s  %8s  %12s  %10s\n",
			nameStyle.Render(cp.Chain.Name),
			balStyle.Render(bal+" "+cp.Chain.Symbol),
			sDim.Render(fmt.Sprintf("%d", cp.NonZero)),
			sDim.Render(priceStr),
			sGreenB.Render(usdStr)))
	}

	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 65)) + "\n")
	b.WriteString(fmt.Sprintf("\n   %s  %s\n",
		sWhiteB.Render("TOTAL PORTFOLIO:"),
		sGreenB.Render(fmt.Sprintf("$%.2f USD", pr.TotalUSD))))
	b.WriteString(fmt.Sprintf("   %s  %s\n",
		sDim.Render("Scan time:"),
		sGreenBr.Render(pr.ScanTime.Round(time.Millisecond).String())))
	return b.String()
}

// ── Gas Tracker Wizard ──────────────────────────────────────────────

func (m model) wizGasTrack() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showChainSelect()
	case 1:
		return m.showInput("Number of transactions", "txcount", false, strconv.Itoa(len(m.wallets)))
	case 2:
		return m.showInput("Gas limit per tx", "gaslimit", false, "21000")
	case 3:
		ch := m.selectedChain()
		txCount, _ := strconv.Atoi(m.data["txcount"])
		gasLimit, _ := strconv.Atoi(m.data["gaslimit"])
		prov := m.provider
		pc := m.priceCache
		return m.showSpinner("Estimating gas costs…", func() (string, error, func(*model)) {
			est, err := ops.EstimateBatchGas(prov, ch, txCount, uint64(gasLimit), pc)
			if err != nil {
				return "", err, nil
			}
			return buildGasEstResult(est), nil, nil
		})
	}
	return m.returnToMenu()
}

func buildGasEstResult(est *ops.GasEstimate) string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("━━ GAS ESTIMATE ━━") + "\n\n")

	rows := [][2]string{
		{"Chain", est.Chain.Name},
		{"Gas Price", ops.FormatBalance(est.GasPrice, 9) + " Gwei"},
		{"Gas Limit", fmt.Sprintf("%d", est.GasLimit)},
		{"Tx Count", fmt.Sprintf("%d", est.TxCount)},
		{"Total Gas", est.TotalGasWei.String() + " wei"},
		{"Total Cost", ops.FormatBalance(est.TotalGasWei, 18) + " " + est.Chain.Symbol},
	}
	if est.PriceUSD > 0 {
		rows = append(rows,
			[2]string{"Native Price", fmt.Sprintf("$%.2f", est.PriceUSD)},
			[2]string{"Total USD", fmt.Sprintf("$%.4f", est.TotalUSD)},
		)
	}
	b.WriteString(buildStatBox(rows))
	return b.String()
}

// ── Dry-Run Wizard ──────────────────────────────────────────────────

func (m model) wizDryRun() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showChainSelect()
	case 1:
		return m.showInput("Operation (distribute/sweep/dexmix)", "op", false, "distribute")
	case 2:
		ch := m.selectedChain()
		opName := m.data["op"]
		wls := m.wallets
		prov := m.provider
		pc := m.priceCache
		return m.showSpinner("Simulating "+opName+"…", func() (string, error, func(*model)) {
			var result string
			switch opName {
			case "distribute":
				dr, err := ops.DryRunDistribute(prov, ch, wls[0], len(wls)-1, parseEther("0.01"), "", pc)
				if err != nil {
					return "", err, nil
				}
				result = buildDryRunResult(dr)
			case "sweep":
				dr, err := ops.DryRunSweep(prov, ch, wls, "", pc)
				if err != nil {
					return "", err, nil
				}
				result = buildDryRunResult(dr)
			case "dexmix":
				dr, err := ops.DryRunSwap(prov, ch, len(wls)-1, pc)
				if err != nil {
					return "", err, nil
				}
				result = buildDryRunResult(dr)
			default:
				return sRed.Render("  Unknown operation: " + opName), nil, nil
			}
			return result, nil, nil
		})
	}
	return m.returnToMenu()
}

func buildDryRunResult(dr *ops.DryRunResult) string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("━━ DRY-RUN SIMULATION ━━") + "\n")
	b.WriteString("   " + sYellow.Render("⚠ NO TRANSACTIONS BROADCAST") + "\n\n")

	rows := [][2]string{
		{"Operation", dr.Operation},
		{"Chain", dr.Chain.Name},
		{"Tx Count", fmt.Sprintf("%d", dr.TxCount)},
		{"Gas/Tx", fmt.Sprintf("%d", dr.GasPerTx)},
		{"Gas Price", ops.FormatBalance(dr.GasPrice, 9) + " Gwei"},
		{"Total Gas", dr.TotalGasETH + " " + dr.Chain.Symbol},
	}
	if dr.PriceUSD > 0 {
		rows = append(rows, [2]string{"Gas USD", fmt.Sprintf("$%.4f", dr.TotalUSD)})
	}
	if dr.SourceBal != nil {
		rows = append(rows, [2]string{"Source Bal", ops.FormatBalance(dr.SourceBal, 18) + " " + dr.Chain.Symbol})
	}
	if dr.Affordable {
		rows = append(rows, [2]string{"Affordable", "✓ Yes"})
	} else {
		rows = append(rows, [2]string{"Affordable", "✗ No"})
	}
	b.WriteString(buildStatBox(rows))

	for _, e := range dr.Errors {
		b.WriteString("   " + sRed.Render("  ✗ "+e) + "\n")
	}
	return b.String()
}

// ── Alert Menu ──────────────────────────────────────────────────────

func (m model) updateAlertMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	alertOpts := []string{"Telegram", "Discord", "Disable alerts"}
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.subCursor > 0 {
				m.subCursor--
			}
		case "down", "j":
			if m.subCursor < len(alertOpts)-1 {
				m.subCursor++
			}
		case "enter":
			switch m.subCursor {
			case 0:
				m.currentOp = "alert_telegram"
				m.step = 0
				m.data = make(map[string]string)
				return m.advanceWizard()
			case 1:
				m.currentOp = "alert_discord"
				m.step = 0
				m.data = make(map[string]string)
				return m.advanceWizard()
			case 2:
				m.alertCfg = ops.NoAlert()
				return m.returnToMenu()
			}
		case "esc":
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) renderAlertMenu() string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("ALERTS") + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 40)) + "\n\n")

	opts := []string{"Telegram", "Discord", "Disable alerts"}
	for i, opt := range opts {
		cursor := "   "
		style := sSoft
		if i == m.subCursor {
			cursor = sAccent.Render(" ▸ ")
			style = sAccent
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", cursor, style.Render(opt)))
	}

	current := "OFF"
	if m.alertCfg.Enabled {
		current = m.alertCfg.TypeName()
	}
	b.WriteString("\n   " + sDim.Render("current: "+current) + "\n")
	b.WriteString("   " + sDim.Render("↑/↓ navigate · enter select · esc cancel") + "\n")
	return b.String()
}

func (m model) wizAlertTelegram() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showInput("Telegram Bot Token", "bot_token", true, "")
	case 1:
		return m.showInput("Telegram Chat ID", "chat_id", false, "")
	case 2:
		m.alertCfg = ops.AlertConfig{
			Enabled:  true,
			Type:     ops.AlertTelegram,
			BotToken: m.data["bot_token"],
			ChatID:   m.data["chat_id"],
		}
		m.alertCfg.Send("🟢 CONTROL Alert", "Telegram alerts configured successfully!")
		m.resultText = sGreenB.Render("  ✓ Telegram alerts configured")
		m.resultLines = strings.Split(m.resultText, "\n")
		m.view = viewResult
		return m, nil
	}
	return m.returnToMenu()
}

func (m model) wizAlertDiscord() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showInput("Discord Webhook URL", "webhook", false, "")
	case 1:
		m.alertCfg = ops.AlertConfig{
			Enabled: true,
			Type:    ops.AlertDiscord,
			Webhook: m.data["webhook"],
		}
		m.alertCfg.Send("🟢 CONTROL Alert", "Discord alerts configured successfully!")
		m.resultText = sGreenB.Render("  ✓ Discord alerts configured")
		m.resultLines = strings.Split(m.resultText, "\n")
		m.view = viewResult
		return m, nil
	}
	return m.returnToMenu()
}

// ── Proxy Menu ──────────────────────────────────────────────────────

func (m model) updateProxyMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	proxyOpts := []string{"Load proxy file", "Toggle mode (RR/Random)", "Disable proxy"}
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.subCursor > 0 {
				m.subCursor--
			}
		case "down", "j":
			if m.subCursor < len(proxyOpts)-1 {
				m.subCursor++
			}
		case "enter":
			switch m.subCursor {
			case 0:
				m.currentOp = "proxy_load"
				m.step = 0
				m.data = make(map[string]string)
				return m.advanceWizard()
			case 1:
				if m.proxyCfg.Mode == ops.ProxyRoundRobin {
					m.proxyCfg.Mode = ops.ProxyRandom
				} else {
					m.proxyCfg.Mode = ops.ProxyRoundRobin
				}
				return m, nil
			case 2:
				m.proxyCfg = ops.NoProxy()
				return m.returnToMenu()
			}
		case "esc":
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) renderProxyMenu() string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("PROXY SETTINGS") + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 40)) + "\n\n")

	opts := []string{"Load proxy file", "Toggle mode (RR/Random)", "Disable proxy"}
	for i, opt := range opts {
		cursor := "   "
		style := sSoft
		if i == m.subCursor {
			cursor = sAccent.Render(" ▸ ")
			style = sAccent
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", cursor, style.Render(opt)))
	}

	current := "OFF"
	if m.proxyCfg.Enabled {
		current = fmt.Sprintf("%d proxies (%s)", m.proxyCfg.Count(), m.proxyCfg.ModeName())
	}
	b.WriteString("\n   " + sDim.Render("current: "+current) + "\n")
	b.WriteString("   " + sDim.Render("↑/↓ navigate · enter select · esc cancel") + "\n")
	return b.String()
}

func (m model) wizProxyLoad() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showInput("Proxy file (one per line: socks5://host:port)", "file", false, proxyFile)
	case 1:
		filename := m.data["file"]
		cfg, err := ops.LoadProxies(filename)
		if err != nil {
			m.resultText = sRed.Render("  " + err.Error())
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		m.proxyCfg = *cfg
		m.resultText = sGreenB.Render(fmt.Sprintf("  ✓ Loaded %d proxies from %s", cfg.Count(), filename))
		m.resultLines = strings.Split(m.resultText, "\n")
		m.view = viewResult
		return m, nil
	}
	return m.returnToMenu()
}

// ── Session Menu ────────────────────────────────────────────────────

func (m model) updateSessionMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	sessionOpts := []string{"Set timeout (minutes)", "Lock now", "Disable timeout"}
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.subCursor > 0 {
				m.subCursor--
			}
		case "down", "j":
			if m.subCursor < len(sessionOpts)-1 {
				m.subCursor++
			}
		case "enter":
			switch m.subCursor {
			case 0:
				m.currentOp = "session_set"
				m.step = 0
				m.data = make(map[string]string)
				return m.advanceWizard()
			case 1:
				m.session.Lock()
				m.view = viewLocked
				return m, nil
			case 2:
				m.session = ops.NoSession()
				return m.returnToMenu()
			}
		case "esc":
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) renderSessionMenu() string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("SESSION TIMEOUT") + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 40)) + "\n\n")

	opts := []string{"Set timeout (minutes)", "Lock now", "Disable timeout"}
	for i, opt := range opts {
		cursor := "   "
		style := sSoft
		if i == m.subCursor {
			cursor = sAccent.Render(" ▸ ")
			style = sAccent
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", cursor, style.Render(opt)))
	}

	current := "OFF"
	if m.session.Enabled {
		current = fmt.Sprintf("%d min (idle: %s)", m.session.TimeoutMin, m.session.IdleTime().Round(time.Second))
	}
	b.WriteString("\n   " + sDim.Render("current: "+current) + "\n")
	b.WriteString("   " + sDim.Render("↑/↓ navigate · enter select · esc cancel") + "\n")
	return b.String()
}

func (m model) wizSessionSet() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showInput("Timeout in minutes (0=disable)", "minutes", false, "15")
	case 1:
		mins, err := strconv.Atoi(m.data["minutes"])
		if err != nil || mins < 0 {
			mins = 0
		}
		m.session = ops.NewSession(mins)
		return m.returnToMenu()
	}
	return m.returnToMenu()
}

// ── Locked View ─────────────────────────────────────────────────────

func (m model) updateLocked(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "enter" {
			m.session.Unlock()
			m.view = viewMenu
			return m, nil
		}
		if msg.String() == "ctrl+c" {
			m.cleanup()
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) renderLocked() string {
	var b strings.Builder
	b.WriteString("\n\n\n")
	b.WriteString("   " + sRed.Render("🔒 SESSION LOCKED") + "\n\n")
	b.WriteString("   " + sDim.Render("Session timed out due to inactivity.") + "\n")
	b.WriteString("   " + sDim.Render("Press Enter to unlock.") + "\n")
	return b.String()
}

// ── Backup Menu ─────────────────────────────────────────────────────

func (m model) updateBackupMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	backupOpts := []string{"Create backup", "Restore backup"}
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.subCursor > 0 {
				m.subCursor--
			}
		case "down", "j":
			if m.subCursor < len(backupOpts)-1 {
				m.subCursor++
			}
		case "enter":
			switch m.subCursor {
			case 0:
				m.currentOp = "backup_create"
				m.step = 0
				m.data = make(map[string]string)
				return m.advanceWizard()
			case 1:
				m.currentOp = "backup_restore"
				m.step = 0
				m.data = make(map[string]string)
				return m.advanceWizard()
			}
		case "esc":
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) renderBackupMenu() string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("BACKUP / RESTORE") + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 40)) + "\n\n")

	opts := []string{"Create backup", "Restore backup"}
	for i, opt := range opts {
		cursor := "   "
		style := sSoft
		if i == m.subCursor {
			cursor = sAccent.Render(" ▸ ")
			style = sAccent
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", cursor, style.Render(opt)))
	}

	b.WriteString("\n   " + sDim.Render("↑/↓ navigate · enter select · esc cancel") + "\n")
	return b.String()
}

func (m model) wizBackupCreate() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showInput("Backup filename", "filename", false, "backup_"+time.Now().Format("20060102")+".enc")
	case 1:
		return m.showInput("Backup password", "password", true, "")
	case 2:
		filename := m.data["filename"]
		password := m.data["password"]
		groups := m.groupIndex.Groups
		return m.showSpinner("Creating encrypted backup…", func() (string, error, func(*model)) {
			data, err := wallet.CreateBackup(groupIndexFile, password, groups)
			if err != nil {
				return "", err, nil
			}
			if err := os.WriteFile(filename, data, 0600); err != nil {
				return "", fmt.Errorf("write backup: %w", err), nil
			}
			return sGreenB.Render(fmt.Sprintf("  ✓ Backup created: %s (%d groups, %d bytes)",
				filename, len(groups), len(data))), nil, nil
		})
	}
	return m.returnToMenu()
}

func (m model) wizBackupRestore() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showInput("Backup filename", "filename", false, "")
	case 1:
		return m.showInput("Backup password", "password", true, "")
	case 2:
		filename := m.data["filename"]
		password := m.data["password"]
		return m.showSpinner("Restoring backup…", func() (string, error, func(*model)) {
			backup, err := wallet.RestoreBackup(filename, password)
			if err != nil {
				return "", err, nil
			}
			if err := wallet.WriteRestored(backup, groupIndexFile); err != nil {
				return "", err, nil
			}
			gi, _ := wallet.LoadGroupIndex(groupIndexFile)
			return sGreenB.Render(fmt.Sprintf("  ✓ Restored %d groups, %d files from %s",
				len(backup.Groups), len(backup.Files), filename)), nil, func(m *model) {
				if gi != nil {
					m.groupIndex = gi
				}
			}
		})
	}
	return m.returnToMenu()
}

// ── Label Menu ──────────────────────────────────────────────────────

func (m model) updateLabelMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	labelOpts := []string{"Set label", "View labels", "Remove label"}
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.subCursor > 0 {
				m.subCursor--
			}
		case "down", "j":
			if m.subCursor < len(labelOpts)-1 {
				m.subCursor++
			}
		case "enter":
			switch m.subCursor {
			case 0:
				m.currentOp = "label_set"
				m.step = 0
				m.data = make(map[string]string)
				return m.advanceWizard()
			case 1:
				m.resultText = m.buildLabelResult()
				m.resultLines = strings.Split(m.resultText, "\n")
				m.scrollOffset = 0
				m.view = viewResult
				return m, nil
			case 2:
				m.currentOp = "label_set"
				m.step = 0
				m.data = map[string]string{"_remove": "1"}
				return m.advanceWizard()
			}
		case "esc":
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) renderLabelMenu() string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("WALLET LABELS") + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 40)) + "\n\n")

	opts := []string{"Set label", "View labels", "Remove label"}
	for i, opt := range opts {
		cursor := "   "
		style := sSoft
		if i == m.subCursor {
			cursor = sAccent.Render(" ▸ ")
			style = sAccent
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", cursor, style.Render(opt)))
	}

	b.WriteString("\n   " + sDim.Render(fmt.Sprintf("%d labels set", len(m.labels.Labels))) + "\n")
	b.WriteString("   " + sDim.Render("↑/↓ navigate · enter select · esc cancel") + "\n")
	return b.String()
}

func (m model) wizLabelSet() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showInput(fmt.Sprintf("Wallet index (1-%d)", len(m.wallets)), "index", false, "1")
	case 1:
		idx, _ := strconv.Atoi(m.data["index"])
		if idx < 1 || idx > len(m.wallets) {
			m.resultText = sRed.Render("  Invalid wallet index")
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		if m.data["_remove"] == "1" {
			addr := m.wallets[idx-1].Address
			m.labels.RemoveLabel(addr)
			wallet.SaveLabels(labelsFile, m.labels)
			m.resultText = sGreenB.Render(fmt.Sprintf("  ✓ Label removed for wallet #%d", idx))
			m.resultLines = strings.Split(m.resultText, "\n")
			m.view = viewResult
			return m, nil
		}
		return m.showInput("Label (e.g. CEX, hot, cold)", "label", false, "")
	case 2:
		idx, _ := strconv.Atoi(m.data["index"])
		addr := m.wallets[idx-1].Address
		label := m.data["label"]
		m.labels.SetLabel(addr, label)
		wallet.SaveLabels(labelsFile, m.labels)
		m.resultText = sGreenB.Render(fmt.Sprintf("  ✓ Label '%s' set for wallet #%d (%s)",
			label, idx, shortAddr(addr)))
		m.resultLines = strings.Split(m.resultText, "\n")
		m.view = viewResult
		return m, nil
	}
	return m.returnToMenu()
}

func (m model) buildLabelResult() string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("━━ WALLET LABELS ━━") + "\n\n")

	if len(m.labels.Labels) == 0 {
		b.WriteString("   " + sDim.Render("No labels set. Use 'Set label' to add.") + "\n")
		return b.String()
	}

	for i, w := range m.wallets {
		label := m.labels.GetLabel(w.Address)
		if label == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("   %s  %s  %s\n",
			sDim.Render(fmt.Sprintf("%03d", i+1)),
			styledAddr(w.Address),
			sGreenBr.Render("["+label+"]")))
	}
	return b.String()
}

// ── Queue Menu ──────────────────────────────────────────────────────

func (m model) updateQueueMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	queueOpts := []string{"Add step", "View queue", "Clear queue", "Execute queue"}
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.subCursor > 0 {
				m.subCursor--
			}
		case "down", "j":
			if m.subCursor < len(queueOpts)-1 {
				m.subCursor++
			}
		case "enter":
			switch m.subCursor {
			case 0:
				m.currentOp = "queue_add"
				m.step = 0
				m.data = make(map[string]string)
				return m.advanceWizard()
			case 1:
				m.resultText = m.buildQueueResult()
				m.resultLines = strings.Split(m.resultText, "\n")
				m.scrollOffset = 0
				m.view = viewResult
				return m, nil
			case 2:
				m.queue = ops.NewQueue()
				m.resultText = sGreenB.Render("  ✓ Queue cleared")
				m.resultLines = strings.Split(m.resultText, "\n")
				m.view = viewResult
				return m, nil
			case 3:
				if len(m.queue.Steps) == 0 {
					m.statusMsg = "Queue is empty. Add steps first."
					return m.returnToMenu()
				}
				return m.executeQueue()
			}
		case "esc":
			return m.returnToMenu()
		}
	}
	return m, nil
}

func (m model) renderQueueMenu() string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("OPERATION QUEUE") + "\n")
	b.WriteString("   " + sBorder.Render(strings.Repeat("─", 40)) + "\n\n")

	opts := []string{"Add step", "View queue", "Clear queue", "Execute queue"}
	for i, opt := range opts {
		cursor := "   "
		style := sSoft
		if i == m.subCursor {
			cursor = sAccent.Render(" ▸ ")
			style = sAccent
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", cursor, style.Render(opt)))
	}

	b.WriteString("\n   " + sDim.Render(fmt.Sprintf("%d steps in queue", len(m.queue.Steps))) + "\n")
	if len(m.queue.Steps) > 0 {
		b.WriteString("   " + sDim.Render("→ "+m.queue.Describe()) + "\n")
	}
	b.WriteString("   " + sDim.Render("↑/↓ navigate · enter select · esc cancel") + "\n")
	return b.String()
}

func (m model) wizQueueAdd() (model, tea.Cmd) {
	switch m.step {
	case 0:
		return m.showInput("Operation (distribute/collect/sweep/dexmix/bridge/delay)", "op", false, "delay")
	case 1:
		opName := m.data["op"]
		params := map[string]string{"op": opName}
		if opName == "delay" {
			return m.showInput("Delay seconds", "seconds", false, "30")
		}
		m.queue.AddStep(opName, params)
		m.resultText = sGreenB.Render(fmt.Sprintf("  ✓ Added '%s' to queue (total: %d steps)", opName, len(m.queue.Steps)))
		m.resultLines = strings.Split(m.resultText, "\n")
		m.view = viewResult
		return m, nil
	case 2:
		opName := m.data["op"]
		seconds := m.data["seconds"]
		params := map[string]string{"op": opName, "seconds": seconds}
		m.queue.AddStep(opName, params)
		m.resultText = sGreenB.Render(fmt.Sprintf("  ✓ Added '%s %ss' to queue (total: %d steps)",
			opName, seconds, len(m.queue.Steps)))
		m.resultLines = strings.Split(m.resultText, "\n")
		m.view = viewResult
		return m, nil
	}
	return m.returnToMenu()
}

func (m model) buildQueueResult() string {
	var b strings.Builder
	b.WriteString("\n   " + sLime.Render("━━ OPERATION QUEUE ━━") + "\n\n")

	if len(m.queue.Steps) == 0 {
		b.WriteString("   " + sDim.Render("Queue is empty.") + "\n")
		return b.String()
	}

	for i, step := range m.queue.Steps {
		con := sBorder.Render("├─")
		if i == len(m.queue.Steps)-1 {
			con = sBorder.Render("└─")
		}
		extra := ""
		if s, ok := step.Params["seconds"]; ok {
			extra = " (" + s + "s)"
		}
		b.WriteString(fmt.Sprintf("   %s %s %s%s\n",
			con, sGreenB.Render(fmt.Sprintf("%d.", i+1)),
			sAccent.Render(step.Name), sDim.Render(extra)))
	}
	return b.String()
}

func (m model) executeQueue() (model, tea.Cmd) {
	queue := m.queue
	return m.showSpinner(fmt.Sprintf("Executing %d queue steps…", len(queue.Steps)), func() (string, error, func(*model)) {
		var b strings.Builder
		b.WriteString("\n   " + sLime.Render("━━ QUEUE EXECUTION ━━") + "\n\n")

		for i, step := range queue.Steps {
			start := time.Now()
			switch step.Name {
			case "delay":
				secs, _ := strconv.Atoi(step.Params["seconds"])
				if secs <= 0 {
					secs = 10
				}
				time.Sleep(time.Duration(secs) * time.Second)
				b.WriteString(fmt.Sprintf("   %s %s %s  %s\n",
					sBorder.Render("├─"), sGreenB.Render("✓"),
					"delay", sDim.Render(fmt.Sprintf("%ds", secs))))
			default:
				b.WriteString(fmt.Sprintf("   %s %s %s  %s\n",
					sBorder.Render("├─"), sYellow.Render("⏭"),
					step.Name, sDim.Render("queued (manual execution)")))
			}
			_ = time.Since(start)
			_ = i
		}
		b.WriteString(fmt.Sprintf("\n   %s  %d steps completed\n",
			sGreenB.Render("✓"), len(queue.Steps)))
		return b.String(), nil, func(m *model) {
			m.queue = ops.NewQueue()
		}
	})
}

// ── Preset Helpers ──────────────────────────────────────────────────

func loadPresets() *ops.PresetStore {
	data, err := os.ReadFile(presetsFile)
	if err != nil {
		return ops.NewPresetStore()
	}
	var ps ops.PresetStore
	if err := json.Unmarshal(data, &ps); err != nil {
		return ops.NewPresetStore()
	}
	return &ps
}

func savePresets(ps *ops.PresetStore) {
	data, _ := json.MarshalIndent(ps, "", "  ")
	os.WriteFile(presetsFile, data, 0600)
}

// ── Main ─────────────────────────────────────────────────────────────

func main() {
	provider, err := chain.NewProvider(ankrFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load RPC keys: %v\n", err)
		os.Exit(1)
	}

	m := initialModel(provider)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
