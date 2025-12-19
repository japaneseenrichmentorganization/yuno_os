// Package chroot handles chroot environment setup and management.
package chroot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/utils"
)

// Manager handles chroot operations.
type Manager struct {
	config    *config.InstallConfig
	targetDir string
	mounted   []string
}

// NewManager creates a new chroot manager.
func NewManager(cfg *config.InstallConfig, targetDir string) *Manager {
	return &Manager{
		config:    cfg,
		targetDir: targetDir,
		mounted:   []string{},
	}
}

// MountPoint represents a mount point for chroot.
type MountPoint struct {
	Source string
	Target string
	FSType string
	Flags  string
	Bind   bool
}

// DefaultMounts returns the default mount points for a chroot environment.
func DefaultMounts(targetDir string) []MountPoint {
	return []MountPoint{
		{Source: "proc", Target: filepath.Join(targetDir, "proc"), FSType: "proc"},
		{Source: "sysfs", Target: filepath.Join(targetDir, "sys"), FSType: "sysfs"},
		{Source: "devtmpfs", Target: filepath.Join(targetDir, "dev"), FSType: "devtmpfs"},
		{Source: "devpts", Target: filepath.Join(targetDir, "dev/pts"), FSType: "devpts", Flags: "gid=5,mode=620"},
		{Source: "tmpfs", Target: filepath.Join(targetDir, "dev/shm"), FSType: "tmpfs", Flags: "nosuid,nodev"},
		{Source: "/run", Target: filepath.Join(targetDir, "run"), Bind: true},
		{Source: "tmpfs", Target: filepath.Join(targetDir, "tmp"), FSType: "tmpfs"},
		{Source: "efivarfs", Target: filepath.Join(targetDir, "sys/firmware/efi/efivars"), FSType: "efivarfs"},
	}
}

// Setup prepares the chroot environment.
func (m *Manager) Setup() error {
	utils.Info("Setting up chroot environment at %s", m.targetDir)

	mounts := DefaultMounts(m.targetDir)

	for _, mount := range mounts {
		// Skip efivarfs if not UEFI
		if mount.FSType == "efivarfs" && !utils.IsUEFI() {
			continue
		}

		// Create mount point directory
		if err := utils.CreateDir(mount.Target, 0755); err != nil {
			return utils.NewError("chroot", fmt.Sprintf("failed to create %s", mount.Target), err)
		}

		// Skip if already mounted
		if utils.IsMounted(mount.Target) {
			utils.Debug("Already mounted: %s", mount.Target)
			continue
		}

		var err error
		if mount.Bind {
			err = utils.BindMount(mount.Source, mount.Target)
		} else {
			err = utils.Mount(mount.Source, mount.Target, mount.FSType, mount.Flags)
		}

		if err != nil {
			// efivarfs might fail, that's okay
			if mount.FSType == "efivarfs" {
				utils.Warn("Could not mount efivarfs (non-fatal)")
				continue
			}
			return err
		}

		m.mounted = append(m.mounted, mount.Target)
	}

	// Copy DNS resolution
	if err := m.copyResolv(); err != nil {
		utils.Warn("Failed to copy resolv.conf: %v", err)
	}

	return nil
}

// copyResolv copies /etc/resolv.conf into the chroot.
func (m *Manager) copyResolv() error {
	srcPath := "/etc/resolv.conf"
	dstPath := filepath.Join(m.targetDir, "etc/resolv.conf")

	// If source is a symlink, we might need to copy the actual file
	srcInfo, err := os.Lstat(srcPath)
	if err != nil {
		return err
	}

	// Create /etc directory if it doesn't exist
	if err := utils.CreateDir(filepath.Join(m.targetDir, "etc"), 0755); err != nil {
		return err
	}

	if srcInfo.Mode()&os.ModeSymlink != 0 {
		// It's a symlink, resolve it
		realPath, err := filepath.EvalSymlinks(srcPath)
		if err != nil {
			return err
		}
		srcPath = realPath
	}

	// Remove existing resolv.conf if it exists
	os.Remove(dstPath)

	return utils.CopyFile(srcPath, dstPath)
}

// Teardown unmounts all chroot filesystems.
func (m *Manager) Teardown() error {
	utils.Info("Tearing down chroot environment")

	// Unmount in reverse order
	for i := len(m.mounted) - 1; i >= 0; i-- {
		mount := m.mounted[i]
		if utils.IsMounted(mount) {
			if err := utils.Unmount(mount); err != nil {
				utils.Warn("Failed to unmount %s: %v", mount, err)
			}
		}
	}

	m.mounted = []string{}
	return nil
}

// Run executes a command inside the chroot.
func (m *Manager) Run(name string, args ...string) *utils.CommandResult {
	return utils.RunInChroot(m.targetDir, name, args...)
}

// RunWithEnv executes a command inside the chroot with environment variables.
func (m *Manager) RunWithEnv(env map[string]string, name string, args ...string) *utils.CommandResult {
	return utils.RunInChrootWithEnv(m.targetDir, env, name, args...)
}

// RunInteractive executes an interactive shell in the chroot.
func (m *Manager) RunInteractive() error {
	utils.Info("Entering interactive chroot shell")

	// Set up environment
	env := map[string]string{
		"HOME":   "/root",
		"TERM":   os.Getenv("TERM"),
		"PS1":    "(chroot) \\u@\\h:\\w$ ",
		"PATH":   "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}

	result := utils.RunInChrootWithEnv(m.targetDir, env, "/bin/bash", "--login")
	return result.Error
}

// Emerge runs emerge in the chroot.
func (m *Manager) Emerge(packages ...string) error {
	args := append([]string{"--ask=n", "--quiet-build"}, packages...)

	result := m.RunWithEnv(map[string]string{
		"USE":             m.config.Portage.UseFlags[0], // This needs proper handling
		"FEATURES":        "parallel-fetch",
		"EMERGE_DEFAULT_OPTS": "--jobs=4 --load-average=4",
	}, "emerge", args...)

	if result.Error != nil {
		return utils.NewError("chroot", fmt.Sprintf("emerge failed: %s", result.Stderr), result.Error)
	}

	return nil
}

// EmergeWithOutput runs emerge with output streaming.
func (m *Manager) EmergeWithOutput(callback func(line string), packages ...string) error {
	args := append([]string{m.targetDir, "emerge", "--ask=n"}, packages...)

	return utils.RunCommandWithOutput(callback, "chroot", args...)
}

// WriteFile writes a file inside the chroot.
func (m *Manager) WriteFile(path, content string, perm os.FileMode) error {
	fullPath := filepath.Join(m.targetDir, path)
	return utils.WriteFile(fullPath, content, perm)
}

// ReadFile reads a file from the chroot.
func (m *Manager) ReadFile(path string) (string, error) {
	fullPath := filepath.Join(m.targetDir, path)
	return utils.ReadFile(fullPath)
}

// AppendFile appends content to a file inside the chroot.
func (m *Manager) AppendFile(path, content string) error {
	fullPath := filepath.Join(m.targetDir, path)
	return utils.AppendToFile(fullPath, content)
}

// FileExists checks if a file exists inside the chroot.
func (m *Manager) FileExists(path string) bool {
	fullPath := filepath.Join(m.targetDir, path)
	return utils.FileExists(fullPath)
}

// CreateDir creates a directory inside the chroot.
func (m *Manager) CreateDir(path string, perm os.FileMode) error {
	fullPath := filepath.Join(m.targetDir, path)
	return utils.CreateDir(fullPath, perm)
}

// Symlink creates a symbolic link inside the chroot.
func (m *Manager) Symlink(oldname, newname string) error {
	fullNewname := filepath.Join(m.targetDir, newname)

	// Remove existing symlink if present
	os.Remove(fullNewname)

	return os.Symlink(oldname, fullNewname)
}

// SourceProfile sources /etc/profile in the chroot.
func (m *Manager) SourceProfile() *utils.CommandResult {
	return m.Run("/bin/bash", "-c", "source /etc/profile")
}

// SyncPortage syncs the portage tree.
func (m *Manager) SyncPortage() error {
	utils.Info("Syncing Portage tree")

	result := m.Run("emerge-webrsync")
	if result.Error != nil {
		return utils.NewError("chroot", "failed to sync portage", result.Error)
	}

	return nil
}

// SelectProfile selects a Gentoo profile.
func (m *Manager) SelectProfile(profile string) error {
	utils.Info("Selecting profile: %s", profile)

	// List profiles to find the number
	result := m.Run("eselect", "profile", "list")
	if result.Error != nil {
		return utils.NewError("chroot", "failed to list profiles", result.Error)
	}

	// Find the profile number
	lines := filepath.SplitList(result.Stdout)
	var profileNum string
	for _, line := range lines {
		if filepath.Base(line) == profile || line == profile {
			// Extract number from line like "[23]  default/linux/amd64/23.0/desktop"
			fmt.Sscanf(line, "[%s]", &profileNum)
			break
		}
	}

	// Set the profile
	if profileNum != "" {
		result = m.Run("eselect", "profile", "set", profileNum)
	} else {
		// Try setting by name
		result = m.Run("eselect", "profile", "set", profile)
	}

	if result.Error != nil {
		return utils.NewError("chroot", "failed to set profile", result.Error)
	}

	return nil
}

// UpdateEnvironment updates the environment after profile changes.
func (m *Manager) UpdateEnvironment() error {
	utils.Info("Updating environment")

	result := m.Run("env-update")
	if result.Error != nil {
		return utils.NewError("chroot", "failed to update environment", result.Error)
	}

	return nil
}
