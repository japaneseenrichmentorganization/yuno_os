// Package encryption handles disk encryption operations.
package encryption

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/utils"
)

// Manager handles encryption operations.
type Manager struct {
	config *config.InstallConfig
}

// NewManager creates a new encryption manager.
func NewManager(cfg *config.InstallConfig) *Manager {
	return &Manager{config: cfg}
}

// LUKSInfo contains information about a LUKS encrypted device.
type LUKSInfo struct {
	Device     string
	Name       string // Mapped device name
	MappedPath string // /dev/mapper/<name>
	Cipher     string
	KeySize    int
	Hash       string
	Version    int // 1 or 2
}

// SetupLUKS creates a LUKS encrypted partition.
func (m *Manager) SetupLUKS(device, name, password string) (*LUKSInfo, error) {
	cfg := m.config.Encryption
	utils.Info("Setting up LUKS encryption on %s", device)

	// Determine LUKS version
	luksType := "luks2"
	if cfg.Type == config.EncryptLUKS {
		luksType = "luks1"
	}

	// Build cryptsetup arguments
	args := []string{
		"luksFormat",
		"--type", luksType,
		"--batch-mode",
	}

	// Add cipher options if specified
	if cfg.Cipher != "" {
		args = append(args, "--cipher", cfg.Cipher)
	} else {
		// Default to AES-XTS for LUKS2
		args = append(args, "--cipher", "aes-xts-plain64")
	}

	if cfg.KeySize > 0 {
		args = append(args, "--key-size", fmt.Sprint(cfg.KeySize))
	} else {
		args = append(args, "--key-size", "512") // 256-bit AES
	}

	if cfg.Hash != "" {
		args = append(args, "--hash", cfg.Hash)
	} else {
		args = append(args, "--hash", "sha256")
	}

	args = append(args, device)

	// Format the device with LUKS
	// We need to provide the password via stdin
	result := runWithStdin(password, "cryptsetup", args...)
	if result.Error != nil {
		return nil, utils.NewError("encryption", "failed to format LUKS device", result.Error)
	}

	// Open the LUKS device
	mappedPath, err := m.OpenLUKS(device, name, password)
	if err != nil {
		return nil, err
	}

	info := &LUKSInfo{
		Device:     device,
		Name:       name,
		MappedPath: mappedPath,
		Cipher:     "aes-xts-plain64",
		KeySize:    512,
		Hash:       "sha256",
		Version:    2,
	}

	if luksType == "luks1" {
		info.Version = 1
	}

	return info, nil
}

// OpenLUKS opens an existing LUKS device.
func (m *Manager) OpenLUKS(device, name, password string) (string, error) {
	utils.Info("Opening LUKS device %s as %s", device, name)

	result := runWithStdin(password, "cryptsetup", "luksOpen", device, name)
	if result.Error != nil {
		return "", utils.NewError("encryption", "failed to open LUKS device", result.Error)
	}

	mappedPath := filepath.Join("/dev/mapper", name)

	// Verify the mapped device exists
	if !utils.FileExists(mappedPath) {
		return "", utils.NewError("encryption", fmt.Sprintf("mapped device %s not found", mappedPath), nil)
	}

	return mappedPath, nil
}

// CloseLUKS closes an open LUKS device.
func (m *Manager) CloseLUKS(name string) error {
	utils.Info("Closing LUKS device %s", name)

	result := utils.RunCommand("cryptsetup", "luksClose", name)
	if result.Error != nil {
		return utils.NewError("encryption", "failed to close LUKS device", result.Error)
	}

	return nil
}

// AddLUKSKey adds an additional key to a LUKS device.
func (m *Manager) AddLUKSKey(device, existingPassword, newPassword string) error {
	utils.Info("Adding new key to LUKS device %s", device)

	// Create a temporary file with the existing password
	tmpfile, err := os.CreateTemp("", "luks-key-")
	if err != nil {
		return utils.NewError("encryption", "failed to create temp file", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(existingPassword); err != nil {
		tmpfile.Close()
		return utils.NewError("encryption", "failed to write temp file", err)
	}
	tmpfile.Close()

	result := runWithStdin(newPassword, "cryptsetup", "luksAddKey", device, "--key-file", tmpfile.Name())
	if result.Error != nil {
		return utils.NewError("encryption", "failed to add LUKS key", result.Error)
	}

	return nil
}

// AddLUKSKeyFile adds a key file to a LUKS device.
func (m *Manager) AddLUKSKeyFile(device, password, keyFilePath string) error {
	utils.Info("Adding key file to LUKS device %s", device)

	result := runWithStdin(password, "cryptsetup", "luksAddKey", device, keyFilePath)
	if result.Error != nil {
		return utils.NewError("encryption", "failed to add LUKS key file", result.Error)
	}

	return nil
}

// GenerateKeyFile generates a random key file.
func (m *Manager) GenerateKeyFile(path string, size int) error {
	utils.Info("Generating key file at %s", path)

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := utils.CreateDir(dir, 0700); err != nil {
		return utils.NewError("encryption", "failed to create key file directory", err)
	}

	// Generate random data
	result := utils.RunCommand("dd", "if=/dev/urandom", fmt.Sprintf("of=%s", path),
		fmt.Sprintf("bs=%d", size), "count=1", "status=none")
	if result.Error != nil {
		return utils.NewError("encryption", "failed to generate key file", result.Error)
	}

	// Set restrictive permissions
	if err := os.Chmod(path, 0400); err != nil {
		return utils.NewError("encryption", "failed to set key file permissions", err)
	}

	return nil
}

// SetupDMCrypt sets up plain dm-crypt encryption (no LUKS header).
func (m *Manager) SetupDMCrypt(device, name, password string) (string, error) {
	utils.Info("Setting up dm-crypt on %s", device)

	// Plain dm-crypt requires calculating the key from the password
	args := []string{
		"open", "--type", "plain",
		"--cipher", "aes-xts-plain64",
		"--key-size", "256",
		"--hash", "sha256",
		device, name,
	}

	result := runWithStdin(password, "cryptsetup", args...)
	if result.Error != nil {
		return "", utils.NewError("encryption", "failed to setup dm-crypt", result.Error)
	}

	return filepath.Join("/dev/mapper", name), nil
}

// ZFSEncryption handles ZFS native encryption.
type ZFSEncryption struct {
	manager *Manager
}

// NewZFSEncryption creates a ZFS encryption handler.
func NewZFSEncryption(m *Manager) *ZFSEncryption {
	return &ZFSEncryption{manager: m}
}

// CreateEncryptedPool creates a ZFS pool with native encryption.
func (z *ZFSEncryption) CreateEncryptedPool(poolName, device, password string) error {
	utils.Info("Creating encrypted ZFS pool %s on %s", poolName, device)

	// Create a temporary file for the key
	keyFile, err := os.CreateTemp("", "zfs-key-")
	if err != nil {
		return utils.NewError("encryption", "failed to create temp key file", err)
	}
	defer os.Remove(keyFile.Name())

	if _, err := keyFile.WriteString(password); err != nil {
		keyFile.Close()
		return utils.NewError("encryption", "failed to write key file", err)
	}
	keyFile.Close()

	// Create the pool with encryption
	args := []string{
		"create",
		"-o", "ashift=12",
		"-O", "encryption=aes-256-gcm",
		"-O", "keyformat=passphrase",
		"-O", fmt.Sprintf("keylocation=file://%s", keyFile.Name()),
		"-O", "acltype=posixacl",
		"-O", "xattr=sa",
		"-O", "dnodesize=auto",
		"-O", "normalization=formD",
		"-O", "relatime=on",
		"-O", "canmount=off",
		"-O", "mountpoint=none",
		poolName,
		device,
	}

	result := utils.RunCommand("zpool", args...)
	if result.Error != nil {
		return utils.NewError("encryption", "failed to create ZFS pool", result.Error)
	}

	// Change key location to prompt (for boot)
	result = utils.RunCommand("zfs", "set", "keylocation=prompt", poolName)
	if result.Error != nil {
		utils.Warn("Failed to change key location: %v", result.Error)
	}

	return nil
}

// CreateEncryptedDataset creates an encrypted ZFS dataset.
func (z *ZFSEncryption) CreateEncryptedDataset(dataset, mountpoint string, inheritEncryption bool) error {
	utils.Info("Creating ZFS dataset %s", dataset)

	args := []string{"create"}

	if !inheritEncryption {
		// This dataset won't inherit encryption from parent
		args = append(args, "-o", "encryption=off")
	}

	if mountpoint != "" {
		args = append(args, "-o", fmt.Sprintf("mountpoint=%s", mountpoint))
	}

	args = append(args, dataset)

	result := utils.RunCommand("zfs", args...)
	if result.Error != nil {
		return utils.NewError("encryption", "failed to create ZFS dataset", result.Error)
	}

	return nil
}

// LoadKey loads the encryption key for a ZFS dataset.
func (z *ZFSEncryption) LoadKey(dataset, password string) error {
	utils.Info("Loading ZFS encryption key for %s", dataset)

	result := runWithStdin(password, "zfs", "load-key", dataset)
	if result.Error != nil {
		return utils.NewError("encryption", "failed to load ZFS key", result.Error)
	}

	return nil
}

// UnloadKey unloads the encryption key for a ZFS dataset.
func (z *ZFSEncryption) UnloadKey(dataset string) error {
	utils.Info("Unloading ZFS encryption key for %s", dataset)

	result := utils.RunCommand("zfs", "unload-key", dataset)
	if result.Error != nil {
		return utils.NewError("encryption", "failed to unload ZFS key", result.Error)
	}

	return nil
}

// GenerateCrypttab generates /etc/crypttab entries for LUKS devices.
func (m *Manager) GenerateCrypttab(devices []LUKSInfo, targetRoot string) error {
	utils.Info("Generating crypttab")

	var entries []string
	entries = append(entries, "# <target name> <source device> <key file> <options>")

	for _, dev := range devices {
		// Get UUID of the device
		result := utils.RunCommand("blkid", "-s", "UUID", "-o", "value", dev.Device)
		uuid := strings.TrimSpace(result.Stdout)

		keyFile := "none"
		if m.config.Encryption.KeyFile != "" {
			keyFile = m.config.Encryption.KeyFile
		}

		options := "luks"
		if dev.Version == 2 {
			options = "luks,discard"
		}

		if uuid != "" {
			entries = append(entries, fmt.Sprintf("%s UUID=%s %s %s", dev.Name, uuid, keyFile, options))
		} else {
			entries = append(entries, fmt.Sprintf("%s %s %s %s", dev.Name, dev.Device, keyFile, options))
		}
	}

	content := strings.Join(entries, "\n") + "\n"
	crypttabPath := filepath.Join(targetRoot, "etc", "crypttab")

	if err := utils.WriteFile(crypttabPath, content, 0644); err != nil {
		return utils.NewError("encryption", "failed to write crypttab", err)
	}

	return nil
}

// UpdateInitramfs updates the initramfs to include encryption support.
func (m *Manager) UpdateInitramfs(targetRoot string) error {
	utils.Info("Updating initramfs for encryption support")

	// Check which initramfs system is in use
	if utils.FileExists(filepath.Join(targetRoot, "usr/bin/dracut")) {
		// Using dracut
		result := utils.RunInChroot(targetRoot, "dracut", "--force", "--hostonly")
		if result.Error != nil {
			return utils.NewError("encryption", "failed to update dracut initramfs", result.Error)
		}
	} else if utils.FileExists(filepath.Join(targetRoot, "usr/bin/genkernel")) {
		// Using genkernel
		result := utils.RunInChroot(targetRoot, "genkernel", "--luks", "initramfs")
		if result.Error != nil {
			return utils.NewError("encryption", "failed to update genkernel initramfs", result.Error)
		}
	}

	return nil
}

// Helper function to run commands with stdin input
func runWithStdin(input string, name string, args ...string) *utils.CommandResult {
	utils.Debug("Running command with stdin: %s %s", name, strings.Join(args, " "))

	cmd := utils.RunCommand("sh", "-c",
		fmt.Sprintf("echo -n '%s' | %s %s",
			strings.ReplaceAll(input, "'", "'\"'\"'"),
			name,
			strings.Join(args, " ")))

	return cmd
}

// IsLUKS checks if a device is a LUKS encrypted device.
func IsLUKS(device string) bool {
	result := utils.RunCommand("cryptsetup", "isLuks", device)
	return result.ExitCode == 0
}

// GetLUKSUUID returns the UUID of a LUKS device.
func GetLUKSUUID(device string) string {
	result := utils.RunCommand("cryptsetup", "luksUUID", device)
	if result.Error != nil {
		return ""
	}
	return strings.TrimSpace(result.Stdout)
}
