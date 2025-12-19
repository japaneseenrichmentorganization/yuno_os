# Yuno OS

A Gentoo-based Linux distribution with an easy-to-use installer.

## Features

- **Dual Installer**: TUI (terminal) and Calamares GUI
- **Init Systems**: OpenRC and systemd support
- **Full Disk Encryption**: LUKS, LUKS2, ZFS encryption, dm-crypt
- **Overlay Support**: LTO overlay, GURU, custom overlays
- **Customization**: CFLAGS presets, USE flags, kernel selection
- **Graphics**: Auto-detection with NVIDIA, AMD, Intel driver support
- **Desktop Environments**: KDE Plasma, GNOME, XFCE, LXQt, Cinnamon
- **Window Managers**: i3, Sway, Hyprland, bspwm, dwm
- **Binary Packages**: Optional binpkg support for faster installs
- **Secure Boot**: UEFI Secure Boot with MOK signing

## Project Structure

```
yuno-os/
├── cmd/                    # Entry points
│   └── yuno-tui/          # TUI installer
├── pkg/                    # Core libraries
│   ├── config/            # Configuration types
│   ├── utils/             # Utilities
│   ├── partition/         # Disk partitioning
│   ├── encryption/        # Disk encryption
│   ├── stage3/            # Stage3 handling
│   ├── chroot/            # Chroot management
│   ├── overlays/          # Overlay management
│   ├── portage/           # Portage configuration
│   ├── kernel/            # Kernel installation
│   ├── graphics/          # GPU drivers
│   ├── desktop/           # DE/WM installation
│   ├── bootloader/        # Bootloader setup
│   ├── binpkg/            # Binary packages
│   ├── users/             # User management
│   └── installer/         # Installation orchestrator
├── internal/
│   └── tui/               # TUI implementation
├── calamares/             # Calamares GUI modules
│   ├── modules/           # Custom Calamares modules
│   └── branding/          # Yuno OS branding
├── scripts/               # Build scripts
│   └── build-iso.sh       # ISO build script
└── iso-build/             # Catalyst specs
```

## Building

### Requirements

- Go 1.22+
- Gentoo Linux (for ISO building)
- Root access (for ISO building)

### Build TUI Installer

```bash
go mod tidy
go build -o yuno-tui ./cmd/yuno-tui
```

### Build ISO

```bash
sudo ./scripts/build-iso.sh
```

The ISO will be created in the `output/` directory.

## Usage

### TUI Installer

Boot from the Yuno OS live media and run:

```bash
sudo yuno-tui
```

### GUI Installer

Boot from the Yuno OS live media and click "Install Yuno OS" on the desktop.

## Configuration Options

### CFLAGS Presets

| Preset | Flags | Description |
|--------|-------|-------------|
| Safe | `-march=x86-64 -O2 -pipe` | Maximum compatibility |
| Optimized | `-march=native -O2 -pipe` | Native CPU optimizations |
| Aggressive | `-march=native -O3 -pipe -flto=auto` | Maximum performance with LTO |

### Kernel Options

| Kernel | Description |
|--------|-------------|
| gentoo-kernel-bin | Pre-compiled, fastest install |
| gentoo-kernel | Distribution kernel |
| gentoo-sources | Full customization with genkernel |
| zen-sources | Desktop-optimized |
| xanmod-sources | Performance-focused |

### Supported Desktops

**Full Desktop Environments:**
- KDE Plasma
- GNOME
- XFCE
- LXQt
- Cinnamon
- MATE

**Window Managers:**
- i3 (X11)
- Sway (Wayland)
- Hyprland (Wayland)
- bspwm
- dwm
- Awesome
- Openbox

## Documentation

- [Gentoo Handbook](https://wiki.gentoo.org/wiki/Handbook:AMD64)
- [Gentoo Wiki](https://wiki.gentoo.org)

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

This project is open source. See individual files for licensing information.

## Acknowledgments

- Gentoo Linux community
- Calamares installer project
- Charm (Bubble Tea TUI framework)
