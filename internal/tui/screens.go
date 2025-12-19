// TUI screen views
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
)

// viewWelcome renders the welcome screen
func (a *App) viewWelcome() string {
	logo := `
 __   __                    ___  ____
 \ \ / /   _ _ __   ___    / _ \/ ___|
  \ V / | | | '_ \ / _ \  | | | \___ \
   | || |_| | | | | (_) | | |_| |___) |
   |_| \__,_|_| |_|\___/   \___/|____/

`
	title := titleStyle.Render("Welcome to Yuno OS Installer")
	subtitle := subtitleStyle.Render("A Gentoo-based distribution with an easy installer")

	features := boxStyle.Render(`Features:
• TUI and GUI installers
• OpenRC and systemd support
• Full disk encryption (LUKS, ZFS)
• LTO overlay and custom CFLAGS
• Multiple desktop environments
• Binary package support
• Secure Boot support`)

	instructions := helpStyle.Render("\nPress Enter to begin installation...")

	return fmt.Sprintf("%s\n%s\n%s\n\n%s\n%s",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Render(logo),
		title,
		subtitle,
		features,
		instructions,
	)
}

// viewDisk renders the disk selection screen
func (a *App) viewDisk() string {
	title := titleStyle.Render("Select Installation Disk")
	subtitle := subtitleStyle.Render("Choose the disk where Yuno OS will be installed.\n⚠️  All data on the selected disk will be erased!")

	var diskList strings.Builder
	for i, disk := range a.diskList {
		cursor := "  "
		style := normalStyle
		if i == a.selectedDisk {
			cursor = "▸ "
			style = selectedStyle
		}
		diskList.WriteString(style.Render(fmt.Sprintf("%s%s - %s (%s)\n",
			cursor, disk.Path, disk.Model, disk.Size)))
	}

	if len(a.diskList) == 0 {
		diskList.WriteString(errorStyle.Render("No disks detected!"))
	}

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, diskList.String())
}

// viewPartition renders the partitioning screen
func (a *App) viewPartition() string {
	title := titleStyle.Render("Partitioning")
	subtitle := subtitleStyle.Render("Choose how to partition the disk")

	options := []string{
		"Automatic (recommended) - Erase disk and create optimal layout",
		"Manual - Configure partitions yourself",
	}

	var optionList strings.Builder
	for i, opt := range options {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		optionList.WriteString(style.Render(fmt.Sprintf("%s%s\n", cursor, opt)))
	}

	// Show proposed layout
	layout := boxStyle.Render(`Automatic layout:
├─ /boot (ESP)  1 GB   FAT32
├─ swap         8 GB   swap
└─ /            rest   ext4`)

	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s", title, subtitle, optionList.String(), layout)
}

// viewEncryption renders the encryption selection screen
func (a *App) viewEncryption() string {
	title := titleStyle.Render("Disk Encryption")
	subtitle := subtitleStyle.Render("Choose encryption method for your installation")

	options := []struct {
		name string
		desc string
	}{
		{"None", "No encryption (fastest)"},
		{"LUKS2", "Linux Unified Key Setup - Standard Linux encryption"},
		{"LUKS", "LUKS version 1 - Better compatibility"},
		{"ZFS Encryption", "Native ZFS encryption (requires ZFS root)"},
	}

	var optionList strings.Builder
	for i, opt := range options {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		optionList.WriteString(style.Render(fmt.Sprintf("%s%-15s %s\n", cursor, opt.name, opt.desc)))
	}

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, optionList.String())
}

// viewInitSystem renders the init system selection screen
func (a *App) viewInitSystem() string {
	title := titleStyle.Render("Init System")
	subtitle := subtitleStyle.Render("Choose your init system")

	options := []struct {
		name string
		desc string
	}{
		{"OpenRC", "Traditional Gentoo init system - Simple and fast"},
		{"systemd", "Modern init system - More features, wider compatibility"},
	}

	var optionList strings.Builder
	for i, opt := range options {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		optionList.WriteString(style.Render(fmt.Sprintf("%s%-10s %s\n", cursor, opt.name, opt.desc)))
	}

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, optionList.String())
}

// viewOverlays renders the overlay selection screen
func (a *App) viewOverlays() string {
	title := titleStyle.Render("Portage Overlays")
	subtitle := subtitleStyle.Render("Select additional overlays to enable (Space to toggle)")

	overlays := []struct {
		name     string
		desc     string
		selected bool
	}{
		{"LTO", "Link-Time Optimization for better performance", false},
		{"GURU", "Gentoo User Repository - community packages", false},
		{"Steam", "Steam and gaming packages", false},
	}

	var overlayList strings.Builder
	for i, ov := range overlays {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		checkbox := "[ ]"
		if ov.selected {
			checkbox = "[✓]"
		}
		overlayList.WriteString(style.Render(fmt.Sprintf("%s%s %-10s %s\n", cursor, checkbox, ov.name, ov.desc)))
	}

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, overlayList.String())
}

// viewCFlags renders the CFLAGS configuration screen
func (a *App) viewCFlags() string {
	title := titleStyle.Render("Compiler Flags")
	subtitle := subtitleStyle.Render("Choose optimization level for compiled packages")

	presets := []struct {
		name  string
		flags string
		desc  string
	}{
		{"Safe", "-march=x86-64 -O2 -pipe", "Maximum compatibility"},
		{"Optimized", "-march=native -O2 -pipe", "Native CPU optimizations (Recommended)"},
		{"Aggressive", "-march=native -O3 -pipe -flto=auto", "Maximum performance with LTO"},
		{"Custom", "", "Specify your own CFLAGS"},
	}

	var presetList strings.Builder
	for i, preset := range presets {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		presetList.WriteString(style.Render(fmt.Sprintf("%s%-12s %s\n", cursor, preset.name, preset.desc)))
		if preset.flags != "" {
			presetList.WriteString(helpStyle.Render(fmt.Sprintf("              %s\n", preset.flags)))
		}
	}

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, presetList.String())
}

// viewUseFlags renders the USE flags configuration screen
func (a *App) viewUseFlags() string {
	title := titleStyle.Render("USE Flags")
	subtitle := subtitleStyle.Render("Select a USE flag preset")

	presets := []struct {
		name string
		desc string
	}{
		{"Desktop KDE", "KDE Plasma desktop with Qt applications"},
		{"Desktop GNOME", "GNOME desktop with GTK applications"},
		{"Desktop XFCE", "Lightweight XFCE desktop"},
		{"Laptop", "Power management and wireless support"},
		{"Gaming", "Steam, Vulkan, and gaming optimizations"},
		{"Server", "Minimal server installation"},
		{"Custom", "Configure USE flags manually"},
	}

	var presetList strings.Builder
	for i, preset := range presets {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		presetList.WriteString(style.Render(fmt.Sprintf("%s%-15s %s\n", cursor, preset.name, preset.desc)))
	}

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, presetList.String())
}

// viewKernel renders the kernel selection screen
func (a *App) viewKernel() string {
	title := titleStyle.Render("Kernel Selection")
	subtitle := subtitleStyle.Render("Choose which kernel to install")

	kernels := []struct {
		name string
		desc string
	}{
		{"gentoo-kernel-bin", "Pre-compiled kernel - Fastest install (Recommended)"},
		{"gentoo-kernel", "Distribution kernel - Compiled during install"},
		{"gentoo-sources", "Full customization with genkernel"},
		{"zen-sources", "Desktop-optimized kernel"},
		{"xanmod-sources", "Performance-focused kernel"},
	}

	var kernelList strings.Builder
	for i, k := range kernels {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		kernelList.WriteString(style.Render(fmt.Sprintf("%s%-20s %s\n", cursor, k.name, k.desc)))
	}

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, kernelList.String())
}

// viewGraphics renders the graphics driver selection screen
func (a *App) viewGraphics() string {
	title := titleStyle.Render("Graphics Drivers")
	subtitle := subtitleStyle.Render("Select your graphics driver")

	// Show detected GPU
	detected := boxStyle.Render("Detected: NVIDIA GeForce RTX 3080")

	drivers := []struct {
		name string
		desc string
	}{
		{"NVIDIA (proprietary)", "Best performance for NVIDIA cards"},
		{"NVIDIA (open)", "Open kernel modules for newer NVIDIA cards"},
		{"Nouveau", "Open-source NVIDIA driver (limited performance)"},
		{"AMDGPU", "Open-source AMD driver"},
		{"Intel", "Intel integrated graphics"},
		{"Auto-detect", "Automatically detect and configure"},
	}

	var driverList strings.Builder
	for i, d := range drivers {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		driverList.WriteString(style.Render(fmt.Sprintf("%s%-25s %s\n", cursor, d.name, d.desc)))
	}

	displayType := helpStyle.Render("\nDisplay Server: [X11] [Wayland]")

	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n%s", title, subtitle, detected, driverList.String(), displayType)
}

// viewDesktop renders the desktop environment selection screen
func (a *App) viewDesktop() string {
	title := titleStyle.Render("Desktop Environment")
	subtitle := subtitleStyle.Render("Choose your desktop environment or window manager")

	desktops := []struct {
		name string
		desc string
	}{
		{"KDE Plasma", "Full-featured, modern desktop"},
		{"GNOME", "Clean, simple, touch-friendly"},
		{"XFCE", "Lightweight, traditional desktop"},
		{"LXQt", "Lightweight Qt-based desktop"},
		{"Cinnamon", "Traditional, GNOME-based"},
		{"───────────", "─── Window Managers ───"},
		{"i3", "Tiling window manager (X11)"},
		{"Sway", "i3-compatible Wayland compositor"},
		{"Hyprland", "Dynamic Wayland compositor"},
		{"None", "Server/minimal installation"},
	}

	var desktopList strings.Builder
	for i, d := range desktops {
		if strings.HasPrefix(d.name, "───") {
			desktopList.WriteString(helpStyle.Render(d.name + "\n"))
			continue
		}
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		desktopList.WriteString(style.Render(fmt.Sprintf("%s%-15s %s\n", cursor, d.name, d.desc)))
	}

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, desktopList.String())
}

// viewPackages renders the package preference screen
func (a *App) viewPackages() string {
	title := titleStyle.Render("Package Installation")
	subtitle := subtitleStyle.Render("Choose how packages should be installed")

	options := []struct {
		name string
		desc string
	}{
		{"Binary preferred", "Use pre-built packages when available (Recommended)"},
		{"Source only", "Compile everything from source (traditional Gentoo)"},
		{"Binary only", "Only install pre-built packages"},
	}

	var optionList strings.Builder
	for i, opt := range options {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		optionList.WriteString(style.Render(fmt.Sprintf("%s%-20s %s\n", cursor, opt.name, opt.desc)))
	}

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, optionList.String())
}

// viewSecureBoot renders the Secure Boot configuration screen
func (a *App) viewSecureBoot() string {
	title := titleStyle.Render("Secure Boot")
	subtitle := subtitleStyle.Render("Configure UEFI Secure Boot")

	options := []struct {
		name string
		desc string
	}{
		{"Disabled", "Do not configure Secure Boot"},
		{"Custom keys", "Generate and enroll custom MOK keys"},
		{"Shim", "Use shim for compatibility with existing keys"},
	}

	var optionList strings.Builder
	for i, opt := range options {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		optionList.WriteString(style.Render(fmt.Sprintf("%s%-15s %s\n", cursor, opt.name, opt.desc)))
	}

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, optionList.String())
}

// viewTimezone renders the timezone selection screen
func (a *App) viewTimezone() string {
	title := titleStyle.Render("Timezone & Locale")
	subtitle := subtitleStyle.Render("Configure your timezone and language")

	timezones := []string{
		"UTC",
		"America/New_York",
		"America/Los_Angeles",
		"Europe/London",
		"Europe/Berlin",
		"Asia/Tokyo",
	}

	var tzList strings.Builder
	tzList.WriteString("Timezone:\n")
	for i, tz := range timezones {
		cursor := "  "
		style := normalStyle
		if i == a.focusIndex {
			cursor = "▸ "
			style = selectedStyle
		}
		tzList.WriteString(style.Render(fmt.Sprintf("%s%s\n", cursor, tz)))
	}

	locale := boxStyle.Render(fmt.Sprintf("Locale: %s\nKeymap: %s", a.config.Locale, a.config.Keymap))

	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s", title, subtitle, tzList.String(), locale)
}

// viewUsers renders the user configuration screen
func (a *App) viewUsers() string {
	title := titleStyle.Render("User Accounts")
	subtitle := subtitleStyle.Render("Configure user accounts")

	content := `Root Password:     [••••••••]
Create User:       [Yes]

Username:          [user         ]
Full Name:         [              ]
Password:          [••••••••      ]
Shell:             [/bin/bash     ]

Groups:            [x] wheel (sudo)
                   [x] audio
                   [x] video
                   [x] input

Privilege:         [sudo] / [doas]`

	return fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, boxStyle.Render(content))
}

// viewSummary renders the installation summary screen
func (a *App) viewSummary() string {
	title := titleStyle.Render("Installation Summary")
	subtitle := subtitleStyle.Render("Review your configuration before installing")

	summary := fmt.Sprintf(`
  Disk:           %s
  Encryption:     %s
  Init System:    %s

  Kernel:         %s
  Graphics:       %s
  Desktop:        %s

  Hostname:       %s
  Timezone:       %s
  Locale:         %s

  Packages:       %s
  Secure Boot:    %s
`,
		a.config.Disk.Device,
		a.config.Encryption.Type,
		a.config.InitSystem,
		a.config.Kernel.Type,
		a.config.Graphics.Driver,
		a.config.Desktop.Type,
		a.config.Hostname,
		a.config.Timezone,
		a.config.Locale,
		a.config.Packages.UseBinary,
		boolToYesNo(a.config.Bootloader.SecureBoot.Enabled),
	)

	warning := errorStyle.Render("\n⚠️  This will ERASE all data on the selected disk!")
	instruction := selectedStyle.Render("\nPress Enter to begin installation...")

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", title, subtitle, boxStyle.Render(summary), warning, instruction)
}

// viewInstall renders the installation progress screen
func (a *App) viewInstall() string {
	title := titleStyle.Render("Installing Yuno OS")

	steps := []string{
		"Partitioning disk",
		"Setting up encryption",
		"Installing stage3",
		"Configuring Portage",
		"Installing kernel",
		"Installing graphics drivers",
		"Installing desktop",
		"Creating users",
		"Installing bootloader",
		"Finalizing",
	}

	var stepList strings.Builder
	for i, step := range steps {
		status := "  "
		style := normalStyle
		if i < a.installStep {
			status = "✓ "
			style = progressCompleteStyle
		} else if i == a.installStep {
			status = a.spinner.View() + " "
			style = progressActiveStyle
		}
		stepList.WriteString(style.Render(fmt.Sprintf("%s%s\n", status, step)))
	}

	// Show recent log entries
	var logView strings.Builder
	logView.WriteString(helpStyle.Render("\nLog:\n"))
	start := len(a.installLog) - 5
	if start < 0 {
		start = 0
	}
	for _, line := range a.installLog[start:] {
		logView.WriteString(helpStyle.Render(line + "\n"))
	}

	return fmt.Sprintf("%s\n\n%s\n%s", title, stepList.String(), logView.String())
}

// viewComplete renders the installation complete screen
func (a *App) viewComplete() string {
	logo := `
    ✓ Installation Complete!
`
	title := titleStyle.Render("Yuno OS has been installed successfully!")

	content := boxStyle.Render(`What's next:

1. Remove the installation media
2. Reboot into your new system
3. Log in with your created user account
4. Run 'emerge --sync' to update the package database
5. Enjoy your new Gentoo-based system!

For help and documentation:
  https://wiki.gentoo.org
  https://github.com/japaneseenrichmentorganization/yuno_os`)

	instruction := selectedStyle.Render("\nPress Enter to reboot, or 'q' to exit...")

	return fmt.Sprintf("%s\n%s\n\n%s\n%s",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render(logo),
		title,
		content,
		instruction,
	)
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
