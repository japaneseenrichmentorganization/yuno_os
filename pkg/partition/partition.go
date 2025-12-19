// Package partition handles disk partitioning operations.
package partition

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/japaneseenrichmentorganization/yuno_os/pkg/config"
	"github.com/japaneseenrichmentorganization/yuno_os/pkg/utils"
)

// Manager handles partition operations.
type Manager struct {
	config *config.InstallConfig
}

// NewManager creates a new partition manager.
func NewManager(cfg *config.InstallConfig) *Manager {
	return &Manager{config: cfg}
}

// Disk represents a physical disk device.
type Disk struct {
	Name       string       `json:"name"`
	Path       string       `json:"path"`
	Size       int64        `json:"size"`
	SizeHuman  string       `json:"size_human"`
	Model      string       `json:"model"`
	Type       string       `json:"type"`       // disk, part, rom, etc.
	Mountpoint string       `json:"mountpoint"` // If mounted
	FSType     string       `json:"fstype"`
	Children   []Partition  `json:"children"`
	Removable  bool         `json:"rm"`
	ReadOnly   bool         `json:"ro"`
}

// Partition represents a partition on a disk.
type Partition struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	SizeHuman  string `json:"size_human"`
	FSType     string `json:"fstype"`
	Mountpoint string `json:"mountpoint"`
	Label      string `json:"label"`
	UUID       string `json:"uuid"`
	PartUUID   string `json:"partuuid"`
	Type       string `json:"type"`
}

// ListDisks returns all available disk devices.
func (m *Manager) ListDisks() ([]Disk, error) {
	result := utils.RunCommand("lsblk", "-J", "-b", "-o",
		"NAME,PATH,SIZE,MODEL,TYPE,MOUNTPOINT,FSTYPE,RM,RO,LABEL,UUID,PARTUUID")

	if result.Error != nil {
		return nil, utils.NewError("partition", "failed to list disks", result.Error)
	}

	var output struct {
		BlockDevices []struct {
			Name       string `json:"name"`
			Path       string `json:"path"`
			Size       any    `json:"size"` // Can be string or int
			Model      string `json:"model"`
			Type       string `json:"type"`
			Mountpoint string `json:"mountpoint"`
			FSType     string `json:"fstype"`
			RM         any    `json:"rm"` // Can be bool or string
			RO         any    `json:"ro"`
			Label      string `json:"label"`
			UUID       string `json:"uuid"`
			PartUUID   string `json:"partuuid"`
			Children   []struct {
				Name       string `json:"name"`
				Path       string `json:"path"`
				Size       any    `json:"size"`
				FSType     string `json:"fstype"`
				Mountpoint string `json:"mountpoint"`
				Label      string `json:"label"`
				UUID       string `json:"uuid"`
				PartUUID   string `json:"partuuid"`
				Type       string `json:"type"`
			} `json:"children"`
		} `json:"blockdevices"`
	}

	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		return nil, utils.NewError("partition", "failed to parse lsblk output", err)
	}

	var disks []Disk
	for _, dev := range output.BlockDevices {
		if dev.Type != "disk" {
			continue
		}

		disk := Disk{
			Name:       dev.Name,
			Path:       dev.Path,
			Size:       parseSize(dev.Size),
			Model:      strings.TrimSpace(dev.Model),
			Type:       dev.Type,
			Mountpoint: dev.Mountpoint,
			FSType:     dev.FSType,
			Removable:  parseBool(dev.RM),
			ReadOnly:   parseBool(dev.RO),
		}
		disk.SizeHuman = humanSize(disk.Size)

		for _, child := range dev.Children {
			part := Partition{
				Name:       child.Name,
				Path:       child.Path,
				Size:       parseSize(child.Size),
				FSType:     child.FSType,
				Mountpoint: child.Mountpoint,
				Label:      child.Label,
				UUID:       child.UUID,
				PartUUID:   child.PartUUID,
				Type:       child.Type,
			}
			part.SizeHuman = humanSize(part.Size)
			disk.Children = append(disk.Children, part)
		}

		disks = append(disks, disk)
	}

	return disks, nil
}

// GetDisk returns information about a specific disk.
func (m *Manager) GetDisk(device string) (*Disk, error) {
	disks, err := m.ListDisks()
	if err != nil {
		return nil, err
	}

	for _, disk := range disks {
		if disk.Path == device || disk.Name == device {
			return &disk, nil
		}
	}

	return nil, utils.NewError("partition", fmt.Sprintf("disk %s not found", device), nil)
}

// WipeDisk removes all partitions and signatures from a disk.
func (m *Manager) WipeDisk(device string) error {
	utils.Info("Wiping disk %s", device)

	// Unmount any mounted partitions
	disk, err := m.GetDisk(device)
	if err != nil {
		return err
	}

	for _, part := range disk.Children {
		if part.Mountpoint != "" {
			if err := utils.Unmount(part.Mountpoint); err != nil {
				utils.Warn("Failed to unmount %s: %v", part.Mountpoint, err)
			}
		}
	}

	// Wipe signatures
	result := utils.RunCommand("wipefs", "-a", device)
	if result.Error != nil {
		return utils.NewError("partition", "failed to wipe disk signatures", result.Error)
	}

	// Zero out first and last MB (partition tables)
	utils.RunCommand("dd", "if=/dev/zero", fmt.Sprintf("of=%s", device), "bs=1M", "count=1", "status=none")
	utils.RunCommand("dd", "if=/dev/zero", fmt.Sprintf("of=%s", device), "bs=1M", "seek="+fmt.Sprint(disk.Size/1024/1024-1), "count=1", "status=none")

	utils.SyncFilesystems()
	return nil
}

// CreatePartitionTable creates a new partition table on a disk.
func (m *Manager) CreatePartitionTable(device string, scheme config.PartitionScheme) error {
	utils.Info("Creating %s partition table on %s", scheme, device)

	var label string
	switch scheme {
	case config.PartSchemeGPT:
		label = "gpt"
	case config.PartSchemeMBR:
		label = "msdos"
	default:
		return utils.NewError("partition", fmt.Sprintf("unsupported partition scheme: %s", scheme), nil)
	}

	result := utils.RunCommand("parted", "-s", device, "mklabel", label)
	if result.Error != nil {
		return utils.NewError("partition", "failed to create partition table", result.Error)
	}

	return nil
}

// CreatePartition creates a single partition.
func (m *Manager) CreatePartition(device string, partNum int, start, end, fstype string, flags []string) error {
	utils.Info("Creating partition %d on %s (%s - %s)", partNum, device, start, end)

	// Create the partition
	args := []string{"-s", device, "mkpart", "primary"}
	if fstype != "" && fstype != "none" {
		args = append(args, fstype)
	}
	args = append(args, start, end)

	result := utils.RunCommand("parted", args...)
	if result.Error != nil {
		return utils.NewError("partition", fmt.Sprintf("failed to create partition %d", partNum), result.Error)
	}

	// Set flags
	for _, flag := range flags {
		result = utils.RunCommand("parted", "-s", device, "set", fmt.Sprint(partNum), flag, "on")
		if result.Error != nil {
			utils.Warn("Failed to set flag %s on partition %d: %v", flag, partNum, result.Error)
		}
	}

	utils.SyncFilesystems()
	return nil
}

// FormatPartition formats a partition with the specified filesystem.
func (m *Manager) FormatPartition(device string, fs config.Filesystem, label string) error {
	utils.Info("Formatting %s as %s", device, fs)

	var result *utils.CommandResult

	switch fs {
	case config.FSExt4:
		args := []string{"-F"}
		if label != "" {
			args = append(args, "-L", label)
		}
		args = append(args, device)
		result = utils.RunCommand("mkfs.ext4", args...)

	case config.FSBtrfs:
		args := []string{"-f"}
		if label != "" {
			args = append(args, "-L", label)
		}
		args = append(args, device)
		result = utils.RunCommand("mkfs.btrfs", args...)

	case config.FSXfs:
		args := []string{"-f"}
		if label != "" {
			args = append(args, "-L", label)
		}
		args = append(args, device)
		result = utils.RunCommand("mkfs.xfs", args...)

	case config.FSF2fs:
		args := []string{}
		if label != "" {
			args = append(args, "-l", label)
		}
		args = append(args, device)
		result = utils.RunCommand("mkfs.f2fs", args...)

	case config.FSFat32:
		args := []string{"-F", "32"}
		if label != "" {
			args = append(args, "-n", strings.ToUpper(label))
		}
		args = append(args, device)
		result = utils.RunCommand("mkfs.vfat", args...)

	case config.FSSwap:
		args := []string{}
		if label != "" {
			args = append(args, "-L", label)
		}
		args = append(args, device)
		result = utils.RunCommand("mkswap", args...)

	case config.FSZfs:
		// ZFS is handled separately
		return nil

	case config.FSNone:
		// No formatting needed
		return nil

	default:
		return utils.NewError("partition", fmt.Sprintf("unsupported filesystem: %s", fs), nil)
	}

	if result.Error != nil {
		return utils.NewError("partition", fmt.Sprintf("failed to format %s", device), result.Error)
	}

	return nil
}

// PartitionLayout represents a complete partition layout.
type PartitionLayout struct {
	Scheme     config.PartitionScheme
	Partitions []LayoutPartition
}

// LayoutPartition represents a partition in the layout.
type LayoutPartition struct {
	Number     int
	Start      string
	End        string
	Size       string
	Filesystem config.Filesystem
	MountPoint string
	Label      string
	Flags      []string
	Encrypt    bool
}

// CreateAutoLayout creates an automatic partition layout for the disk.
func (m *Manager) CreateAutoLayout(device string, isUEFI bool, useEncryption bool) (*PartitionLayout, error) {
	disk, err := m.GetDisk(device)
	if err != nil {
		return nil, err
	}

	layout := &PartitionLayout{
		Scheme: config.PartSchemeGPT,
	}

	if !isUEFI {
		layout.Scheme = config.PartSchemeMBR
	}

	partNum := 1
	currentPos := "1MiB" // Start after 1MiB for alignment

	if isUEFI {
		// GPT layout with ESP
		layout.Partitions = append(layout.Partitions, LayoutPartition{
			Number:     partNum,
			Start:      currentPos,
			End:        "1025MiB", // 1GB ESP
			Size:       "1024MiB",
			Filesystem: config.FSFat32,
			MountPoint: "/boot",
			Label:      "ESP",
			Flags:      []string{"boot", "esp"},
		})
		partNum++
		currentPos = "1025MiB"
	} else {
		// MBR layout with BIOS boot partition
		layout.Partitions = append(layout.Partitions, LayoutPartition{
			Number:     partNum,
			Start:      currentPos,
			End:        "3MiB",
			Size:       "2MiB",
			Filesystem: config.FSNone,
			Label:      "BIOS",
			Flags:      []string{"bios_grub"},
		})
		partNum++
		currentPos = "3MiB"

		// Separate /boot partition for MBR
		layout.Partitions = append(layout.Partitions, LayoutPartition{
			Number:     partNum,
			Start:      currentPos,
			End:        "515MiB",
			Size:       "512MiB",
			Filesystem: config.FSExt4,
			MountPoint: "/boot",
			Label:      "boot",
			Flags:      []string{"boot"},
		})
		partNum++
		currentPos = "515MiB"
	}

	// Swap partition (size based on RAM, max 8GB)
	memMB := utils.GetMemoryMB()
	swapMB := memMB
	if swapMB > 8192 {
		swapMB = 8192
	}
	if swapMB < 1024 {
		swapMB = 1024
	}

	swapEnd := fmt.Sprintf("%dMiB", parseStartMiB(currentPos)+swapMB)
	layout.Partitions = append(layout.Partitions, LayoutPartition{
		Number:     partNum,
		Start:      currentPos,
		End:        swapEnd,
		Size:       fmt.Sprintf("%dMiB", swapMB),
		Filesystem: config.FSSwap,
		Label:      "swap",
	})
	partNum++
	currentPos = swapEnd

	// Root partition (rest of disk)
	layout.Partitions = append(layout.Partitions, LayoutPartition{
		Number:     partNum,
		Start:      currentPos,
		End:        "100%",
		Size:       fmt.Sprintf("%dMiB", disk.Size/1024/1024-parseStartMiB(currentPos)),
		Filesystem: config.FSExt4,
		MountPoint: "/",
		Label:      "root",
		Encrypt:    useEncryption,
	})

	return layout, nil
}

// ApplyLayout applies a partition layout to a disk.
func (m *Manager) ApplyLayout(device string, layout *PartitionLayout) error {
	utils.Info("Applying partition layout to %s", device)

	// Wipe the disk first
	if err := m.WipeDisk(device); err != nil {
		return err
	}

	// Create partition table
	if err := m.CreatePartitionTable(device, layout.Scheme); err != nil {
		return err
	}

	// Create partitions
	for _, part := range layout.Partitions {
		fstype := ""
		if part.Filesystem == config.FSFat32 {
			fstype = "fat32"
		} else if part.Filesystem == config.FSSwap {
			fstype = "linux-swap"
		}

		if err := m.CreatePartition(device, part.Number, part.Start, part.End, fstype, part.Flags); err != nil {
			return err
		}
	}

	// Wait for device nodes to appear
	utils.RunCommand("partprobe", device)
	utils.RunCommand("udevadm", "settle")

	// Format partitions
	for _, part := range layout.Partitions {
		partDevice := getPartitionDevice(device, part.Number)

		// Skip encrypted partitions for now (handled by encryption manager)
		if part.Encrypt {
			continue
		}

		if err := m.FormatPartition(partDevice, part.Filesystem, part.Label); err != nil {
			return err
		}
	}

	return nil
}

// MountPartitions mounts all partitions according to their mount points.
func (m *Manager) MountPartitions(device string, layout *PartitionLayout, targetRoot string) error {
	utils.Info("Mounting partitions to %s", targetRoot)

	// Sort partitions by mount point depth (/ first, then /boot, etc.)
	type mountInfo struct {
		device     string
		mountPoint string
		fstype     string
	}

	var mounts []mountInfo
	for _, part := range layout.Partitions {
		if part.MountPoint == "" {
			continue
		}

		partDevice := getPartitionDevice(device, part.Number)
		mounts = append(mounts, mountInfo{
			device:     partDevice,
			mountPoint: part.MountPoint,
			fstype:     string(part.Filesystem),
		})
	}

	// Sort by mount point length (shorter = mount first)
	for i := 0; i < len(mounts)-1; i++ {
		for j := i + 1; j < len(mounts); j++ {
			if len(mounts[j].mountPoint) < len(mounts[i].mountPoint) {
				mounts[i], mounts[j] = mounts[j], mounts[i]
			}
		}
	}

	// Mount each partition
	for _, mount := range mounts {
		target := targetRoot + mount.mountPoint
		if err := utils.CreateDir(target, 0755); err != nil {
			return utils.NewError("partition", fmt.Sprintf("failed to create mount point %s", target), err)
		}

		fstype := mount.fstype
		if fstype == "fat32" {
			fstype = "vfat"
		}

		if err := utils.Mount(mount.device, target, fstype, ""); err != nil {
			return err
		}
	}

	// Enable swap
	for _, part := range layout.Partitions {
		if part.Filesystem == config.FSSwap {
			partDevice := getPartitionDevice(device, part.Number)
			utils.RunCommand("swapon", partDevice)
		}
	}

	return nil
}

// UnmountPartitions unmounts all partitions.
func (m *Manager) UnmountPartitions(targetRoot string) error {
	utils.Info("Unmounting partitions from %s", targetRoot)

	// Disable swap first
	utils.RunCommand("swapoff", "-a")

	// Unmount recursively
	return utils.Unmount(targetRoot)
}

// Helper functions

func parseSize(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	case string:
		var size int64
		fmt.Sscanf(val, "%d", &size)
		return size
	default:
		return 0
	}
}

func parseBool(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "1" || val == "true"
	case int:
		return val == 1
	case float64:
		return val == 1
	default:
		return false
	}
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func parseStartMiB(s string) int {
	// Parse "1234MiB" to integer
	re := regexp.MustCompile(`(\d+)`)
	match := re.FindString(s)
	if match == "" {
		return 0
	}
	val, _ := strconv.Atoi(match)
	return val
}

func getPartitionDevice(disk string, partNum int) string {
	// Handle NVMe and regular disks
	if strings.Contains(disk, "nvme") || strings.Contains(disk, "mmcblk") {
		return fmt.Sprintf("%sp%d", disk, partNum)
	}
	return fmt.Sprintf("%s%d", disk, partNum)
}
