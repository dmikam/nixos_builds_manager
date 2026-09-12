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
	IsCurrent bool
	Marked    bool
}

// SanitizeLabel strips unallowed characters from the label.
// Only alphanumeric characters, dashes, underscores, and dots are allowed.
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

		isCurrent := target == currentPath

		generations = append(generations, Generation{
			ID:        id,
			Path:      file,
			Target:    target,
			Timestamp: ts,
			Label:     label,
			IsCurrent: isCurrent,
			Marked:    false,
		})
	}

	sort.Slice(generations, func(i, j int) bool {
		return generations[i].Timestamp.After(generations[j].Timestamp)
	})

	return generations, nil
}

func extractSystemLabel(storePath string) string {
	nixosVersionFile := filepath.Join(storePath, "nixos-version")
	if data, err := os.ReadFile(nixosVersionFile); err == nil {
		return strings.TrimSpace(string(data))
	}
	return "NixOS System"
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

func runCmdStream(env []string, outChan chan<- string, name string, args ...string) error {
	cmd := exec.Command(name, args...)

	// Force execution inside /tmp so any generated symlinks stay out of your working directory
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

	// Cleanup any result symlink left in /tmp after execution
	_ = os.Remove(filepath.Join(os.TempDir(), "result"))

	return err
}