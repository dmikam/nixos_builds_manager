# NixOS Generation Manager & Store Optimizer TUI

A terminal application written in Go using `bubbletea` and `lipgloss` to safely manage NixOS generations and optimize the Nix store.

## Build

You can compile the static binary using Docker Compose without needing Go installed locally:

```bash
docker compose up build