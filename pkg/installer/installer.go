// Package installer orchestrates the complete installation process.
package installer

import (
	"fmt"
	"strings"

	"github.com/japaneseenrichmentorganization/yuno_os/pkg/binpkg"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/bootloader"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/chroot"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/desktop"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/encryption"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/graphics"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/kernel"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/overlays"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/partition"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/portage"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/stage3"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/users"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/utils"
)

const (
	TargetDir = "/mnt/gentoo"
)

// Step represents an installation step.
type Step int

const (
	StepPartition Step = iota
	StepEncryption
	StepMountPartitions
	StepStage3
	StepChrootSetup
	StepPortageConfig
	StepPortageSync
	StepOverlays
	StepBasePackages
	StepKernel
	StepGraphics
	StepDesktop
	StepUsers
	StepBootloader
	StepFinalize
)

func (s Step) String() string {
	names := []string{
		"Partitioning disk",
		"Setting up encryption",
		"Mounting partitions",
		"Installing stage3",
		"Setting up chroot",
		"Configuring Portage",
		"Syncing Portage tree",
		"Adding overlays",
		"Installing base packages",
		"Installing kernel",
		"Configuring graphics",
		"Installing desktop",
		"Creating users",
		"Installing bootloader",
		"Finalizing installation",
	}
	if int(s) < len(names) {
		return names[s]
	}
	return "Unknown step"
}

// Installer orchestrates the installation process.
type Installer struct {
	config        *config.InstallConfig
	targetDir     string
	currentStep   Step
	progressCb    func(step Step, progress int, message string)
	outputCb      func(line string)
	chrootManager *chroot.Manager
	layout        *partition.PartitionLayout
}

// NewInstaller creates a new installer instance.
func NewInstaller(cfg *config.InstallConfig) *Installer {
	return &Installer{
		config:    cfg,
		targetDir: TargetDir,
	}
}

// SetProgressCallback sets the progress callback.
func (i *Installer) SetProgressCallback(cb func(step Step, progress int, message string)) {
	i.progressCb = cb
}

// SetOutputCallback sets the output callback for command output.
func (i *Installer) SetOutputCallback(cb func(line string)) {
	i.outputCb = cb
}

// progress reports progress.
func (i *Installer) progress(progress int, message string) {
	if i.progressCb != nil {
		i.progressCb(i.currentStep, progress, message)
	}
}

// output sends output line.
func (i *Installer) output(line string) {
	if i.outputCb != nil {
		i.outputCb(line)
	}
}

// Install performs the complete installation.
func (i *Installer) Install() error {
	steps := []func() error{
		i.partitionDisk,
		i.setupEncryption,
		i.mountPartitions,
		i.installStage3,
		i.setupChroot,
		i.configurePortage,
		i.syncPortage,
		i.setupOverlays,
		i.installBasePackages,
		i.installKernel,
		i.installGraphics,
		i.installDesktop,
		i.setupUsers,
		i.installBootloader,
		i.finalize,
	}

	for step, fn := range steps {
		i.currentStep = Step(step)
		i.progress(0, fmt.Sprintf("Starting: %s", i.currentStep))

		if err := fn(); err != nil {
			return fmt.Errorf("step %s failed: %w", i.currentStep, err)
		}

		i.progress(100, fmt.Sprintf("Completed: %s", i.currentStep))
	}

	return nil
}

// partitionDisk partitions the target disk.
func (i *Installer) partitionDisk() error {
	partMgr := partition.NewManager(i.config)

	isUEFI := utils.IsUEFI()
	useEncrypt := i.config.Encryption.Type != config.EncryptNone

	i.progress(10, "Creating partition layout")

	// Create auto layout
	layout, err := partMgr.CreateAutoLayout(i.config.Disk.Device, isUEFI, useEncrypt)
	if err != nil {
		return err
	}
	i.layout = layout

	i.progress(30, "Applying partition layout")

	// Apply layout
	if err := partMgr.ApplyLayout(i.config.Disk.Device, layout); err != nil {
		return err
	}

	i.progress(100, "Partitioning complete")
	return nil
}

// setupEncryption sets up disk encryption.
func (i *Installer) setupEncryption() error {
	if i.config.Encryption.Type == config.EncryptNone {
		i.progress(100, "Encryption not configured")
		return nil
	}

	encMgr := encryption.NewManager(i.config)

	i.progress(20, "Setting up LUKS encryption")

	// Find encrypted partition
	for _, part := range i.layout.Partitions {
		if part.Encrypt {
			device := getPartitionDevice(i.config.Disk.Device, part.Number)
			_, err := encMgr.SetupLUKS(device, "cryptroot", i.config.Encryption.Password)
			if err != nil {
				return err
			}
		}
	}

	i.progress(100, "Encryption setup complete")
	return nil
}

// mountPartitions mounts all partitions.
func (i *Installer) mountPartitions() error {
	partMgr := partition.NewManager(i.config)

	i.progress(50, "Mounting partitions")

	if err := partMgr.MountPartitions(i.config.Disk.Device, i.layout, i.targetDir); err != nil {
		return err
	}

	i.progress(100, "Partitions mounted")
	return nil
}

// installStage3 installs the stage3 tarball.
func (i *Installer) installStage3() error {
	stage3Mgr := stage3.NewManager(i.config, i.targetDir)

	i.progress(10, "Finding latest stage3")

	if err := stage3Mgr.Install(func(current, total int64, msg string) {
		i.output(msg)
	}); err != nil {
		return err
	}

	i.progress(100, "Stage3 installed")
	return nil
}

// setupChroot sets up the chroot environment.
func (i *Installer) setupChroot() error {
	i.chrootManager = chroot.NewManager(i.config, i.targetDir)

	i.progress(50, "Mounting chroot filesystems")

	if err := i.chrootManager.Setup(); err != nil {
		return err
	}

	i.progress(100, "Chroot ready")
	return nil
}

// configurePortage configures Portage and make.conf.
func (i *Installer) configurePortage() error {
	portageMgr := portage.NewManager(i.config, i.targetDir)

	i.progress(20, "Generating make.conf")

	if err := portageMgr.Setup(); err != nil {
		return err
	}

	// Setup binary packages if configured
	if i.config.Packages.UseBinary != config.BinaryNone {
		i.progress(50, "Configuring binary packages")
		binpkgMgr := binpkg.NewManager(i.config, i.targetDir)
		if err := binpkgMgr.Setup(); err != nil {
			return err
		}
	}

	i.progress(100, "Portage configured")
	return nil
}

// syncPortage syncs the Portage tree.
func (i *Installer) syncPortage() error {
	portageMgr := portage.NewManager(i.config, i.targetDir)

	i.progress(10, "Syncing Portage tree (this may take a while)")

	if err := portageMgr.SyncPortage(); err != nil {
		return err
	}

	i.progress(80, "Selecting profile")

	if err := portageMgr.SelectProfile(); err != nil {
		return err
	}

	i.progress(100, "Portage synced")
	return nil
}

// setupOverlays adds configured overlays.
func (i *Installer) setupOverlays() error {
	if len(i.config.Overlays) == 0 {
		i.progress(100, "No overlays to add")
		return nil
	}

	overlayMgr := overlays.NewManager(i.config, i.targetDir)

	i.progress(10, "Setting up overlays")

	if err := overlayMgr.SetupFromConfig(); err != nil {
		return err
	}

	i.progress(100, "Overlays configured")
	return nil
}

// installKernel installs the kernel.
func (i *Installer) installKernel() error {
	kernelMgr := kernel.NewManager(i.config, i.targetDir)

	i.progress(10, "Installing kernel")

	if err := kernelMgr.Setup(i.output); err != nil {
		return err
	}

	i.progress(100, "Kernel installed")
	return nil
}

// installGraphics installs graphics drivers.
func (i *Installer) installGraphics() error {
	if i.config.Graphics.Driver == "" {
		i.progress(100, "No specific graphics driver selected")
		return nil
	}

	graphicsMgr := graphics.NewManager(i.config, i.targetDir)

	i.progress(10, "Installing graphics drivers")

	if err := graphicsMgr.Setup(i.output); err != nil {
		return err
	}

	i.progress(100, "Graphics drivers installed")
	return nil
}

// installDesktop installs the desktop environment.
func (i *Installer) installDesktop() error {
	if i.config.Desktop.Type == config.DesktopNone {
		i.progress(100, "No desktop environment selected")
		return nil
	}

	desktopMgr := desktop.NewManager(i.config, i.targetDir)

	i.progress(10, "Installing desktop environment")

	if err := desktopMgr.Setup(i.output); err != nil {
		return err
	}

	i.progress(100, "Desktop environment installed")
	return nil
}

// setupUsers creates user accounts.
func (i *Installer) setupUsers() error {
	userMgr := users.NewManager(i.config, i.targetDir)

	i.progress(20, "Setting up users")

	if err := userMgr.Setup(); err != nil {
		return err
	}

	i.progress(100, "Users configured")
	return nil
}

// installBootloader installs the bootloader.
func (i *Installer) installBootloader() error {
	bootMgr := bootloader.NewManager(i.config, i.targetDir)

	i.progress(20, "Installing bootloader")

	if err := bootMgr.Setup(); err != nil {
		return err
	}

	i.progress(100, "Bootloader installed")
	return nil
}

// finalize performs final configuration steps.
func (i *Installer) finalize() error {
	i.progress(10, "Setting hostname")

	// Set hostname
	hostnamePath := i.targetDir + "/etc/hostname"
	if err := utils.WriteFile(hostnamePath, i.config.Hostname+"\n", 0644); err != nil {
		return err
	}

	// Set hosts
	i.progress(20, "Configuring /etc/hosts")
	hostsContent := fmt.Sprintf(`127.0.0.1	localhost
::1		localhost
127.0.1.1	%s.localdomain	%s
`, i.config.Hostname, i.config.Hostname)
	hostsPath := i.targetDir + "/etc/hosts"
	if err := utils.WriteFile(hostsPath, hostsContent, 0644); err != nil {
		return err
	}

	// Set timezone
	i.progress(30, "Setting timezone")
	if err := i.setTimezone(); err != nil {
		utils.Warn("Failed to set timezone: %v", err)
	}

	// Set locale
	i.progress(40, "Configuring locale")
	if err := i.setLocale(); err != nil {
		utils.Warn("Failed to set locale: %v", err)
	}

	// Set keymap
	i.progress(50, "Configuring keymap")
	if err := i.setKeymap(); err != nil {
		utils.Warn("Failed to set keymap: %v", err)
	}

	// Generate fstab
	i.progress(60, "Generating fstab")
	if err := i.generateFstab(); err != nil {
		return err
	}

	// Enable essential services
	i.progress(80, "Enabling services")
	if err := i.enableServices(); err != nil {
		utils.Warn("Failed to enable some services: %v", err)
	}

	// Cleanup
	i.progress(90, "Cleaning up")
	if i.chrootManager != nil {
		i.chrootManager.Teardown()
	}

	utils.SyncFilesystems()

	i.progress(100, "Installation complete!")
	return nil
}

// setTimezone sets the system timezone.
func (i *Installer) setTimezone() error {
	tz := i.config.Timezone
	if tz == "" {
		tz = "UTC"
	}

	// Remove existing localtime
	localtimePath := i.targetDir + "/etc/localtime"
	utils.RunCommand("rm", "-f", localtimePath)

	// Create symlink
	tzPath := "/usr/share/zoneinfo/" + tz
	return utils.RunCommand("ln", "-sf", tzPath, localtimePath).Error
}

// setLocale sets the system locale.
func (i *Installer) setLocale() error {
	locale := i.config.Locale
	if locale == "" {
		locale = "en_US.UTF-8"
	}

	// Configure locale.gen
	localeGenPath := i.targetDir + "/etc/locale.gen"
	localeContent := fmt.Sprintf("en_US.UTF-8 UTF-8\n%s UTF-8\n", locale)
	if err := utils.WriteFile(localeGenPath, localeContent, 0644); err != nil {
		return err
	}

	// Generate locales
	utils.RunInChroot(i.targetDir, "locale-gen")

	// Set default locale
	localeConfPath := i.targetDir + "/etc/locale.conf"
	localeConfContent := fmt.Sprintf("LANG=%s\n", locale)
	return utils.WriteFile(localeConfPath, localeConfContent, 0644)
}

// setKeymap sets the keyboard layout.
func (i *Installer) setKeymap() error {
	keymap := i.config.Keymap
	if keymap == "" {
		keymap = "us"
	}

	if i.config.InitSystem == config.InitSystemd {
		// systemd uses vconsole.conf
		vconsoleContent := fmt.Sprintf("KEYMAP=%s\n", keymap)
		return utils.WriteFile(i.targetDir+"/etc/vconsole.conf", vconsoleContent, 0644)
	}

	// OpenRC uses conf.d/keymaps
	keymapContent := fmt.Sprintf(`keymap="%s"
`, keymap)
	return utils.WriteFile(i.targetDir+"/etc/conf.d/keymaps", keymapContent, 0644)
}

// generateFstab generates /etc/fstab.
func (i *Installer) generateFstab() error {
	var fstab strings.Builder

	fstab.WriteString("# /etc/fstab: static file system information.\n")
	fstab.WriteString("# <file system> <mount point> <type> <options> <dump> <pass>\n\n")

	for _, part := range i.layout.Partitions {
		if part.MountPoint == "" {
			continue
		}

		device := getPartitionDevice(i.config.Disk.Device, part.Number)

		// Get UUID
		result := utils.RunCommand("blkid", "-s", "UUID", "-o", "value", device)
		uuid := strings.TrimSpace(result.Stdout)

		fsType := string(part.Filesystem)
		if fsType == "fat32" {
			fsType = "vfat"
		}

		options := "defaults"
		dump := "0"
		pass := "0"

		switch part.MountPoint {
		case "/":
			options = "defaults,noatime"
			pass = "1"
		case "/boot":
			if part.Filesystem == config.FSFat32 {
				options = "defaults,umask=0077"
			}
			pass = "2"
		case "/home":
			options = "defaults,noatime"
			pass = "2"
		}

		if uuid != "" {
			fstab.WriteString(fmt.Sprintf("UUID=%s\t%s\t%s\t%s\t%s\t%s\n",
				uuid, part.MountPoint, fsType, options, dump, pass))
		} else {
			fstab.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\n",
				device, part.MountPoint, fsType, options, dump, pass))
		}
	}

	// Add swap
	for _, part := range i.layout.Partitions {
		if part.Filesystem == config.FSSwap {
			device := getPartitionDevice(i.config.Disk.Device, part.Number)
			result := utils.RunCommand("blkid", "-s", "UUID", "-o", "value", device)
			uuid := strings.TrimSpace(result.Stdout)

			if uuid != "" {
				fstab.WriteString(fmt.Sprintf("UUID=%s\tnone\tswap\tsw\t0\t0\n", uuid))
			} else {
				fstab.WriteString(fmt.Sprintf("%s\tnone\tswap\tsw\t0\t0\n", device))
			}
		}
	}

	fstabPath := i.targetDir + "/etc/fstab"
	return utils.WriteFile(fstabPath, fstab.String(), 0644)
}

// enableServices enables essential system services.
func (i *Installer) enableServices() error {
	services := []string{"sshd", "metalog"}

	// Add init-specific services
	if i.config.InitSystem == config.InitSystemd {
		services = append(services, "NetworkManager")
	} else {
		services = append(services, "NetworkManager", "dbus")
	}

	for _, svc := range services {
		if i.config.InitSystem == config.InitSystemd {
			utils.RunInChroot(i.targetDir, "systemctl", "enable", svc)
		} else {
			utils.RunInChroot(i.targetDir, "rc-update", "add", svc, "default")
		}
	}

	return nil
}

// installBasePackages installs essential base packages including metalog.
func (i *Installer) installBasePackages() error {
	i.progress(10, "Installing metalog (logging daemon)")

	// Install metalog - simple logger with built-in rotation
	result := utils.RunInChroot(i.targetDir, "emerge", "--ask=n", "--quiet-build", "app-admin/metalog")
	if result.Error != nil {
		return utils.NewError("installer", "failed to install metalog", result.Error)
	}

	return nil
}

// Helper functions

func getPartitionDevice(disk string, partNum int) string {
	if containsAny(disk, "nvme", "mmcblk") {
		return fmt.Sprintf("%sp%d", disk, partNum)
	}
	return fmt.Sprintf("%s%d", disk, partNum)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
