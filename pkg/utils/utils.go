// Package utils provides common utilities for the Yuno OS installer.
package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger handles logging for the installer.
type Logger struct {
	mu       sync.Mutex
	file     *os.File
	verbose  bool
	callback func(level LogLevel, msg string)
}

// LogLevel defines log severity levels.
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

func (l LogLevel) String() string {
	switch l {
	case LogDebug:
		return "DEBUG"
	case LogInfo:
		return "INFO"
	case LogWarn:
		return "WARN"
	case LogError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

var defaultLogger *Logger

// InitLogger initializes the default logger.
func InitLogger(logPath string, verbose bool) error {
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	defaultLogger = &Logger{
		file:    file,
		verbose: verbose,
	}
	return nil
}

// SetLogCallback sets a callback for log messages (useful for TUI).
func SetLogCallback(callback func(level LogLevel, msg string)) {
	if defaultLogger != nil {
		defaultLogger.callback = callback
	}
}

// Log writes a log message.
func Log(level LogLevel, format string, args ...interface{}) {
	if defaultLogger == nil {
		return
	}

	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg)

	if defaultLogger.file != nil {
		defaultLogger.file.WriteString(logLine)
	}

	if defaultLogger.verbose || level >= LogWarn {
		fmt.Print(logLine)
	}

	if defaultLogger.callback != nil {
		defaultLogger.callback(level, msg)
	}
}

// Debug logs a debug message.
func Debug(format string, args ...interface{}) {
	Log(LogDebug, format, args...)
}

// Info logs an info message.
func Info(format string, args ...interface{}) {
	Log(LogInfo, format, args...)
}

// Warn logs a warning message.
func Warn(format string, args ...interface{}) {
	Log(LogWarn, format, args...)
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	Log(LogError, format, args...)
}

// CloseLogger closes the log file.
func CloseLogger() {
	if defaultLogger != nil && defaultLogger.file != nil {
		defaultLogger.file.Close()
	}
}

// CommandResult holds the result of a command execution.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
}

// RunCommand executes a command and returns the result.
func RunCommand(name string, args ...string) *CommandResult {
	Debug("Running command: %s %s", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &CommandResult{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
		Error:  err,
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		Debug("Command failed: %v, stderr: %s", err, result.Stderr)
	}

	return result
}

// RunCommandWithOutput executes a command and streams output to a callback.
func RunCommandWithOutput(callback func(line string), name string, args ...string) error {
	Debug("Running command with output: %s %s", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Read stdout and stderr concurrently
	var wg sync.WaitGroup
	wg.Add(2)

	readPipe := func(pipe io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := scanner.Text()
			if callback != nil {
				callback(line)
			}
		}
	}

	go readPipe(stdout)
	go readPipe(stderr)

	wg.Wait()

	return cmd.Wait()
}

// RunInChroot executes a command inside a chroot environment.
func RunInChroot(chrootPath string, name string, args ...string) *CommandResult {
	chrootArgs := append([]string{chrootPath, name}, args...)
	return RunCommand("chroot", chrootArgs...)
}

// RunInChrootWithEnv executes a command inside a chroot with environment variables.
func RunInChrootWithEnv(chrootPath string, env map[string]string, name string, args ...string) *CommandResult {
	Debug("Running in chroot %s: %s %s", chrootPath, name, strings.Join(args, " "))

	cmd := exec.Command("chroot", append([]string{chrootPath, name}, args...)...)

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &CommandResult{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
		Error:  err,
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	return result
}

// FileExists checks if a file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists checks if a directory exists.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// CreateDir creates a directory with all parents.
func CreateDir(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Get source file permissions
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// WriteFile writes content to a file.
func WriteFile(path string, content string, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := CreateDir(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, []byte(content), perm)
}

// ReadFile reads a file and returns its content.
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// AppendToFile appends content to a file.
func AppendToFile(path string, content string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// IsUEFI checks if the system is booted in UEFI mode.
func IsUEFI() bool {
	return DirExists("/sys/firmware/efi")
}

// GetCPUCount returns the number of CPU cores.
func GetCPUCount() int {
	result := RunCommand("nproc")
	if result.Error != nil {
		return 1
	}
	var count int
	fmt.Sscanf(result.Stdout, "%d", &count)
	if count <= 0 {
		return 1
	}
	return count
}

// GetMemoryMB returns the total memory in MB.
func GetMemoryMB() int {
	result := RunCommand("grep", "MemTotal", "/proc/meminfo")
	if result.Error != nil {
		return 0
	}
	var total int
	fmt.Sscanf(result.Stdout, "MemTotal: %d kB", &total)
	return total / 1024
}

// YunoError represents an installer error with context.
type YunoError struct {
	Op      string // Operation that failed
	Message string // Error message
	Err     error  // Underlying error
}

func (e *YunoError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

func (e *YunoError) Unwrap() error {
	return e.Err
}

// NewError creates a new YunoError.
func NewError(op, message string, err error) *YunoError {
	return &YunoError{
		Op:      op,
		Message: message,
		Err:     err,
	}
}

// ProgressCallback is a function called to report progress.
type ProgressCallback func(current, total int64, message string)

// DownloadFile downloads a file from a URL with progress reporting.
func DownloadFile(url, destPath string, progress ProgressCallback) error {
	// We'll use wget or curl for downloading as they handle redirects better
	args := []string{
		"-q", "--show-progress", "--progress=bar:force",
		"-O", destPath,
		url,
	}

	if progress != nil {
		return RunCommandWithOutput(func(line string) {
			progress(0, 0, line)
		}, "wget", args...)
	}

	result := RunCommand("wget", args...)
	if result.Error != nil {
		return NewError("download", fmt.Sprintf("failed to download %s", url), result.Error)
	}

	return nil
}

// ExtractTarball extracts a tarball to a destination directory.
func ExtractTarball(tarPath, destPath string, progress ProgressCallback) error {
	Info("Extracting %s to %s", tarPath, destPath)

	// Use tar with proper flags for stage3
	args := []string{
		"xpf", tarPath,
		"--xattrs-include=*.*",
		"--numeric-owner",
		"-C", destPath,
	}

	if progress != nil {
		args = append([]string{"xpvf", tarPath, "--xattrs-include=*.*", "--numeric-owner", "-C", destPath}, args[5:]...)
		return RunCommandWithOutput(func(line string) {
			progress(0, 0, line)
		}, "tar", args...)
	}

	result := RunCommand("tar", args...)
	if result.Error != nil {
		return NewError("extract", fmt.Sprintf("failed to extract %s", tarPath), result.Error)
	}

	return nil
}

// MountPoint represents a mount point.
type MountPoint struct {
	Source string
	Target string
	FSType string
	Flags  string
}

// Mount mounts a filesystem.
func Mount(source, target, fstype, flags string) error {
	args := []string{}
	if fstype != "" {
		args = append(args, "-t", fstype)
	}
	if flags != "" {
		args = append(args, "-o", flags)
	}
	args = append(args, source, target)

	result := RunCommand("mount", args...)
	if result.Error != nil {
		return NewError("mount", fmt.Sprintf("failed to mount %s on %s", source, target), result.Error)
	}

	return nil
}

// Unmount unmounts a filesystem.
func Unmount(target string) error {
	result := RunCommand("umount", "-R", target)
	if result.Error != nil {
		return NewError("unmount", fmt.Sprintf("failed to unmount %s", target), result.Error)
	}
	return nil
}

// BindMount creates a bind mount.
func BindMount(source, target string) error {
	return Mount(source, target, "", "bind")
}

// IsMounted checks if a path is mounted.
func IsMounted(path string) bool {
	result := RunCommand("mountpoint", "-q", path)
	return result.ExitCode == 0
}

// SyncFilesystems syncs all filesystems.
func SyncFilesystems() {
	RunCommand("sync")
}

// GeneratePassword generates a hashed password for /etc/shadow.
func GeneratePassword(plaintext string) (string, error) {
	result := RunCommand("openssl", "passwd", "-6", "-stdin")
	if result.Error != nil {
		// Fallback to mkpasswd if available
		result = RunCommand("mkpasswd", "-m", "sha-512", plaintext)
		if result.Error != nil {
			return "", NewError("password", "failed to generate password hash", result.Error)
		}
	}
	return result.Stdout, nil
}
