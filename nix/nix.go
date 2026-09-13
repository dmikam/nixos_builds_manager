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

func GetCurrentProfileGenerationID() (int, error) {
	target, err := os.Readlink("/nix/var/nix/profiles/system")
	if err != nil {
		return 0, fmt.Errorf("failed to readlink /nix/var/nix/profiles/system: %w", err)
	}
	base := filepath.Base(target)
	re := regexp.MustCompile(`system-(\d+)-link$`)
	matches := re.FindStringSubmatch(base)
	if len(matches) > 1 {
		return strconv.Atoi(matches[1])
	}
	return 0, fmt.Errorf("could not determine generation ID from profile target: %s", target)
}

func ListGenerations() ([]Generation, error) {
	currentPath, _ := GetCurrentBootedPath()

	files, err := filepath.Glob("/nix/var/nix/profiles/system-*-link")
	if err != nil {
		return nil, fmt.Errorf("failed to scan system profiles: %w", err)
	}

	reSystem := regexp.MustCompile(`system-(\d+)-link$`)

	seenIDs := make(map[int]bool)
	var generations []Generation

	for _, file := range files {
		target, err := os.Readlink(file)
		if err != nil {
			target = "Unknown"
		}

		var ts time.Time
		if info, err := os.Lstat(file); err == nil {
			ts = info.ModTime()
		}

		var id int
		var label string

		base := filepath.Base(file)
		matches := reSystem.FindStringSubmatch(base)
		if len(matches) > 1 {
			id, _ = strconv.Atoi(matches[1])
			label = extractSystemLabel(target)
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
	switchScript := filepath.Join(gen.Path, "bin", "switch-to-configuration")
	if _, err := os.Stat(switchScript); err == nil {
		return runCmdStream(nil, outChan, switchScript, "switch")
	}

	systemSwitchScript := "/nix/var/nix/profiles/system/bin/switch-to-configuration"
	if _, err := os.Stat(systemSwitchScript); err == nil {
		return runCmdStream(nil, outChan, systemSwitchScript, "switch")
	}

	return fmt.Errorf("failed to locate switch-to-configuration script at %s", switchScript)
}

func PurgeGenerationsStream(gens []Generation, outChan chan<- string) error {
	if len(gens) == 0 {
		outChan <- "No generations selected to purge."
		return nil
	}

	// Check if any generation to be deleted is currently pointed to by /nix/var/nix/profiles/system
	profileID, err := GetCurrentProfileGenerationID()
	if err == nil {
		isDeletingProfileTarget := false
		for _, g := range gens {
			if g.ID == profileID {
				isDeletingProfileTarget = true
				break
			}
		}

		if isDeletingProfileTarget {
			allGens, _ := ListGenerations()
			var safeGen *Generation
			// 1. Prefer the currently booted system
			for i := range allGens {
				if allGens[i].IsCurrent {
					safeGen = &allGens[i]
					break
				}
			}
			// 2. Fallback to newest generation that is not being purged
			if safeGen == nil {
				purgeSet := make(map[int]bool)
				for _, g := range gens {
					purgeSet[g.ID] = true
				}
				for i := range allGens {
					if !purgeSet[allGens[i].ID] {
						safeGen = &allGens[i]
						break
					}
				}
			}

			if safeGen != nil {
				outChan <- fmt.Sprintf("Profile points to Generation %d (scheduled for deletion).", profileID)
				outChan <- fmt.Sprintf("Switching profile pointer to Generation %d first...", safeGen.ID)
				err := runCmdStream(nil, outChan, "nix-env", "-p", "/nix/var/nix/profiles/system", "--switch-generation", strconv.Itoa(safeGen.ID))
				if err != nil {
					return fmt.Errorf("failed to switch profile pointer: %w", err)
				}
			}
		}
	}

	for _, g := range gens {
		outChan <- fmt.Sprintf("Deleting generation %d...", g.ID)
		err := runCmdStream(nil, outChan, "nix-env", "-p", "/nix/var/nix/profiles/system", "--delete-generations", strconv.Itoa(g.ID))
		if err != nil {
			return err
		}
	}

	outChan <- "Rebuilding bootloader menu..."
	bootScript := "/nix/var/nix/profiles/system/bin/switch-to-configuration"
	if _, err := os.Stat(bootScript); err == nil {
		if err := runCmdStream(nil, outChan, bootScript, "boot"); err != nil {
			outChan <- fmt.Sprintf("Warning: failed to update bootloader menu: %v", err)
		}
	} else {
		outChan <- "Warning: switch-to-configuration not found, skipping bootloader menu update"
	}

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