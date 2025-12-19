// Package binpkg handles binary package configuration.
package binpkg

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/utils"
)

// Manager handles binary package configuration.
type Manager struct {
	config    *config.InstallConfig
	targetDir string
}

// NewManager creates a new binary package manager.
func NewManager(cfg *config.InstallConfig, targetDir string) *Manager {
	return &Manager{
		config:    cfg,
		targetDir: targetDir,
	}
}

// BinaryHost represents a binary package host.
type BinaryHost struct {
	Name        string
	URL         string
	Description string
	Arch        string
	Profile     string
}

// OfficialBinaryHosts returns the official Gentoo binary hosts.
func OfficialBinaryHosts() []BinaryHost {
	return []BinaryHost{
		{
			Name:        "Gentoo Official (amd64)",
			URL:         "https://distfiles.gentoo.org/releases/amd64/binpackages/23.0/x86-64/",
			Description: "Official Gentoo binary packages for amd64",
			Arch:        "amd64",
			Profile:     "23.0",
		},
		{
			Name:        "Gentoo Official (amd64-v3)",
			URL:         "https://distfiles.gentoo.org/releases/amd64/binpackages/23.0/x86-64-v3/",
			Description: "Official Gentoo binary packages for amd64-v3 (AVX2+)",
			Arch:        "amd64",
			Profile:     "23.0/x86-64-v3",
		},
	}
}

// Configure sets up binary package support.
func (m *Manager) Configure() error {
	pref := m.config.Packages.UseBinary
	if pref == config.BinaryNone {
		return nil
	}

	utils.Info("Configuring binary package support")

	// Set up binrepos.conf
	if err := m.setupBinreposConf(); err != nil {
		return err
	}

	// Update make.conf for binary packages
	if err := m.updateMakeConf(); err != nil {
		return err
	}

	// Set up package.use/binpkg if needed
	if err := m.setupPackageAcceptRestrict(); err != nil {
		return err
	}

	return nil
}

// setupBinreposConf creates the binrepos.conf file.
func (m *Manager) setupBinreposConf() error {
	reposDir := filepath.Join(m.targetDir, "etc/portage/binrepos.conf")
	if err := utils.CreateDir(reposDir, 0755); err != nil {
		return err
	}

	host := m.config.Packages.BinaryHost
	if host == "" {
		// Use official host
		hosts := OfficialBinaryHosts()
		if len(hosts) > 0 {
			host = hosts[0].URL
		}
	}

	content := fmt.Sprintf(`# Yuno OS binary package repository

[binhost]
priority = 9999
sync-uri = %s
`, host)

	confPath := filepath.Join(reposDir, "gentoobinhost.conf")
	return utils.WriteFile(confPath, content, 0644)
}

// updateMakeConf updates make.conf for binary packages.
func (m *Manager) updateMakeConf() error {
	makeConfPath := filepath.Join(m.targetDir, "etc/portage/make.conf")

	content, err := utils.ReadFile(makeConfPath)
	if err != nil {
		return utils.NewError("binpkg", "failed to read make.conf", err)
	}

	// Add binary package settings
	var additions strings.Builder

	pref := m.config.Packages.UseBinary

	// Set FEATURES
	additions.WriteString("\n# Binary package configuration\n")

	switch pref {
	case config.BinaryPrefer:
		additions.WriteString("FEATURES=\"${FEATURES} getbinpkg binpkg-request-signature\"\n")
		additions.WriteString("EMERGE_DEFAULT_OPTS=\"${EMERGE_DEFAULT_OPTS} --binpkg-respect-use=y --binpkg-changed-deps=y\"\n")
	case config.BinaryOnly:
		additions.WriteString("FEATURES=\"${FEATURES} getbinpkg binpkg-request-signature\"\n")
		additions.WriteString("EMERGE_DEFAULT_OPTS=\"${EMERGE_DEFAULT_OPTS} --usepkg --binpkg-respect-use=y\"\n")
	}

	// Only add if not already configured
	if !strings.Contains(content, "getbinpkg") {
		content += additions.String()
		if err := utils.WriteFile(makeConfPath, content, 0644); err != nil {
			return utils.NewError("binpkg", "failed to update make.conf", err)
		}
	}

	return nil
}

// setupPackageAcceptRestrict sets up package-specific binary restrictions.
func (m *Manager) setupPackageAcceptRestrict() error {
	// Some packages should always be compiled from source
	restrictDir := filepath.Join(m.targetDir, "etc/portage/package.env")
	if err := utils.CreateDir(restrictDir, 0755); err != nil {
		return err
	}

	// Create environment file for source-only packages
	envDir := filepath.Join(m.targetDir, "etc/portage/env")
	if err := utils.CreateDir(envDir, 0755); err != nil {
		return err
	}

	// Source-only environment
	sourceOnlyEnv := `# Force compilation from source
EMERGE_DEFAULT_OPTS="${EMERGE_DEFAULT_OPTS/--usepkg/}"
`
	envPath := filepath.Join(envDir, "source-only.conf")
	if err := utils.WriteFile(envPath, sourceOnlyEnv, 0644); err != nil {
		return err
	}

	// Packages that should be compiled from source
	// (security-sensitive, optimization-sensitive)
	sourceOnlyPkgs := `# Packages that should be compiled from source
sys-libs/glibc source-only.conf
sys-devel/gcc source-only.conf
dev-libs/openssl source-only.conf
app-crypt/gnupg source-only.conf
`
	pkgEnvPath := filepath.Join(restrictDir, "source-only")
	return utils.WriteFile(pkgEnvPath, sourceOnlyPkgs, 0644)
}

// SyncBinhost synchronizes the binary package repository.
func (m *Manager) SyncBinhost() error {
	utils.Info("Synchronizing binary package repository")

	result := utils.RunInChroot(m.targetDir, "emaint", "sync", "--auto")
	if result.Error != nil {
		utils.Warn("Failed to sync binhost: %v", result.Error)
		// Non-fatal, continue anyway
	}

	return nil
}

// InstallPackage installs a package, preferring binary if available.
func (m *Manager) InstallPackage(pkg string, progress func(line string)) error {
	args := []string{m.targetDir, "emerge", "--ask=n"}

	// Add binary package preference based on config
	switch m.config.Packages.UseBinary {
	case config.BinaryPrefer:
		args = append(args, "--getbinpkg")
	case config.BinaryOnly:
		args = append(args, "--usepkg", "--getbinpkg")
	}

	args = append(args, pkg)

	if progress != nil {
		return utils.RunCommandWithOutput(progress, "chroot", args...)
	}

	result := utils.RunCommand("chroot", args...)
	if result.Error != nil {
		return utils.NewError("binpkg", fmt.Sprintf("failed to install %s", pkg), result.Error)
	}

	return nil
}

// BuildLocalBinpkg builds a binary package locally.
func (m *Manager) BuildLocalBinpkg(pkg string) error {
	utils.Info("Building binary package for %s", pkg)

	result := utils.RunInChroot(m.targetDir, "emerge", "--ask=n", "--buildpkg", pkg)
	if result.Error != nil {
		return utils.NewError("binpkg", fmt.Sprintf("failed to build binpkg for %s", pkg), result.Error)
	}

	return nil
}

// CleanBinpkgCache cleans the binary package cache.
func (m *Manager) CleanBinpkgCache() error {
	utils.Info("Cleaning binary package cache")

	result := utils.RunInChroot(m.targetDir, "eclean-pkg", "--deep")
	if result.Error != nil {
		// eclean might not be installed
		utils.Warn("Failed to clean binpkg cache: %v", result.Error)
	}

	return nil
}

// Setup performs binary package configuration.
func (m *Manager) Setup() error {
	if m.config.Packages.UseBinary == config.BinaryNone {
		return nil
	}

	return m.Configure()
}

// BinaryPreferenceDescriptions returns descriptions for binary preferences.
func BinaryPreferenceDescriptions() map[config.BinaryPreference]string {
	return map[config.BinaryPreference]string{
		config.BinaryNone:   "Source only - Compile everything from source",
		config.BinaryPrefer: "Prefer binary - Use binaries when available, compile otherwise",
		config.BinaryOnly:   "Binary only - Only install pre-built packages",
	}
}
