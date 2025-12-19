// Package overlays handles Gentoo overlay management.
package overlays

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/utils"
)

// Manager handles overlay operations.
type Manager struct {
	config    *config.InstallConfig
	targetDir string
}

// NewManager creates a new overlay manager.
func NewManager(cfg *config.InstallConfig, targetDir string) *Manager {
	return &Manager{
		config:    cfg,
		targetDir: targetDir,
	}
}

// Overlay represents a Gentoo overlay.
type Overlay struct {
	Name        string
	Location    string
	SyncType    string // git, rsync, mercurial
	SyncURI     string
	AutoSync    bool
	Priority    int
	Description string
}

// PredefinedOverlays contains well-known overlay configurations.
var PredefinedOverlays = map[string]Overlay{
	"lto": {
		Name:        "lto-overlay",
		SyncType:    "git",
		SyncURI:     "https://github.com/InBetweenNames/gentooLTO.git",
		AutoSync:    true,
		Priority:    50,
		Description: "Link-Time Optimization overlay with optimized ebuilds",
	},
	"guru": {
		Name:        "guru",
		SyncType:    "rsync",
		SyncURI:     "rsync://rsync.gentoo.org/guru",
		AutoSync:    true,
		Priority:    10,
		Description: "Gentoo User Repository - community contributed ebuilds",
	},
	"gentoo-zh": {
		Name:        "gentoo-zh",
		SyncType:    "git",
		SyncURI:     "https://github.com/gentoo-mirror/gentoo-zh.git",
		AutoSync:    true,
		Priority:    10,
		Description: "Chinese Gentoo overlay",
	},
	"steam": {
		Name:        "steam-overlay",
		SyncType:    "git",
		SyncURI:     "https://github.com/anyc/steam-overlay.git",
		AutoSync:    true,
		Priority:    10,
		Description: "Steam and gaming related packages",
	},
	"brave": {
		Name:        "brave-overlay",
		SyncType:    "git",
		SyncURI:     "https://gitlab.com/aspect/aspect-overlay.git",
		AutoSync:    true,
		Priority:    10,
		Description: "Brave browser overlay",
	},
	"wayland": {
		Name:        "wayland-desktop",
		SyncType:    "git",
		SyncURI:     "https://github.com/aspect-forks/wayland-desktop.git",
		AutoSync:    true,
		Priority:    10,
		Description: "Wayland desktop packages",
	},
	"gentoo-kernel": {
		Name:        "gentoo-kernel",
		SyncType:    "git",
		SyncURI:     "https://github.com/AdelieLinux/gentoo-kernel.git",
		AutoSync:    true,
		Priority:    10,
		Description: "Additional kernel sources",
	},
}

// ListAvailable returns a list of available predefined overlays.
func (m *Manager) ListAvailable() []Overlay {
	var overlays []Overlay
	for _, overlay := range PredefinedOverlays {
		overlays = append(overlays, overlay)
	}
	return overlays
}

// ListInstalled returns currently installed overlays.
func (m *Manager) ListInstalled() ([]Overlay, error) {
	result := m.runInChroot("eselect", "repository", "list", "-i")
	if result.Error != nil {
		return nil, utils.NewError("overlays", "failed to list installed overlays", result.Error)
	}

	var overlays []Overlay
	lines := strings.Split(result.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "No") {
			continue
		}

		// Parse line like "[1] gentoo (rsync)"
		var name, syncType string
		if n, _ := fmt.Sscanf(line, "[%*d] %s (%s)", &name, &syncType); n >= 1 {
			overlays = append(overlays, Overlay{
				Name:     strings.TrimSpace(name),
				SyncType: strings.TrimSuffix(strings.TrimSpace(syncType), ")"),
			})
		}
	}

	return overlays, nil
}

// EnsureEselectRepository installs eselect-repository if not present.
func (m *Manager) EnsureEselectRepository() error {
	if m.fileExists("/usr/bin/eselect") {
		// Check if repository module is available
		result := m.runInChroot("eselect", "repository", "list")
		if result.ExitCode == 0 {
			return nil
		}
	}

	utils.Info("Installing eselect-repository")
	result := m.runInChroot("emerge", "--ask=n", "app-eselect/eselect-repository", "dev-vcs/git")
	if result.Error != nil {
		return utils.NewError("overlays", "failed to install eselect-repository", result.Error)
	}

	return nil
}

// Add adds an overlay by name (predefined) or custom configuration.
func (m *Manager) Add(name string) error {
	// Check if it's a predefined overlay
	if overlay, ok := PredefinedOverlays[name]; ok {
		return m.AddCustom(overlay)
	}

	// Try to add from official list
	utils.Info("Adding overlay %s from official list", name)

	if err := m.EnsureEselectRepository(); err != nil {
		return err
	}

	result := m.runInChroot("eselect", "repository", "enable", name)
	if result.Error != nil {
		return utils.NewError("overlays", fmt.Sprintf("failed to enable overlay %s", name), result.Error)
	}

	return nil
}

// AddCustom adds a custom overlay.
func (m *Manager) AddCustom(overlay Overlay) error {
	utils.Info("Adding custom overlay %s", overlay.Name)

	if err := m.EnsureEselectRepository(); err != nil {
		return err
	}

	// For git overlays, use eselect repository add
	if overlay.SyncType == "git" {
		result := m.runInChroot("eselect", "repository", "add", overlay.Name, "git", overlay.SyncURI)
		if result.Error != nil {
			return utils.NewError("overlays", fmt.Sprintf("failed to add overlay %s", overlay.Name), result.Error)
		}
	} else {
		// For rsync overlays, enable from the list
		result := m.runInChroot("eselect", "repository", "enable", overlay.Name)
		if result.Error != nil {
			return utils.NewError("overlays", fmt.Sprintf("failed to enable overlay %s", overlay.Name), result.Error)
		}
	}

	return nil
}

// Remove removes an overlay.
func (m *Manager) Remove(name string) error {
	utils.Info("Removing overlay %s", name)

	result := m.runInChroot("eselect", "repository", "disable", "-f", name)
	if result.Error != nil {
		return utils.NewError("overlays", fmt.Sprintf("failed to remove overlay %s", name), result.Error)
	}

	return nil
}

// Sync synchronizes one or all overlays.
func (m *Manager) Sync(name string) error {
	if name == "" {
		utils.Info("Syncing all overlays")
		result := m.runInChroot("emaint", "sync", "-a")
		if result.Error != nil {
			return utils.NewError("overlays", "failed to sync overlays", result.Error)
		}
	} else {
		utils.Info("Syncing overlay %s", name)
		result := m.runInChroot("emaint", "sync", "-r", name)
		if result.Error != nil {
			return utils.NewError("overlays", fmt.Sprintf("failed to sync overlay %s", name), result.Error)
		}
	}

	return nil
}

// SetupLTO sets up the LTO overlay with proper configuration.
func (m *Manager) SetupLTO() error {
	utils.Info("Setting up LTO overlay")

	// Add the overlay
	if err := m.Add("lto"); err != nil {
		return err
	}

	// Sync the overlay
	if err := m.Sync("lto-overlay"); err != nil {
		return err
	}

	// Create necessary configuration
	configDir := filepath.Join(m.targetDir, "etc/portage/package.use")
	if err := utils.CreateDir(configDir, 0755); err != nil {
		return err
	}

	// Add LTO use flags
	ltoUse := `# LTO overlay configuration
*/* lto
sys-devel/gcc graphite lto pgo
sys-libs/glibc -lto
dev-qt/* -lto
`
	usePath := filepath.Join(configDir, "lto")
	if err := utils.WriteFile(usePath, ltoUse, 0644); err != nil {
		return utils.NewError("overlays", "failed to write LTO use flags", err)
	}

	// Create env file for LTO
	envDir := filepath.Join(m.targetDir, "etc/portage/env")
	if err := utils.CreateDir(envDir, 0755); err != nil {
		return err
	}

	ltoEnv := `# LTO compilation flags
CFLAGS="${CFLAGS} -flto=auto -ffat-lto-objects"
CXXFLAGS="${CXXFLAGS} -flto=auto -ffat-lto-objects"
LDFLAGS="${LDFLAGS} -flto=auto -fuse-linker-plugin"
`
	envPath := filepath.Join(envDir, "lto.conf")
	if err := utils.WriteFile(envPath, ltoEnv, 0644); err != nil {
		return utils.NewError("overlays", "failed to write LTO env", err)
	}

	// Create package.env to apply LTO
	pkgEnvDir := filepath.Join(m.targetDir, "etc/portage/package.env")
	if err := utils.CreateDir(pkgEnvDir, 0755); err != nil {
		return err
	}

	pkgEnv := `# Apply LTO to all packages
*/* lto.conf
# Packages that don't work with LTO
sys-libs/glibc -lto.conf
dev-qt/* -lto.conf
`
	pkgEnvPath := filepath.Join(pkgEnvDir, "lto")
	if err := utils.WriteFile(pkgEnvPath, pkgEnv, 0644); err != nil {
		return utils.NewError("overlays", "failed to write package.env", err)
	}

	utils.Info("LTO overlay configured successfully")
	return nil
}

// SetupFromConfig sets up overlays based on configuration.
func (m *Manager) SetupFromConfig() error {
	for _, overlayConfig := range m.config.Overlays {
		var overlay Overlay

		// Check if it's a predefined overlay
		if predefined, ok := PredefinedOverlays[overlayConfig.Name]; ok {
			overlay = predefined
		} else {
			overlay = Overlay{
				Name:     overlayConfig.Name,
				SyncType: overlayConfig.SyncType,
				SyncURI:  overlayConfig.URL,
				AutoSync: overlayConfig.AutoSync,
				Priority: overlayConfig.Priority,
			}
		}

		if overlayConfig.Name == "lto" || overlayConfig.Name == "lto-overlay" {
			if err := m.SetupLTO(); err != nil {
				return err
			}
		} else {
			if err := m.AddCustom(overlay); err != nil {
				return err
			}
		}
	}

	// Sync all overlays
	return m.Sync("")
}

// WriteReposConf generates the repos.conf file for an overlay.
func (m *Manager) WriteReposConf(overlay Overlay) error {
	reposDir := filepath.Join(m.targetDir, "etc/portage/repos.conf")
	if err := utils.CreateDir(reposDir, 0755); err != nil {
		return err
	}

	location := overlay.Location
	if location == "" {
		location = filepath.Join("/var/db/repos", overlay.Name)
	}

	autoSync := "yes"
	if !overlay.AutoSync {
		autoSync = "no"
	}

	content := fmt.Sprintf(`[%s]
location = %s
sync-type = %s
sync-uri = %s
auto-sync = %s
`, overlay.Name, location, overlay.SyncType, overlay.SyncURI, autoSync)

	if overlay.Priority > 0 {
		content += fmt.Sprintf("priority = %d\n", overlay.Priority)
	}

	confPath := filepath.Join(reposDir, overlay.Name+".conf")
	return utils.WriteFile(confPath, content, 0644)
}

// Helper functions

func (m *Manager) runInChroot(name string, args ...string) *utils.CommandResult {
	return utils.RunInChroot(m.targetDir, name, args...)
}

func (m *Manager) fileExists(path string) bool {
	fullPath := filepath.Join(m.targetDir, path)
	return utils.FileExists(fullPath)
}
