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

	var generations []Generation

	for _, file := range files {
		target, err := os.Readlink(file)
		if err != nil {
			target = "Unknown"
		}

		info, err := os.Lstat(file)
		var ts time.Time
		if err == nil {
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

	sort.Slice(generations, func(i, j int) bool {
		return generations[i].Timestamp.After(generations[j].Timestamp)
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
	base := filepath.Base(target)
	re := regexp.MustCompile(`linux-(.+)$`)
	matches := re.FindStringSubmatch(base)
	if len(matches) > 1 {
		return matches[1]
	}
	return base
}

func AnalyzeStorePathSize(storePath string) (string, error) {
	cmd := exec.Command("nix-path-info", "-Sh", storePath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to analyze store path: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) >= 2 {
		return fields[1], nil
	}
	return strings.TrimSpace(string(out)), nil
}

func RenameCustomProfile(gen Generation, newLabel string) error {
	cleanLabel := SanitizeLabel(newLabel)
	if cleanLabel == "" {
		return fmt.Errorf("invalid or empty label")
	}

	if !strings.HasPrefix(gen.Path, "/nix/var/nix/profiles/system-profiles/") {
		return fmt.Errorf("only custom profiles can be renamed directly")
	}

	dir := filepath.Dir(gen.Path)
	newLinkPath := filepath.Join(dir, fmt.Sprintf("%s-%d-link", cleanLabel, gen.ID))

	err := os.Rename(gen.Path, newLinkPath)
	if err != nil {
		return fmt.Errorf("failed to rename profile link: %w", err)
	}

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