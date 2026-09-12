package nix

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type Generation struct {
	ID        int
	Path      string
	Target    string
	Timestamp time.Time
	Label     string
	Kernel    string
	IsCurrent bool
	Marked    bool
}

func SanitizeLabel(label string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	sanitized := re.ReplaceAllString(label, "_")
	return strings.TrimSpace(sanitized)
}

func GetNixStoreFreeSpace() string {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/nix/store", &stat)
	if err != nil {
		return "Disk: Unknown"
	}
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	return fmt.Sprintf("Free: %.2f GB", float64(freeBytes)/(1024*1024*1024))
}

func GetCurrentBootedPath() (string, error) {
	target, err := os.Readlink("/run/current-system")
	if err != nil {
		return "", fmt.Errorf("failed to readlink /run/current-system: %w", err)
	}
	return target, nil
}

func ListGenerations() ([]Generation, error) {
	currentPath, _ := GetCurrentBootedPath()

	files, err := filepath.Glob("/nix/var/nix/profiles/system-*-link")
	if err != nil {
		return nil, fmt.Errorf("failed to scan system profiles: %w", err)
	}

	profileFiles, _ := filepath.Glob("/nix/var/nix/profiles/system-profiles/*-link")
	files = append(files, profileFiles...)

	reSystem := regexp.MustCompile(`system-(\d+)-link$`)
	reCustom := regexp.MustCompile(`^(.+)-(\d+)-link$`)

	seenIDs := make(map[int]bool)
	var generations []Generation

	for _, file := range files {
		target, err := os.Readlink(file)
		if err != nil {
			target = "Unknown"
		}

		// Use Lstat on the symlink itself (store path targets are reset to 1970 by Nix)
		var ts time.Time
		if info, err := os.Lstat(file); err == nil {
			ts = info.ModTime()
		}

		var id int
		var label string

		base := filepath.Base(file)
		if strings.HasPrefix(file, "/nix/var/nix/profiles/system-profiles/") {
			matches := reCustom.FindStringSubmatch(base)
			if len(matches) > 2 {
				label = matches[1]
				id, _ = strconv.Atoi(matches[2])
			}
		} else {
			matches := reSystem.FindStringSubmatch(base)
			if len(matches) > 1 {
				id, _ = strconv.Atoi(matches[1])
				label = extractSystemLabel(target)
			}
		}

		if id > 0 && seenIDs[id] {
			continue
		}
		if id > 0 {
			seenIDs[id] = true
		}

		kernelVer := extractKernelVersion(target)

		generations = append(generations, Generation{
			ID:        id,
			Path:      file,
			Target:    target,
			Timestamp: ts,
			Label:     label,
			Kernel:    kernelVer,
			IsCurrent: false,
			Marked:    false,
		})
	}

	// Keep list ordered by Generation ID descending
	sort.Slice(generations, func(i, j int) bool {
		return generations[i].ID > generations[j].ID
	})

	currentFound := false
	for i := range generations {
		if generations[i].Target == currentPath && !currentFound {
			generations[i].IsCurrent = true
			currentFound = true
		}
	}

	return generations, nil
}

func extractSystemLabel(storePath string) string {
	nixosVersionFile := filepath.Join(storePath, "nixos-version")
	if data, err := os.ReadFile(nixosVersionFile); err == nil {
		return strings.TrimSpace(string(data))
	}
	return "NixOS System"
}

func extractKernelVersion(storePath string) string {
	kernelLink := filepath.Join(storePath, "kernel")
	target, err := os.Readlink(kernelLink)
	if err != nil {
		return "Unknown"
	}

	parentDir := filepath.Dir(target)
	baseParent := filepath.Base(parentDir)

	re := regexp.MustCompile(`linux-(.+)$`)
	matches := re.FindStringSubmatch(baseParent)
	if len(matches) > 1 {
		return matches[1]
	}

	modulesLink := filepath.Join(storePath, "kernel-modules")
	if modTarget, err := os.Readlink(modulesLink); err == nil {
		modBase := filepath.Base(modTarget)
		modMatches := re.FindStringSubmatch(modBase)
		if len(modMatches) > 1 {
			return modMatches[1]
		}
	}

	return "Linux"
}

func AnalyzeStorePathSize(storePath string) (string, error) {
	cmd := exec.Command("nix", "path-info", "-S", storePath)
	out, err := cmd.Output()
	if err == nil {
		fields := strings.Fields(string(out))
		if len(fields) >= 2 {
			bytesVal, parseErr := strconv.ParseInt(fields[1], 10, 64)
			if parseErr == nil {
				return formatBytes(bytesVal), nil
			}
		}
	}

	cmdFallback := exec.Command("du", "-sh", storePath)
	outFallback, errFallback := cmdFallback.Output()
	if errFallback == nil {
		fields := strings.Fields(string(outFallback))
		if len(fields) >= 1 {
			return fields[0], nil
		}
	}

	return "", fmt.Errorf("failed to analyze store path size")
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Helper function to update timestamp on a symlink directly
func setSymlinkTimestamp(path string, ts time.Time) error {
	times := []unix.Timespec{
		unix.NsecToTimespec(ts.UnixNano()), // Access time
		unix.NsecToTimespec(ts.UnixNano()), // Modification time
	}
	return unix.UtimesNanoAt(unix.AT_FDCWD, path, times, unix.AT_SYMLINK_NOFOLLOW)
}

func RenameCustomProfile(gen Generation, newLabel string) error {
	cleanLabel := SanitizeLabel(newLabel)
	if cleanLabel == "" {
		return fmt.Errorf("invalid or empty label")
	}

	origTimestamp := gen.Timestamp

	// 1. Rename existing custom profile symlink while maintaining original timestamp
	if strings.HasPrefix(gen.Path, "/nix/var/nix/profiles/system-profiles/") {
		dir := filepath.Dir(gen.Path)
		newPath := filepath.Join(dir, fmt.Sprintf("%s-%d-link", cleanLabel, gen.ID))
		if err := os.Rename(gen.Path, newPath); err != nil {
			return err
		}
		_ = setSymlinkTimestamp(newPath, origTimestamp)
		return nil
	}

	// 2. Create custom profile symlink, copy original timestamp, remove standard symlink
	profileDir := "/nix/var/nix/profiles/system-profiles"
	_ = os.MkdirAll(profileDir, 0755)

	newProfileLink := filepath.Join(profileDir, fmt.Sprintf("%s-%d-link", cleanLabel, gen.ID))
	_ = os.Remove(newProfileLink)

	if err := os.Symlink(gen.Target, newProfileLink); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Restore original symlink timestamp
	_ = setSymlinkTimestamp(newProfileLink, origTimestamp)

	// Remove default symlink to prevent legacy accumulation
	_ = os.Remove(gen.Path)

	return nil
}

func RebuildSystemStream(label string, isProfile bool, switchBuild bool, outChan chan<- string) error {
	action := "boot"
	if switchBuild {
		action = "switch"
	}

	cleanLabel := SanitizeLabel(label)

	var args []string
	var env []string

	if cleanLabel != "" {
		if isProfile {
			args = append(args, action, "-p", cleanLabel)
		} else {
			args = append(args, action)
			env = append(os.Environ(), fmt.Sprintf("NIXOS_LABEL=%s", cleanLabel))
		}
		outChan <- fmt.Sprintf("Using sanitized build label: %s", cleanLabel)
	} else {
		args = append(args, action)
	}

	return runCmdStream(env, outChan, "nixos-rebuild", args...)
}

func SwitchToGenerationStream(gen Generation, outChan chan<- string) error {
	outChan <- fmt.Sprintf("Switching active system to Generation %d (%s)...", gen.ID, gen.Label)

	err := runCmdStream(nil, outChan, "nix-env", "-p", "/nix/var/nix/profiles/system", "--switch-generation", strconv.Itoa(gen.ID))
	if err != nil {
		return fmt.Errorf("failed to switch generation symlink: %w", err)
	}

	outChan <- "Activating system configuration..."
	switchScript := filepath.Join(gen.Path, "bin", "switch")
	if _, err := os.Stat(switchScript); err == nil {
		return runCmdStream(nil, outChan, switchScript, "switch")
	}

	return runCmdStream(nil, outChan, "nixos-rebuild", "switch")
}

func PurgeGenerationsStream(gens []Generation, outChan chan<- string) error {
	if len(gens) == 0 {
		outChan <- "No generations selected to purge."
		return nil
	}

	for _, g := range gens {
		if strings.HasPrefix(g.Path, "/nix/var/nix/profiles/system-profiles/") {
			_ = os.Remove(g.Path)
			parentSymlink := strings.TrimSuffix(g.Path, fmt.Sprintf("-%d-link", g.ID))
			_ = os.Remove(parentSymlink)
			outChan <- fmt.Sprintf("Removed custom profile: %s", g.Label)
		} else {
			outChan <- fmt.Sprintf("Deleting generation %d...", g.ID)
			err := runCmdStream(nil, outChan, "nix-env", "-p", "/nix/var/nix/profiles/system", "--delete-generations", strconv.Itoa(g.ID))
			if err != nil {
				return err
			}
		}
	}

	outChan <- "Rebuilding bootloader menu..."
	_ = runCmdStream(nil, outChan, "nixos-rebuild", "boot")

	outChan <- "Collecting garbage..."
	_ = runCmdStream(nil, outChan, "nix-collect-garbage")

	return nil
}

func OptimizeStoreStream(outChan chan<- string) error {
	return runCmdStream(nil, outChan, "nix-store", "--optimise")
}

func CollectGarbageStream(outChan chan<- string) error {
	outChan <- "Running Nix Garbage Collection..."
	return runCmdStream(nil, outChan, "nix-collect-garbage")
}

func runCmdStream(env []string, outChan chan<- string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = os.TempDir()

	if len(env) > 0 {
		cmd.Env = env
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	reader := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		outChan <- scanner.Text()
	}

	err = cmd.Wait()
	_ = os.Remove(filepath.Join(os.TempDir(), "result"))

	return err
}