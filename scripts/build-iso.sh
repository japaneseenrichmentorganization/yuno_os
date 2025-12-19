#!/bin/bash
#
# Yuno OS ISO Build Script
#
# This script builds a bootable Yuno OS installation ISO
# based on Gentoo LiveGUI with our custom installers.
#

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="${BUILD_DIR:-/var/tmp/yuno-build}"
CACHE_DIR="${CACHE_DIR:-/var/cache/yuno}"
OUTPUT_DIR="${OUTPUT_DIR:-$PROJECT_DIR/output}"

# Gentoo settings
GENTOO_MIRROR="${GENTOO_MIRROR:-https://distfiles.gentoo.org}"
ARCH="amd64"
STAGE3_VARIANT="desktop-openrc"

# ISO settings
ISO_NAME="yuno-os"
ISO_VERSION="1.0"
ISO_LABEL="YUNO_OS"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*"
    exit 1
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root"
    fi
}

check_dependencies() {
    log "Checking dependencies..."

    local deps=(
        "wget"
        "tar"
        "squashfs-tools"
        "grub"
        "xorriso"
        "mtools"
    )

    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            error "Missing dependency: $dep"
        fi
    done

    log "All dependencies satisfied"
}

setup_directories() {
    log "Setting up build directories..."

    mkdir -p "$BUILD_DIR"/{rootfs,iso,work}
    mkdir -p "$CACHE_DIR"
    mkdir -p "$OUTPUT_DIR"
}

fetch_stage3() {
    log "Fetching latest stage3 tarball..."

    local latest_url="$GENTOO_MIRROR/releases/$ARCH/autobuilds/latest-stage3-$ARCH-$STAGE3_VARIANT.txt"
    local latest_file

    latest_file=$(wget -qO- "$latest_url" | grep -v "^#" | head -1 | awk '{print $1}')

    if [[ -z "$latest_file" ]]; then
        error "Could not determine latest stage3"
    fi

    local stage3_url="$GENTOO_MIRROR/releases/$ARCH/autobuilds/$latest_file"
    local stage3_filename=$(basename "$latest_file")
    local stage3_path="$CACHE_DIR/$stage3_filename"

    if [[ -f "$stage3_path" ]]; then
        log "Stage3 already cached: $stage3_filename"
    else
        log "Downloading: $stage3_filename"
        wget -q --show-progress -O "$stage3_path" "$stage3_url"
    fi

    echo "$stage3_path"
}

extract_stage3() {
    local stage3_path="$1"
    local rootfs="$BUILD_DIR/rootfs"

    log "Extracting stage3..."

    tar xpf "$stage3_path" \
        --xattrs-include='*.*' \
        --numeric-owner \
        -C "$rootfs"
}

setup_chroot() {
    local rootfs="$BUILD_DIR/rootfs"

    log "Setting up chroot environment..."

    # Mount essential filesystems
    mount -t proc proc "$rootfs/proc"
    mount -t sysfs sysfs "$rootfs/sys"
    mount -t devtmpfs devtmpfs "$rootfs/dev"
    mount -t devpts devpts "$rootfs/dev/pts"
    mount -t tmpfs tmpfs "$rootfs/tmp"

    # Copy DNS resolution
    cp -L /etc/resolv.conf "$rootfs/etc/resolv.conf"
}

teardown_chroot() {
    local rootfs="$BUILD_DIR/rootfs"

    log "Tearing down chroot..."

    umount -lf "$rootfs/dev/pts" 2>/dev/null || true
    umount -lf "$rootfs/dev" 2>/dev/null || true
    umount -lf "$rootfs/sys" 2>/dev/null || true
    umount -lf "$rootfs/proc" 2>/dev/null || true
    umount -lf "$rootfs/tmp" 2>/dev/null || true
}

install_packages() {
    local rootfs="$BUILD_DIR/rootfs"

    log "Installing packages..."

    # Configure make.conf for ISO build
    cat > "$rootfs/etc/portage/make.conf" << 'EOF'
COMMON_FLAGS="-march=x86-64 -O2 -pipe"
CFLAGS="${COMMON_FLAGS}"
CXXFLAGS="${COMMON_FLAGS}"
MAKEOPTS="-j$(nproc)"
FEATURES="parallel-fetch candy"
ACCEPT_LICENSE="*"
USE="X wayland pulseaudio networkmanager -systemd"
VIDEO_CARDS="amdgpu radeonsi intel i965 iris nvidia nouveau"
INPUT_DEVICES="libinput"
GRUB_PLATFORMS="efi-64 pc"
EOF

    # Sync portage
    chroot "$rootfs" emerge-webrsync

    # Select profile
    chroot "$rootfs" eselect profile set default/linux/amd64/23.0/desktop

    # Install essential packages for live environment
    chroot "$rootfs" emerge --ask=n --quiet-build \
        sys-kernel/gentoo-kernel-bin \
        sys-kernel/linux-firmware \
        sys-boot/grub \
        app-misc/calamares \
        kde-plasma/plasma-meta \
        kde-apps/konsole \
        kde-apps/dolphin \
        x11-misc/sddm \
        net-misc/networkmanager \
        sys-apps/dbus \
        app-editors/nano \
        dev-vcs/git \
        app-eselect/eselect-repository \
        app-admin/metalog \
        app-misc/fastfetch

    # Install our TUI installer
    # (This would copy the compiled Go binary)
}

install_yuno_installer() {
    local rootfs="$BUILD_DIR/rootfs"

    log "Installing Yuno OS installer..."

    # Build the Go TUI installer
    cd "$PROJECT_DIR"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$rootfs/usr/bin/yuno-tui" ./cmd/yuno-tui

    # Copy Calamares configuration
    mkdir -p "$rootfs/etc/calamares"
    cp -r "$PROJECT_DIR/calamares/"* "$rootfs/etc/calamares/"

    # Create desktop entry for GUI installer
    mkdir -p "$rootfs/usr/share/applications"
    cat > "$rootfs/usr/share/applications/yuno-install.desktop" << 'EOF'
[Desktop Entry]
Name=Install Yuno OS
Comment=Install Yuno OS to your computer
Exec=pkexec calamares
Icon=calamares
Terminal=false
Type=Application
Categories=System;
EOF

    # Create desktop entry for TUI installer
    cat > "$rootfs/usr/share/applications/yuno-tui.desktop" << 'EOF'
[Desktop Entry]
Name=Install Yuno OS (TUI)
Comment=Install Yuno OS using the terminal interface
Exec=pkexec konsole -e yuno-tui
Icon=utilities-terminal
Terminal=false
Type=Application
Categories=System;
EOF

    # Install Yuno OS branding assets
    log "Installing Yuno OS branding..."

    mkdir -p "$rootfs/usr/share/yuno-os/avatars"
    mkdir -p "$rootfs/usr/share/yuno-os/wallpapers"
    mkdir -p "$rootfs/usr/share/fastfetch/presets"
    mkdir -p "$rootfs/etc/skel/.config/fastfetch"

    # Copy branding assets
    cp "$PROJECT_DIR/branding/avatars/default-avatar.jpg" "$rootfs/usr/share/yuno-os/avatars/"
    cp "$PROJECT_DIR/branding/wallpapers/yuno-wallpaper.jpg" "$rootfs/usr/share/yuno-os/wallpapers/"
    cp "$PROJECT_DIR/branding/fastfetch/yuno.txt" "$rootfs/usr/share/yuno-os/"
    cp "$PROJECT_DIR/branding/fastfetch/config.jsonc" "$rootfs/usr/share/fastfetch/presets/yuno.jsonc"

    # Set up default fastfetch config for new users
    cp "$PROJECT_DIR/branding/fastfetch/config.jsonc" "$rootfs/etc/skel/.config/fastfetch/config.jsonc"

    # Copy wallpaper to backgrounds
    mkdir -p "$rootfs/usr/share/backgrounds/yuno"
    cp "$PROJECT_DIR/branding/wallpapers/yuno-wallpaper.jpg" "$rootfs/usr/share/backgrounds/yuno/"

    # Set default avatar for live user
    cp "$PROJECT_DIR/branding/avatars/default-avatar.jpg" "$rootfs/etc/skel/.face"
    cp "$PROJECT_DIR/branding/avatars/default-avatar.jpg" "$rootfs/etc/skel/.face.icon"
}

configure_live_system() {
    local rootfs="$BUILD_DIR/rootfs"

    log "Configuring live system..."

    # Set hostname
    echo "yuno-live" > "$rootfs/etc/hostname"

    # Configure hosts
    cat > "$rootfs/etc/hosts" << 'EOF'
127.0.0.1   localhost
::1         localhost
127.0.1.1   yuno-live.localdomain yuno-live
EOF

    # Enable services
    chroot "$rootfs" rc-update add dbus default
    chroot "$rootfs" rc-update add NetworkManager default
    chroot "$rootfs" rc-update add sddm default

    # Create live user
    chroot "$rootfs" useradd -m -G wheel,audio,video,input,plugdev -s /bin/bash live
    echo "live:live" | chroot "$rootfs" chpasswd

    # Allow live user to run installer without password
    cat > "$rootfs/etc/polkit-1/rules.d/49-yuno-installer.rules" << 'EOF'
polkit.addRule(function(action, subject) {
    if (action.id == "org.freedesktop.calamares" && subject.user == "live") {
        return polkit.Result.YES;
    }
});
EOF

    # Auto-login for live session
    mkdir -p "$rootfs/etc/sddm.conf.d"
    cat > "$rootfs/etc/sddm.conf.d/autologin.conf" << 'EOF'
[Autologin]
User=live
Session=plasma
EOF
}

create_squashfs() {
    local rootfs="$BUILD_DIR/rootfs"
    local iso_dir="$BUILD_DIR/iso"

    log "Creating squashfs image..."

    mkdir -p "$iso_dir/LiveOS"

    mksquashfs "$rootfs" "$iso_dir/LiveOS/squashfs.img" \
        -comp xz \
        -Xbcj x86 \
        -b 1M \
        -no-duplicates \
        -progress
}

setup_bootloader() {
    local rootfs="$BUILD_DIR/rootfs"
    local iso_dir="$BUILD_DIR/iso"

    log "Setting up bootloader..."

    mkdir -p "$iso_dir/boot/grub"
    mkdir -p "$iso_dir/EFI/BOOT"

    # Copy kernel and initramfs
    cp "$rootfs/boot/vmlinuz-"* "$iso_dir/boot/vmlinuz"
    cp "$rootfs/boot/initramfs-"*.img "$iso_dir/boot/initramfs.img"

    # Create GRUB configuration
    cat > "$iso_dir/boot/grub/grub.cfg" << 'EOF'
set timeout=10
set default=0

menuentry "Yuno OS Live" {
    linux /boot/vmlinuz root=live:CDLABEL=YUNO_OS rd.live.image quiet splash
    initrd /boot/initramfs.img
}

menuentry "Yuno OS Live (Safe Mode)" {
    linux /boot/vmlinuz root=live:CDLABEL=YUNO_OS rd.live.image nomodeset
    initrd /boot/initramfs.img
}

menuentry "Boot from local disk" {
    exit
}
EOF

    # Create EFI bootloader
    grub-mkstandalone \
        --format=x86_64-efi \
        --output="$iso_dir/EFI/BOOT/BOOTX64.EFI" \
        --locales="" \
        --fonts="" \
        "boot/grub/grub.cfg=$iso_dir/boot/grub/grub.cfg"

    # Create EFI image
    dd if=/dev/zero of="$iso_dir/boot/efiboot.img" bs=1M count=10
    mkfs.vfat "$iso_dir/boot/efiboot.img"
    mmd -i "$iso_dir/boot/efiboot.img" ::/EFI ::/EFI/BOOT
    mcopy -i "$iso_dir/boot/efiboot.img" "$iso_dir/EFI/BOOT/BOOTX64.EFI" ::/EFI/BOOT/
}

create_iso() {
    local iso_dir="$BUILD_DIR/iso"
    local output_iso="$OUTPUT_DIR/${ISO_NAME}-${ISO_VERSION}.iso"

    log "Creating ISO image..."

    xorriso -as mkisofs \
        -iso-level 3 \
        -full-iso9660-filenames \
        -volid "$ISO_LABEL" \
        -eltorito-boot boot/grub/eltorito.img \
        -no-emul-boot \
        -boot-load-size 4 \
        -boot-info-table \
        --eltorito-catalog boot/grub/boot.cat \
        --grub2-boot-info \
        --grub2-mbr /usr/lib/grub/i386-pc/boot_hybrid.img \
        -eltorito-alt-boot \
        -e boot/efiboot.img \
        -no-emul-boot \
        -append_partition 2 0xef "$iso_dir/boot/efiboot.img" \
        -output "$output_iso" \
        "$iso_dir"

    log "ISO created: $output_iso"

    # Calculate checksums
    cd "$OUTPUT_DIR"
    sha256sum "$(basename "$output_iso")" > "$(basename "$output_iso").sha256"

    log "Build complete!"
    log "ISO: $output_iso"
    log "SHA256: $output_iso.sha256"
}

cleanup() {
    log "Cleaning up..."
    teardown_chroot
    rm -rf "$BUILD_DIR/rootfs" "$BUILD_DIR/iso" "$BUILD_DIR/work"
}

main() {
    log "Yuno OS ISO Build Script"
    log "========================"

    check_root
    check_dependencies

    trap cleanup EXIT

    setup_directories

    local stage3_path
    stage3_path=$(fetch_stage3)

    extract_stage3 "$stage3_path"
    setup_chroot
    install_packages
    install_yuno_installer
    configure_live_system
    teardown_chroot
    create_squashfs
    setup_bootloader
    create_iso
}

main "$@"
