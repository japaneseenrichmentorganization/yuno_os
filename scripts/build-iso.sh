#!/bin/bash
#
# Yuno OS ISO Build Script
#
# This script builds a bootable Yuno OS installation ISO
# based on Gentoo LiveGUI with our custom installers.
#
# Can be run from ANY Linux distribution - not just Gentoo!
# The script bootstraps a Gentoo build environment automatically.
#
# Reference: https://wiki.gentoo.org/wiki/Handbook:AMD64/Full/Installation
#

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="${BUILD_DIR:-/var/tmp/yuno-build}"
CACHE_DIR="${CACHE_DIR:-/var/cache/yuno}"
OUTPUT_DIR="${OUTPUT_DIR:-$PROJECT_DIR/output}"

# Build environment (for non-Gentoo hosts)
BUILD_ENV_DIR="$BUILD_DIR/gentoo-buildenv"

# Gentoo settings
GENTOO_MIRROR="${GENTOO_MIRROR:-https://distfiles.gentoo.org}"
ARCH="amd64"
INIT_SYSTEM="${INIT_SYSTEM:-openrc}"  # openrc or systemd

# Build optimization settings
USE_NATIVE="${USE_NATIVE:-false}"      # Use -march=native -mtune=native
OPT_LEVEL="${OPT_LEVEL:-2}"            # Optimization level: 2 or 3
ENABLE_LTO="${ENABLE_LTO:-false}"      # Enable LTO via GentooLTO overlay
USE_PIPE="${USE_PIPE:-true}"           # Use -pipe (faster, uses more RAM)
USE_TESTING="${USE_TESTING:-false}"    # Use ~amd64 (testing) instead of stable
STAGE1_BUILD="${STAGE1_BUILD:-false}"  # Rebuild toolchain from scratch (stage1)

# ISO settings
ISO_NAME="yuno-os"
ISO_VERSION="1.0"
ISO_LABEL="YUNO_OS"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m'

# Detect if we're on Gentoo
IS_GENTOO=false
if [[ -f /etc/gentoo-release ]]; then
    IS_GENTOO=true
fi

log() {
    echo -e "${GREEN}[INFO]${NC} $*" >&2
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*" >&2
}

error() {
    echo -e "${RED}[ERROR]${NC} $*"
    exit 1
}

header() {
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}" >&2
    echo -e "${MAGENTA}  $*${NC}" >&2
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}" >&2
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root"
    fi
}

check_host_dependencies() {
    log "Checking host system dependencies..."

    # Minimal dependencies needed on the host
    local deps=(
        "wget"
        "tar"
        "xz"
    )

    local missing=()
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            missing+=("$dep")
        fi
    done

    if [[ ${#missing[@]} -gt 0 ]]; then
        error "Missing host dependencies: ${missing[*]}"
    fi

    log "Host dependencies satisfied"
}

setup_directories() {
    log "Setting up build directories..."

    mkdir -p "$BUILD_DIR"/{rootfs,iso,work}
    mkdir -p "$BUILD_ENV_DIR"
    mkdir -p "$CACHE_DIR"
    mkdir -p "$OUTPUT_DIR"
}

# ============================================================================
# GENTOO BUILD ENVIRONMENT SETUP (for non-Gentoo hosts)
# Reference: Gentoo Handbook - Installing from a non-Gentoo LiveCD
# ============================================================================

fetch_stage3() {
    local target_dir="$1"
    local variant="${2:-desktop-${INIT_SYSTEM}}"

    log "Fetching latest stage3 tarball (variant: $variant)..."

    local latest_url="$GENTOO_MIRROR/releases/$ARCH/autobuilds/latest-stage3-$ARCH-$variant.txt"
    local latest_file

    latest_file=$(wget -qO- "$latest_url" | grep "\.tar\.xz" | head -1 | awk '{print $1}')

    if [[ -z "$latest_file" ]]; then
        error "Could not determine latest stage3"
    fi

    local stage3_url="$GENTOO_MIRROR/releases/$ARCH/autobuilds/$latest_file"
    local stage3_filename=$(basename "$latest_file")
    local stage3_path="$CACHE_DIR/$stage3_filename"

    if [[ -f "$stage3_path" ]]; then
        log "Stage3 already cached: $stage3_filename"
    else
        log "Downloading: $stage3_filename (this may take a while)..."
        # Download without progress to avoid stdout pollution
        # Progress would interfere with command substitution
        if ! wget -q -O "$stage3_path" "$stage3_url"; then
            error "Failed to download stage3 tarball"
        fi
        log "Download complete: $stage3_filename"

        # Also fetch the signature for verification
        wget -q -O "$stage3_path.asc" "$stage3_url.asc" 2>/dev/null || true
    fi

    # Return only the path to stdout
    echo "$stage3_path"
}

extract_stage3() {
    local stage3_path="$1"
    local target_dir="$2"

    log "Extracting stage3 to $target_dir..."

    # Extract with proper options as per handbook
    tar xpf "$stage3_path" \
        --xattrs-include='*.*' \
        --numeric-owner \
        -C "$target_dir"
}

setup_buildenv_chroot() {
    local rootfs="$1"

    log "Setting up chroot environment..."

    # Mount essential filesystems (as per Gentoo Handbook)
    mount --types proc /proc "$rootfs/proc"
    mount --rbind /sys "$rootfs/sys"
    mount --make-rslave "$rootfs/sys"
    mount --rbind /dev "$rootfs/dev"
    mount --make-rslave "$rootfs/dev"
    mount --bind /run "$rootfs/run" 2>/dev/null || true
    mount --make-slave "$rootfs/run" 2>/dev/null || true

    # For non-Gentoo hosts, also bind /dev/shm properly
    if [[ -L /dev/shm ]]; then
        rm "$rootfs/dev/shm" 2>/dev/null || true
        mkdir "$rootfs/dev/shm"
        mount --types tmpfs --options nosuid,nodev,noexec shm "$rootfs/dev/shm"
        chmod 1777 "$rootfs/dev/shm" /run/shm 2>/dev/null || true
    fi

    # Copy DNS resolution
    cp --dereference /etc/resolv.conf "$rootfs/etc/resolv.conf"
}

teardown_chroot() {
    local rootfs="$1"

    log "Tearing down chroot..."

    # Unmount in reverse order
    umount -lf "$rootfs/dev/shm" 2>/dev/null || true
    umount -lf "$rootfs/run" 2>/dev/null || true
    umount -lf "$rootfs/dev/pts" 2>/dev/null || true
    umount -lf "$rootfs/dev" 2>/dev/null || true
    umount -lf "$rootfs/sys" 2>/dev/null || true
    umount -lf "$rootfs/proc" 2>/dev/null || true
}

run_in_chroot() {
    local rootfs="$1"
    shift

    chroot "$rootfs" /bin/bash -c "source /etc/profile && $*"
}

# Setup Gentoo build environment on non-Gentoo host
setup_gentoo_buildenv() {
    if [[ "$IS_GENTOO" == "true" ]]; then
        log "Running on Gentoo - using host tools directly"
        return 0
    fi

    header "Setting up Gentoo build environment"
    log "Host is not Gentoo - bootstrapping Gentoo build environment..."

    if [[ -f "$BUILD_ENV_DIR/usr/bin/emerge" ]]; then
        log "Build environment already exists, reusing..."
        setup_buildenv_chroot "$BUILD_ENV_DIR"
        return 0
    fi

    # Fetch a minimal stage3 for the build environment
    local stage3_path
    stage3_path=$(fetch_stage3 "$BUILD_ENV_DIR" "openrc")

    extract_stage3 "$stage3_path" "$BUILD_ENV_DIR"
    setup_buildenv_chroot "$BUILD_ENV_DIR"

    # Configure make.conf for the build environment
    local nproc_count
    nproc_count=$(nproc)
    cat > "$BUILD_ENV_DIR/etc/portage/make.conf" << EOF
COMMON_FLAGS="-march=x86-64 -O2 -pipe"
CFLAGS="\${COMMON_FLAGS}"
CXXFLAGS="\${COMMON_FLAGS}"
MAKEOPTS="-j${nproc_count}"
FEATURES="parallel-fetch -sandbox -usersandbox"
ACCEPT_LICENSE="*"
GRUB_PLATFORMS="efi-64 pc"
EOF

    # Sync portage tree
    log "Syncing Portage tree in build environment..."
    run_in_chroot "$BUILD_ENV_DIR" "emerge-webrsync"

    # Install required tools for ISO building
    log "Installing ISO build tools in build environment..."
    run_in_chroot "$BUILD_ENV_DIR" "emerge --ask=n --quiet-build \
        sys-fs/squashfs-tools \
        sys-boot/grub \
        dev-libs/libisoburn \
        sys-fs/mtools \
        sys-fs/dosfstools \
        dev-lang/go"

    log "Build environment ready!"
}

# Wrapper to run commands in the build environment
buildenv_run() {
    if [[ "$IS_GENTOO" == "true" ]]; then
        # Run directly on Gentoo host
        "$@"
    else
        # Run in the Gentoo build environment chroot
        run_in_chroot "$BUILD_ENV_DIR" "$*"
    fi
}

# ============================================================================
# ISO BUILDING FUNCTIONS
# ============================================================================

install_packages() {
    local rootfs="$BUILD_DIR/rootfs"

    header "Installing packages in ISO rootfs"

    # Configure make.conf for ISO build
    local nproc_count
    nproc_count=$(nproc)

    # Set compiler flags based on optimization settings
    local march_flags
    if [[ "$USE_NATIVE" == "true" ]]; then
        march_flags="-march=native -mtune=native"
        log "Using native CPU optimizations (-march=native -mtune=native)"
    else
        march_flags="-march=x86-64 -mtune=generic"
        log "Using generic x86-64 optimizations (portable)"
    fi

    local opt_flag="-O${OPT_LEVEL}"
    log "Optimization level: ${opt_flag}"

    # Set pipe flag
    local pipe_flag=""
    if [[ "$USE_PIPE" == "true" ]]; then
        pipe_flag="-pipe"
        log "Using -pipe (faster compilation, requires more RAM)"
    fi

    # Set USE flags based on init system
    local use_flags="X wayland pulseaudio pipewire networkmanager bluetooth"
    if [[ "$INIT_SYSTEM" == "systemd" ]]; then
        use_flags="$use_flags systemd"
    else
        use_flags="$use_flags -systemd"
    fi

    # Add LTO USE flag if enabled
    if [[ "$ENABLE_LTO" == "true" ]]; then
        use_flags="$use_flags lto"
        log "LTO (Link Time Optimization) enabled"
    fi

    # Detect CPU flags if using native optimizations
    local cpu_flags=""
    if [[ "$USE_NATIVE" == "true" ]]; then
        # Try to detect CPU flags using cpuid2cpuflags if available
        if command -v cpuid2cpuflags &> /dev/null; then
            cpu_flags=$(cpuid2cpuflags | sed 's/CPU_FLAGS_X86: //')
            log "Detected CPU flags: $cpu_flags"
        else
            # Fallback: detect common flags from /proc/cpuinfo
            log "cpuid2cpuflags not found, detecting flags from /proc/cpuinfo..."
            local detected_flags=""
            local cpuinfo_flags=$(grep -m1 "^flags" /proc/cpuinfo | cut -d: -f2)

            # Map common CPU features to Gentoo CPU_FLAGS_X86
            [[ $cpuinfo_flags == *" mmx "* ]] && detected_flags="$detected_flags mmx"
            [[ $cpuinfo_flags == *" mmxext "* ]] && detected_flags="$detected_flags mmxext"
            [[ $cpuinfo_flags == *" sse "* ]] && detected_flags="$detected_flags sse"
            [[ $cpuinfo_flags == *" sse2 "* ]] && detected_flags="$detected_flags sse2"
            [[ $cpuinfo_flags == *" sse3 "* || $cpuinfo_flags == *" pni "* ]] && detected_flags="$detected_flags sse3"
            [[ $cpuinfo_flags == *" ssse3 "* ]] && detected_flags="$detected_flags ssse3"
            [[ $cpuinfo_flags == *" sse4_1 "* ]] && detected_flags="$detected_flags sse4_1"
            [[ $cpuinfo_flags == *" sse4_2 "* ]] && detected_flags="$detected_flags sse4_2"
            [[ $cpuinfo_flags == *" avx "* ]] && detected_flags="$detected_flags avx"
            [[ $cpuinfo_flags == *" avx2 "* ]] && detected_flags="$detected_flags avx2"
            [[ $cpuinfo_flags == *" avx512f "* ]] && detected_flags="$detected_flags avx512f"
            [[ $cpuinfo_flags == *" avx512dq "* ]] && detected_flags="$detected_flags avx512dq"
            [[ $cpuinfo_flags == *" avx512cd "* ]] && detected_flags="$detected_flags avx512cd"
            [[ $cpuinfo_flags == *" avx512bw "* ]] && detected_flags="$detected_flags avx512bw"
            [[ $cpuinfo_flags == *" avx512vl "* ]] && detected_flags="$detected_flags avx512vl"
            [[ $cpuinfo_flags == *" aes "* ]] && detected_flags="$detected_flags aes"
            [[ $cpuinfo_flags == *" pclmulqdq "* ]] && detected_flags="$detected_flags pclmul"
            [[ $cpuinfo_flags == *" popcnt "* ]] && detected_flags="$detected_flags popcnt"
            [[ $cpuinfo_flags == *" f16c "* ]] && detected_flags="$detected_flags f16c"
            [[ $cpuinfo_flags == *" fma "* ]] && detected_flags="$detected_flags fma3"
            [[ $cpuinfo_flags == *" bmi1 "* ]] && detected_flags="$detected_flags bmi"
            [[ $cpuinfo_flags == *" bmi2 "* ]] && detected_flags="$detected_flags bmi2"

            cpu_flags=$(echo $detected_flags | xargs)  # trim whitespace
            log "Detected CPU flags: $cpu_flags"
        fi
    fi

    # Set ACCEPT_KEYWORDS based on testing flag
    local accept_keywords="amd64"
    if [[ "$USE_TESTING" == "true" ]]; then
        accept_keywords="~amd64"
        log "Using testing branch (~amd64)"
    else
        log "Using stable branch (amd64)"
    fi

    cat > "$rootfs/etc/portage/make.conf" << EOF
COMMON_FLAGS="${march_flags} ${opt_flag} ${pipe_flag}"
CFLAGS="\${COMMON_FLAGS}"
CXXFLAGS="\${COMMON_FLAGS}"
MAKEOPTS="-j${nproc_count}"
FEATURES="parallel-fetch candy -getbinpkg"
ACCEPT_LICENSE="*"
ACCEPT_KEYWORDS="${accept_keywords}"
USE="${use_flags}"
VIDEO_CARDS="amdgpu radeonsi intel i965 iris nvidia nouveau virgl"
INPUT_DEVICES="libinput"
GRUB_PLATFORMS="efi-64 pc"
EOF

    # Add CPU_FLAGS_X86 if detected
    if [[ -n "$cpu_flags" ]]; then
        cat >> "$rootfs/etc/portage/make.conf" << EOF

# CPU-specific instruction set flags
CPU_FLAGS_X86="${cpu_flags}"
EOF
    fi

    # Add LTO-specific configuration if enabled
    if [[ "$ENABLE_LTO" == "true" ]]; then
        cat >> "$rootfs/etc/portage/make.conf" << 'EOF'

# LTO Configuration
LDFLAGS="${LDFLAGS} -fuse-linker-plugin"
EOF
    fi

    # Sync portage
    log "Syncing Portage tree..."
    run_in_chroot "$rootfs" "emerge-webrsync"

    # Install cpuid2cpuflags inside chroot and regenerate flags if needed
    if [[ "$USE_NATIVE" == "true" && -z "$cpu_flags" ]]; then
        log "Installing cpuid2cpuflags for accurate CPU flag detection..."
        run_in_chroot "$rootfs" "emerge --ask=n --oneshot app-portage/cpuid2cpuflags"
        local chroot_cpu_flags
        chroot_cpu_flags=$(run_in_chroot "$rootfs" "cpuid2cpuflags" | sed 's/CPU_FLAGS_X86: //')
        if [[ -n "$chroot_cpu_flags" ]]; then
            log "Detected CPU flags (via chroot): $chroot_cpu_flags"
            echo "CPU_FLAGS_X86=\"${chroot_cpu_flags}\"" >> "$rootfs/etc/portage/make.conf"
        fi
    fi

    # Setup GentooLTO overlay if LTO is enabled
    if [[ "$ENABLE_LTO" == "true" ]]; then
        log "Setting up GentooLTO overlay..."
        run_in_chroot "$rootfs" "emerge --ask=n app-eselect/eselect-repository"
        run_in_chroot "$rootfs" "eselect repository enable lto-overlay"
        run_in_chroot "$rootfs" "emerge --sync lto-overlay"
    fi

    # Select profile based on init system
    log "Selecting desktop profile (${INIT_SYSTEM})..."
    if [[ "$INIT_SYSTEM" == "systemd" ]]; then
        run_in_chroot "$rootfs" "eselect profile set default/linux/amd64/23.0/desktop/systemd"
    else
        run_in_chroot "$rootfs" "eselect profile set default/linux/amd64/23.0/desktop"
    fi

    # Handle GCC upgrade for testing branch
    # When using ~amd64, we likely have a newer GCC available that needs to be
    # installed and set as the default compiler before building other packages
    if [[ "$USE_TESTING" == "true" ]]; then
        header "Upgrading GCC for testing branch"
        log "Testing branch (~amd64) detected - checking for newer GCC..."

        # First, update portage itself
        log "Updating portage..."
        run_in_chroot "$rootfs" "emerge --ask=n --oneshot sys-apps/portage" || true

        # Install the newest GCC available
        log "Installing latest GCC from testing branch..."
        run_in_chroot "$rootfs" "emerge --ask=n --oneshot sys-devel/gcc"

        # Get the list of installed GCC versions and select the newest
        log "Switching to the newest GCC version..."
        run_in_chroot "$rootfs" "
            # List available GCC versions
            gcc-config -l

            # Get the newest version (last in the list)
            NEWEST_GCC=\$(gcc-config -l | tail -1 | awk '{print \$2}' | tr -d '[]')

            # If we got a valid profile, switch to it
            if [[ -n \"\$NEWEST_GCC\" ]]; then
                echo \"Switching to GCC profile: \$NEWEST_GCC\"
                gcc-config \"\$NEWEST_GCC\"
            fi
        "

        # Source the updated environment to use the new GCC
        log "Updating environment for new GCC..."
        run_in_chroot "$rootfs" "env-update && source /etc/profile"

        # Verify GCC version
        log "New GCC version:"
        run_in_chroot "$rootfs" "gcc --version | head -1"

        # Rebuild libtool to fix .la files with new GCC
        log "Rebuilding libtool for new GCC..."
        run_in_chroot "$rootfs" "emerge --ask=n --oneshot sys-devel/libtool" || true
    fi

    # Stage 1 build: rebuild the entire toolchain with our optimization flags
    if [[ "$STAGE1_BUILD" == "true" ]]; then
        header "Stage 1: Rebuilding toolchain"
        warn "This will take a VERY long time (several hours)..."

        # If we're also on testing, GCC was already upgraded above
        # Now we rebuild the full toolchain with our optimization flags

        # Rebuild binutils with our flags
        log "Rebuilding binutils with optimization flags..."
        run_in_chroot "$rootfs" "emerge --ask=n --oneshot sys-devel/binutils"

        # Rebuild GCC with our optimization flags
        log "Rebuilding GCC with optimization flags..."
        run_in_chroot "$rootfs" "emerge --ask=n --oneshot sys-devel/gcc"

        # Switch to the newly built GCC (it will have the same version but rebuilt with our flags)
        log "Switching to rebuilt GCC..."
        run_in_chroot "$rootfs" "
            # Get the current/newest GCC profile
            CURRENT_GCC=\$(gcc-config -c)
            echo \"Current GCC profile: \$CURRENT_GCC\"

            # Get all profiles for this version
            gcc-config -l

            # Select the newest profile (in case a new slot was created)
            NEWEST_GCC=\$(gcc-config -l | tail -1 | awk '{print \$2}' | tr -d '[]')
            if [[ -n \"\$NEWEST_GCC\" && \"\$NEWEST_GCC\" != \"\$CURRENT_GCC\" ]]; then
                echo \"Switching to: \$NEWEST_GCC\"
                gcc-config \"\$NEWEST_GCC\"
            fi
        "

        # Update environment
        run_in_chroot "$rootfs" "env-update && source /etc/profile"

        # Verify the compiler is working
        log "Verifying GCC after rebuild..."
        run_in_chroot "$rootfs" "gcc --version | head -1"
        run_in_chroot "$rootfs" "echo 'int main(){}' | gcc -x c - -o /tmp/test && rm /tmp/test && echo 'GCC working!'"

        # Rebuild glibc with the new compiler
        log "Rebuilding glibc with optimized toolchain..."
        run_in_chroot "$rootfs" "emerge --ask=n --oneshot sys-libs/glibc"

        # Rebuild libtool to fix any .la files
        log "Rebuilding libtool..."
        run_in_chroot "$rootfs" "emerge --ask=n --oneshot sys-devel/libtool"

        # Fix any library inconsistencies
        log "Fixing library dependencies..."
        run_in_chroot "$rootfs" "emerge --ask=n --oneshot sys-devel/binutils" || true

        # Now rebuild the entire system with the new toolchain
        log "Rebuilding @world with optimized toolchain..."
        log "This is the longest step - rebuilding all packages with the new compiler..."
        run_in_chroot "$rootfs" "emerge --ask=n --emptytree --deep --with-bdeps=y @world" || {
            warn "Full @world rebuild had some failures, attempting to continue..."
            # Try to fix broken packages
            run_in_chroot "$rootfs" "emerge --ask=n --resume --skipfirst" || true
        }

        # Clean up old GCC versions
        log "Cleaning up old compiler versions..."
        run_in_chroot "$rootfs" "emerge --ask=n --depclean sys-devel/gcc" || true

        log "Stage 1 toolchain rebuild complete!"
    else
        # Normal update
        log "Updating @world..."
        run_in_chroot "$rootfs" "emerge --update --deep --newuse @world" || true
    fi

    # Install essential packages for live environment
    log "Installing essential packages..."
    run_in_chroot "$rootfs" "emerge --ask=n --quiet-build \
        sys-kernel/gentoo-kernel-bin \
        sys-kernel/linux-firmware \
        sys-boot/grub \
        sys-boot/efibootmgr"

    log "Installing desktop environment..."
    run_in_chroot "$rootfs" "emerge --ask=n --quiet-build \
        kde-plasma/plasma-meta \
        kde-apps/konsole \
        kde-apps/dolphin \
        kde-apps/gwenview \
        x11-misc/sddm"

    log "Installing system utilities..."
    local logger_pkg=""
    if [[ "$INIT_SYSTEM" == "openrc" ]]; then
        logger_pkg="app-admin/metalog"
    fi
    # systemd uses journald, no separate logger needed

    run_in_chroot "$rootfs" "emerge --ask=n --quiet-build \
        net-misc/networkmanager \
        sys-apps/dbus \
        ${logger_pkg} \
        app-misc/fastfetch \
        app-editors/nano \
        app-editors/vim \
        dev-vcs/git \
        app-eselect/eselect-repository \
        sys-fs/dosfstools \
        sys-fs/e2fsprogs \
        sys-fs/btrfs-progs \
        sys-fs/xfsprogs \
        sys-fs/cryptsetup \
        app-arch/zstd"

    # Install Calamares if available, or prepare for TUI-only mode
    log "Attempting to install Calamares..."
    run_in_chroot "$rootfs" "emerge --ask=n --quiet-build app-misc/calamares" || {
        warn "Calamares not available - TUI installer will be the primary option"
    }
}

install_yuno_installer() {
    local rootfs="$BUILD_DIR/rootfs"

    header "Installing Yuno OS installer"

    # Build the Go tools
    log "Building Yuno tools..."
    cd "$PROJECT_DIR"

    if [[ "$IS_GENTOO" == "true" ]]; then
        log "Building yuno-tui..."
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$rootfs/usr/bin/yuno-tui" ./cmd/yuno-tui
        log "Building yuno-use..."
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$rootfs/usr/bin/yuno-use" ./cmd/yuno-use
    else
        # Build inside the build environment
        cp -r "$PROJECT_DIR" "$BUILD_ENV_DIR/tmp/yuno-build"
        run_in_chroot "$BUILD_ENV_DIR" "cd /tmp/yuno-build && \
            CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/yuno-tui ./cmd/yuno-tui && \
            CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/yuno-use ./cmd/yuno-use"
        cp "$BUILD_ENV_DIR/tmp/yuno-tui" "$rootfs/usr/bin/yuno-tui"
        cp "$BUILD_ENV_DIR/tmp/yuno-use" "$rootfs/usr/bin/yuno-use"
        rm -rf "$BUILD_ENV_DIR/tmp/yuno-build" "$BUILD_ENV_DIR/tmp/yuno-tui" "$BUILD_ENV_DIR/tmp/yuno-use"
    fi

    chmod +x "$rootfs/usr/bin/yuno-tui"
    chmod +x "$rootfs/usr/bin/yuno-use"
    log "Yuno tools installed: yuno-tui, yuno-use 💕"

    # Install man pages
    log "Installing man pages..."
    mkdir -p "$rootfs/usr/share/man/man1"
    if [[ -f "$PROJECT_DIR/man/yuno-use.1" ]]; then
        cp "$PROJECT_DIR/man/yuno-use.1" "$rootfs/usr/share/man/man1/"
        gzip -f "$rootfs/usr/share/man/man1/yuno-use.1" 2>/dev/null || true
    fi

    # Copy Calamares configuration
    if [[ -d "$PROJECT_DIR/calamares" ]]; then
        mkdir -p "$rootfs/etc/calamares"
        cp -r "$PROJECT_DIR/calamares/"* "$rootfs/etc/calamares/"
    fi

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

    # Copy branding assets if they exist
    if [[ -f "$PROJECT_DIR/branding/avatars/default-avatar.jpg" ]]; then
        cp "$PROJECT_DIR/branding/avatars/default-avatar.jpg" "$rootfs/usr/share/yuno-os/avatars/"
        cp "$PROJECT_DIR/branding/avatars/default-avatar.jpg" "$rootfs/etc/skel/.face"
        cp "$PROJECT_DIR/branding/avatars/default-avatar.jpg" "$rootfs/etc/skel/.face.icon"
    fi

    if [[ -f "$PROJECT_DIR/branding/wallpapers/yuno-wallpaper.jpg" ]]; then
        cp "$PROJECT_DIR/branding/wallpapers/yuno-wallpaper.jpg" "$rootfs/usr/share/yuno-os/wallpapers/"
        mkdir -p "$rootfs/usr/share/backgrounds/yuno"
        cp "$PROJECT_DIR/branding/wallpapers/yuno-wallpaper.jpg" "$rootfs/usr/share/backgrounds/yuno/"
    fi

    if [[ -f "$PROJECT_DIR/branding/fastfetch/yuno.txt" ]]; then
        cp "$PROJECT_DIR/branding/fastfetch/yuno.txt" "$rootfs/usr/share/yuno-os/"
    fi

    if [[ -f "$PROJECT_DIR/branding/fastfetch/config.jsonc" ]]; then
        cp "$PROJECT_DIR/branding/fastfetch/config.jsonc" "$rootfs/usr/share/fastfetch/presets/yuno.jsonc"
        cp "$PROJECT_DIR/branding/fastfetch/config.jsonc" "$rootfs/etc/skel/.config/fastfetch/config.jsonc"
    fi
}

configure_live_system() {
    local rootfs="$BUILD_DIR/rootfs"

    header "Configuring live system"

    # Set hostname
    echo "yuno-live" > "$rootfs/etc/hostname"

    # Configure hosts
    cat > "$rootfs/etc/hosts" << 'EOF'
127.0.0.1   localhost
::1         localhost
127.0.1.1   yuno-live.localdomain yuno-live
EOF

    # Set timezone to UTC
    ln -sf /usr/share/zoneinfo/UTC "$rootfs/etc/localtime"

    # Configure locale
    echo "en_US.UTF-8 UTF-8" > "$rootfs/etc/locale.gen"
    run_in_chroot "$rootfs" "locale-gen"
    echo "LANG=en_US.UTF-8" > "$rootfs/etc/locale.conf"

    # Enable services based on init system
    log "Enabling services (${INIT_SYSTEM})..."
    if [[ "$INIT_SYSTEM" == "systemd" ]]; then
        run_in_chroot "$rootfs" "systemctl enable dbus"
        run_in_chroot "$rootfs" "systemctl enable NetworkManager"
        run_in_chroot "$rootfs" "systemctl enable sddm" || true
    else
        run_in_chroot "$rootfs" "rc-update add dbus default"
        run_in_chroot "$rootfs" "rc-update add NetworkManager default"
        run_in_chroot "$rootfs" "rc-update add metalog default"
        run_in_chroot "$rootfs" "rc-update add sddm default" || true
    fi

    # Create live user
    log "Creating live user..."
    run_in_chroot "$rootfs" "useradd -m -G wheel,audio,video,input,plugdev,usb -s /bin/bash live" || true
    echo "live:live" | chroot "$rootfs" chpasswd

    # Configure sudo for live user
    mkdir -p "$rootfs/etc/sudoers.d"
    echo "live ALL=(ALL:ALL) NOPASSWD: ALL" > "$rootfs/etc/sudoers.d/live"
    chmod 440 "$rootfs/etc/sudoers.d/live"

    # Allow live user to run installer without password
    mkdir -p "$rootfs/etc/polkit-1/rules.d"
    cat > "$rootfs/etc/polkit-1/rules.d/49-yuno-installer.rules" << 'EOF'
polkit.addRule(function(action, subject) {
    if ((action.id == "org.freedesktop.calamares" ||
         action.id.indexOf("org.freedesktop.policykit.exec") === 0) &&
        subject.user == "live") {
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

    # Configure SDDM theme
    cat > "$rootfs/etc/sddm.conf.d/theme.conf" << 'EOF'
[Theme]
Current=breeze
EOF

    # Create welcome message
    cat > "$rootfs/etc/motd" << 'EOF'

  💕 Welcome to Yuno OS Live! 💕

  To install Yuno OS to your system, run:
    sudo yuno-tui

  Or click "Install Yuno OS" on the desktop.

  💡 Helpful tools:
    yuno-use    - Fix USE flag errors automatically!
                  Example: emerge foo 2>&1 | sudo yuno-use

  Enjoy! - Yuno 🔪

EOF

    # Add fastfetch to bashrc
    cat >> "$rootfs/etc/skel/.bashrc" << 'EOF'

# Show system info on login
if command -v fastfetch &> /dev/null; then
    fastfetch
fi
EOF
}

create_squashfs() {
    local rootfs="$BUILD_DIR/rootfs"
    local iso_dir="$BUILD_DIR/iso"

    header "Creating squashfs image"

    mkdir -p "$iso_dir/LiveOS"

    # Clean up before squashing
    run_in_chroot "$rootfs" "emerge --depclean" || true
    rm -rf "$rootfs/var/cache/distfiles/"*
    rm -rf "$rootfs/var/cache/binpkgs/"*
    rm -rf "$rootfs/var/tmp/"*
    rm -rf "$rootfs/tmp/"*

    if [[ "$IS_GENTOO" == "true" ]]; then
        mksquashfs "$rootfs" "$iso_dir/LiveOS/squashfs.img" \
            -comp zstd \
            -Xcompression-level 19 \
            -b 1M \
            -no-duplicates \
            -progress
    else
        # Use build environment's mksquashfs
        run_in_chroot "$BUILD_ENV_DIR" "mksquashfs /mnt/rootfs /mnt/iso/LiveOS/squashfs.img \
            -comp zstd \
            -Xcompression-level 19 \
            -b 1M \
            -no-duplicates \
            -progress"
    fi
}

setup_bootloader() {
    local rootfs="$BUILD_DIR/rootfs"
    local iso_dir="$BUILD_DIR/iso"

    header "Setting up bootloader"

    mkdir -p "$iso_dir/boot/grub/i386-pc"
    mkdir -p "$iso_dir/boot/grub/x86_64-efi"
    mkdir -p "$iso_dir/EFI/BOOT"

    # Copy kernel and initramfs
    log "Copying kernel and initramfs..."
    cp "$rootfs/boot/vmlinuz-"* "$iso_dir/boot/vmlinuz" || cp "$rootfs/boot/kernel-"* "$iso_dir/boot/vmlinuz"
    cp "$rootfs/boot/initramfs-"*.img "$iso_dir/boot/initramfs.img" || \
        cp "$rootfs/boot/initrd-"* "$iso_dir/boot/initramfs.img" || \
        run_in_chroot "$rootfs" "dracut --force /boot/initramfs.img"

    [[ -f "$iso_dir/boot/initramfs.img" ]] || cp "$rootfs/boot/initramfs-"* "$iso_dir/boot/initramfs.img"

    # Create GRUB configuration
    log "Creating GRUB configuration..."
    cat > "$iso_dir/boot/grub/grub.cfg" << 'EOF'
set timeout=10
set default=0

# Load video modules
insmod all_video
insmod gfxterm
set gfxmode=auto
terminal_output gfxterm

# Menu colors
set menu_color_normal=white/black
set menu_color_highlight=black/light-magenta

menuentry "Yuno OS Live" --class linux {
    linux /boot/vmlinuz root=live:CDLABEL=YUNO_OS rd.live.image rd.live.overlay.overlayfs=1 quiet splash
    initrd /boot/initramfs.img
}

menuentry "Yuno OS Live (Safe Graphics)" --class linux {
    linux /boot/vmlinuz root=live:CDLABEL=YUNO_OS rd.live.image rd.live.overlay.overlayfs=1 nomodeset
    initrd /boot/initramfs.img
}

menuentry "Yuno OS Live (Copy to RAM)" --class linux {
    linux /boot/vmlinuz root=live:CDLABEL=YUNO_OS rd.live.image rd.live.ram=1 quiet splash
    initrd /boot/initramfs.img
}

menuentry "Boot from local disk" --class disk {
    exit
}
EOF

    # Create BIOS boot image
    log "Creating BIOS boot image..."
    if [[ "$IS_GENTOO" == "true" ]]; then
        grub-mkimage \
            -O i386-pc \
            -o "$iso_dir/boot/grub/i386-pc/eltorito.img" \
            -p /boot/grub \
            biosdisk iso9660 part_msdos part_gpt linux normal configfile search

        # Copy GRUB modules
        cp -r /usr/lib/grub/i386-pc/*.mod "$iso_dir/boot/grub/i386-pc/" 2>/dev/null || true

        # Create EFI boot image
        grub-mkimage \
            -O x86_64-efi \
            -o "$iso_dir/EFI/BOOT/BOOTX64.EFI" \
            -p /boot/grub \
            fat iso9660 part_msdos part_gpt linux normal configfile search efi_gop efi_uga
    else
        # Use build environment
        run_in_chroot "$BUILD_ENV_DIR" "grub-mkimage \
            -O i386-pc \
            -o /mnt/iso/boot/grub/i386-pc/eltorito.img \
            -p /boot/grub \
            biosdisk iso9660 part_msdos part_gpt linux normal configfile search"

        run_in_chroot "$BUILD_ENV_DIR" "cp -r /usr/lib/grub/i386-pc/*.mod /mnt/iso/boot/grub/i386-pc/" 2>/dev/null || true

        run_in_chroot "$BUILD_ENV_DIR" "grub-mkimage \
            -O x86_64-efi \
            -o /mnt/iso/EFI/BOOT/BOOTX64.EFI \
            -p /boot/grub \
            fat iso9660 part_msdos part_gpt linux normal configfile search efi_gop efi_uga"
    fi

    # Create EFI image
    log "Creating EFI boot image..."
    dd if=/dev/zero of="$iso_dir/boot/efiboot.img" bs=1M count=16 2>/dev/null
    mkfs.vfat -F 32 "$iso_dir/boot/efiboot.img"

    # Mount and copy EFI files
    local efi_mount="$BUILD_DIR/work/efi_mount"
    mkdir -p "$efi_mount"
    mount -o loop "$iso_dir/boot/efiboot.img" "$efi_mount"
    mkdir -p "$efi_mount/EFI/BOOT"
    cp "$iso_dir/EFI/BOOT/BOOTX64.EFI" "$efi_mount/EFI/BOOT/"
    umount "$efi_mount"
}

create_iso() {
    local iso_dir="$BUILD_DIR/iso"
    local output_iso="$OUTPUT_DIR/${ISO_NAME}-${ISO_VERSION}-${INIT_SYSTEM}.iso"

    header "Creating ISO image"

    # Find the hybrid MBR image
    local hybrid_mbr=""
    if [[ -f "/usr/lib/grub/i386-pc/boot_hybrid.img" ]]; then
        hybrid_mbr="/usr/lib/grub/i386-pc/boot_hybrid.img"
    elif [[ -f "$BUILD_ENV_DIR/usr/lib/grub/i386-pc/boot_hybrid.img" ]]; then
        hybrid_mbr="$BUILD_ENV_DIR/usr/lib/grub/i386-pc/boot_hybrid.img"
    fi

    if [[ "$IS_GENTOO" == "true" ]]; then
        xorriso -as mkisofs \
            -iso-level 3 \
            -full-iso9660-filenames \
            -joliet \
            -joliet-long \
            -rational-rock \
            -volid "$ISO_LABEL" \
            -eltorito-boot boot/grub/i386-pc/eltorito.img \
            -no-emul-boot \
            -boot-load-size 4 \
            -boot-info-table \
            --eltorito-catalog boot/grub/boot.cat \
            ${hybrid_mbr:+--grub2-mbr "$hybrid_mbr"} \
            -eltorito-alt-boot \
            -e boot/efiboot.img \
            -no-emul-boot \
            -isohybrid-gpt-basdat \
            -output "$output_iso" \
            "$iso_dir"
    else
        # Bind mount ISO directory into build env
        mkdir -p "$BUILD_ENV_DIR/mnt/iso"
        mount --bind "$iso_dir" "$BUILD_ENV_DIR/mnt/iso"

        run_in_chroot "$BUILD_ENV_DIR" "xorriso -as mkisofs \
            -iso-level 3 \
            -full-iso9660-filenames \
            -joliet \
            -joliet-long \
            -rational-rock \
            -volid '$ISO_LABEL' \
            -eltorito-boot boot/grub/i386-pc/eltorito.img \
            -no-emul-boot \
            -boot-load-size 4 \
            -boot-info-table \
            --eltorito-catalog boot/grub/boot.cat \
            -eltorito-alt-boot \
            -e boot/efiboot.img \
            -no-emul-boot \
            -isohybrid-gpt-basdat \
            -output /tmp/yuno-os.iso \
            /mnt/iso"

        cp "$BUILD_ENV_DIR/tmp/yuno-os.iso" "$output_iso"
        umount "$BUILD_ENV_DIR/mnt/iso"
    fi

    log "ISO created: $output_iso"

    # Calculate checksums
    cd "$OUTPUT_DIR"
    sha256sum "$(basename "$output_iso")" > "$(basename "$output_iso").sha256"

    log "Build complete!"
    echo ""
    log "ISO: $output_iso"
    log "SHA256: $output_iso.sha256"
    log "Size: $(du -h "$output_iso" | cut -f1)"
}

cleanup() {
    log "Cleaning up..."
    teardown_chroot "$BUILD_DIR/rootfs"
    teardown_chroot "$BUILD_ENV_DIR"

    # Unmount any bind mounts
    umount "$BUILD_ENV_DIR/mnt/iso" 2>/dev/null || true
    umount "$BUILD_ENV_DIR/mnt/rootfs" 2>/dev/null || true
}

show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Build a Yuno OS installation ISO"
    echo ""
    echo "Options:"
    echo "  --init-system SYS   Init system: openrc (default) or systemd"
    echo "  --native            Use -march=native -mtune=native (CPU-specific optimizations)"
    echo "  --o3                Use -O3 optimization (default: -O2)"
    echo "  --lto               Enable LTO (Link Time Optimization) via GentooLTO overlay"
    echo "  --testing           Use ~amd64 testing branch (default: stable amd64)"
    echo "  --stage1            Rebuild toolchain from scratch (VERY slow, maximum optimization)"
    echo "  --no-pipe           Disable -pipe flag (use if low on RAM)"
    echo "  --clean             Clean build directories before starting"
    echo "  --no-cache          Don't use cached stage3 tarballs"
    echo "  --help              Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Build with defaults (OpenRC, -O2, stable)"
    echo "  $0 --init-system systemd              # Build with systemd"
    echo "  $0 --native --o3                      # Native CPU opts with -O3"
    echo "  $0 --native --o3 --lto                # Full optimization with LTO"
    echo "  $0 --testing                          # Use ~amd64 testing packages"
    echo "  $0 --native --o3 --lto --stage1       # Maximum optimization (takes hours!)"
    echo ""
    echo "Environment Variables:"
    echo "  BUILD_DIR       Build directory (default: /var/tmp/yuno-build)"
    echo "  CACHE_DIR       Cache directory (default: /var/cache/yuno)"
    echo "  OUTPUT_DIR      Output directory (default: ./output)"
    echo "  GENTOO_MIRROR   Gentoo mirror URL"
    echo "  INIT_SYSTEM     Init system: openrc or systemd (default: openrc)"
    echo "  USE_NATIVE      Use native CPU flags: true or false (default: false)"
    echo "  OPT_LEVEL       Optimization level: 2 or 3 (default: 2)"
    echo "  ENABLE_LTO      Enable LTO: true or false (default: false)"
    echo "  USE_PIPE        Use -pipe flag: true or false (default: true)"
    echo "  USE_TESTING     Use ~amd64 testing: true or false (default: false)"
    echo "  STAGE1_BUILD    Rebuild toolchain: true or false (default: false)"
    echo ""
    echo "This script can be run from any Linux distribution."
    echo "If not running on Gentoo, a build environment will be bootstrapped automatically."
}

main() {
    # Parse arguments
    local clean_build=false
    local no_cache=false

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --init-system)
                INIT_SYSTEM="$2"
                if [[ "$INIT_SYSTEM" != "openrc" && "$INIT_SYSTEM" != "systemd" ]]; then
                    error "Invalid init system: $INIT_SYSTEM (must be 'openrc' or 'systemd')"
                fi
                shift 2
                ;;
            --native)
                USE_NATIVE=true
                shift
                ;;
            --o3)
                OPT_LEVEL=3
                shift
                ;;
            --lto)
                ENABLE_LTO=true
                shift
                ;;
            --testing)
                USE_TESTING=true
                shift
                ;;
            --stage1)
                STAGE1_BUILD=true
                shift
                ;;
            --no-pipe)
                USE_PIPE=false
                shift
                ;;
            --clean)
                clean_build=true
                shift
                ;;
            --no-cache)
                no_cache=true
                shift
                ;;
            --help)
                show_usage
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                ;;
        esac
    done

    header "Yuno OS ISO Build Script"

    log "Init system: ${INIT_SYSTEM}"
    log "Compiler flags: march=$([ "$USE_NATIVE" == "true" ] && echo "native" || echo "x86-64") -O${OPT_LEVEL} $([ "$USE_PIPE" == "true" ] && echo "-pipe" || echo "")"
    log "Package branch: $([ "$USE_TESTING" == "true" ] && echo "~amd64 (testing)" || echo "amd64 (stable)")"
    [[ "$ENABLE_LTO" == "true" ]] && log "LTO: enabled"
    [[ "$STAGE1_BUILD" == "true" ]] && warn "Stage1 build: enabled (this will take a LONG time!)"
    if [[ "$IS_GENTOO" == "true" ]]; then
        log "Running on Gentoo Linux"
    else
        log "Running on non-Gentoo host - will bootstrap build environment"
    fi

    check_root
    check_host_dependencies

    trap cleanup EXIT

    if [[ "$clean_build" == "true" ]]; then
        log "Cleaning build directories..."
        rm -rf "$BUILD_DIR"
    fi

    if [[ "$no_cache" == "true" ]]; then
        log "Clearing stage3 cache..."
        rm -rf "$CACHE_DIR"/*.tar.*
    fi

    setup_directories
    setup_gentoo_buildenv

    # For non-Gentoo hosts, bind mount directories into build env
    if [[ "$IS_GENTOO" != "true" ]]; then
        mkdir -p "$BUILD_ENV_DIR/mnt/rootfs"
        mount --bind "$BUILD_DIR/rootfs" "$BUILD_ENV_DIR/mnt/rootfs"
    fi

    local stage3_path
    stage3_path=$(fetch_stage3 "$BUILD_DIR/rootfs")

    extract_stage3 "$stage3_path" "$BUILD_DIR/rootfs"
    setup_buildenv_chroot "$BUILD_DIR/rootfs"

    install_packages
    install_yuno_installer
    configure_live_system

    teardown_chroot "$BUILD_DIR/rootfs"

    create_squashfs
    setup_bootloader
    create_iso
}

main "$@"
