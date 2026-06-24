# hyprsunset-controller

`hyprsunset-controller` is a terminal user interface for managing `hyprsunset` on Hyprland.

Instead of only editing a static config file, it provides an interactive way to control temperature, gamma, and runtime behavior from the terminal. The goal is to make `hyprsunset` feel more like a controllable desktop utility while still fitting naturally into keyboard-driven Hyprland workflows.

It is designed for users who want quick manual control, reusable profiles, and a simple TUI experience without needing a full graphical settings app.

## Build & run

```bash
go build -o hyprsunset-controller .
./hyprsunset-controller        # or: go run .
```

Keys: `tab` switch panel · `space` toggle enabled · `←/→` adjust · `↓/↑` select · `backspace` clear field · `n` new profile · `d` delete profile · `s` save · `q` quit.

The Enabled checkbox launches `hyprsunset` through `uwsm app` and stops it by terminating the `hyprsunset` process. Temperature, gamma, and the other fields are written to the config with `s`; the running `hyprsunset` daemon reads the config and applies the profile matching the current time.

## Layout

- `tui.go` — Bubble Tea Model/Update, editable fields, key bindings.
- `view.go` — terminal rendering.
- `config.go` — profile parsing and persistence (`~/.config/hypr/hyprsunset.conf`).
- `hyprsunset.go` — daemon control (`SetHyprsunsetRunning`, `IsHyprsunsetRunning`) via `uwsm app`, `pgrep`, and `pkill`.

Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea). Other TUI approaches were prototyped on `tui/*` branches.
