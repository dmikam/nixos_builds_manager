package nix

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type ConfigType string

const (
	ConfigTypeFlake   ConfigType = "Flake"
	ConfigTypeClassic ConfigType = "Classic"
	ConfigTypeNone    ConfigType = "None"
)

type EnvironmentInfo struct {
	IsFlakeSupported bool
	ConfigType       ConfigType
	ConfigPath       string
	FlakeURI         string
	FlakeHost        string
	GitDirty         bool
	GitRev           string
}

type Generation struct {
	ID        int
	Profile   string
	Path      string
	Target    string
	Timestamp time.Time
	Label     string
	Kernel    string
	IsCurrent bool
	IsOrphan  bool
	Marked    bool
}

func DetectEnvironment() EnvironmentInfo {
	info := EnvironmentInfo{
		IsFlakeSupported: checkFlakeSupported(),
		ConfigType:       ConfigTypeNone,
		ConfigPath:       "None",
		FlakeHost:        resolveHostname(),
	}

	// 1. Check NIXOS_FLAKE environment variable
	if envFlake := os.Getenv("NIXOS_FLAKE"); envFlake != "" {
		parts := strings.Split(envFlake, "#")
		flakeDir := parts[0]
		targetPath := filepath.Join(flakeDir, "flake.nix")
		if _, err := os.Stat(targetPath); err == nil {
			info.ConfigType = ConfigTypeFlake
			info.ConfigPath = targetPath
			info.FlakeURI = flakeDir
			if len(parts) > 1 && parts[1] != "" {
				info.FlakeHost = parts[1]
			}
			checkGitInfo(&info)
			return info
		}
	}

	// 2. Candidate flake directories
	candidates := getFlakeCandidates()
	for _, cand := range candidates {
		flakePath := filepath.Join(cand, "flake.nix")
		if _, err := os.Stat(flakePath); err == nil {
			info.ConfigType = ConfigTypeFlake
			info.ConfigPath = flakePath
			info.FlakeURI = cand
			checkGitInfo(&info)
			return info
		}
	}

	// 3. Check for classic configuration.nix
	classicCandidates := []string{
		"/etc/nixos/configuration.nix",
	}
	if userHome := getUserHome(); userHome != "" {
		classicCandidates = append(classicCandidates, filepath.Join(userHome, ".config/nixos/configuration.nix"))
	}

	for _, path := range classicCandidates {
		if _, err := os.Stat(path); err == nil {
			info.ConfigType = ConfigTypeClassic
			info.ConfigPath = path
			return info
		}
	}

	return info
}

func getFlakeCandidates() []string {
	var candidates []string

	// Current working directory
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}

	// Standard system location
	candidates = append(candidates, "/etc/nixos")

	// User locations
	if userHome := getUserHome(); userHome != "" {
		candidates = append(candidates,
			filepath.Join(userHome, ".config/nixos"),
			filepath.Join(userHome, ".config/nixpkgs"),
			filepath.Join(userHome, "dotfiles"),
			filepath.Join(userHome, "dotfiles/nixos"),
			filepath.Join(userHome, "nixos-config"),
			filepath.Join(userHome, "nixos"),
			filepath.Join(userHome, "dev/nixos"),
		)
	}

	return candidates
}

func getUserHome() string {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && sudoUser != "root" {
		if u, err := user.Lookup(sudoUser); err == nil && u.HomeDir != "" {
			return u.HomeDir
		}
		return filepath.Join("/home", sudoUser)
	}
	if home := os.Getenv("HOME"); home != "" && home != "/root" {
		return home
	}
	return ""
}

func checkFlakeSupported() bool {
	data, err := os.ReadFile("/etc/nix/nix.conf")
	if err == nil {
		content := string(data)
		if strings.Contains(content, "flakes") {
			return true
		}
	}
	return false
}

func resolveHostname() string {
	if data, err := os.ReadFile("/etc/hostname"); err == nil {
		h := strings.TrimSpace(string(data))
		if h != "" {
			return h
		}
	}
	if h, err := os.Hostname(); err == nil {
		return strings.TrimSpace(h)
	}
	return "localhost"
}

func checkGitInfo(info *EnvironmentInfo) {
	gitDir := filepath.Join(info.FlakeURI, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return
	}
	cmd := exec.Command("git", "-C", info.FlakeURI, "rev-parse", "--short", "HEAD")
	if out, err := cmd.Output(); err == nil {
		info.GitRev = strings.TrimSpace(string(out))
	}
	cmdDirty := exec.Command("git", "-C", info.FlakeURI, "status", "--porcelain")
	if outDirty, err := cmdDirty.Output(); err == nil {
		if len(strings.TrimSpace(string(outDirty))) > 0 {
			info.GitDirty = true
		}
	}
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

func parseBootConf(confPath string) (target string, label string, kernel string) {
	f, err := os.Open(confPath)
	if err != nil {
		return "", "Orphaned Boot Entry", "Unknown"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	reInit := regexp.MustCompile(`init=(/nix/store/[^ \t\r\n]+)/init`)
	reVersion := regexp.MustCompile(`^version\s+(.+)$`)
	reLinux := regexp.MustCompile(`^linux\s+(.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := reInit.FindStringSubmatch(line); len(matches) > 1 {
			target = matches[1]
		}
		if matches := reVersion.FindStringSubmatch(line); len(matches) > 1 {
			label = matches[1]
		}
		if matches := reLinux.FindStringSubmatch(line); len(matches) > 1 {
			kernel = filepath.Base(matches[1])
		}
	}

	if target != "" {
		if _, err := os.Stat(target); err == nil {
			if l := extractSystemLabel(target); l != "NixOS System" {
				label = l
			}
			if k := extractKernelVersion(target); k != "Unknown" {
				kernel = k
			}
		} else {
			target = target + " (closure deleted)"
		}
	}
	if label == "" {
		label = "Orphaned Boot Entry"
	}
	return target, label, kernel
}

func ListGenerations() ([]Generation, error) {
	currentPath, _ := GetCurrentBootedPath()

	seenKeys := make(map[string]bool)
	var generations []Generation

	// 1. Default system profile generations: /nix/var/nix/profiles/system-*-link
	files, _ := filepath.Glob("/nix/var/nix/profiles/system-*-link")
	reSystem := regexp.MustCompile(`system-(\d+)-link$`)

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

		key := fmt.Sprintf("system:%d", id)
		if id > 0 && seenKeys[key] {
			continue
		}
		if id > 0 {
			seenKeys[key] = true
		}

		kernelVer := extractKernelVersion(target)

		generations = append(generations, Generation{
			ID:        id,
			Profile:   "system",
			Path:      file,
			Target:    target,
			Timestamp: ts,
			Label:     label,
			Kernel:    kernelVer,
			IsCurrent: false,
			IsOrphan:  false,
			Marked:    false,
		})
	}

	// 2. Custom named profiles: /nix/var/nix/profiles/system-profiles/*-link
	namedFiles, _ := filepath.Glob("/nix/var/nix/profiles/system-profiles/*-link")
	reNamed := regexp.MustCompile(`^(.+)-(\d+)-link$`)

	for _, file := range namedFiles {
		base := filepath.Base(file)
		matches := reNamed.FindStringSubmatch(base)
		if len(matches) <= 2 {
			continue
		}

		profileName := matches[1]
		id, _ := strconv.Atoi(matches[2])
		key := fmt.Sprintf("%s:%d", profileName, id)
		if seenKeys[key] {
			continue
		}
		seenKeys[key] = true

		target, err := os.Readlink(file)
		if err != nil {
			target = "Unknown"
		}

		var ts time.Time
		if info, err := os.Lstat(file); err == nil {
			ts = info.ModTime()
		}

		label := extractSystemLabel(target)
		kernelVer := extractKernelVersion(target)

		generations = append(generations, Generation{
			ID:        id,
			Profile:   profileName,
			Path:      file,
			Target:    target,
			Timestamp: ts,
			Label:     label,
			Kernel:    kernelVer,
			IsCurrent: false,
			IsOrphan:  false,
			Marked:    false,
		})
	}

	// 3. Bootloader entries: /boot/loader/entries/*.conf
	bootEntries, _ := filepath.Glob("/boot/loader/entries/*.conf")
	reSysConf := regexp.MustCompile(`^nixos-generation-(\d+)\.conf$`)
	reNamedConf := regexp.MustCompile(`^nixos-(.+)-generation-(\d+)\.conf$`)

	for _, confFile := range bootEntries {
		base := filepath.Base(confFile)
		var profile string
		var id int

		if matches := reSysConf.FindStringSubmatch(base); len(matches) > 1 {
			profile = "system"
			id, _ = strconv.Atoi(matches[1])
		} else if matches := reNamedConf.FindStringSubmatch(base); len(matches) > 2 {
			profile = matches[1]
			id, _ = strconv.Atoi(matches[2])
		} else {
			continue
		}

		key := fmt.Sprintf("%s:%d", profile, id)
		if seenKeys[key] {
			continue
		}
		seenKeys[key] = true

		target, label, kernel := parseBootConf(confFile)
		var ts time.Time
		if info, err := os.Stat(confFile); err == nil {
			ts = info.ModTime()
		}

		generations = append(generations, Generation{
			ID:        id,
			Profile:   profile,
			Path:      confFile,
			Target:    target,
			Timestamp: ts,
			Label:     label,
			Kernel:    kernel,
			IsCurrent: false,
			IsOrphan:  true,
			Marked:    false,
		})
	}

	// Sort: newest timestamp first; if equal, highest ID first
	sort.Slice(generations, func(i, j int) bool {
		if generations[i].Timestamp.Equal(generations[j].Timestamp) {
			return generations[i].ID > generations[j].ID
		}
		return generations[i].Timestamp.After(generations[j].Timestamp)
	})

	currentFound := false
	for i := range generations {
		if !generations[i].IsOrphan && generations[i].Target == currentPath && !currentFound {
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

// ListExistingProfiles returns a sorted list of unique named profiles discovered
// on the system (from /nix/var/nix/profiles/system-profiles/ and from generations).
// The default "system" profile is excluded as it represents the default profile without -p.
func ListExistingProfiles(gens []Generation) []string {
	seen := make(map[string]bool)
	var profiles []string

	for _, g := range gens {
		if g.Profile != "" && g.Profile != "system" && !seen[g.Profile] {
			seen[g.Profile] = true
			profiles = append(profiles, g.Profile)
		}
	}

	namedFiles, _ := filepath.Glob("/nix/var/nix/profiles/system-profiles/*-link")
	reNamed := regexp.MustCompile(`^(.+)-(\d+)-link$`)
	for _, file := range namedFiles {
		base := filepath.Base(file)
		matches := reNamed.FindStringSubmatch(base)
		if len(matches) > 1 {
			p := matches[1]
			if p != "" && p != "system" && !seen[p] {
				seen[p] = true
				profiles = append(profiles, p)
			}
		}
	}

	sort.Strings(profiles)
	return profiles
}

func cleanStaleProfileLocks() {
	lockFiles, _ := filepath.Glob("/nix/var/nix/profiles/system-profiles/*.lock")
	for _, f := range lockFiles {
		_ = os.Remove(f)
	}
	rootLocks, _ := filepath.Glob("/nix/var/nix/profiles/*.lock")
	for _, f := range rootLocks {
		_ = os.Remove(f)
	}
}

func RebuildSystemStream(profile string, label string, switchBuild bool, outChan chan<- string) error {
	cleanStaleProfileLocks()

	action := "boot"
	if switchBuild {
		action = "switch"
	}

	cleanProfile := SanitizeLabel(profile)
	cleanLabel := SanitizeLabel(label)

	var args []string
	var env []string

	if cleanProfile != "" && cleanProfile != "system" {
		args = append(args, action, "-p", cleanProfile)
		outChan <- fmt.Sprintf("Using target profile: %s", cleanProfile)
	} else {
		args = append(args, action)
	}

	if cleanLabel != "" {
		env = append(os.Environ(), fmt.Sprintf("NIXOS_LABEL=%s", cleanLabel))
		outChan <- fmt.Sprintf("Using sanitized build label: %s", cleanLabel)
	}

	return runCmdStream(env, outChan, "nixos-rebuild", args...)
}

func SwitchToGenerationStream(gen Generation, outChan chan<- string) error {
	if gen.IsOrphan {
		return fmt.Errorf("cannot switch to orphaned generation: generation symlink no longer exists in profiles")
	}

	outChan <- fmt.Sprintf("Switching active system to %s (Gen %d - %s)...", gen.Profile, gen.ID, gen.Label)

	profilePath := "/nix/var/nix/profiles/system"
	if gen.Profile != "system" && gen.Profile != "" {
		profilePath = filepath.Join("/nix/var/nix/profiles/system-profiles", gen.Profile)
	}

	err := runCmdStream(nil, outChan, "nix-env", "-p", profilePath, "--switch-generation", strconv.Itoa(gen.ID))
	if err != nil {
		outChan <- fmt.Sprintf("Note: profile pointer update: %v", err)
	}

	outChan <- "Activating system configuration..."
	// 1. Try store path target directly
	switchScript := filepath.Join(gen.Target, "bin", "switch-to-configuration")
	if _, err := os.Stat(switchScript); err == nil {
		return runCmdStream(nil, outChan, switchScript, "switch")
	}

	// 2. Try link path
	switchScriptGen := filepath.Join(gen.Path, "bin", "switch-to-configuration")
	if _, err := os.Stat(switchScriptGen); err == nil {
		return runCmdStream(nil, outChan, switchScriptGen, "switch")
	}

	// 3. Try profile pointer
	profileSwitchScript := filepath.Join(profilePath, "bin", "switch-to-configuration")
	if _, err := os.Stat(profileSwitchScript); err == nil {
		return runCmdStream(nil, outChan, profileSwitchScript, "switch")
	}

	return fmt.Errorf("failed to locate switch-to-configuration script at %s", switchScript)
}

func PurgeGenerationsStream(gens []Generation, outChan chan<- string) error {
	if len(gens) == 0 {
		outChan <- "No generations selected to purge."
		return nil
	}

	for _, g := range gens {
		if g.IsOrphan {
			outChan <- fmt.Sprintf("Removing orphaned bootloader entry: %s...", filepath.Base(g.Path))
			if err := os.Remove(g.Path); err != nil {
				outChan <- fmt.Sprintf("Warning: failed to remove %s: %v", g.Path, err)
			}
			continue
		}

		profilePath := "/nix/var/nix/profiles/system"
		if g.Profile != "system" && g.Profile != "" {
			profilePath = filepath.Join("/nix/var/nix/profiles/system-profiles", g.Profile)
		}

		// Handle auto-switch if active profile pointer
		profileTarget, err := os.Readlink(profilePath)
		if err == nil && filepath.Base(profileTarget) == filepath.Base(g.Path) {
			allGens, _ := ListGenerations()
			var safeGen *Generation
			// Prefer currently booted system if same profile
			for i := range allGens {
				if allGens[i].Profile == g.Profile && allGens[i].IsCurrent && allGens[i].ID != g.ID && !allGens[i].IsOrphan {
					safeGen = &allGens[i]
					break
				}
			}
			// Fallback to any other non-orphan gen in this profile
			if safeGen == nil {
				for i := range allGens {
					if allGens[i].Profile == g.Profile && allGens[i].ID != g.ID && !allGens[i].IsOrphan {
						safeGen = &allGens[i]
						break
					}
				}
			}

			if safeGen != nil {
				outChan <- fmt.Sprintf("Profile %s points to Gen %d (scheduled for deletion).", g.Profile, g.ID)
				outChan <- fmt.Sprintf("Switching profile pointer to Gen %d first...", safeGen.ID)
				_ = runCmdStream(nil, outChan, "nix-env", "-p", profilePath, "--switch-generation", strconv.Itoa(safeGen.ID))
			}
		}

		outChan <- fmt.Sprintf("Deleting %s generation %d...", g.Profile, g.ID)
		err = runCmdStream(nil, outChan, "nix-env", "-p", profilePath, "--delete-generations", strconv.Itoa(g.ID))
		if err != nil {
			outChan <- fmt.Sprintf("Error deleting generation: %v", err)
			return err
		}

		// If named profile has no more generations left, clean up the profile symlink
		if g.Profile != "system" && g.Profile != "" {
			cleanupNamedProfileIfEmpty(g.Profile, outChan)
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

func cleanupNamedProfileIfEmpty(profileName string, outChan chan<- string) {
	profileLink := filepath.Join("/nix/var/nix/profiles/system-profiles", profileName)
	links, _ := filepath.Glob(filepath.Join("/nix/var/nix/profiles/system-profiles", profileName+"-*-link"))
	if len(links) == 0 {
		outChan <- fmt.Sprintf("Removing empty profile pointer: %s...", profileName)
		_ = os.Remove(profileLink)
	}
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