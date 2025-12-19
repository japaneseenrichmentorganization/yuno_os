// Package kernel handles kernel installation and configuration.
package kernel

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/utils"
)

// Manager handles kernel operations.
type Manager struct {
	config    *config.InstallConfig
	targetDir string
}

// NewManager creates a new kernel manager.
func NewManager(cfg *config.InstallConfig, targetDir string) *Manager {
	return &Manager{
		config:    cfg,
		targetDir: targetDir,
	}
}

// KernelInfo contains information about an installed kernel.
type KernelInfo struct {
	Version   string
	Type      config.KernelType
	Path      string
	Initramfs string
}

// Install installs the kernel based on configuration.
func (m *Manager) Install(progress func(line string)) error {
	kernelType := m.config.Kernel.Type

	utils.Info("Installing kernel: %s", kernelType)

	switch kernelType {
	case config.KernelBin:
		return m.installDistKernel("sys-kernel/gentoo-kernel-bin", progress)
	case config.KernelDist:
		return m.installDistKernel("sys-kernel/gentoo-kernel", progress)
	case config.KernelSources:
		return m.installSources("sys-kernel/gentoo-sources", progress)
	case config.KernelZen:
		return m.installSources("sys-kernel/zen-sources", progress)
	case config.KernelXanmod:
		return m.installSources("sys-kernel/xanmod-sources", progress)
	case config.KernelLiquorix:
		return m.installSources("sys-kernel/liquorix-sources", progress)
	case config.KernelVanilla:
		return m.installSources("sys-kernel/vanilla-sources", progress)
	default:
		return m.installDistKernel("sys-kernel/gentoo-kernel-bin", progress)
	}
}

// installDistKernel installs a distribution kernel (pre-configured).
func (m *Manager) installDistKernel(pkg string, progress func(line string)) error {
	utils.Info("Installing distribution kernel: %s", pkg)

	// Install the kernel package
	args := []string{m.targetDir, "emerge", "--ask=n"}

	// Add installkernel for initramfs generation
	packages := []string{pkg, "sys-kernel/installkernel"}

	// Add initramfs generator based on config
	switch m.config.Kernel.Initramfs {
	case "dracut":
		packages = append(packages, "sys-kernel/dracut")
	case "genkernel":
		packages = append(packages, "sys-kernel/genkernel")
	default:
		packages = append(packages, "sys-kernel/dracut")
	}

	args = append(args, packages...)

	if progress != nil {
		if err := utils.RunCommandWithOutput(progress, "chroot", args...); err != nil {
			return utils.NewError("kernel", "failed to install kernel", err)
		}
	} else {
		result := utils.RunCommand("chroot", args...)
		if result.Error != nil {
			return utils.NewError("kernel", "failed to install kernel", result.Error)
		}
	}

	return nil
}

// installSources installs kernel sources and builds with genkernel.
func (m *Manager) installSources(pkg string, progress func(line string)) error {
	utils.Info("Installing kernel sources: %s", pkg)

	// Install kernel sources and genkernel
	args := []string{m.targetDir, "emerge", "--ask=n", pkg, "sys-kernel/genkernel"}

	if progress != nil {
		if err := utils.RunCommandWithOutput(progress, "chroot", args...); err != nil {
			return utils.NewError("kernel", "failed to install kernel sources", err)
		}
	} else {
		result := utils.RunCommand("chroot", args...)
		if result.Error != nil {
			return utils.NewError("kernel", "failed to install kernel sources", result.Error)
		}
	}

	// Build kernel with genkernel
	return m.buildWithGenkernel(progress)
}

// buildWithGenkernel builds the kernel using genkernel.
func (m *Manager) buildWithGenkernel(progress func(line string)) error {
	utils.Info("Building kernel with genkernel")

	args := []string{m.targetDir, "genkernel", "all"}

	// Add options based on encryption
	if m.config.Encryption.Type != config.EncryptNone {
		args = append(args, "--luks")
	}

	// Add custom config if specified
	if m.config.Kernel.CustomConfig != "" {
		args = append(args, "--kernel-config="+m.config.Kernel.CustomConfig)
	}

	// Add module options
	for _, mod := range m.config.Kernel.Modules {
		args = append(args, "--kernel-append-localversion=-"+mod)
	}

	if progress != nil {
		return utils.RunCommandWithOutput(progress, "chroot", args...)
	}

	result := utils.RunCommand("chroot", args...)
	if result.Error != nil {
		return utils.NewError("kernel", "genkernel build failed", result.Error)
	}

	return nil
}

// GenerateInitramfs generates an initramfs using the configured method.
func (m *Manager) GenerateInitramfs() error {
	utils.Info("Generating initramfs")

	switch m.config.Kernel.Initramfs {
	case "dracut":
		return m.generateDracutInitramfs()
	case "genkernel":
		return m.generateGenkernelInitramfs()
	default:
		return m.generateDracutInitramfs()
	}
}

// generateDracutInitramfs generates initramfs using dracut.
func (m *Manager) generateDracutInitramfs() error {
	// Configure dracut for encryption if needed
	if m.config.Encryption.Type != config.EncryptNone {
		dracutConf := `# Yuno OS dracut configuration
add_dracutmodules+=" crypt dm rootfs-block "
omit_dracutmodules+=" plymouth "
hostonly="yes"
hostonly_cmdline="no"
`
		confPath := filepath.Join(m.targetDir, "etc/dracut.conf.d/yuno.conf")
		if err := utils.WriteFile(confPath, dracutConf, 0644); err != nil {
			utils.Warn("Failed to write dracut config: %v", err)
		}
	}

	// Generate initramfs
	result := utils.RunInChroot(m.targetDir, "dracut", "--force", "--hostonly")
	if result.Error != nil {
		return utils.NewError("kernel", "dracut failed", result.Error)
	}

	return nil
}

// generateGenkernelInitramfs generates initramfs using genkernel.
func (m *Manager) generateGenkernelInitramfs() error {
	args := []string{"genkernel", "initramfs"}

	if m.config.Encryption.Type != config.EncryptNone {
		args = append(args, "--luks")
	}

	result := utils.RunInChroot(m.targetDir, args[0], args[1:]...)
	if result.Error != nil {
		return utils.NewError("kernel", "genkernel initramfs failed", result.Error)
	}

	return nil
}

// GetInstalledKernel returns information about the installed kernel.
func (m *Manager) GetInstalledKernel() (*KernelInfo, error) {
	// Find kernel in /boot
	bootDir := filepath.Join(m.targetDir, "boot")

	result := utils.RunCommand("ls", bootDir)
	if result.Error != nil {
		return nil, utils.NewError("kernel", "failed to list /boot", result.Error)
	}

	var kernelPath, initramfsPath, version string
	for _, file := range strings.Split(result.Stdout, "\n") {
		file = strings.TrimSpace(file)
		if strings.HasPrefix(file, "vmlinuz-") {
			kernelPath = "/boot/" + file
			version = strings.TrimPrefix(file, "vmlinuz-")
		}
		if strings.HasPrefix(file, "initramfs-") && strings.HasSuffix(file, ".img") {
			initramfsPath = "/boot/" + file
		}
	}

	if kernelPath == "" {
		return nil, utils.NewError("kernel", "no kernel found in /boot", nil)
	}

	return &KernelInfo{
		Version:   version,
		Type:      m.config.Kernel.Type,
		Path:      kernelPath,
		Initramfs: initramfsPath,
	}, nil
}

// SetupModules configures kernel modules.
func (m *Manager) SetupModules() error {
	utils.Info("Setting up kernel modules")

	modulesDir := filepath.Join(m.targetDir, "etc/modules-load.d")
	if err := utils.CreateDir(modulesDir, 0755); err != nil {
		return utils.NewError("kernel", "failed to create modules-load.d", err)
	}

	// Add modules based on configuration
	var modules []string

	// Graphics modules
	switch m.config.Graphics.Driver {
	case config.GPUNvidia, config.GPUNvidiaOpen:
		modules = append(modules, "nvidia", "nvidia_modeset", "nvidia_uvm", "nvidia_drm")
	case config.GPUAmdgpu:
		modules = append(modules, "amdgpu")
	case config.GPUIntel:
		modules = append(modules, "i915")
	}

	// Encryption modules
	if m.config.Encryption.Type != config.EncryptNone {
		modules = append(modules, "dm_crypt", "dm_mod")
	}

	// Add custom modules
	modules = append(modules, m.config.Kernel.Modules...)

	if len(modules) > 0 {
		content := "# Yuno OS kernel modules\n" + strings.Join(modules, "\n") + "\n"
		modulesPath := filepath.Join(modulesDir, "yuno.conf")
		if err := utils.WriteFile(modulesPath, content, 0644); err != nil {
			return utils.NewError("kernel", "failed to write modules config", err)
		}
	}

	return nil
}

// SetupModprobeConfig sets up modprobe configuration.
func (m *Manager) SetupModprobeConfig() error {
	modprobeDir := filepath.Join(m.targetDir, "etc/modprobe.d")
	if err := utils.CreateDir(modprobeDir, 0755); err != nil {
		return utils.NewError("kernel", "failed to create modprobe.d", err)
	}

	// NVIDIA-specific configuration
	if m.config.Graphics.Driver == config.GPUNvidia || m.config.Graphics.Driver == config.GPUNvidiaOpen {
		nvidiaConf := `# NVIDIA driver options
options nvidia_drm modeset=1
options nvidia NVreg_PreserveVideoMemoryAllocations=1
`
		confPath := filepath.Join(modprobeDir, "nvidia.conf")
		if err := utils.WriteFile(confPath, nvidiaConf, 0644); err != nil {
			utils.Warn("Failed to write nvidia modprobe config: %v", err)
		}
	}

	return nil
}

// ConfigureSysctl sets up kernel parameters via sysctl.
func (m *Manager) ConfigureSysctl() error {
	utils.Info("Configuring sysctl")

	sysctlDir := filepath.Join(m.targetDir, "etc/sysctl.d")
	if err := utils.CreateDir(sysctlDir, 0755); err != nil {
		return utils.NewError("kernel", "failed to create sysctl.d", err)
	}

	// Default sysctl configuration
	sysctlConf := `# Yuno OS sysctl configuration

# Virtual memory
vm.swappiness = 10
vm.vfs_cache_pressure = 50

# Network tuning
net.core.somaxconn = 65535
net.core.netdev_max_backlog = 65535

# IPv4 settings
net.ipv4.tcp_fastopen = 3
net.ipv4.tcp_tw_reuse = 1

# Security
kernel.kptr_restrict = 2
kernel.dmesg_restrict = 1
kernel.unprivileged_bpf_disabled = 1
`

	confPath := filepath.Join(sysctlDir, "99-yuno.conf")
	if err := utils.WriteFile(confPath, sysctlConf, 0644); err != nil {
		return utils.NewError("kernel", "failed to write sysctl config", err)
	}

	return nil
}

// Setup performs complete kernel setup.
func (m *Manager) Setup(progress func(line string)) error {
	// Install kernel
	if err := m.Install(progress); err != nil {
		return err
	}

	// Setup modules
	if err := m.SetupModules(); err != nil {
		return err
	}

	// Setup modprobe config
	if err := m.SetupModprobeConfig(); err != nil {
		return err
	}

	// Configure sysctl
	if err := m.ConfigureSysctl(); err != nil {
		return err
	}

	return nil
}

// KernelTypes returns available kernel types with descriptions.
func KernelTypes() map[config.KernelType]string {
	return map[config.KernelType]string{
		config.KernelBin:      "gentoo-kernel-bin - Pre-compiled, fastest installation",
		config.KernelDist:     "gentoo-kernel - Distribution kernel, compiled on install",
		config.KernelSources:  "gentoo-sources - Full control with genkernel",
		config.KernelZen:      "zen-sources - Desktop-optimized kernel",
		config.KernelXanmod:   "xanmod-sources - Performance-focused kernel",
		config.KernelLiquorix: "liquorix-sources - Desktop/gaming optimized",
		config.KernelVanilla:  "vanilla-sources - Upstream vanilla kernel",
	}
}

// GetRecommendedKernel returns the recommended kernel based on use case.
func GetRecommendedKernel(desktop config.DesktopType) config.KernelType {
	switch desktop {
	case config.DesktopNone:
		return config.KernelBin // Fast server install
	case config.WMi3, config.WMSway, config.WMHyprland, config.WMBspwm:
		return config.KernelZen // Good for power users
	default:
		return config.KernelBin // Safe default
	}
}
