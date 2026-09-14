# Project Structure & Implementation Details: NixOS Builds Manager

## 1. Overview

**NixOS Builds Manager** (`nixos_builds_manager`) is a full-screen Terminal User Interface (TUI) utility inspired by classic terminal tools such as Midnight Commander (MC) and `htop`. It is written in Go and designed for NixOS administrators and power users to inspect, switch, build, analyze, and purge system generations, as well as optimize disk usage in the `/nix/store`.

Because the tool executes system-level operations on NixOS profiles and invokes administrative commands (`nixos-rebuild`, `nix-env`, `nix-store`, and `nix-collect-garbage`), it enforces a root permission check at startup (`os.Geteuid() == 0`).

---

## 2. Directory Tree

```
nixos_builds_manager/
├── .gitignore               # Ignores compiled binaries (bin/, bin/**)
├── Dockerfile               # Multi-stage Docker build configuration
├── docker-compose.yml       # Docker Compose build service for compiling static binary
├── go.mod                   # Go module definition (Go 1.22) & direct/indirect dependencies
├── go.sum                   # Checksums for Go dependencies
├── main.go                  # Application entrypoint & root permission verification
├── README.md                # Project introduction and build instructions
├── DOCS/
│   └── structure.md         # Architecture and implementation details documentation
├── bin/
│   └── nixos_builds_manager # Static ELF binary output (created upon compilation)
├── nix/
│   └── nix.go               # NixOS inspection, profile parsing, and CLI command execution
└── ui/
    ├── model.go             # Bubble Tea Model state, message types, and initialization
    ├── update.go            # State transition handlers, keyboard/mouse input & streaming
    ├── view.go              # Lip Gloss layout rendering, table view, and modal dialogs
    └── styles/
        └── styles.go        # Lip Gloss styles, color schemes, borders, and badges
```

---

## 3. Technology Stack & Core Dependencies

| Component / Library | Version | Role in Project |
| :--- | :--- | :--- |
| **Go** | 1.22 | Primary implementation language |
| **Bubble Tea** (`github.com/charmbracelet/bubbletea`) | `v0.25.0` | TUI runtime based on the Elm architecture (`Model`, `Init`, `Update`, `View`) |
| **Bubbles** (`github.com/charmbracelet/bubbles`) | `v0.18.0` | Reusable TUI widgets: `spinner`, `textinput`, and `viewport` |
| **Lip Gloss** (`github.com/charmbracelet/lipgloss`) | `v0.9.1` | Terminal layout, styling, colors, borders, and padding |
| **Docker Compose / Alpine Go** | `1.22-alpine` | Isolated reproducible environment for building static Linux binaries |

---

## 4. Architecture & Component Deep Dive

The project follows a clean modular structure separating system interactions (`nix` package) from terminal presentation and interaction (`ui` package), tied together by `main.go`.

```
                    +-----------------------------+
                    |           main.go           |
                    |  (Checks root, runs TUI)   |
                    +--------------+--------------+
                                   |
                                   v
                    +-----------------------------+
                    |          ui.Model           |
                    |    (Bubble Tea Runtime)     |
                    +--------------+--------------+
                     /             |             \
                    v              v              v
     +------------------+ +-----------------+ +-------------------+
     |   ui/update.go   | |   ui/view.go    | | ui/styles/styles  |
     | (Event Handling, | | (View Layout &  | | (Lip Gloss Styles |
     |  State Machine,  | |  Modal Windows) | |  & Color Palette) |
     |  Streaming Cmds) | +-----------------+ +-------------------+
     +--------+---------+
              | (Calls system operations)
              v
     +------------------+
     |   nix/nix.go     |
     | (Profiles, Links,|
     |  Nix CLI Exec,   |
     |  Log Streaming)  |
     +------------------+
              | (Executes commands / reads sysfs)
              v
     +---------------------------------------------------------+
     |                      NixOS System                       |
     |  - /nix/var/nix/profiles/system-*-link                 |
     |  - /run/current-system                                  |
     |  - nixos-rebuild, nix-env, nix-store, nix-collect-garbage|
     +---------------------------------------------------------+
```

---

### 4.1 Entrypoint (`main.go`)

- **Root Privilege Enforcement**:
  - Executes `os.Geteuid() != 0`.
  - If executed as a non-root user, it displays an error and exits with code 1, directing the user to run via `sudo`.
- **Bubble Tea Initialization**:
  - Initializes `ui.InitialModel()`.
  - Configures terminal alternate screen buffer using `tea.WithAltScreen()`, ensuring the terminal screen is preserved upon exit.
  - Starts the event loop with `p.Run()`.

---

### 4.2 NixOS Integration Layer (`nix/nix.go`)

This package is responsible for discovering NixOS generations, querying store information, and running mutating operations while streaming standard output and standard error back to the UI.

#### 4.2.1 Data Models
- **`Generation` Struct**:
  - `ID int`: Generation sequence number extracted from symlink names or bootloader configs.
  - `Profile string`: Name of profile (`system` for default, or named profile like `EnableXRDP`).
  - `Path string`: Full symlink path or boot entry path (e.g. `/boot/loader/entries/nixos-generation-13.conf`).
  - `Target string`: Resolved Nix store path (e.g. `/nix/store/...-nixos-system-...`).
  - `Timestamp time.Time`: Generation creation timestamp based on symlink or `.conf` `ModTime()`.
  - `Label string`: Human-readable version label extracted from `<storePath>/nixos-version` or bootloader metadata.
  - `Kernel string`: Kernel version string extracted from `<storePath>/kernel` or `<storePath>/kernel-modules`.
  - `IsCurrent bool`: `true` if target path matches `/run/current-system`.
  - `IsOrphan bool`: `true` if generation entry only exists in `/boot/loader/entries/` and has no active profile symlink.
  - `Marked bool`: User selection toggle for batch purge operations.
- **`EnvironmentInfo` Struct & `ConfigType`**:
  - `IsFlakeSupported bool`: Checks whether Nix Flakes feature is enabled in `/etc/nix/nix.conf`.
  - `ConfigType ConfigType`: Mode of active system configuration (`Flake`, `Classic`, or `None`).
  - `ConfigPath string`: Absolute path to active configuration file (e.g. `/etc/nixos/flake.nix` or `/etc/nixos/configuration.nix`).
  - `FlakeURI string`: Directory URI of the active flake.
  - `FlakeHost string`: Target system hostname (from `/etc/hostname` or `os.Hostname()`).
  - `GitRev string` & `GitDirty bool`: Short Git commit hash and working tree status if flake is in a git repository.

#### 4.2.2 System Queries
- **`DetectEnvironment() EnvironmentInfo`**:
  - Automatically identifies whether the system uses Flakes or traditional Channels.
  - Inspects `NIXOS_FLAKE` environment variable, current working directory, Git roots, `/etc/nixos/flake.nix`, user config directories (`~/.config/nixos`, `~/dotfiles`, etc.), and `/etc/nixos/configuration.nix`.
  - Inspects `/etc/nix/nix.conf` for `experimental-features = ... flakes ...`.
- **`GetNixStoreFreeSpace() string`**:
  - Executes `syscall.Statfs("/nix/store", &stat)`.
  - Calculates free disk space via `stat.Bavail * stat.Bsize` and formats it as `Free: X.XX GB`.
- **`GetCurrentBootedPath() (string, error)`**:
  - Calls `os.Readlink("/run/current-system")` to determine the currently active system generation store path.
- **`ListGenerations() ([]Generation, error)`**:
  - Multi-source generation discovery across:
    1. Default system profiles: `/nix/var/nix/profiles/system-*-link`
    2. Custom named profiles: `/nix/var/nix/profiles/system-profiles/*-link` (created via `nixos-rebuild -p <name>`)
    3. Bootloader entries: `/boot/loader/entries/*.conf` (detects orphaned entries whose profile links were previously deleted).
  - Deduplicates by composite key `(Profile, ID)`.
  - Inspects targets to extract labels and kernel versions.
  - Sorts generations in descending order by timestamp and ID (newest first).
  - Flags the entry matching `GetCurrentBootedPath()` as `IsCurrent = true`.
- **`extractSystemLabel(storePath string) string`**:
  - Reads `<storePath>/nixos-version`. Returns `"NixOS System"` on failure.
- **`extractKernelVersion(storePath string) string`**:
  - Reads symlink target of `<storePath>/kernel`.
  - Matches parent directory against `linux-(.+)$`.
  - Falls back to checking `<storePath>/kernel-modules`.
- **`AnalyzeStorePathSize(storePath string) (string, error)`**:
  - Executes `nix path-info -S <storePath>` to measure the complete closure size of a generation.
  - Parses bytes and formats them into B, KB, MB, GB, TB, etc.
  - Automatically falls back to `du -sh <storePath>` if `nix path-info` fails.

#### 4.2.3 Mutating Operations & Command Streaming
All mutating operations run through `runCmdStream(...)`:
- **`RebuildSystemStream(label, isProfile, switchBuild, outChan)`**:
- **`ListExistingProfiles(gens []Generation) []string`**:
  - Scans `/nix/var/nix/profiles/system-profiles/*-link` and existing generations to return a deduplicated, sorted list of named profiles.
- **`RebuildSystemStream(profile, label, switchBuild, outChan)`**:
  - Invokes `nixos-rebuild boot` or `nixos-rebuild switch`.
  - Sanitizes labels using `SanitizeLabel` (replacing non-alphanumeric chars with `_`).
  - Applies `-p <label>` for profiles or sets environment variable `NIXOS_LABEL=<label>`.
  - Sanitizes profile and label using `SanitizeLabel` (replacing non-alphanumeric chars with `_`).
  - If a profile is specified (and not `"system"`), applies `-p <profile>`.
  - If a label is specified, sets environment variable `NIXOS_LABEL=<label>`.
  - Profile and label can be combined in the same build.
- **`SwitchToGenerationStream(gen, outChan)`**:
  - Switches profile pointer via `nix-env -p <profilePath> --switch-generation <ID>`.
  - Executes system activation script: `<gen.Target>/bin/switch-to-configuration switch` (activates configuration without rebuilding).
  - Disallows switching directly to orphaned entries lacking profile links.
- **`PurgeGenerationsStream(gens, outChan)`**:
  - Handles orphaned bootloader entries by removing their `.conf` files directly from `/boot/loader/entries/`.
  - For active generations, handles active profile pointer auto-switching to avoid `cannot delete current version of profile` errors.
  - Deletes specified generations via `nix-env -p <profilePath> --delete-generations <ID>`.
  - Cleans up empty profile pointers in `/nix/var/nix/profiles/system-profiles/` if all generations in a named profile were purged.
  - Updates the bootloader menu via `/nix/var/nix/profiles/system/bin/switch-to-configuration boot`.
  - Collects garbage via `nix-collect-garbage`.
- **`OptimizeStoreStream(outChan)`**:
  - Hard-links identical files across the Nix store using `nix-store --optimise`.
- **`CollectGarbageStream(outChan)`**:
  - Removes unreferenced store paths using `nix-collect-garbage`.
- **`runCmdStream(env, outChan, name, args...)`**:
  - Sets working directory to `os.TempDir()`.
  - Combines `stdout` and `stderr` using `io.MultiReader`.
  - Uses `bufio.Scanner` to push each line into `outChan chan<- string` in real time.
  - Cleans up temporary `/tmp/result` build symlinks upon completion.

---

### 4.3 UI State Management (`ui/model.go`)

`ui.Model` holds the entire state of the TUI application:

- **Dimensions & Navigation**:
  - `Width`, `Height`: Window dimensions updated dynamically via `tea.WindowSizeMsg`.
  - `Cursor int`: Index of the highlighted generation in the table.
  - `Focus FocusArea`: Can be `FocusList` (main table) or `FocusFooter` (bottom buttons).
  - `ActiveButton int`: Index of currently selected button in footer (0 to 7).
- **Data & System State**:
  - `Generations []nix.Generation`: Loaded generations list.
  - `FreeSpace string`: Disk space string displayed in the header/footer.
- **Modal Dialog Flags & States**:
  - `AboutModal bool`: Application information modal.
  - `BuildModal bool`: Create new build modal. Includes `LabelInput textinput.Model`, `IsProfile bool`, `SwitchBuild bool`, and `BuildModalOption int` (focus state between input, checkboxes, and buttons).
  - `BuildModal bool`: Create new build modal. Includes `ProfileInput textinput.Model`, `LabelInput textinput.Model`, `KnownProfiles []string`, `SelectedProfileIdx int`, `SwitchBuild bool`, and `BuildModalOption int` (focus state between profile input, label input, switch checkbox, and buttons).
  - `AnalyzeModal bool`: Displays closure size details (`AnalyzeGen`, `AnalyzeResult`).
  - `ConfirmModal bool`: Purge confirmation dialog (`PurgeModalOption int`).
  - `ConfirmOptimizeModal bool`: Store optimization confirmation (`OptimizeModalOption int`).
  - `ConfirmGCModal bool`: Garbage collection confirmation (`GCModalOption int`).
  - `ConfirmQuitModal bool`: Exit confirmation (`QuitModalOption int`).
  - `SwitchModal bool`: Generation activation confirmation (`SwitchTargetGen`, `SwitchModalOption`).
- **Execution & Log Viewport**:
  - `IsLoading bool`, `LoadingMsg string`: Controls full-screen spinner state.
  - `Spinner spinner.Model`: Charm spinner widget (`spinner.Dot`).
  - `ShowLog bool`: Toggles display of the operation output log viewport.
  - `LogData string`: Accumulated output buffer.
  - `Viewport viewport.Model`: Scrollable viewport for operation logs.

#### Message Types
- `GenerationsLoadedMsg []nix.Generation`: Dispatched when generations are read from disk.
- `StreamOutputMsg string`: Dispatched whenever a background command outputs a line.
- `OperationCompletedMsg { Err error }`: Dispatched when an asynchronous command finishes.

---

### 4.4 Event Handling & Concurrency (`ui/update.go`)

The update cycle implements reactive event handling and asynchronous command orchestration without blocking the TUI thread.

#### 4.4.1 Asynchronous Command Streaming Pattern
Operations that run long CLI commands (`nixos-rebuild`, `nix-store`, etc.) use a channel-based streaming pattern:
1. A buffered channel `globalStreamChan = make(chan string, 100)` is created.
2. `tea.Batch` triggers two tasks:
   - A background goroutine executing the `nix` package streaming function, writing lines to `globalStreamChan` and closing it when done, then returning `OperationCompletedMsg`.
   - `waitForStreamCmd()`, which awaits the next line from `globalStreamChan`.
3. When `StreamOutputMsg` is received in `Update`:
   - Line is appended to `m.LogData`.
   - `m.Viewport.SetContent(m.LogData)` is called and scrolled to bottom (`GotoBottom()`).
   - `waitForStreamCmd()` is re-invoked to read subsequent lines until channel closure.
4. When `OperationCompletedMsg` arrives, `IsLoading` is set to `false`, success/error is appended to log, and the log view remains open for user review.

#### 4.4.2 Keybindings & Navigation Reference

##### Main Screen Navigation
| Key | Action | Context / Conditions |
| :--- | :--- | :--- |
| `Up` / `k` | Move cursor up | `Focus == FocusList` |
| `Down` / `j` | Move cursor down | `Focus == FocusList` |
| `Space` | Mark / unmark generation for purge | Cannot mark `CURRENT` generation |
| `Enter` | Open Switch Modal (if on row) or activate button | On `FocusList` (non-current) or `FocusFooter` |
| `Tab` | Toggle focus between List and Footer buttons | Main view |
| `Left` / `Right` | Move active footer button selection | `Focus == FocusFooter` |
| `Mouse Wheel` | Scroll table up / down | `Focus == FocusList` |
| `F1` / `a` | Open **About** dialog | Any main view state |
| `F3` / `n` | Open **New Build** dialog | Any main view state |
| `F4` / `s` | Open **Storage Path Analyzer** | Any main view state |
| `F5` / `r` | **Refresh** generations list | Re-scans `/nix/var/nix/profiles/` |
| `F6` / `o` | Open **Optimize Store** confirmation | Any main view state |
| `F7` / `c` | Open **Clean GC** confirmation | Any main view state |
| `F8` / `p` | Open **Purge** confirmation | Enabled only when $\ge 1$ generations marked |
| `F10` / `q` / `Ctrl+C` | Open **Quit** confirmation | Any main view state |

##### Modal Screen Controls
- **Confirmation Modals (Switch, Purge, Optimize, GC, Quit)**:
  - `Left` / `Right` / `h` / `l` / `Tab`: Toggle between "Yes" and "Cancel" buttons.
  - `y` / `Y`: Immediately confirm action.
  - `n` / `N` / `Esc`: Dismiss modal.
  - `Enter`: Trigger the highlighted button.
- **Build Modal**:
  - `Tab` / `Shift+Tab` / `Up` / `Down`: Cycle through form fields (Label Input $\to$ Profile Checkbox $\to$ Switch Checkbox $\to$ Start Button $\to$ Cancel Button).
  - `Space`: Toggle checkbox state.
  - `Enter`: Trigger Start Build or Cancel depending on selected button.
  - `Esc`: Cancel and close modal.
- **Log Viewport Screen**:
  - `Up` / `Down` / `PageUp` / `PageDown`: Scroll through command output log.
  - `Esc` / `Enter` / `q`: Close log view (when operation is completed) and trigger generation reload.

---

### 4.5 Visual Design & Layout (`ui/view.go` & `ui/styles/styles.go`)

The layout follows a 4-part vertical partition using Lip Gloss (`Header`, `SubHeader`, `Panel`, `Footer`):

```
+-------------------------------------------------------------------------------+
| NixOS Builds Manager | Active: Gen 42 (24.05.20240501)       Host: myhost | Free: 48.2 GB | <- Header
| [CLASSIC] /etc/nixos/configuration.nix (Flakes: Supported)                    | <- SubHeader
+-------------------------------------------------------------------------------+
| Mark  ID     Build Label            Kernel           Date & Time      Status  |
|-------------------------------------------------------------------------------|
| [ ]   43     custom_rollback        6.6.30           2024-05-02 10:15         |
| [ ]   42     24.05.20240501         6.6.28           2024-05-01 14:22 CURRENT | <- Panel
| [X]   41     old_kernel_test        6.1.85           2024-04-20 09:11 PURGE   |    (Scrollable
| [X]   40     23.11.20240410         6.1.80           2024-04-10 18:00 PURGE   |     Table)
+-------------------------------------------------------------------------------+
| F1 [A]bout F3 [N]ew F4 [S]torage F5 [R]efresh F6 [O]ptimize ... | Free: 48.2 GB | <- Footer
+-------------------------------------------------------------------------------+
```

#### Styling Definitions (`ui/styles/styles.go`)
- **Color Palette**:
  - Background: Black (`#000000`)
  - Primary Accent / Emerald: `#10B981` (active buttons, title, current badge, modal borders)
  - Secondary Accent / Red: `#EF4444` (purge/marked badge)
  - Surface & Panel Borders: `#111827`, `#1F2937`, `#374151`
  - Text: Light Gray `#F9FAFB`, Neutral `#D1D5DB`, Dimmed `#9CA3AF`, Disabled `#4B5563`
- **Dynamic Table Scrolling**:
  - Calculates `visibleRows = panelHeight - 4`.
  - Automatically shifts window (`startIdx = m.Cursor - visibleRows + 1`) to keep the cursor row in view.
  - Dynamically truncates long store path targets (`maxTargetWidth`) with leading ellipsis `...` to adapt to narrower terminal windows.

---

## 5. Build & Deployment

### 5.1 Docker Compose Build Workflow
The project provides a containerized build process that produces a statically linked binary:

- **`Dockerfile`**:
  ```dockerfile
  FROM golang:1.22-alpine AS builder
  WORKDIR /app
  COPY go.mod go.sum ./
  RUN go mod download || true
  COPY . .
  RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/nixos_builds_manager main.go

  FROM scratch
  COPY --from=builder /app/bin/nixos_builds_manager /nixos_builds_manager
  ENTRYPOINT ["/nixos_builds_manager"]
  ```

- **`docker-compose.yml`**:
  ```yaml
  version: '3.8'
  services:
    build:
      image: golang:1.22-alpine
      working_dir: /app
      volumes:
        - .:/app
      command: >
        sh -c "go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o ./bin/nixos_builds_manager main.go"
  ```

### 5.2 Running the Application
To build and execute on a target NixOS machine:
```bash
# 1. Build static binary
docker compose up build

# 2. Run with root privileges
sudo ./bin/nixos_builds_manager
```

---

## 6. Safety Features & Design Considerations

1. **Current Generation Protection**:
   - The booted generation (`/run/current-system`) is marked with the green `CURRENT` badge.
   - The user is prevented from selecting/marking the current generation for purge operations.
2. **Double Confirmation Dialogs**:
   - Destructive operations (purging generations, running garbage collection, optimizing store) and switching system states require confirmation via custom modal dialogs.
3. **Shell Argument Sanitization**:
   - User-supplied build labels are filtered through `SanitizeLabel()` (`[^a-zA-Z0-9._-]` $\to$ `_`), preventing argument injection into `nixos-rebuild`.
4. **Execution Resilience & Fallbacks**:
   - Storage size measurement first attempts `nix path-info -S`; if unavailable, it falls back to `du -sh`.
   - Kernel extraction inspects both `/kernel` and `/kernel-modules`.
   - Switching generations checks for `<gen>/bin/switch-to-configuration` (and falls back to `/nix/var/nix/profiles/system/bin/switch-to-configuration`).
   - Updating bootloader menu invokes `/nix/var/nix/profiles/system/bin/switch-to-configuration boot` to update GRUB / systemd-boot without triggering unwanted builds.
5. **Non-Blocking UI**:
   - All Nix subprocess execution occurs in separate goroutines connected via buffered channels, ensuring that terminal UI rendering and spinners remain responsive regardless of command duration.

