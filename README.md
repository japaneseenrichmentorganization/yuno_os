<p align="center">
  <img src="branding/readme.png" alt="Yuno OS" width="600"/>
</p>

<h1 align="center">💕 Yuno OS 💕</h1>

<p align="center">
  <em>"Yukki~ Let me install Gentoo for you!" 🔪💗</em>
</p>

<p align="center">
  <a href="#-features">Features</a> •
  <a href="#-installation">Installation</a> •
  <a href="#-customization">Customization</a> •
  <a href="#-building">Building</a> •
  <a href="#-contributing">Contributing</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Based%20On-Gentoo-purple?style=for-the-badge&logo=gentoo" alt="Gentoo"/>
  <img src="https://img.shields.io/badge/Made%20With-Love-ff69b4?style=for-the-badge" alt="Love"/>
  <img src="https://img.shields.io/badge/License-AGPL--3.0-blue?style=for-the-badge" alt="License"/>
</p>

---

## 🌸 About

**Yuno OS** is a Gentoo-based Linux distribution that makes installing Gentoo as easy as... well, easier than it normally is! 💕

Just like Yuno Gasai would do anything for her Yukki, Yuno OS will do everything to give you a perfect Gentoo installation~ 🔪✨

Whether you're a seasoned Gentoo veteran or a curious newcomer, Yuno OS provides both a beautiful TUI and a Calamares GUI installer to guide you through the process!

---

## ✨ Features

### 💖 Dual Installer Options
- **TUI Installer** - Beautiful terminal interface using Bubble Tea 🍵
- **Calamares GUI** - Graphical installer for those who prefer clicking~ 🖱️

### 🔐 Security & Encryption
- **LUKS / LUKS2** - Full disk encryption 🔒
- **ZFS Native Encryption** - For the ZFS lovers 💙
- **dm-crypt** - Raw encryption support
- **Secure Boot** - UEFI Secure Boot with MOK signing 🛡️

### ⚙️ Init System Support
- **OpenRC** - The classic Gentoo way 🏛️
- **systemd** - For those who prefer it 🔧

### 🎨 Desktop Environments & Window Managers

**Full Desktop Environments:**
| DE | Description |
|:---:|:---|
| 💎 KDE Plasma | Beautiful and powerful |
| 👣 GNOME | Clean and modern |
| 🐭 XFCE | Lightweight and fast |
| 🪶 LXQt | Super lightweight |
| 🍂 Cinnamon | Familiar and elegant |
| 🧉 MATE | Classic GNOME 2 feel |

**Window Managers:**
| WM | Type | Description |
|:---:|:---:|:---|
| 🔷 i3 | X11 | Tiling perfection |
| 🌊 Sway | Wayland | i3 for Wayland |
| 💫 Hyprland | Wayland | Eye candy tiling |
| 🌳 bspwm | X11 | Binary space partitioning |
| 🎯 dwm | X11 | Suckless and simple |
| 😎 Awesome | X11 | Highly configurable |
| 📦 Openbox | X11 | Floating and flexible |

### 🚀 Performance Options
- **LTO Overlay** - Link-Time Optimization for maximum speed 🏎️
- **Custom CFLAGS** - Tune to your CPU 🎯
- **Binary Packages** - Fast installs when you need them 📦

### 🎮 Graphics Support
- **NVIDIA** - Proprietary drivers with auto-detection 💚
- **AMD** - AMDGPU and RadeonSI 🔴
- **Intel** - i915 and Xe drivers 🔵

### 🌟 Extra Goodies
- **Overlay Support** - LTO, GURU, and custom overlays 📚
- **Kernel Selection** - Choose your kernel type 🐧
- **USE Flag Presets** - Desktop, gaming, server, minimal 🎛️

---

## 📥 Installation

### From Live ISO

1. **Boot** from the Yuno OS live media 💿
2. **Choose** your installer:
   - **TUI**: Run `sudo yuno-tui` in terminal
   - **GUI**: Click "Install Yuno OS" on desktop
3. **Follow** the installation steps 📋
4. **Reboot** and enjoy your new system! 🎉

### Installation Steps

```
┌─────────────────────────────────────────┐
│  1. 💕 Welcome                          │
│  2. 💾 Disk Selection                   │
│  3. 📊 Partitioning                     │
│  4. 🔐 Encryption                       │
│  5. ⚙️  Init System (OpenRC/systemd)    │
│  6. 📚 Overlays                         │
│  7. 🏎️  Compiler Flags                  │
│  8. 🎛️  USE Flags                       │
│  9. 🐧 Kernel Selection                 │
│ 10. 🎮 Graphics Drivers                 │
│ 11. 🖥️  Desktop Environment             │
│ 12. 📦 Package Preferences              │
│ 13. 🛡️  Secure Boot                     │
│ 14. 🌍 Timezone & Locale                │
│ 15. 👤 User Accounts                    │
│ 16. 📋 Summary                          │
│ 17. 🚀 Installation                     │
│ 18. ✅ Complete!                        │
└─────────────────────────────────────────┘
```

---

## 🎨 Customization

### CFLAGS Presets

| Preset | Flags | Best For |
|:---:|:---|:---|
| 🛡️ Safe | `-march=x86-64 -O2 -pipe` | Maximum compatibility |
| 🎯 Optimized | `-march=native -O2 -pipe` | Your specific CPU |
| 🏎️ Aggressive | `-march=native -O3 -pipe -flto=auto` | Maximum performance |
| 🔧 Custom | *Your choice!* | Power users |

### Kernel Options

| Kernel | Install Time | Description |
|:---:|:---:|:---|
| 🏃 gentoo-kernel-bin | Fastest | Pre-compiled binary |
| 📦 gentoo-kernel | Medium | Distribution kernel |
| 🔧 gentoo-sources | Longer | Full customization |
| ⚡ zen-sources | Longer | Desktop optimized |
| 🚀 xanmod-sources | Longer | Performance focused |
| 🔥 liquorix-sources | Longer | Gaming/desktop focus |

### USE Flag Presets

- 🖥️ **Desktop** - Full desktop experience
- 🎮 **Gaming** - Steam, Wine, Proton ready
- 💼 **Server** - Headless, minimal GUI deps
- 🪶 **Minimal** - Just the essentials
- 💻 **Laptop** - Power management, WiFi

---

## 🔨 Building

### Requirements

- Go 1.22+ 🐹
- Gentoo Linux (for ISO building) 🐧
- Root access (for ISO building) 🔑

### Build TUI Installer

```bash
# Get dependencies
go mod tidy

# Build the TUI installer
go build -o yuno-tui ./cmd/yuno-tui

# Run it~ 💕
sudo ./yuno-tui
```

### Build ISO

```bash
# This needs root and a Gentoo system!
sudo ./scripts/build-iso.sh
```

The ISO will be created in the `output/` directory 📀

---

## 📁 Project Structure

```
yuno-os/
├── 💕 cmd/                    # Entry points
│   └── yuno-tui/              # TUI installer
├── 📦 pkg/                    # Core libraries
│   ├── config/                # Configuration types
│   ├── utils/                 # Utilities
│   ├── partition/             # Disk partitioning
│   ├── encryption/            # Disk encryption
│   ├── stage3/                # Stage3 handling
│   ├── chroot/                # Chroot management
│   ├── overlays/              # Overlay management
│   ├── portage/               # Portage configuration
│   ├── kernel/                # Kernel installation
│   ├── graphics/              # GPU drivers
│   ├── desktop/               # DE/WM installation
│   ├── bootloader/            # Bootloader setup
│   ├── binpkg/                # Binary packages
│   ├── users/                 # User management
│   └── installer/             # Installation orchestrator
├── 🎨 internal/
│   └── tui/                   # TUI implementation
├── 🖼️  calamares/              # Calamares GUI modules
│   ├── modules/               # Custom Calamares modules
│   └── branding/              # Yuno OS branding
├── 🎀 branding/               # Yuno OS assets
│   ├── fastfetch/             # Fastfetch/neofetch config
│   ├── wallpapers/            # Desktop wallpapers
│   └── avatars/               # Default user avatars
├── 🔧 scripts/                # Build scripts
│   └── build-iso.sh           # ISO build script
└── 📀 iso-build/              # Catalyst specs
```

---

## 🤝 Contributing

Contributions are super welcome! Whether it's bug fixes, new features, or documentation improvements~ 💕

1. 🍴 Fork the repository
2. 🌿 Create your feature branch (`git checkout -b feature/amazing-feature`)
3. 💾 Commit your changes (`git commit -m 'Add some amazing feature'`)
4. 📤 Push to the branch (`git push origin feature/amazing-feature`)
5. 🎉 Open a Pull Request

---

## 📚 Resources

- 📖 [Gentoo Handbook](https://wiki.gentoo.org/wiki/Handbook:AMD64) - The Gentoo Bible
- 📚 [Gentoo Wiki](https://wiki.gentoo.org) - All the knowledge
- 🎨 [Calamares](https://calamares.io/) - The installer framework
- 🍵 [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework

---

## 📜 License

This project is licensed under the **AGPL-3.0** License - see the [LICENSE](LICENSE) file for details.

---

## 💕 Acknowledgments

- 🐧 **Gentoo Linux** community for the amazing distro
- 🎨 **Calamares** project for the installer framework
- ✨ **Charm** for the beautiful Bubble Tea TUI framework
- 💗 **Yuno Gasai** for the inspiration (and the axe 🔪)

---

<p align="center">
  <em>Made with 💕 and a little bit of yandere energy~</em>
</p>

<p align="center">
  <img src="branding/avatars/default-avatar.jpg" alt="Yuno" width="150" style="border-radius: 50%;"/>
</p>

<p align="center">
  <strong>"Yukki~ Your perfect Gentoo system awaits!" 💕🔪</strong>
</p>
