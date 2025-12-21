// yuno-use - Automatically fix Portage USE flag requirements 💕
//
// Yuno will parse emerge output and create package.use files for you~
// Because manually editing USE flags is tedious, and Yuno wants to help! 🔪
//
// Usage:
//
//	emerge foo 2>&1 | yuno-use
//	yuno-use < emerge-output.txt
//	yuno-use --dry-run < emerge-output.txt
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ANSI colors 💕
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorPink   = "\033[0;35m"
	colorCyan   = "\033[0;36m"
)

// Config holds the program configuration
type Config struct {
	DryRun        bool
	Verbose       bool
	PackageUseDir string
	KeywordsDir   string
}

// UseRequirement represents a parsed USE flag requirement
type UseRequirement struct {
	Atom  string
	Flags []string
}

// KeywordRequirement represents a parsed keyword requirement
type KeywordRequirement struct {
	Atom    string
	Keyword string
}

var config Config

func main() {
	// Parse flags
	flag.BoolVar(&config.DryRun, "n", false, "Dry-run mode (show what would be done)")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Dry-run mode (show what would be done)")
	flag.BoolVar(&config.Verbose, "v", false, "Verbose output")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose output")
	flag.StringVar(&config.PackageUseDir, "d", "/etc/portage/package.use", "Package.use directory")
	flag.StringVar(&config.PackageUseDir, "dir", "/etc/portage/package.use", "Package.use directory")
	flag.StringVar(&config.KeywordsDir, "k", "/etc/portage/package.accept_keywords", "Package.accept_keywords directory")

	flag.Usage = usage
	flag.Parse()

	config.KeywordsDir = "/etc/portage/package.accept_keywords"

	// Check if running as root (unless dry-run)
	if !config.DryRun && os.Geteuid() != 0 {
		errorMsg("Yuno needs root access to write to /etc/portage! 🔪")
		errorMsg("Try: emerge ... 2>&1 | sudo yuno-use")
		os.Exit(1)
	}

	// Check if stdin has data
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		errorMsg("No input provided! Pipe emerge output to yuno-use 💕")
		fmt.Println()
		usage()
		os.Exit(1)
	}

	fmt.Printf("%s💕 Yuno is analyzing emerge output... 💕%s\n\n", colorPink, colorReset)

	if config.DryRun {
		warnMsg("Dry-run mode - no changes will be made")
		fmt.Println()
	}

	// Ensure directories exist
	if err := ensurePackageUseDir(); err != nil {
		errorMsg("Failed to setup package.use directory: " + err.Error())
		os.Exit(1)
	}

	// Read and parse input
	scanner := bufio.NewScanner(os.Stdin)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		errorMsg("Error reading input: " + err.Error())
		os.Exit(1)
	}

	input := strings.Join(lines, "\n")

	// Parse USE requirements
	useReqs := parseUseRequirements(input)
	for _, req := range useReqs {
		processUseRequirement(req)
	}

	// Parse keyword requirements
	keywordReqs := parseKeywordRequirements(input)
	for _, req := range keywordReqs {
		processKeywordRequirement(req)
	}

	fmt.Println()
	if config.DryRun {
		fmt.Printf("%sDry-run complete! Use without --dry-run to apply changes~ 💕%s\n", colorPink, colorReset)
	} else {
		fmt.Printf("%sYuno fixed everything for you~ 💕🔪%s\n", colorPink, colorReset)
		fmt.Printf("%sNow try your emerge command again!%s\n", colorCyan, colorReset)
	}
}

func usage() {
	fmt.Printf("%s💕 yuno-use - Portage USE flag fixer 💕%s\n", colorPink, colorReset)
	fmt.Println()
	fmt.Println("Yuno will automatically create package.use files from emerge output~")
	fmt.Println()
	fmt.Printf("%sUsage:%s\n", colorCyan, colorReset)
	fmt.Println("  emerge <package> 2>&1 | yuno-use [OPTIONS]")
	fmt.Println("  yuno-use [OPTIONS] < emerge-output.txt")
	fmt.Println()
	fmt.Printf("%sOptions:%s\n", colorCyan, colorReset)
	fmt.Println("  -n, --dry-run     Show what would be done without making changes")
	fmt.Println("  -v, --verbose     Show more details")
	fmt.Println("  -d, --dir DIR     Use custom package.use directory")
	fmt.Println("  -h, --help        Show this help message")
	fmt.Println()
	fmt.Printf("%sExamples:%s\n", colorCyan, colorReset)
	fmt.Println("  # Fix USE flags while emerging")
	fmt.Println("  emerge --ask dev-libs/foo 2>&1 | yuno-use")
	fmt.Println()
	fmt.Println("  # Preview changes first")
	fmt.Println("  emerge -pv @world 2>&1 | yuno-use --dry-run")
	fmt.Println()
	fmt.Println("  # Save emerge output and process later")
	fmt.Println("  emerge -pv foo > output.txt 2>&1")
	fmt.Println("  yuno-use < output.txt")
	fmt.Println()
	fmt.Printf("%sYuno will take care of everything~ 💕🔪%s\n", colorPink, colorReset)
}

func logMsg(msg string) {
	fmt.Printf("%s[yuno]%s %s\n", colorGreen, colorReset, msg)
}

func warnMsg(msg string) {
	fmt.Printf("%s[yuno]%s %s\n", colorYellow, colorReset, msg)
}

func errorMsg(msg string) {
	fmt.Fprintf(os.Stderr, "%s[yuno]%s %s\n", colorRed, colorReset, msg)
}

func debugMsg(msg string) {
	if config.Verbose {
		fmt.Fprintf(os.Stderr, "%s[debug]%s %s\n", colorCyan, colorReset, msg)
	}
}

func ensurePackageUseDir() error {
	info, err := os.Stat(config.PackageUseDir)

	if err == nil && !info.IsDir() {
		// It's a file, need to convert to directory
		if config.DryRun {
			warnMsg("Would convert " + config.PackageUseDir + " from file to directory")
			return nil
		}

		warnMsg("Converting " + config.PackageUseDir + " from file to directory...")

		// Read existing content
		content, err := os.ReadFile(config.PackageUseDir)
		if err != nil {
			return err
		}

		// Remove file and create directory
		if err := os.Remove(config.PackageUseDir); err != nil {
			return err
		}
		if err := os.MkdirAll(config.PackageUseDir, 0755); err != nil {
			return err
		}

		// Write old content to legacy file
		legacyFile := filepath.Join(config.PackageUseDir, "legacy")
		if err := os.WriteFile(legacyFile, content, 0644); err != nil {
			return err
		}
		logMsg("Moved old package.use content to " + legacyFile)

	} else if os.IsNotExist(err) {
		if config.DryRun {
			warnMsg("Would create directory: " + config.PackageUseDir)
			return nil
		}
		if err := os.MkdirAll(config.PackageUseDir, 0755); err != nil {
			return err
		}
		logMsg("Created directory: " + config.PackageUseDir)
	}

	// Also ensure keywords directory
	if _, err := os.Stat(config.KeywordsDir); os.IsNotExist(err) {
		if !config.DryRun {
			os.MkdirAll(config.KeywordsDir, 0755)
		}
	}

	return nil
}

func parseUseRequirements(input string) []UseRequirement {
	var requirements []UseRequirement
	seen := make(map[string]bool)

	// Pattern to match package atoms with USE flags
	// Examples:
	//   >=dev-libs/openssl-3.0.0 -bindist
	//   >=app-crypt/gnupg-2.0 smartcard tools
	//   #>=dev-libs/foo-1.0 bar (required by something)

	// Regex patterns
	patterns := []*regexp.Regexp{
		// Standard format: >=category/package-version flags
		regexp.MustCompile(`(?m)^\s*#?\s*(>=?|<=?|=|~)?([a-z0-9-]+/[a-zA-Z0-9._+-]+(?:-[0-9][a-zA-Z0-9._-]*)?)\s+([a-zA-Z0-9_ -]+?)(?:\s*\(|$)`),
		// Alternative: just category/package flags (without version constraint)
		regexp.MustCompile(`(?m)^\s*(>=?|<=?|=|~)?([a-z0-9-]+/[a-zA-Z0-9._+-]+)\s+(-?[a-zA-Z][a-zA-Z0-9_-]*(?:\s+-?[a-zA-Z][a-zA-Z0-9_-]*)*)\s*$`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(input, -1)
		for _, match := range matches {
			var atom, flags string

			if len(match) >= 4 {
				constraint := match[1]
				pkg := match[2]
				flags = strings.TrimSpace(match[3])

				if constraint != "" {
					atom = constraint + pkg
				} else {
					atom = pkg
				}
			} else if len(match) >= 3 {
				atom = match[1]
				flags = strings.TrimSpace(match[2])
			}

			if atom == "" || flags == "" {
				continue
			}

			// Skip if flags look like version numbers or other non-flag content
			if strings.HasPrefix(flags, "[") || strings.HasPrefix(flags, "(") {
				continue
			}

			// Parse individual flags
			flagList := parseFlags(flags)
			if len(flagList) == 0 {
				continue
			}

			// Deduplicate
			key := atom + ":" + strings.Join(flagList, ",")
			if seen[key] {
				continue
			}
			seen[key] = true

			debugMsg(fmt.Sprintf("Found USE requirement: %s %v", atom, flagList))

			requirements = append(requirements, UseRequirement{
				Atom:  atom,
				Flags: flagList,
			})
		}
	}

	return requirements
}

func parseFlags(flagStr string) []string {
	var flags []string
	parts := strings.Fields(flagStr)

	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Skip empty or invalid
		if part == "" {
			continue
		}

		// Skip things that look like versions or constraints
		if strings.HasPrefix(part, "(") || strings.HasPrefix(part, "[") {
			continue
		}

		// Valid USE flags: start with letter or -, contain alphanumeric, _, -
		if isValidUseFlag(part) {
			flags = append(flags, part)
		}
	}

	return flags
}

func isValidUseFlag(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Can start with - (disable) or letter
	start := s[0]
	if start == '-' {
		if len(s) < 2 {
			return false
		}
		s = s[1:]
		start = s[0]
	}

	// Must start with letter
	if !((start >= 'a' && start <= 'z') || (start >= 'A' && start <= 'Z')) {
		return false
	}

	// Rest must be alphanumeric, _, or -
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-' || c == '+') {
			return false
		}
	}

	return true
}

func parseKeywordRequirements(input string) []KeywordRequirement {
	var requirements []KeywordRequirement
	seen := make(map[string]bool)

	// Pattern: >=category/package-version ~amd64 or **
	pattern := regexp.MustCompile(`(?m)(>=?|<=?|=|~)?([a-z0-9-]+/[a-zA-Z0-9._+-]+(?:-[0-9][a-zA-Z0-9._-]*)?)\s+(~[a-z0-9]+|\*\*)`)

	matches := pattern.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		constraint := match[1]
		pkg := match[2]
		keyword := match[3]

		var atom string
		if constraint != "" {
			atom = constraint + pkg
		} else {
			atom = pkg
		}

		key := atom + ":" + keyword
		if seen[key] {
			continue
		}
		seen[key] = true

		debugMsg(fmt.Sprintf("Found keyword requirement: %s %s", atom, keyword))

		requirements = append(requirements, KeywordRequirement{
			Atom:    atom,
			Keyword: keyword,
		})
	}

	return requirements
}

func sanitizeFilename(atom string) string {
	// Extract package name from atom
	// >=dev-libs/openssl-3.0 -> openssl
	name := atom

	// Remove constraint prefix
	name = strings.TrimPrefix(name, ">=")
	name = strings.TrimPrefix(name, "<=")
	name = strings.TrimPrefix(name, ">")
	name = strings.TrimPrefix(name, "<")
	name = strings.TrimPrefix(name, "=")
	name = strings.TrimPrefix(name, "~")

	// Remove category
	if idx := strings.Index(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	// Remove version
	// Find first occurrence of -[0-9]
	for i := 0; i < len(name)-1; i++ {
		if name[i] == '-' && name[i+1] >= '0' && name[i+1] <= '9' {
			name = name[:i]
			break
		}
	}

	return strings.ToLower(name)
}

func processUseRequirement(req UseRequirement) {
	pkgName := sanitizeFilename(req.Atom)
	useFile := filepath.Join(config.PackageUseDir, pkgName+".use")
	useLine := req.Atom + " " + strings.Join(req.Flags, " ")

	logMsg("📦 " + req.Atom)
	fmt.Printf("   %sUSE flags:%s %s\n", colorCyan, colorReset, strings.Join(req.Flags, " "))
	fmt.Printf("   %sFile:%s %s\n", colorCyan, colorReset, useFile)

	if config.DryRun {
		fmt.Printf("   %sWould add:%s %s\n", colorYellow, colorReset, useLine)
		return
	}

	// Check if line already exists
	if fileContainsLine(useFile, useLine) {
		fmt.Printf("   %sAlready exists!%s\n", colorGreen, colorReset)
		return
	}

	// Append to file
	f, err := os.OpenFile(useFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		errorMsg("Failed to open " + useFile + ": " + err.Error())
		return
	}
	defer f.Close()

	if _, err := f.WriteString(useLine + "\n"); err != nil {
		errorMsg("Failed to write to " + useFile + ": " + err.Error())
		return
	}

	fmt.Printf("   %sAdded! 💕%s\n", colorGreen, colorReset)
}

func processKeywordRequirement(req KeywordRequirement) {
	pkgName := sanitizeFilename(req.Atom)
	keywordFile := filepath.Join(config.KeywordsDir, pkgName+".accept_keywords")
	keywordLine := req.Atom + " " + req.Keyword

	logMsg("🔑 " + req.Atom)
	fmt.Printf("   %sKeyword:%s %s\n", colorCyan, colorReset, req.Keyword)
	fmt.Printf("   %sFile:%s %s\n", colorCyan, colorReset, keywordFile)

	if config.DryRun {
		fmt.Printf("   %sWould add:%s %s\n", colorYellow, colorReset, keywordLine)
		return
	}

	// Check if line already exists
	if fileContainsLine(keywordFile, keywordLine) {
		fmt.Printf("   %sAlready exists!%s\n", colorGreen, colorReset)
		return
	}

	// Ensure directory exists
	os.MkdirAll(config.KeywordsDir, 0755)

	// Append to file
	f, err := os.OpenFile(keywordFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		errorMsg("Failed to open " + keywordFile + ": " + err.Error())
		return
	}
	defer f.Close()

	if _, err := f.WriteString(keywordLine + "\n"); err != nil {
		errorMsg("Failed to write to " + keywordFile + ": " + err.Error())
		return
	}

	fmt.Printf("   %sAdded! 💕%s\n", colorGreen, colorReset)
}

func fileContainsLine(filepath, line string) bool {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return false
	}

	lines := strings.Split(string(content), "\n")
	for _, l := range lines {
		if strings.TrimSpace(l) == strings.TrimSpace(line) {
			return true
		}
	}
	return false
}
