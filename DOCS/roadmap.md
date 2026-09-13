# NixOS Builds Manager: Development Roadmap

This document outlines the planned feature milestones for evolving **NixOS Builds Manager** into a comprehensive, multi-profile management interface supporting Nix Flakes, standalone Home-Manager, and advanced closure inspection tools.

---

## Roadmap Overview

```
+-----------------------------------------------------------------------------------+
| Milestone 1: Nix Flakes Support                                                   |
| - Flake detection, --flake builds, git metadata & commit inspection               |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          v
+-----------------------------------------------------------------------------------+
| Milestone 2: Multi-Profile & Home-Manager                                         |
| - Profile tabs (System vs User), HM generation switching & purging               |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          v
+-----------------------------------------------------------------------------------+
| Milestone 3: Generation Diffing & Inspection                                      |
| - Visual closure comparison with nvd / nix store diff-closures                    |
+-----------------------------------------+-----------------------------------------+
                                          |
                                          v
+-----------------------------------------------------------------------------------+
| Milestone 4: Advanced Store & Boot Diagnostics                                    |
| - Bootloader default indicators, retention policies, closure size breakdown       |
+-----------------------------------------------------------------------------------+
```

---

## Milestone 1: Nix Flakes Support

### 1.1 Flake Auto-Detection & Target Resolution
* **Automatic Flake Detection**:
  * Scan standard locations (`/etc/nixos/flake.nix`, `~/.config/nixos/flake.nix`, dotfiles repository in `$PWD`).
  * Determine active system hostname via `/etc/hostname` or `os.Hostname()` to suggest default target attribute `#<hostname>`.
* **Git Status & Purity Validation**:
  * Check for uncommitted or untracked changes in the flake repository.
  * Warn user if untracked files might break the flake evaluation.

### 1.2 Enhanced "New Build" Modal (`F3` / `N`)
* **Flake URI Input Field**:
  * Allow specifying custom flake paths (e.g., `.#desktop`, `/home/dima/dotfiles#nixos`).
  * Persist the last-used flake path for quick repeat builds.
* **Build Flags**:
  * Checkbox for `--impure` (when configurations depend on non-hermetic system state).
  * Checkbox for `--show-trace` (for detailed error diagnosis).
  * Checkbox for `--recreate-lock-file` or input updating (`--update-input <name>`).
* **Command Execution**:
  * When flake mode is enabled, invoke:
    ```bash
    nixos-rebuild [boot|switch] --flake <uri>#<host> [flags]
    ```

### 1.3 Git Commit & Metadata Display
* **Commit SHA in Generation List**:
  * Read generation provenance from `<generation>/etc/nixos/flake-info` or Git commit metadata embedded in system labels.
  * Add an optional column or expanded detail panel showing the Git commit hash and commit message corresponding to each generation.

---

## Milestone 2: Multi-Profile & Standalone Home-Manager Support

### 2.1 Profile Switcher Interface
* **Dual Profile View (System vs. User)**:
  * Add a tabbed interface or toggle hotkey (e.g. <kbd>F2</kbd> or <kbd>Tab</kbd>) to switch views between:
    * **System Generations**: `/nix/var/nix/profiles/system`
    * **Home-Manager Generations**: `/nix/var/nix/profiles/per-user/$SUDO_USER/home-manager` (or `~/.local/state/nix/profiles/home-manager`)
    * **Root Profile**: `/nix/var/nix/profiles/default`
* **User Context Awareness**:
  * Since the tool runs via `sudo`, inspect `$SUDO_USER` to discover the active non-root user and locate their user profiles automatically.

### 2.2 Home-Manager Generation Operations
* **Listing & Metadata**:
  * Parse `home-manager-*-link` symlinks, modification times, and closure paths.
* **Switching Generations**:
  * Activate past user generations without rebooting or rebuilding:
    ```bash
    su - $SUDO_USER -c "<generation-path>/activate"
    ```
* **Purging User Generations**:
  * Mark and delete old Home-Manager generations to reclaim user-level disk space:
    ```bash
    home-manager expire-generations "-<N> days"
    # or direct profile deletion
    nix-env -p /nix/var/nix/profiles/per-user/$USER/home-manager --delete-generations <ID>
    ```

---

## Milestone 3: Generation Diffing & Inspection

### 3.1 Package & Closure Diffing
* **Two-Generation Selection**:
  * Allow marking two generations and triggering a diff view with hotkey <kbd>D</kbd>.
* **Diff Providers**:
  * **`nvd diff`** (Preferred): Provides a clean, colored terminal summary of packages added, removed, upgraded, or downgraded.
  * **`nix store diff-closures`** (Built-in fallback): Compares store paths and versions without third-party dependencies.
* **Diff Modal / Viewport**:
  * Dedicated scrollable viewport presenting categorized package changes (e.g. `[ADDED]`, `[REMOVED]`, `[UPGRADED]`).

### 3.2 Quick Rollback Action
* **One-Key Rollback (<kbd>B</kbd> / <kbd>F9</kbd>)**:
  * Instantly switch profile and activate the immediate previous generation ($N-1$), providing a fast recovery shortcut if an update causes issues.

---

## Milestone 4: Advanced Store & Boot Diagnostics

### 4.1 Enhanced Status Badges & Bootloader State
* **Multi-Badge System**:
  * `BOOTED`: Currently active running system (`/run/current-system`).
  * `PROFILE DEFAULT`: Current target of `/nix/var/nix/profiles/system`.
  * `BOOT DEFAULT`: Generation configured as the default entry in systemd-boot / GRUB bootloader configuration.
* **Alerts**:
  * Visual indicator when booted system and profile default diverge (e.g., when a build was created with `boot` but the system hasn't rebooted yet).

### 4.2 Disk Usage Breakdown & Retention Policies
* **Shared vs. Unique Closure Analysis**:
  * Identify which generations retain the most unique store paths (i.e. generations that, if purged, would free the most disk space).
* **Batch Selection Shortcuts**:
  * "Mark all older than 30 days".
  * "Keep only last $N$ generations" (e.g., keep last 5 generations, mark the rest).
* **Store Health Indicators**:
  * Warning badges when `/nix/store` free space falls below configurable thresholds (e.g. < 15 GB).

---

## Technical Architecture Considerations

1. **Privilege Separation**:
   * System operations require root (`sudo`).
   * Standalone Home-Manager operations should execute as `$SUDO_USER` to avoid creating root-owned files in user home directories.
2. **Graceful Tool Fallbacks**:
   * Auxiliary tools like `nvd` or `git` may not be installed in all environments. The application should detect tool availability and fall back gracefully to standard `nix` CLI commands.
3. **Responsive UI**:
   * All diffing, flake evaluations, and background generation queries must run asynchronously via Bubble Tea commands (`tea.Cmd`) and channels, keeping the interface fluid and responsive.

