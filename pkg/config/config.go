// Package config defines all configuration types for the Yuno OS installer.
package config

import (
	"fmt"
	"os"
	"runtime"

	"gopkg.in/yaml.v3"
)

// InstallConfig holds the complete installation configuration.
type InstallConfig struct {
	// System configuration
	Hostname string `yaml:"hostname"`
	Timezone string `yaml:"timezone"`
	Locale   string `yaml:"locale"`
	Keymap   string `yaml:"keymap"`

	// Disk and partitioning
	Disk       DiskConfig       `yaml:"disk"`
	Partitions []PartitionConfig `yaml:"partitions"`
	Encryption EncryptionConfig `yaml:"encryption"`

	// Init system
	InitSystem InitSystem `yaml:"init_system"`

	// Portage configuration
	Portage PortageConfig `yaml:"portage"`

	// Overlays
	Overlays []OverlayConfig `yaml:"overlays"`

	// Kernel
	Kernel KernelConfig `yaml:"kernel"`

	// Graphics
	Graphics GraphicsConfig `yaml:"graphics"`

	// Desktop
	Desktop DesktopConfig `yaml:"desktop"`

	// Bootloader
	Bootloader BootloaderConfig `yaml:"bootloader"`

	// Users
	RootPassword string       `yaml:"root_password"`
	Users        []UserConfig `yaml:"users"`

	// Package management
	Packages PackageConfig `yaml:"packages"`
}

// DiskConfig holds disk selection configuration.
type DiskConfig struct {
	Device     string           `yaml:"device"`      // e.g., /dev/sda, /dev/nvme0n1
	WipeAll    bool             `yaml:"wipe_all"`    // Erase entire disk
	PartScheme PartitionScheme  `yaml:"part_scheme"` // GPT or MBR
}

// PartitionScheme defines the partition table type.
type PartitionScheme string

const (
	PartSchemeGPT PartitionScheme = "gpt"
	PartSchemeMBR PartitionScheme = "mbr"
)

// PartitionConfig defines a single partition.
type PartitionConfig struct {
	Label      string     `yaml:"label"`       // Partition label
	Size       string     `yaml:"size"`        // Size (e.g., "512M", "50G", "100%FREE")
	Filesystem Filesystem `yaml:"filesystem"`  // Filesystem type
	MountPoint string     `yaml:"mount_point"` // Mount point (e.g., "/", "/boot", "/home")
	Flags      []string   `yaml:"flags"`       // Partition flags (e.g., "boot", "esp")
	Encrypt    bool       `yaml:"encrypt"`     // Whether to encrypt this partition
}

// Filesystem defines supported filesystem types.
type Filesystem string

const (
	FSExt4  Filesystem = "ext4"
	FSBtrfs Filesystem = "btrfs"
	FSXfs   Filesystem = "xfs"
	FSF2fs  Filesystem = "f2fs"
	FSZfs   Filesystem = "zfs"
	FSFat32 Filesystem = "fat32"
	FSSwap  Filesystem = "swap"
	FSNone  Filesystem = "none" // For BIOS boot partition
)

// EncryptionConfig defines encryption settings.
type EncryptionConfig struct {
	Type       EncryptionType `yaml:"type"`
	Password   string         `yaml:"password"`
	KeyFile    string         `yaml:"key_file,omitempty"`
	Cipher     string         `yaml:"cipher,omitempty"`      // For LUKS
	KeySize    int            `yaml:"key_size,omitempty"`    // For LUKS
	Hash       string         `yaml:"hash,omitempty"`        // For LUKS
	ZFSDataset string         `yaml:"zfs_dataset,omitempty"` // For ZFS encryption
}

// EncryptionType defines supported encryption types.
type EncryptionType string

const (
	EncryptNone    EncryptionType = "none"
	EncryptLUKS    EncryptionType = "luks"
	EncryptLUKS2   EncryptionType = "luks2"
	EncryptZFS     EncryptionType = "zfs"
	EncryptDMCrypt EncryptionType = "dm-crypt"
)

// InitSystem defines the init system choice.
type InitSystem string

const (
	InitOpenRC  InitSystem = "openrc"
	InitSystemd InitSystem = "systemd"
)

// PortageConfig holds Portage/make.conf configuration.
type PortageConfig struct {
	Profile    string            `yaml:"profile"`     // Gentoo profile path
	CFlagsPreset CFlagsPreset    `yaml:"cflags_preset"`
	CFlags     string            `yaml:"cflags"`      // Custom CFLAGS if preset is "custom"
	CXXFlags   string            `yaml:"cxxflags"`    // Usually "${CFLAGS}"
	MakeOpts   string            `yaml:"makeopts"`    // e.g., "-j8"
	UseFlags   []string          `yaml:"use_flags"`   // Global USE flags
	Features   []string          `yaml:"features"`    // FEATURES
	Mirrors    []string          `yaml:"mirrors"`     // GENTOO_MIRRORS
	AcceptKeywords string        `yaml:"accept_keywords,omitempty"`
	AcceptLicense  string        `yaml:"accept_license,omitempty"`
	VideoCards []string          `yaml:"video_cards"` // VIDEO_CARDS
	InputDevices []string        `yaml:"input_devices"` // INPUT_DEVICES
	Extra      map[string]string `yaml:"extra,omitempty"` // Additional make.conf entries
}

// CFlagsPreset defines preset CFLAGS configurations.
type CFlagsPreset string

const (
	CFlagsSafe       CFlagsPreset = "safe"       // -march=x86-64 -O2 -pipe
	CFlagsOptimized  CFlagsPreset = "optimized"  // -march=native -O2 -pipe
	CFlagsAggressive CFlagsPreset = "aggressive" // -march=native -O3 -pipe -flto=auto
	CFlagsCustom     CFlagsPreset = "custom"     // User-defined
)

// GentooProfile represents a Gentoo profile with metadata.
type GentooProfile struct {
	Path        string      // Full profile path (e.g., "default/linux/amd64/23.0/desktop")
	Name        string      // Human-readable name
	Description string      // Description
	InitSystem  InitSystem  // Required init system (empty = both supported)
	Category    ProfileCategory
	Stable      bool        // Whether this is a stable profile
}

// ProfileCategory categorizes profiles.
type ProfileCategory string

const (
	ProfileCategoryDesktop  ProfileCategory = "desktop"
	ProfileCategoryServer   ProfileCategory = "server"
	ProfileCategoryHardened ProfileCategory = "hardened"
	ProfileCategoryMinimal  ProfileCategory = "minimal"
	ProfileCategoryDeveloper ProfileCategory = "developer"
	ProfileCategorySystemd  ProfileCategory = "systemd"
	ProfileCategoryMusl     ProfileCategory = "musl"
	ProfileCategorySelinux  ProfileCategory = "selinux"
)

// AvailableProfiles returns all available Gentoo profiles for amd64.
func AvailableProfiles() []GentooProfile {
	return []GentooProfile{
		// Standard Desktop Profiles (OpenRC)
		{
			Path:        "default/linux/amd64/23.0/desktop",
			Name:        "Desktop (OpenRC)",
			Description: "Standard desktop profile with OpenRC init",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryDesktop,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/desktop/gnome",
			Name:        "Desktop GNOME (OpenRC)",
			Description: "Desktop profile optimized for GNOME",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryDesktop,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/desktop/plasma",
			Name:        "Desktop KDE Plasma (OpenRC)",
			Description: "Desktop profile optimized for KDE Plasma",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryDesktop,
			Stable:      true,
		},
		// Standard Desktop Profiles (systemd)
		{
			Path:        "default/linux/amd64/23.0/desktop/systemd",
			Name:        "Desktop (systemd)",
			Description: "Standard desktop profile with systemd init",
			InitSystem:  InitSystemd,
			Category:    ProfileCategoryDesktop,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/desktop/gnome/systemd",
			Name:        "Desktop GNOME (systemd)",
			Description: "Desktop profile optimized for GNOME with systemd",
			InitSystem:  InitSystemd,
			Category:    ProfileCategoryDesktop,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/desktop/plasma/systemd",
			Name:        "Desktop KDE Plasma (systemd)",
			Description: "Desktop profile optimized for KDE Plasma with systemd",
			InitSystem:  InitSystemd,
			Category:    ProfileCategoryDesktop,
			Stable:      true,
		},
		// Minimal/Server Profiles
		{
			Path:        "default/linux/amd64/23.0",
			Name:        "Default (OpenRC)",
			Description: "Base profile without desktop USE flags",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryMinimal,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/systemd",
			Name:        "Default (systemd)",
			Description: "Base profile without desktop USE flags, with systemd",
			InitSystem:  InitSystemd,
			Category:    ProfileCategorySystemd,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/no-multilib",
			Name:        "No Multilib (OpenRC)",
			Description: "64-bit only, no 32-bit library support",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryMinimal,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/no-multilib/systemd",
			Name:        "No Multilib (systemd)",
			Description: "64-bit only, no 32-bit library support, with systemd",
			InitSystem:  InitSystemd,
			Category:    ProfileCategorySystemd,
			Stable:      true,
		},
		// Hardened Profiles (OpenRC)
		{
			Path:        "default/linux/amd64/23.0/hardened",
			Name:        "Hardened (OpenRC)",
			Description: "Security-hardened profile with PaX/grsecurity features",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryHardened,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/hardened/selinux",
			Name:        "Hardened SELinux (OpenRC)",
			Description: "Hardened profile with SELinux mandatory access control",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryHardened,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/hardened/no-multilib",
			Name:        "Hardened No Multilib (OpenRC)",
			Description: "Hardened 64-bit only profile",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryHardened,
			Stable:      true,
		},
		// Hardened Profiles (systemd)
		{
			Path:        "default/linux/amd64/23.0/hardened/systemd",
			Name:        "Hardened (systemd)",
			Description: "Security-hardened profile with systemd",
			InitSystem:  InitSystemd,
			Category:    ProfileCategoryHardened,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/hardened/selinux/systemd",
			Name:        "Hardened SELinux (systemd)",
			Description: "Hardened profile with SELinux and systemd",
			InitSystem:  InitSystemd,
			Category:    ProfileCategoryHardened,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/hardened/no-multilib/systemd",
			Name:        "Hardened No Multilib (systemd)",
			Description: "Hardened 64-bit only profile with systemd",
			InitSystem:  InitSystemd,
			Category:    ProfileCategoryHardened,
			Stable:      true,
		},
		// Musl Profiles
		{
			Path:        "default/linux/amd64/23.0/musl",
			Name:        "Musl libc",
			Description: "Profile using musl instead of glibc",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryMusl,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/musl/hardened",
			Name:        "Musl Hardened",
			Description: "Hardened profile with musl libc",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryMusl,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/musl/hardened/selinux",
			Name:        "Musl Hardened SELinux",
			Description: "Hardened musl profile with SELinux",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryMusl,
			Stable:      true,
		},
		// Split-usr profiles
		{
			Path:        "default/linux/amd64/23.0/split-usr",
			Name:        "Split /usr (OpenRC)",
			Description: "Traditional split /usr layout",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryMinimal,
			Stable:      true,
		},
		{
			Path:        "default/linux/amd64/23.0/split-usr/desktop",
			Name:        "Split /usr Desktop (OpenRC)",
			Description: "Split /usr with desktop USE flags",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryDesktop,
			Stable:      true,
		},
		// Developer profile
		{
			Path:        "default/linux/amd64/23.0/desktop",
			Name:        "Developer Desktop",
			Description: "Desktop profile suitable for development",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryDeveloper,
			Stable:      true,
		},
		// x32 ABI profile
		{
			Path:        "default/linux/amd64/23.0/x32",
			Name:        "x32 ABI",
			Description: "x32 ABI profile (32-bit pointers on 64-bit)",
			InitSystem:  InitOpenRC,
			Category:    ProfileCategoryMinimal,
			Stable:      false,
		},
	}
}

// GetProfilesForInitSystem returns profiles compatible with the given init system.
func GetProfilesForInitSystem(init InitSystem) []GentooProfile {
	var result []GentooProfile
	for _, p := range AvailableProfiles() {
		if p.InitSystem == init || p.InitSystem == "" {
			result = append(result, p)
		}
	}
	return result
}

// GetProfilesByCategory returns profiles in the given category.
func GetProfilesByCategory(category ProfileCategory) []GentooProfile {
	var result []GentooProfile
	for _, p := range AvailableProfiles() {
		if p.Category == category {
			result = append(result, p)
		}
	}
	return result
}

// GetHardenedProfiles returns all hardened profiles.
func GetHardenedProfiles() []GentooProfile {
	return GetProfilesByCategory(ProfileCategoryHardened)
}

// FindProfileByPath finds a profile by its path.
func FindProfileByPath(path string) *GentooProfile {
	for _, p := range AvailableProfiles() {
		if p.Path == path {
			return &p
		}
	}
	return nil
}

// GetCFlags returns the actual CFLAGS string for a preset.
func (p CFlagsPreset) GetCFlags() string {
	switch p {
	case CFlagsSafe:
		return "-march=x86-64 -O2 -pipe"
	case CFlagsOptimized:
		return "-march=native -O2 -pipe"
	case CFlagsAggressive:
		return "-march=native -O3 -pipe -flto=auto"
	default:
		return ""
	}
}

// OverlayConfig defines an overlay to add.
type OverlayConfig struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url,omitempty"`          // For custom overlays
	SyncType    string `yaml:"sync_type,omitempty"`    // git, rsync, etc.
	Priority    int    `yaml:"priority,omitempty"`
	AutoSync    bool   `yaml:"auto_sync"`
}

// Predefined overlays
var PredefinedOverlays = map[string]OverlayConfig{
	"lto": {
		Name:     "lto-overlay",
		URL:      "https://github.com/InBetweenNames/gentooLTO.git",
		SyncType: "git",
		AutoSync: true,
	},
	"guru": {
		Name:     "guru",
		SyncType: "rsync",
		AutoSync: true,
	},
	"gentoo-zh": {
		Name:     "gentoo-zh",
		SyncType: "rsync",
		AutoSync: true,
	},
	"steam-overlay": {
		Name:     "steam-overlay",
		URL:      "https://github.com/anyc/steam-overlay.git",
		SyncType: "git",
		AutoSync: true,
	},
}

// KernelConfig defines kernel installation options.
type KernelConfig struct {
	Type         KernelType `yaml:"type"`
	CustomConfig string     `yaml:"custom_config,omitempty"` // Path to custom .config
	Initramfs    string     `yaml:"initramfs"`               // dracut, genkernel
	Modules      []string   `yaml:"modules,omitempty"`       // Additional modules to build
}

// KernelType defines available kernel options.
type KernelType string

const (
	KernelBin        KernelType = "gentoo-kernel-bin" // Pre-compiled
	KernelDist       KernelType = "gentoo-kernel"     // Distribution kernel
	KernelSources    KernelType = "gentoo-sources"    // Manual/genkernel
	KernelZen        KernelType = "zen-sources"       // Zen kernel
	KernelXanmod     KernelType = "xanmod-sources"    // Xanmod kernel
	KernelLiquorix   KernelType = "liquorix-sources"  // Liquorix kernel
	KernelVanilla    KernelType = "vanilla-sources"   // Vanilla kernel
)

// GetPackage returns the Gentoo package name for the kernel type.
func (k KernelType) GetPackage() string {
	switch k {
	case KernelBin:
		return "sys-kernel/gentoo-kernel-bin"
	case KernelDist:
		return "sys-kernel/gentoo-kernel"
	case KernelSources:
		return "sys-kernel/gentoo-sources"
	case KernelZen:
		return "sys-kernel/zen-sources"
	case KernelXanmod:
		return "sys-kernel/xanmod-sources"
	case KernelLiquorix:
		return "sys-kernel/liquorix-sources"
	case KernelVanilla:
		return "sys-kernel/vanilla-sources"
	default:
		return "sys-kernel/gentoo-kernel-bin"
	}
}

// GraphicsConfig defines GPU driver configuration.
type GraphicsConfig struct {
	Driver      GPUDriver    `yaml:"driver"`
	DisplayType DisplayType  `yaml:"display_type"` // X11 or Wayland
	Compositor  string       `yaml:"compositor,omitempty"` // For Wayland
}

// GPUDriver defines GPU driver options.
type GPUDriver string

const (
	GPUNvidia     GPUDriver = "nvidia"
	GPUNvidiaOpen GPUDriver = "nvidia-open"
	GPUNouveau    GPUDriver = "nouveau"
	GPUAmdgpu     GPUDriver = "amdgpu"
	GPURadeon     GPUDriver = "radeon"
	GPUIntel      GPUDriver = "intel"
	GPUIntelXe    GPUDriver = "intel-xe"
	GPUVirtio     GPUDriver = "virtio"
	GPUVMware     GPUDriver = "vmware"
)

// GetVideoCards returns the VIDEO_CARDS value for the driver.
func (g GPUDriver) GetVideoCards() string {
	switch g {
	case GPUNvidia, GPUNvidiaOpen:
		return "nvidia"
	case GPUNouveau:
		return "nouveau"
	case GPUAmdgpu:
		return "amdgpu radeonsi"
	case GPURadeon:
		return "radeon r600"
	case GPUIntel:
		return "intel i965 iris"
	case GPUIntelXe:
		return "intel xe"
	case GPUVirtio:
		return "virgl"
	case GPUVMware:
		return "vmware"
	default:
		return ""
	}
}

// DisplayType defines display server preference.
type DisplayType string

const (
	DisplayX11     DisplayType = "x11"
	DisplayWayland DisplayType = "wayland"
)

// DesktopConfig defines desktop environment/window manager configuration.
type DesktopConfig struct {
	Type           DesktopType    `yaml:"type"`
	DisplayManager DisplayManager `yaml:"display_manager"`
	SessionType    DisplayType    `yaml:"session_type"` // X11 or Wayland session
	ExtraPackages  []string       `yaml:"extra_packages,omitempty"`
}

// DesktopType defines available desktop environments and window managers.
type DesktopType string

const (
	// Desktop Environments
	DesktopKDE      DesktopType = "kde-plasma"
	DesktopGNOME    DesktopType = "gnome"
	DesktopXFCE     DesktopType = "xfce"
	DesktopLXQt     DesktopType = "lxqt"
	DesktopCinnamon DesktopType = "cinnamon"
	DesktopMATE     DesktopType = "mate"
	DesktopBudgie   DesktopType = "budgie"

	// Window Managers
	WMi3       DesktopType = "i3"
	WMSway     DesktopType = "sway"
	WMHyprland DesktopType = "hyprland"
	WMBspwm    DesktopType = "bspwm"
	WMDwm      DesktopType = "dwm"
	WMAwesome  DesktopType = "awesome"
	WMOpenbox  DesktopType = "openbox"

	// None
	DesktopNone DesktopType = "none"
)

// GetPackages returns the packages to install for the desktop type.
func (d DesktopType) GetPackages() []string {
	switch d {
	case DesktopKDE:
		return []string{"kde-plasma/plasma-meta", "kde-apps/konsole", "kde-apps/dolphin"}
	case DesktopGNOME:
		return []string{"gnome-base/gnome", "gnome-base/gnome-shell"}
	case DesktopXFCE:
		return []string{"xfce-base/xfce4-meta", "x11-terms/xfce4-terminal"}
	case DesktopLXQt:
		return []string{"lxqt-base/lxqt-meta"}
	case DesktopCinnamon:
		return []string{"gnome-extra/cinnamon"}
	case DesktopMATE:
		return []string{"mate-base/mate"}
	case DesktopBudgie:
		return []string{"gnome-extra/budgie-desktop"}
	case WMi3:
		return []string{"x11-wm/i3", "x11-misc/i3status", "x11-misc/dmenu", "x11-terms/alacritty"}
	case WMSway:
		return []string{"gui-wm/sway", "gui-apps/waybar", "gui-apps/wofi", "x11-terms/alacritty"}
	case WMHyprland:
		return []string{"gui-wm/hyprland", "gui-apps/waybar", "gui-apps/wofi", "x11-terms/alacritty"}
	case WMBspwm:
		return []string{"x11-wm/bspwm", "x11-misc/sxhkd", "x11-misc/dmenu", "x11-terms/alacritty"}
	case WMDwm:
		return []string{"x11-wm/dwm", "x11-misc/dmenu", "x11-terms/st"}
	case WMAwesome:
		return []string{"x11-wm/awesome", "x11-terms/alacritty"}
	case WMOpenbox:
		return []string{"x11-wm/openbox", "x11-misc/obconf", "x11-terms/alacritty"}
	default:
		return []string{}
	}
}

// DisplayManager defines available display managers.
type DisplayManager string

const (
	DMSDDM    DisplayManager = "sddm"
	DMGDM     DisplayManager = "gdm"
	DMLightDM DisplayManager = "lightdm"
	DMLXDM    DisplayManager = "lxdm"
	DMNone    DisplayManager = "none" // TTY login
)

// GetPackage returns the package for the display manager.
func (d DisplayManager) GetPackage() string {
	switch d {
	case DMSDDM:
		return "x11-misc/sddm"
	case DMGDM:
		return "gnome-base/gdm"
	case DMLightDM:
		return "x11-misc/lightdm"
	case DMLXDM:
		return "lxde-base/lxdm"
	default:
		return ""
	}
}

// BootloaderConfig defines bootloader settings.
type BootloaderConfig struct {
	Type       BootloaderType `yaml:"type"`
	SecureBoot SecureBootConfig `yaml:"secure_boot"`
}

// BootloaderType defines available bootloaders.
type BootloaderType string

const (
	BootGRUB       BootloaderType = "grub"
	BootSystemdBoot BootloaderType = "systemd-boot"
)

// SecureBootConfig defines Secure Boot settings.
type SecureBootConfig struct {
	Enabled     bool   `yaml:"enabled"`
	KeyType     string `yaml:"key_type"` // "custom" or "shim"
	KeyDir      string `yaml:"key_dir,omitempty"`
	EnrollKeys  bool   `yaml:"enroll_keys"`
}

// UserConfig defines a user account.
type UserConfig struct {
	Username    string   `yaml:"username"`
	Password    string   `yaml:"password"`
	FullName    string   `yaml:"full_name,omitempty"`
	Shell       string   `yaml:"shell"`
	Groups      []string `yaml:"groups"`
	Sudo        bool     `yaml:"sudo"`
	UseDoas     bool     `yaml:"use_doas"` // Use doas instead of sudo
}

// PackageConfig defines package installation preferences.
type PackageConfig struct {
	UseBinary      BinaryPreference `yaml:"use_binary"`
	BinaryHost     string           `yaml:"binary_host,omitempty"`
	ExtraPackages  []string         `yaml:"extra_packages,omitempty"`
}

// BinaryPreference defines binary package preference.
type BinaryPreference string

const (
	BinaryNone     BinaryPreference = "source"    // Compile everything
	BinaryPrefer   BinaryPreference = "prefer"    // Use binaries when available
	BinaryOnly     BinaryPreference = "only"      // Only use binaries
)

// NewDefaultConfig creates a config with sensible defaults.
func NewDefaultConfig() *InstallConfig {
	cores := runtime.NumCPU()

	return &InstallConfig{
		Hostname:   "yuno",
		Timezone:   "UTC",
		Locale:     "en_US.UTF-8",
		Keymap:     "us",
		InitSystem: InitOpenRC,
		Disk: DiskConfig{
			PartScheme: PartSchemeGPT,
		},
		Encryption: EncryptionConfig{
			Type: EncryptNone,
		},
		Portage: PortageConfig{
			Profile:      "default/linux/amd64/23.0/desktop",
			CFlagsPreset: CFlagsOptimized,
			MakeOpts:     fmt.Sprintf("-j%d", cores),
			UseFlags:     []string{},
			Features:     []string{"parallel-fetch", "candy"},
			AcceptLicense: "*",
			InputDevices: []string{"libinput"},
		},
		Kernel: KernelConfig{
			Type:      KernelBin,
			Initramfs: "dracut",
		},
		Graphics: GraphicsConfig{
			DisplayType: DisplayWayland,
		},
		Desktop: DesktopConfig{
			Type:           DesktopKDE,
			DisplayManager: DMSDDM,
			SessionType:    DisplayWayland,
		},
		Bootloader: BootloaderConfig{
			Type: BootGRUB,
			SecureBoot: SecureBootConfig{
				Enabled: false,
			},
		},
		Packages: PackageConfig{
			UseBinary: BinaryPrefer,
		},
	}
}

// LoadConfig loads configuration from a YAML file.
func LoadConfig(path string) (*InstallConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := NewDefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to a YAML file.
func (c *InstallConfig) SaveConfig(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid.
func (c *InstallConfig) Validate() error {
	if c.Hostname == "" {
		return fmt.Errorf("hostname is required")
	}
	if c.Disk.Device == "" {
		return fmt.Errorf("disk device is required")
	}
	if len(c.Partitions) == 0 {
		return fmt.Errorf("at least one partition is required")
	}

	// Check for root partition
	hasRoot := false
	for _, p := range c.Partitions {
		if p.MountPoint == "/" {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		return fmt.Errorf("root partition (/) is required")
	}

	// Validate encryption password if encryption is enabled
	if c.Encryption.Type != EncryptNone && c.Encryption.Password == "" && c.Encryption.KeyFile == "" {
		return fmt.Errorf("encryption password or key file is required")
	}

	return nil
}
