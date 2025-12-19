// Package graphics handles GPU detection and driver installation.
package graphics

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/utils"
)

// Manager handles graphics configuration.
type Manager struct {
	config    *config.InstallConfig
	targetDir string
}

// NewManager creates a new graphics manager.
func NewManager(cfg *config.InstallConfig, targetDir string) *Manager {
	return &Manager{
		config:    cfg,
		targetDir: targetDir,
	}
}

// GPU represents a detected graphics card.
type GPU struct {
	Vendor      GPUVendor
	Model       string
	PciID       string
	Driver      config.GPUDriver
	Description string
}

// GPUVendor represents GPU manufacturers.
type GPUVendor string

const (
	VendorNVIDIA  GPUVendor = "NVIDIA"
	VendorAMD     GPUVendor = "AMD"
	VendorIntel   GPUVendor = "Intel"
	VendorVirtio  GPUVendor = "Virtio"
	VendorVMware  GPUVendor = "VMware"
	VendorUnknown GPUVendor = "Unknown"
)

// DetectGPUs detects all graphics cards in the system.
func (m *Manager) DetectGPUs() ([]GPU, error) {
	result := utils.RunCommand("lspci", "-nn")
	if result.Error != nil {
		return nil, utils.NewError("graphics", "failed to run lspci", result.Error)
	}

	var gpus []GPU
	lines := strings.Split(result.Stdout, "\n")

	// Pattern to match VGA/3D controllers
	vgaPattern := regexp.MustCompile(`(?i)(VGA|3D|Display).*controller.*:\s*(.+)\s*\[([0-9a-f:]+)\]`)

	for _, line := range lines {
		match := vgaPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		description := strings.TrimSpace(match[2])
		pciID := match[3]

		gpu := GPU{
			Description: description,
			PciID:       pciID,
		}

		// Determine vendor
		descLower := strings.ToLower(description)
		switch {
		case strings.Contains(descLower, "nvidia"):
			gpu.Vendor = VendorNVIDIA
			gpu.Driver = config.GPUNvidia
		case strings.Contains(descLower, "amd") || strings.Contains(descLower, "radeon"):
			gpu.Vendor = VendorAMD
			gpu.Driver = config.GPUAmdgpu
		case strings.Contains(descLower, "intel"):
			gpu.Vendor = VendorIntel
			gpu.Driver = config.GPUIntel
		case strings.Contains(descLower, "virtio"):
			gpu.Vendor = VendorVirtio
			gpu.Driver = config.GPUVirtio
		case strings.Contains(descLower, "vmware"):
			gpu.Vendor = VendorVMware
			gpu.Driver = config.GPUVMware
		default:
			gpu.Vendor = VendorUnknown
		}

		// Extract model name
		gpu.Model = extractModel(description)

		gpus = append(gpus, gpu)
	}

	return gpus, nil
}

// extractModel extracts the GPU model from the description.
func extractModel(desc string) string {
	// Remove vendor prefix
	desc = strings.TrimPrefix(desc, "NVIDIA Corporation ")
	desc = strings.TrimPrefix(desc, "Advanced Micro Devices, Inc. ")
	desc = strings.TrimPrefix(desc, "Intel Corporation ")

	// Remove common suffixes
	desc = strings.TrimSuffix(desc, " [")

	// Clean up
	if idx := strings.Index(desc, "["); idx > 0 {
		desc = desc[:idx]
	}

	return strings.TrimSpace(desc)
}

// GetRecommendedDriver returns the recommended driver for a GPU.
func (m *Manager) GetRecommendedDriver(gpu GPU) config.GPUDriver {
	switch gpu.Vendor {
	case VendorNVIDIA:
		// Check if it's a newer GPU that supports open drivers
		// For now, default to proprietary
		return config.GPUNvidia
	case VendorAMD:
		return config.GPUAmdgpu
	case VendorIntel:
		return config.GPUIntel
	case VendorVirtio:
		return config.GPUVirtio
	case VendorVMware:
		return config.GPUVMware
	default:
		return ""
	}
}

// Install installs graphics drivers.
func (m *Manager) Install(progress func(line string)) error {
	driver := m.config.Graphics.Driver
	if driver == "" {
		// Auto-detect
		gpus, err := m.DetectGPUs()
		if err != nil {
			return err
		}
		if len(gpus) > 0 {
			driver = m.GetRecommendedDriver(gpus[0])
		}
	}

	utils.Info("Installing graphics driver: %s", driver)

	switch driver {
	case config.GPUNvidia:
		return m.installNvidia(false, progress)
	case config.GPUNvidiaOpen:
		return m.installNvidia(true, progress)
	case config.GPUNouveau:
		return m.installNouveau(progress)
	case config.GPUAmdgpu:
		return m.installAMD(progress)
	case config.GPUIntel:
		return m.installIntel(progress)
	case config.GPUVirtio, config.GPUVMware:
		return m.installVirtual(progress)
	default:
		utils.Warn("No specific driver to install")
		return nil
	}
}

// installNvidia installs NVIDIA drivers.
func (m *Manager) installNvidia(open bool, progress func(line string)) error {
	packages := []string{
		"x11-drivers/nvidia-drivers",
		"media-libs/mesa",
	}

	// Add Wayland support
	if m.config.Graphics.DisplayType == config.DisplayWayland {
		packages = append(packages, "gui-libs/egl-wayland")
	}

	// Configure USE flags for nvidia-drivers
	useFlags := "modules"
	if open {
		useFlags += " kernel-open"
	}

	usePath := filepath.Join(m.targetDir, "etc/portage/package.use/nvidia")
	useContent := fmt.Sprintf("x11-drivers/nvidia-drivers %s\n", useFlags)
	if err := utils.WriteFile(usePath, useContent, 0644); err != nil {
		return utils.NewError("graphics", "failed to write nvidia use flags", err)
	}

	// Accept license
	licensePath := filepath.Join(m.targetDir, "etc/portage/package.license/nvidia")
	licenseContent := "x11-drivers/nvidia-drivers NVIDIA-r2\n"
	if err := utils.WriteFile(licensePath, licenseContent, 0644); err != nil {
		return utils.NewError("graphics", "failed to write nvidia license", err)
	}

	return m.emergePackages(packages, progress)
}

// installNouveau installs the open-source Nouveau driver.
func (m *Manager) installNouveau(progress func(line string)) error {
	packages := []string{
		"media-libs/mesa",
		"x11-drivers/xf86-video-nouveau",
	}

	return m.emergePackages(packages, progress)
}

// installAMD installs AMD graphics drivers.
func (m *Manager) installAMD(progress func(line string)) error {
	packages := []string{
		"media-libs/mesa",
		"x11-drivers/xf86-video-amdgpu",
		"media-libs/vulkan-loader",
	}

	// Add Vulkan support
	packages = append(packages, "media-libs/mesa", "dev-util/vulkan-tools")

	return m.emergePackages(packages, progress)
}

// installIntel installs Intel graphics drivers.
func (m *Manager) installIntel(progress func(line string)) error {
	packages := []string{
		"media-libs/mesa",
		"x11-drivers/xf86-video-intel",
		"media-libs/vulkan-loader",
	}

	// Add VA-API for hardware video acceleration
	packages = append(packages, "media-libs/libva-intel-driver")

	return m.emergePackages(packages, progress)
}

// installVirtual installs virtual machine graphics drivers.
func (m *Manager) installVirtual(progress func(line string)) error {
	packages := []string{
		"media-libs/mesa",
	}

	if m.config.Graphics.Driver == config.GPUVMware {
		packages = append(packages, "x11-drivers/xf86-video-vmware")
	}

	return m.emergePackages(packages, progress)
}

// emergePackages installs packages via emerge.
func (m *Manager) emergePackages(packages []string, progress func(line string)) error {
	args := append([]string{m.targetDir, "emerge", "--ask=n"}, packages...)

	if progress != nil {
		return utils.RunCommandWithOutput(progress, "chroot", args...)
	}

	result := utils.RunCommand("chroot", args...)
	if result.Error != nil {
		return utils.NewError("graphics", "failed to install packages", result.Error)
	}

	return nil
}

// ConfigureXorg generates Xorg configuration if needed.
func (m *Manager) ConfigureXorg() error {
	if m.config.Graphics.DisplayType == config.DisplayWayland {
		return nil // No Xorg config needed
	}

	utils.Info("Configuring Xorg")

	xorgDir := filepath.Join(m.targetDir, "etc/X11/xorg.conf.d")
	if err := utils.CreateDir(xorgDir, 0755); err != nil {
		return utils.NewError("graphics", "failed to create xorg.conf.d", err)
	}

	var content string

	switch m.config.Graphics.Driver {
	case config.GPUNvidia, config.GPUNvidiaOpen:
		content = `# NVIDIA configuration
Section "Device"
    Identifier     "Device0"
    Driver         "nvidia"
    VendorName     "NVIDIA Corporation"
    Option         "NoLogo" "true"
EndSection
`
	case config.GPUAmdgpu:
		content = `# AMD configuration
Section "Device"
    Identifier     "Device0"
    Driver         "amdgpu"
    Option         "TearFree" "true"
EndSection
`
	case config.GPUIntel:
		content = `# Intel configuration
Section "Device"
    Identifier     "Device0"
    Driver         "intel"
    Option         "TearFree" "true"
    Option         "DRI" "3"
EndSection
`
	default:
		return nil
	}

	confPath := filepath.Join(xorgDir, "20-gpu.conf")
	if err := utils.WriteFile(confPath, content, 0644); err != nil {
		return utils.NewError("graphics", "failed to write xorg config", err)
	}

	return nil
}

// ConfigureEnvironment sets up environment variables for graphics.
func (m *Manager) ConfigureEnvironment() error {
	utils.Info("Configuring graphics environment")

	envDir := filepath.Join(m.targetDir, "etc/profile.d")
	if err := utils.CreateDir(envDir, 0755); err != nil {
		return utils.NewError("graphics", "failed to create profile.d", err)
	}

	var content strings.Builder

	content.WriteString("# Yuno OS graphics environment\n")

	// Wayland-specific settings
	if m.config.Graphics.DisplayType == config.DisplayWayland {
		content.WriteString("export MOZ_ENABLE_WAYLAND=1\n")
		content.WriteString("export QT_QPA_PLATFORM=wayland\n")
		content.WriteString("export SDL_VIDEODRIVER=wayland\n")
		content.WriteString("export _JAVA_AWT_WM_NONREPARENTING=1\n")

		if m.config.Graphics.Driver == config.GPUNvidia || m.config.Graphics.Driver == config.GPUNvidiaOpen {
			content.WriteString("export GBM_BACKEND=nvidia-drm\n")
			content.WriteString("export __GLX_VENDOR_LIBRARY_NAME=nvidia\n")
			content.WriteString("export WLR_NO_HARDWARE_CURSORS=1\n")
		}
	}

	// Vulkan ICD
	switch m.config.Graphics.Driver {
	case config.GPUNvidia, config.GPUNvidiaOpen:
		content.WriteString("export VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/nvidia_icd.json\n")
	case config.GPUAmdgpu:
		content.WriteString("export VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/radeon_icd.x86_64.json\n")
	case config.GPUIntel:
		content.WriteString("export VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/intel_icd.x86_64.json\n")
	}

	envPath := filepath.Join(envDir, "99-graphics.sh")
	if err := utils.WriteFile(envPath, content.String(), 0644); err != nil {
		return utils.NewError("graphics", "failed to write graphics env", err)
	}

	return nil
}

// Setup performs complete graphics setup.
func (m *Manager) Setup(progress func(line string)) error {
	// Install drivers
	if err := m.Install(progress); err != nil {
		return err
	}

	// Configure Xorg if needed
	if err := m.ConfigureXorg(); err != nil {
		return err
	}

	// Configure environment
	if err := m.ConfigureEnvironment(); err != nil {
		return err
	}

	return nil
}

// DriverOptions returns available driver options for a vendor.
func DriverOptions(vendor GPUVendor) []config.GPUDriver {
	switch vendor {
	case VendorNVIDIA:
		return []config.GPUDriver{
			config.GPUNvidia,
			config.GPUNvidiaOpen,
			config.GPUNouveau,
		}
	case VendorAMD:
		return []config.GPUDriver{
			config.GPUAmdgpu,
			config.GPURadeon,
		}
	case VendorIntel:
		return []config.GPUDriver{
			config.GPUIntel,
		}
	default:
		return []config.GPUDriver{}
	}
}

// DriverDescriptions returns descriptions for each driver.
func DriverDescriptions() map[config.GPUDriver]string {
	return map[config.GPUDriver]string{
		config.GPUNvidia:     "NVIDIA proprietary - Best performance, closed source",
		config.GPUNvidiaOpen: "NVIDIA open kernel modules - Open source kernel driver",
		config.GPUNouveau:    "Nouveau - Open source, limited performance",
		config.GPUAmdgpu:     "AMDGPU - Open source, excellent performance",
		config.GPURadeon:     "Radeon - Legacy AMD cards",
		config.GPUIntel:      "Intel - Integrated graphics",
		config.GPUVirtio:     "Virtio - Virtual machine graphics",
		config.GPUVMware:     "VMware - VMware virtual graphics",
	}
}
