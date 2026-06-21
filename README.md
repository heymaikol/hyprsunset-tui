# hyprsunset-controller

`hyprsunset-controller` is a terminal user interface for managing `hyprsunset` on Hyprland.

Instead of only editing a static config file, it provides an interactive way to control temperature, gamma, presets, and runtime behavior from the terminal. The goal is to make `hyprsunset` feel more like a controllable desktop utility while still fitting naturally into keyboard-driven Hyprland workflows.

It is designed for users who want quick manual control, reusable profiles, and a simple TUI experience without needing a full graphical settings app.

## Build & run

```bash
go build -o hyprsunset-controller .
./hyprsunset-controller        # or: go run .
```

Keys: `t/T` or `←/→` temperature · `g/G` or `↓/↑` gamma · `1/2/3` presets · `a`/`enter` apply · `i` identity/reset · `q` quit.

Applying needs the `hyprsunset` daemon running (`hyprsunset &`). The UI works without it; `apply` just reports an error.

## Layout

- `main.go` — Bubble Tea TUI (Model/Update/View), presets, key bindings.
- `hyprsunset.go` — runtime IPC seam (`SetTemperature`, `SetGamma`, `Identity`); shells out to `hyprctl` today, swap for a socket client later.

Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea). Other TUI approaches were prototyped on `tui/*` branches.
