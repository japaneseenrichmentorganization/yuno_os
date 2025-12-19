// Package desktop handles desktop environment and window manager installation.
package desktop

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/utils"
)

// Manager handles desktop environment configuration.
type Manager struct {
	config    *config.InstallConfig
	targetDir string
}

// NewManager creates a new desktop manager.
func NewManager(cfg *config.InstallConfig, targetDir string) *Manager {
	return &Manager{
		config:    cfg,
		targetDir: targetDir,
	}
}

// Install installs the selected desktop environment or window manager.
func (m *Manager) Install(progress func(line string)) error {
	desktop := m.config.Desktop.Type

	if desktop == config.DesktopNone {
		utils.Info("No desktop environment selected")
		return nil
	}

	utils.Info("Installing desktop: %s", desktop)

	// Get packages for the desktop
	packages := desktop.GetPackages()

	// Add display manager
	dm := m.config.Desktop.DisplayManager
	if dm != config.DMNone {
		dmPkg := dm.GetPackage()
		if dmPkg != "" {
			packages = append(packages, dmPkg)
		}
	}

	// Add session dependencies
	if m.config.Desktop.SessionType == config.DisplayWayland {
		packages = append(packages, m.getWaylandPackages()...)
	} else {
		packages = append(packages, m.getX11Packages()...)
	}

	// Add extra packages from config
	packages = append(packages, m.config.Desktop.ExtraPackages...)

	// Add common utilities
	packages = append(packages, m.getCommonPackages()...)

	// Remove duplicates
	packages = uniqueStrings(packages)

	// Install packages
	args := append([]string{m.targetDir, "emerge", "--ask=n"}, packages...)

	if progress != nil {
		if err := utils.RunCommandWithOutput(progress, "chroot", args...); err != nil {
			return utils.NewError("desktop", "failed to install desktop", err)
		}
	} else {
		result := utils.RunCommand("chroot", args...)
		if result.Error != nil {
			return utils.NewError("desktop", "failed to install desktop", result.Error)
		}
	}

	return nil
}

// getWaylandPackages returns Wayland session packages.
func (m *Manager) getWaylandPackages() []string {
	packages := []string{
		"dev-libs/wayland",
		"dev-libs/wayland-protocols",
		"gui-libs/wlroots",
		"x11-libs/libxkbcommon",
	}

	// Add XWayland for compatibility
	packages = append(packages, "x11-base/xwayland")

	return packages
}

// getX11Packages returns X11 session packages.
func (m *Manager) getX11Packages() []string {
	return []string{
		"x11-base/xorg-server",
		"x11-apps/xinit",
		"x11-apps/xrandr",
		"x11-apps/setxkbmap",
	}
}

// getCommonPackages returns common utility packages.
func (m *Manager) getCommonPackages() []string {
	packages := []string{
		"app-misc/neofetch",
		"sys-apps/dbus",
		"media-sound/pulseaudio", // or pipewire
		"net-misc/networkmanager",
	}

	// Use pipewire for Wayland
	if m.config.Desktop.SessionType == config.DisplayWayland {
		packages = append(packages, "media-video/pipewire", "media-video/wireplumber")
	}

	return packages
}

// ConfigureDisplayManager configures the display manager.
func (m *Manager) ConfigureDisplayManager() error {
	dm := m.config.Desktop.DisplayManager
	if dm == config.DMNone {
		return nil
	}

	utils.Info("Configuring display manager: %s", dm)

	// Enable the service
	var serviceName string
	switch dm {
	case config.DMSDDM:
		serviceName = "sddm"
		if err := m.configureSDDM(); err != nil {
			return err
		}
	case config.DMGDM:
		serviceName = "gdm"
	case config.DMLightDM:
		serviceName = "lightdm"
		if err := m.configureLightDM(); err != nil {
			return err
		}
	case config.DMLXDM:
		serviceName = "lxdm"
	}

	return m.enableService(serviceName)
}

// configureSDDM configures SDDM display manager.
func (m *Manager) configureSDDM() error {
	sddmDir := filepath.Join(m.targetDir, "etc/sddm.conf.d")
	if err := utils.CreateDir(sddmDir, 0755); err != nil {
		return err
	}

	content := `[General]
HaltCommand=/sbin/shutdown -h now
RebootCommand=/sbin/shutdown -r now

[Theme]
Current=breeze

[Users]
MaximumUid=60000
MinimumUid=1000
`

	// Add Wayland session if configured
	if m.config.Desktop.SessionType == config.DisplayWayland {
		content += `
[Wayland]
SessionDir=/usr/share/wayland-sessions
`
	}

	confPath := filepath.Join(sddmDir, "yuno.conf")
	return utils.WriteFile(confPath, content, 0644)
}

// configureLightDM configures LightDM display manager.
func (m *Manager) configureLightDM() error {
	content := `[Seat:*]
greeter-session=lightdm-gtk-greeter
user-session=default
`

	confPath := filepath.Join(m.targetDir, "etc/lightdm/lightdm.conf.d/yuno.conf")
	if err := utils.CreateDir(filepath.Dir(confPath), 0755); err != nil {
		return err
	}

	return utils.WriteFile(confPath, content, 0644)
}

// enableService enables a service based on init system.
func (m *Manager) enableService(name string) error {
	if m.config.InitSystem == config.InitSystemd {
		result := utils.RunInChroot(m.targetDir, "systemctl", "enable", name)
		if result.Error != nil {
			return utils.NewError("desktop", fmt.Sprintf("failed to enable %s", name), result.Error)
		}
	} else {
		result := utils.RunInChroot(m.targetDir, "rc-update", "add", name, "default")
		if result.Error != nil {
			return utils.NewError("desktop", fmt.Sprintf("failed to enable %s", name), result.Error)
		}
	}

	return nil
}

// ConfigureSession sets up the default session.
func (m *Manager) ConfigureSession() error {
	utils.Info("Configuring session")

	desktop := m.config.Desktop.Type
	if desktop == config.DesktopNone {
		return nil
	}

	// Create .xinitrc or Wayland session launcher for WM users
	switch desktop {
	case config.WMi3:
		return m.createXinitrc("exec i3")
	case config.WMSway:
		return m.createWaylandLauncher("sway")
	case config.WMHyprland:
		return m.createWaylandLauncher("Hyprland")
	case config.WMBspwm:
		return m.createXinitrc("exec bspwm")
	case config.WMDwm:
		return m.createXinitrc("exec dwm")
	case config.WMAwesome:
		return m.createXinitrc("exec awesome")
	case config.WMOpenbox:
		return m.createXinitrc("exec openbox-session")
	}

	return nil
}

// createXinitrc creates a .xinitrc file.
func (m *Manager) createXinitrc(exec string) error {
	content := fmt.Sprintf(`#!/bin/sh
# Yuno OS xinitrc

# Source system xinitrc scripts
if [ -d /etc/X11/xinit/xinitrc.d ]; then
    for f in /etc/X11/xinit/xinitrc.d/?*.sh; do
        [ -x "$f" ] && . "$f"
    done
fi

# Set keyboard layout
setxkbmap %s

# Start the window manager
%s
`, m.config.Keymap, exec)

	// Write to /etc/skel for new users
	skelPath := filepath.Join(m.targetDir, "etc/skel/.xinitrc")
	return utils.WriteFile(skelPath, content, 0644)
}

// createWaylandLauncher creates a Wayland session launcher.
func (m *Manager) createWaylandLauncher(compositor string) error {
	content := fmt.Sprintf(`#!/bin/sh
# Yuno OS Wayland launcher

# Set environment variables
export XDG_SESSION_TYPE=wayland
export XDG_CURRENT_DESKTOP=%s
export MOZ_ENABLE_WAYLAND=1
export QT_QPA_PLATFORM=wayland

# Start the compositor
exec %s
`, strings.ToUpper(compositor), strings.ToLower(compositor))

	scriptPath := filepath.Join(m.targetDir, "etc/skel/.local/bin/start-"+strings.ToLower(compositor))
	if err := utils.CreateDir(filepath.Dir(scriptPath), 0755); err != nil {
		return err
	}

	return utils.WriteFile(scriptPath, content, 0755)
}

// ConfigureNetworkManager configures NetworkManager.
func (m *Manager) ConfigureNetworkManager() error {
	utils.Info("Configuring NetworkManager")

	// Enable NetworkManager service
	if err := m.enableService("NetworkManager"); err != nil {
		return err
	}

	// Create basic configuration
	nmDir := filepath.Join(m.targetDir, "etc/NetworkManager/conf.d")
	if err := utils.CreateDir(nmDir, 0755); err != nil {
		return err
	}

	content := `[main]
plugins=keyfile

[keyfile]
unmanaged-devices=interface-name:lo
`

	confPath := filepath.Join(nmDir, "yuno.conf")
	return utils.WriteFile(confPath, content, 0644)
}

// ConfigureAudio configures audio (PipeWire or PulseAudio).
func (m *Manager) ConfigureAudio() error {
	utils.Info("Configuring audio")

	if m.config.Desktop.SessionType == config.DisplayWayland {
		// PipeWire for Wayland
		return m.configurePipeWire()
	}

	// PulseAudio for X11
	return m.enableService("pulseaudio")
}

// configurePipeWire configures PipeWire audio system.
func (m *Manager) configurePipeWire() error {
	// Enable services
	services := []string{"pipewire", "pipewire-pulse", "wireplumber"}

	for _, service := range services {
		if m.config.InitSystem == config.InitSystemd {
			// For systemd, these are user services
			// Just ensure the packages are installed
			continue
		}
	}

	return nil
}

// Setup performs complete desktop setup.
func (m *Manager) Setup(progress func(line string)) error {
	// Install desktop packages
	if err := m.Install(progress); err != nil {
		return err
	}

	// Configure display manager
	if err := m.ConfigureDisplayManager(); err != nil {
		return err
	}

	// Configure session
	if err := m.ConfigureSession(); err != nil {
		return err
	}

	// Configure NetworkManager
	if err := m.ConfigureNetworkManager(); err != nil {
		return err
	}

	// Configure audio
	if err := m.ConfigureAudio(); err != nil {
		return err
	}

	// Enable essential services
	essentialServices := []string{"dbus"}
	for _, svc := range essentialServices {
		if err := m.enableService(svc); err != nil {
			utils.Warn("Failed to enable %s: %v", svc, err)
		}
	}

	return nil
}

// DesktopDescriptions returns descriptions for each desktop type.
func DesktopDescriptions() map[config.DesktopType]string {
	return map[config.DesktopType]string{
		config.DesktopKDE:      "KDE Plasma - Full-featured, modern desktop",
		config.DesktopGNOME:    "GNOME - Clean, simple, touch-friendly",
		config.DesktopXFCE:     "XFCE - Lightweight, traditional desktop",
		config.DesktopLXQt:     "LXQt - Lightweight Qt-based desktop",
		config.DesktopCinnamon: "Cinnamon - Traditional, GNOME-based",
		config.DesktopMATE:     "MATE - Traditional, GNOME 2 fork",
		config.DesktopBudgie:   "Budgie - Modern, elegant desktop",
		config.WMi3:            "i3 - Tiling window manager",
		config.WMSway:          "Sway - i3-compatible Wayland compositor",
		config.WMHyprland:      "Hyprland - Dynamic Wayland compositor",
		config.WMBspwm:         "bspwm - Binary space partitioning WM",
		config.WMDwm:           "dwm - Dynamic window manager",
		config.WMAwesome:       "Awesome - Highly configurable WM",
		config.WMOpenbox:       "Openbox - Minimalist stacking WM",
		config.DesktopNone:     "None - Server/minimal installation",
	}
}

// DisplayManagerDescriptions returns descriptions for display managers.
func DisplayManagerDescriptions() map[config.DisplayManager]string {
	return map[config.DisplayManager]string{
		config.DMSDDM:    "SDDM - Simple Desktop Display Manager (KDE default)",
		config.DMGDM:     "GDM - GNOME Display Manager",
		config.DMLightDM: "LightDM - Lightweight, flexible",
		config.DMLXDM:    "LXDM - LXDE Display Manager",
		config.DMNone:    "None - TTY login / startx",
	}
}

// GetRecommendedDM returns the recommended display manager for a desktop.
func GetRecommendedDM(desktop config.DesktopType) config.DisplayManager {
	switch desktop {
	case config.DesktopKDE:
		return config.DMSDDM
	case config.DesktopGNOME:
		return config.DMGDM
	case config.DesktopXFCE, config.DesktopLXQt, config.DesktopMATE, config.DesktopCinnamon:
		return config.DMLightDM
	case config.WMi3, config.WMSway, config.WMHyprland, config.WMBspwm, config.WMDwm:
		return config.DMNone // WM users often prefer startx
	default:
		return config.DMNone
	}
}

// uniqueStrings removes duplicates from a string slice.
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}
