package nix

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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

	re := regexp.MustCompile(`system-(\d+)-link$`)
	var generations []Generation

	for _, file := range files {
		matches := re.FindStringSubmatch(file)
		if len(matches) < 2 {
			continue
		}

		id, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		target, err := os.Readlink(file)
		if err != nil {
			target = "Unknown"
		}

		info, err := os.Lstat(file)
		var ts time.Time
		if err == nil {
			ts = info.ModTime()
		}

		label := extractSystemLabel(target)
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
		return generations[i].ID > generations[j].ID
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

func PurgeGenerations(ids []int) (string, error) {
	if len(ids) == 0 {
		return "No generations selected to purge.", nil
	}

	var strIDs []string
	for _, id := range ids {
		strIDs = append(strIDs, strconv.Itoa(id))
	}

	args := append([]string{"-p", "/nix/var/nix/profiles/system", "--delete-generations"}, strIDs...)
	out1, err := runCmd("nix-env", args...)
	if err != nil {
		return out1, fmt.Errorf("failed deleting generations: %w", err)
	}

	out2, err := runCmd("nixos-rebuild", "boot")
	if err != nil {
		return out1 + "\n" + out2, fmt.Errorf("failed rebuilding bootloader: %w", err)
	}

	out3, err := runCmd("nix-collect-garbage")
	if err != nil {
		return out1 + "\n" + out2 + "\n" + out3, fmt.Errorf("failed collecting garbage: %w", err)
	}

	return out1 + "\n" + out2 + "\n" + out3, nil
}

func OptimizeStore() (string, error) {
	return runCmd("nix-store", "--optimise")
}

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	outputStr := string(out)
	if err != nil {
		return outputStr, fmt.Errorf("command '%s %s' failed: %w", name, strings.Join(args, " "), err)
	}
	return outputStr, nil
}