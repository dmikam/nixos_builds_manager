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

func RebuildSystem(label string, isProfile bool) (string, error) {
	var args []string
	var env []string

	if strings.TrimSpace(label) != "" {
		if isProfile {
			args = append(args, "switch", "-p", label)
		} else {
			args = append(args, "switch")
			env = append(os.Environ(), fmt.Sprintf("NIXOS_LABEL=%s", label))
		}
	} else {
		args = append(args, "switch")
	}

	return runCmdWithEnv(env, "nixos-rebuild", args...)
}

func PurgeGenerations(gens []Generation) (string, error) {
	if len(gens) == 0 {
		return "No generations selected to purge.", nil
	}

	var outputs []string

	for _, g := range gens {
		if strings.HasPrefix(g.Path, "/nix/var/nix/profiles/system-profiles/") {
			_ = os.Remove(g.Path)
			parentSymlink := strings.TrimSuffix(g.Path, fmt.Sprintf("-%d-link", g.ID))
			_ = os.Remove(parentSymlink)
			outputs = append(outputs, fmt.Sprintf("Removed custom profile: %s", g.Label))
		} else {
			out, err := runCmd("nix-env", "-p", "/nix/var/nix/profiles/system", "--delete-generations", strconv.Itoa(g.ID))
			if err != nil {
				return strings.Join(outputs, "\n") + "\n" + out, err
			}
			outputs = append(outputs, out)
		}
	}

	out2, _ := runCmd("nixos-rebuild", "boot")
	out3, _ := runCmd("nix-collect-garbage")

	outputs = append(outputs, out2, out3)
	return strings.Join(outputs, "\n"), nil
}

func OptimizeStore() (string, error) {
	return runCmd("nix-store", "--optimise")
}

func runCmd(name string, args ...string) (string, error) {
	return runCmdWithEnv(nil, name, args...)
}

func runCmdWithEnv(env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	outputStr := string(out)
	if err != nil {
		return outputStr, fmt.Errorf("command '%s %s' failed: %w", name, strings.Join(args, " "), err)
	}
	return outputStr, nil
}