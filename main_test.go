package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClamp(t *testing.T) {
	if clamp(500, tempMin, tempMax) != tempMin {
		t.Fatal("below min not clamped")
	}
	if clamp(99999, tempMin, tempMax) != tempMax {
		t.Fatal("above max not clamped")
	}
	if clamp(4000, tempMin, tempMax) != 4000 {
		t.Fatal("in-range changed")
	}
}

func TestPresetKeysMapToIndex(t *testing.T) {
	for i, key := range []byte{'1', '2', '3'} {
		if int(key-'1') != i {
			t.Fatalf("preset key %c maps to wrong index", key)
		}
	}
	if presets[0].name != "Day" || presets[2].temp != 3000 {
		t.Fatal("preset table wrong")
	}
}

func TestCheckDependencies(t *testing.T) {
	t.Run("all dependencies present", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprsunset", "#!/bin/sh\nexit 0\n")
		writeExecutable(t, binDir, "hyprctl", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		if err := CheckDependencies(); err != nil {
			t.Fatalf("CheckDependencies() error = %v", err)
		}
	})

	t.Run("missing hyprsunset", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprctl", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		err := CheckDependencies()
		if err == nil {
			t.Fatal("CheckDependencies() error = nil, want missing hyprsunset error")
		}
		if !strings.Contains(err.Error(), "hyprsunset") {
			t.Fatalf("CheckDependencies() error = %q, want hyprsunset", err)
		}
	})

	t.Run("missing hyprctl", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "hyprsunset", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", binDir)

		err := CheckDependencies()
		if err == nil {
			t.Fatal("CheckDependencies() error = nil, want missing hyprctl error")
		}
		if !strings.Contains(err.Error(), "hyprctl") {
			t.Fatalf("CheckDependencies() error = %q, want hyprctl", err)
		}
	})
}

func TestNotify(t *testing.T) {
	t.Run("missing notify-send", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		err := Notify("title", "body")
		if err == nil {
			t.Fatal("Notify() error = nil, want missing notify-send error")
		}
		if !strings.Contains(err.Error(), "notify-send") {
			t.Fatalf("Notify() error = %q, want notify-send", err)
		}
	})

	t.Run("sends expected arguments", func(t *testing.T) {
		binDir := t.TempDir()
		argsFile := filepath.Join(t.TempDir(), "notify-args")
		t.Setenv("PATH", binDir)
		t.Setenv("NOTIFY_ARGS_FILE", argsFile)

		writeExecutable(t, binDir, "notify-send", "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$NOTIFY_ARGS_FILE\"\n")

		if err := Notify("title", "body"); err != nil {
			t.Fatalf("Notify() error = %v", err)
		}

		gotBytes, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("read notify args: %v", err)
		}

		want := "-a\nhyprsunset-controller\n-u\ncritical\ntitle\nbody\n"
		if got := string(gotBytes); got != want {
			t.Fatalf("notify-send args = %q, want %q", got, want)
		}
	})

	t.Run("returns command error", func(t *testing.T) {
		binDir := t.TempDir()
		writeExecutable(t, binDir, "notify-send", "#!/bin/sh\nexit 42\n")
		t.Setenv("PATH", binDir)

		if err := Notify("title", "body"); err == nil {
			t.Fatal("Notify() error = nil, want command error")
		}
	})
}

func writeExecutable(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", name, err)
	}
}
