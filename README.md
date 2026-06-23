# hyprsunset-controller

`hyprsunset-controller` is a terminal user interface for managing `hyprsunset` on Hyprland.

Instead of only editing a static config file, it provides an interactive way to control temperature, gamma, and runtime behavior from the terminal. The goal is to make `hyprsunset` feel more like a controllable desktop utility while still fitting naturally into keyboard-driven Hyprland workflows.

It is designed for users who want quick manual control, reusable profiles, and a simple TUI experience without needing a full graphical settings app.

## Build & run

```bash
go build -o hyprsunset-controller .
./hyprsunset-controller        # or: go run .
```

Keys: `tab` switch panel · `space` toggle enabled · `←/→` adjust · `↓/↑` select · `enter` apply · `s` save · `q` quit.

The Enabled checkbox launches `hyprsunset` through `uwsm app` and stops it by terminating the `hyprsunset` process. Applying temperature and gamma still uses `hyprctl hyprsunset ...`, so `hyprsunset` must be running for changes to apply.

## Layout

- `tui.go` — Bubble Tea TUI (Model/Update/View), editable fields, key bindings.
- `hyprsunset.go` — runtime IPC seam (`SetTemperature`, `SetGamma`); shells out to `hyprctl` today, swap for a socket client later.

Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea). Other TUI approaches were prototyped on `tui/*` branches.
