// Package stage3 handles Gentoo stage3 tarball operations.
package stage3

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/utils"
)

const (
	// DefaultMirror is the primary Gentoo mirror.
	DefaultMirror = "https://distfiles.gentoo.org"

	// Stage3Path is the path to stage3 tarballs on mirrors.
	Stage3Path = "/releases/amd64/autobuilds"
)

// Manager handles stage3 operations.
type Manager struct {
	config    *config.InstallConfig
	mirror    string
	cacheDir  string
	targetDir string
}

// NewManager creates a new stage3 manager.
func NewManager(cfg *config.InstallConfig, targetDir string) *Manager {
	mirror := DefaultMirror
	if len(cfg.Portage.Mirrors) > 0 {
		mirror = cfg.Portage.Mirrors[0]
	}

	return &Manager{
		config:    cfg,
		mirror:    mirror,
		cacheDir:  "/var/cache/yuno",
		targetDir: targetDir,
	}
}

// Stage3Info contains information about a stage3 tarball.
type Stage3Info struct {
	Filename   string
	URL        string
	Size       int64
	Date       time.Time
	Variant    string
	InitSystem string
}

// Stage3Variant represents different stage3 variants.
type Stage3Variant string

const (
	VariantDesktop        Stage3Variant = "desktop"
	VariantDesktopSystemd Stage3Variant = "desktop-systemd"
	VariantMinimal        Stage3Variant = "minimal"
	VariantHardened       Stage3Variant = "hardened"
	VariantNoMultilib     Stage3Variant = "nomultilib"
)

// GetStage3Pattern returns the filename pattern for a variant.
func (v Stage3Variant) GetStage3Pattern() string {
	switch v {
	case VariantDesktop:
		return "stage3-amd64-desktop-openrc"
	case VariantDesktopSystemd:
		return "stage3-amd64-desktop-systemd"
	case VariantMinimal:
		return "stage3-amd64-openrc"
	case VariantHardened:
		return "stage3-amd64-hardened-openrc"
	case VariantNoMultilib:
		return "stage3-amd64-nomultilib-openrc"
	default:
		return "stage3-amd64-openrc"
	}
}

// ListMirrors returns a list of Gentoo mirrors.
func (m *Manager) ListMirrors() []string {
	return []string{
		"https://distfiles.gentoo.org",
		"https://gentoo.osuosl.org",
		"https://mirrors.mit.edu/gentoo-distfiles",
		"https://mirror.leaseweb.com/gentoo",
		"https://ftp.fau.de/gentoo",
		"https://ftp.jaist.ac.jp/pub/Linux/Gentoo",
		"https://mirror.bytemark.co.uk/gentoo",
		"https://mirrors.tuna.tsinghua.edu.cn/gentoo",
	}
}

// SetMirror sets the mirror to use.
func (m *Manager) SetMirror(mirror string) {
	m.mirror = mirror
}

// GetLatestStage3 finds the latest stage3 tarball for the given variant.
func (m *Manager) GetLatestStage3(variant Stage3Variant) (*Stage3Info, error) {
	utils.Info("Looking for latest %s stage3", variant)

	// Fetch the latest-stage3 file
	latestURL := fmt.Sprintf("%s%s/latest-stage3-amd64-%s.txt",
		m.mirror, Stage3Path, strings.TrimPrefix(string(variant), "amd64-"))

	// Try different URL patterns
	urls := []string{
		latestURL,
		fmt.Sprintf("%s%s/latest-stage3.txt", m.mirror, Stage3Path),
	}

	var content string
	var err error

	for _, url := range urls {
		content, err = m.fetchURL(url)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, utils.NewError("stage3", "failed to fetch latest stage3 info", err)
	}

	// Parse the file to find the stage3 tarball
	pattern := variant.GetStage3Pattern()
	lines := strings.Split(content, "\n")

	var matches []Stage3Info
	re := regexp.MustCompile(`^(\d+T\d+Z/)?(` + regexp.QuoteMeta(pattern) + `-\d+T\d+Z\.tar\.xz)\s+(\d+)`)

	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}

		match := re.FindStringSubmatch(line)
		if match != nil {
			dateStr := match[1]
			filename := match[2]

			info := Stage3Info{
				Filename: filename,
				URL:      fmt.Sprintf("%s%s/%s%s", m.mirror, Stage3Path, dateStr, filename),
				Variant:  string(variant),
			}

			// Parse size if available
			if len(match) > 3 {
				fmt.Sscanf(match[3], "%d", &info.Size)
			}

			// Parse date from filename
			dateMatch := regexp.MustCompile(`(\d{8}T\d{6}Z)`).FindString(filename)
			if dateMatch != "" {
				info.Date, _ = time.Parse("20060102T150405Z", dateMatch)
			}

			matches = append(matches, info)
		}
	}

	if len(matches) == 0 {
		// Try direct listing
		return m.findStage3Direct(variant)
	}

	// Sort by date, newest first
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Date.After(matches[j].Date)
	})

	return &matches[0], nil
}

// findStage3Direct tries to find stage3 by directly parsing the autobuilds directory.
func (m *Manager) findStage3Direct(variant Stage3Variant) (*Stage3Info, error) {
	// Fetch the current directory listing
	url := fmt.Sprintf("%s%s/current-stage3-amd64-%s/",
		m.mirror, Stage3Path, strings.TrimPrefix(string(variant), "amd64-"))

	content, err := m.fetchURL(url)
	if err != nil {
		return nil, err
	}

	// Look for .tar.xz files
	pattern := variant.GetStage3Pattern()
	re := regexp.MustCompile(`href="(` + regexp.QuoteMeta(pattern) + `-\d+T\d+Z\.tar\.xz)"`)

	matches := re.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, utils.NewError("stage3", "no stage3 tarball found", nil)
	}

	// Get the latest one
	filename := matches[len(matches)-1][1]

	return &Stage3Info{
		Filename: filename,
		URL:      url + filename,
		Variant:  string(variant),
	}, nil
}

// Download downloads a stage3 tarball.
func (m *Manager) Download(info *Stage3Info, progress utils.ProgressCallback) (string, error) {
	utils.Info("Downloading stage3 from %s", info.URL)

	// Create cache directory
	if err := utils.CreateDir(m.cacheDir, 0755); err != nil {
		return "", utils.NewError("stage3", "failed to create cache directory", err)
	}

	destPath := filepath.Join(m.cacheDir, info.Filename)

	// Check if already downloaded
	if utils.FileExists(destPath) {
		utils.Info("Stage3 already cached at %s", destPath)
		return destPath, nil
	}

	// Download the file
	if err := utils.DownloadFile(info.URL, destPath, progress); err != nil {
		return "", err
	}

	return destPath, nil
}

// VerifyChecksum verifies the SHA256 checksum of a stage3 tarball.
func (m *Manager) VerifyChecksum(tarballPath string, info *Stage3Info) error {
	utils.Info("Verifying stage3 checksum")

	// Download the DIGESTS file
	digestsURL := info.URL + ".sha256"
	digestsContent, err := m.fetchURL(digestsURL)
	if err != nil {
		// Try .DIGESTS file
		digestsURL = info.URL[:len(info.URL)-7] + ".DIGESTS"
		digestsContent, err = m.fetchURL(digestsURL)
		if err != nil {
			utils.Warn("Could not fetch checksums, skipping verification")
			return nil
		}
	}

	// Parse expected checksum
	var expectedHash string
	lines := strings.Split(digestsContent, "\n")
	for _, line := range lines {
		if strings.Contains(line, info.Filename) && len(line) >= 64 {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				expectedHash = parts[0]
				break
			}
		}
	}

	if expectedHash == "" {
		utils.Warn("Could not find checksum for %s", info.Filename)
		return nil
	}

	// Calculate actual checksum
	file, err := os.Open(tarballPath)
	if err != nil {
		return utils.NewError("stage3", "failed to open tarball", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return utils.NewError("stage3", "failed to calculate checksum", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))

	if !strings.EqualFold(actualHash, expectedHash) {
		return utils.NewError("stage3", fmt.Sprintf("checksum mismatch: expected %s, got %s", expectedHash, actualHash), nil)
	}

	utils.Info("Checksum verified successfully")
	return nil
}

// VerifyGPG verifies the GPG signature of a stage3 tarball.
func (m *Manager) VerifyGPG(tarballPath string, info *Stage3Info) error {
	utils.Info("Verifying GPG signature")

	// Download the signature
	sigURL := info.URL + ".asc"
	sigPath := tarballPath + ".asc"

	if err := utils.DownloadFile(sigURL, sigPath, nil); err != nil {
		utils.Warn("Could not fetch GPG signature, skipping verification")
		return nil
	}

	// Import Gentoo release keys if not present
	result := utils.RunCommand("gpg", "--keyserver", "hkps://keys.gentoo.org",
		"--recv-keys", "13EBBDBEDE7A12775DFDB1BABB572E0E2D182910")
	if result.Error != nil {
		utils.Warn("Could not import Gentoo release key: %v", result.Error)
	}

	// Verify signature
	result = utils.RunCommand("gpg", "--verify", sigPath, tarballPath)
	if result.Error != nil {
		utils.Warn("GPG verification failed: %v", result.Error)
		// Don't fail on GPG verification errors, just warn
		return nil
	}

	utils.Info("GPG signature verified successfully")
	return nil
}

// Extract extracts a stage3 tarball to the target directory.
func (m *Manager) Extract(tarballPath string, progress utils.ProgressCallback) error {
	utils.Info("Extracting stage3 to %s", m.targetDir)

	// Ensure target directory exists
	if err := utils.CreateDir(m.targetDir, 0755); err != nil {
		return utils.NewError("stage3", "failed to create target directory", err)
	}

	// Extract with proper flags for preserving permissions and xattrs
	return utils.ExtractTarball(tarballPath, m.targetDir, progress)
}

// GetVariantForConfig returns the appropriate stage3 variant based on config.
func (m *Manager) GetVariantForConfig() Stage3Variant {
	if m.config.InitSystem == config.InitSystemd {
		return VariantDesktopSystemd
	}

	// Check if desktop packages are requested
	if m.config.Desktop.Type != config.DesktopNone {
		return VariantDesktop
	}

	return VariantMinimal
}

// Install performs the complete stage3 installation.
func (m *Manager) Install(progress utils.ProgressCallback) error {
	// Determine variant
	variant := m.GetVariantForConfig()

	// Find latest stage3
	info, err := m.GetLatestStage3(variant)
	if err != nil {
		return err
	}

	// Download
	tarballPath, err := m.Download(info, progress)
	if err != nil {
		return err
	}

	// Verify checksum
	if err := m.VerifyChecksum(tarballPath, info); err != nil {
		return err
	}

	// Verify GPG (optional)
	m.VerifyGPG(tarballPath, info)

	// Extract
	if err := m.Extract(tarballPath, progress); err != nil {
		return err
	}

	utils.Info("Stage3 installation complete")
	return nil
}

// Helper function to fetch URL content.
func (m *Manager) fetchURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// CleanCache removes cached stage3 tarballs.
func (m *Manager) CleanCache() error {
	utils.Info("Cleaning stage3 cache")

	entries, err := os.ReadDir(m.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "stage3-") {
			path := filepath.Join(m.cacheDir, entry.Name())
			if err := os.Remove(path); err != nil {
				utils.Warn("Failed to remove %s: %v", path, err)
			}
		}
	}

	return nil
}
