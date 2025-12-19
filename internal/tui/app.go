// Package tui provides the terminal user interface for the Yuno OS installer.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
)

// Screen represents different installer screens
type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenDisk
	ScreenPartition
	ScreenEncryption
	ScreenInitSystem
	ScreenOverlays
	ScreenCFlags
	ScreenUseFlags
	ScreenKernel
	ScreenGraphics
	ScreenDesktop
	ScreenPackages
	ScreenSecureBoot
	ScreenTimezone
	ScreenUsers
	ScreenSummary
	ScreenInstall
	ScreenComplete
)

// App is the main TUI application model
type App struct {
	screen       Screen
	config       *config.InstallConfig
	width        int
	height       int
	spinner      spinner.Model
	err          error

	// Screen-specific state
	diskList     []DiskItem
	selectedDisk int

	// Navigation
	focusIndex   int

	// Installation progress
	installStep  int
	installLog   []string
}

// DiskItem represents a disk in the selection list
type DiskItem struct {
	Path  string
	Size  string
	Model string
}

// NewApp creates a new TUI application
func NewApp() *App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &App{
		screen:  ScreenWelcome,
		config:  config.NewDefaultConfig(),
		spinner: s,
	}
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		a.detectDisks,
	)
}

// Update handles messages and updates the model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case disksDetectedMsg:
		a.diskList = msg.disks
		return a, nil

	case errMsg:
		a.err = msg.err
		return a, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd
	}

	return a, nil
}

// handleKeyPress handles keyboard input
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return a, tea.Quit

	case "enter":
		return a.nextScreen()

	case "esc", "backspace":
		return a.prevScreen()

	case "up", "k":
		if a.focusIndex > 0 {
			a.focusIndex--
		}

	case "down", "j":
		a.focusIndex++

	case "tab":
		a.focusIndex++
	}

	return a, nil
}

// nextScreen advances to the next screen
func (a *App) nextScreen() (tea.Model, tea.Cmd) {
	// Validate current screen before proceeding
	if err := a.validateCurrentScreen(); err != nil {
		a.err = err
		return a, nil
	}

	// Save current screen's selections to config
	a.saveScreenToConfig()

	// Advance to next screen
	if a.screen < ScreenComplete {
		a.screen++
		a.focusIndex = 0
		a.err = nil
	}

	// Handle screen-specific initialization
	switch a.screen {
	case ScreenInstall:
		return a, a.startInstallation
	}

	return a, nil
}

// prevScreen goes back to the previous screen
func (a *App) prevScreen() (tea.Model, tea.Cmd) {
	if a.screen > ScreenWelcome && a.screen != ScreenInstall {
		a.screen--
		a.focusIndex = 0
		a.err = nil
	}
	return a, nil
}

// validateCurrentScreen validates the current screen's selections
func (a *App) validateCurrentScreen() error {
	switch a.screen {
	case ScreenDisk:
		if len(a.diskList) == 0 {
			return fmt.Errorf("no disks available")
		}
		if a.selectedDisk >= len(a.diskList) {
			return fmt.Errorf("please select a disk")
		}
	case ScreenUsers:
		if a.config.RootPassword == "" {
			return fmt.Errorf("root password is required")
		}
	}
	return nil
}

// saveScreenToConfig saves the current screen's selections to config
func (a *App) saveScreenToConfig() {
	switch a.screen {
	case ScreenDisk:
		if a.selectedDisk < len(a.diskList) {
			a.config.Disk.Device = a.diskList[a.selectedDisk].Path
		}
	}
}

// View renders the application
func (a *App) View() string {
	// Build the view based on current screen
	var content string

	switch a.screen {
	case ScreenWelcome:
		content = a.viewWelcome()
	case ScreenDisk:
		content = a.viewDisk()
	case ScreenPartition:
		content = a.viewPartition()
	case ScreenEncryption:
		content = a.viewEncryption()
	case ScreenInitSystem:
		content = a.viewInitSystem()
	case ScreenOverlays:
		content = a.viewOverlays()
	case ScreenCFlags:
		content = a.viewCFlags()
	case ScreenUseFlags:
		content = a.viewUseFlags()
	case ScreenKernel:
		content = a.viewKernel()
	case ScreenGraphics:
		content = a.viewGraphics()
	case ScreenDesktop:
		content = a.viewDesktop()
	case ScreenPackages:
		content = a.viewPackages()
	case ScreenSecureBoot:
		content = a.viewSecureBoot()
	case ScreenTimezone:
		content = a.viewTimezone()
	case ScreenUsers:
		content = a.viewUsers()
	case ScreenSummary:
		content = a.viewSummary()
	case ScreenInstall:
		content = a.viewInstall()
	case ScreenComplete:
		content = a.viewComplete()
	}

	return a.applyLayout(content)
}

// applyLayout applies the common layout to content
func (a *App) applyLayout(content string) string {
	// Header
	header := headerStyle.Render("  Yuno OS Installer  ")

	// Progress indicator
	progress := a.renderProgress()

	// Error display
	var errDisplay string
	if a.err != nil {
		errDisplay = errorStyle.Render(fmt.Sprintf("Error: %v", a.err))
	}

	// Footer with help
	footer := helpStyle.Render("↑/↓: Navigate • Enter: Select • Esc: Back • q: Quit")

	// Combine all elements
	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s\n\n%s",
		header,
		progress,
		content,
		errDisplay,
		footer,
	)
}

// renderProgress renders the installation progress bar
func (a *App) renderProgress() string {
	steps := []string{
		"Disk", "Encrypt", "Init", "Overlays", "Flags",
		"Kernel", "Graphics", "Desktop", "Users", "Install",
	}

	var result string
	for i, step := range steps {
		style := progressInactiveStyle
		if i < int(a.screen)-1 {
			style = progressCompleteStyle
		} else if i == int(a.screen)-1 {
			style = progressActiveStyle
		}
		result += style.Render(step) + " "
	}

	return result
}

// Messages

type disksDetectedMsg struct {
	disks []DiskItem
}

type errMsg struct {
	err error
}

type installProgressMsg struct {
	step    int
	message string
}

type installCompleteMsg struct{}

// Commands

func (a *App) detectDisks() tea.Msg {
	// This would use the partition package to detect disks
	// For now, return mock data
	return disksDetectedMsg{
		disks: []DiskItem{
			{Path: "/dev/sda", Size: "500GB", Model: "Samsung SSD"},
			{Path: "/dev/nvme0n1", Size: "1TB", Model: "NVMe Drive"},
		},
	}
}

func (a *App) startInstallation() tea.Msg {
	// Start the installation process
	// This would be handled by the installer package
	return nil
}

// Styles

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 2).
			MarginBottom(1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ABABAB")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	progressActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true)

	progressCompleteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF00"))

	progressInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#626262"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 2)
)
