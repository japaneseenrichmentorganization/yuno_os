#!/usr/bin/env python3
# -*- coding: utf-8 -*-
#
# Yuno OS Stage3 Installation Module for Calamares
#

import os
import subprocess
import hashlib
import urllib.request
import libcalamares

# Stage3 mirrors and configuration
GENTOO_MIRROR = "https://distfiles.gentoo.org"
STAGE3_PATH = "/releases/amd64/autobuilds"

def get_stage3_variant():
    """Determine stage3 variant based on user selections."""
    gs = libcalamares.globalstorage

    init_system = gs.value("initSystem") or "openrc"
    desktop = gs.value("desktopType") or "desktop"

    if init_system == "systemd":
        return "stage3-amd64-desktop-systemd"
    else:
        return "stage3-amd64-desktop-openrc"

def fetch_latest_stage3_url():
    """Fetch the URL for the latest stage3 tarball."""
    variant = get_stage3_variant()
    latest_url = f"{GENTOO_MIRROR}{STAGE3_PATH}/latest-stage3-amd64.txt"

    try:
        with urllib.request.urlopen(latest_url, timeout=30) as response:
            content = response.read().decode('utf-8')

        for line in content.split('\n'):
            if line.startswith('#') or not line.strip():
                continue
            if variant in line:
                parts = line.split()
                if parts:
                    filename = parts[0]
                    return f"{GENTOO_MIRROR}{STAGE3_PATH}/{filename}"

    except Exception as e:
        libcalamares.utils.warning(f"Failed to fetch stage3 URL: {e}")

    # Fallback to a default
    return f"{GENTOO_MIRROR}{STAGE3_PATH}/current-stage3-amd64-desktop-openrc/stage3-amd64-desktop-openrc-*.tar.xz"

def download_stage3(url, dest_path, progress_cb):
    """Download stage3 tarball with progress reporting."""
    libcalamares.utils.debug(f"Downloading stage3 from {url}")

    try:
        # Use wget for better progress handling
        subprocess.run([
            "wget", "-q", "--show-progress",
            "-O", dest_path, url
        ], check=True)
        return True
    except subprocess.CalledProcessError as e:
        libcalamares.utils.warning(f"Download failed: {e}")
        return False

def verify_checksum(tarball_path, checksum_url):
    """Verify the SHA256 checksum of the tarball."""
    try:
        with urllib.request.urlopen(checksum_url, timeout=30) as response:
            content = response.read().decode('utf-8')

        filename = os.path.basename(tarball_path)
        expected_hash = None

        for line in content.split('\n'):
            if filename in line:
                parts = line.split()
                if parts:
                    expected_hash = parts[0]
                    break

        if not expected_hash:
            libcalamares.utils.warning("Could not find checksum")
            return True  # Skip verification

        # Calculate actual checksum
        sha256_hash = hashlib.sha256()
        with open(tarball_path, "rb") as f:
            for chunk in iter(lambda: f.read(8192), b""):
                sha256_hash.update(chunk)

        actual_hash = sha256_hash.hexdigest()

        if actual_hash.lower() != expected_hash.lower():
            libcalamares.utils.warning(f"Checksum mismatch!")
            return False

        return True

    except Exception as e:
        libcalamares.utils.warning(f"Checksum verification failed: {e}")
        return True  # Continue anyway

def extract_stage3(tarball_path, root_mount_point):
    """Extract stage3 tarball to the target root."""
    libcalamares.utils.debug(f"Extracting stage3 to {root_mount_point}")

    try:
        subprocess.run([
            "tar", "xpf", tarball_path,
            "--xattrs-include=*.*",
            "--numeric-owner",
            "-C", root_mount_point
        ], check=True)
        return True
    except subprocess.CalledProcessError as e:
        libcalamares.utils.warning(f"Extraction failed: {e}")
        return False

def run():
    """Main entry point for the module."""
    root_mount_point = libcalamares.globalstorage.value("rootMountPoint")

    if not root_mount_point:
        return ("No root mount point", "Root mount point not found in global storage")

    # Create cache directory
    cache_dir = "/var/cache/yuno"
    os.makedirs(cache_dir, exist_ok=True)

    # Get stage3 URL
    libcalamares.job.setprogress(0.1)
    stage3_url = fetch_latest_stage3_url()

    if not stage3_url:
        return ("Stage3 URL Error", "Could not determine stage3 URL")

    # Download stage3
    libcalamares.job.setprogress(0.2)
    tarball_filename = os.path.basename(stage3_url.split('?')[0])
    tarball_path = os.path.join(cache_dir, tarball_filename)

    if not os.path.exists(tarball_path):
        if not download_stage3(stage3_url, tarball_path, None):
            return ("Download Error", "Failed to download stage3 tarball")

    # Verify checksum
    libcalamares.job.setprogress(0.5)
    checksum_url = stage3_url + ".sha256"
    if not verify_checksum(tarball_path, checksum_url):
        return ("Checksum Error", "Stage3 checksum verification failed")

    # Extract stage3
    libcalamares.job.setprogress(0.6)
    if not extract_stage3(tarball_path, root_mount_point):
        return ("Extraction Error", "Failed to extract stage3 tarball")

    libcalamares.job.setprogress(1.0)
    return None
