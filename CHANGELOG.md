# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.1] - 2026-06-26

### Changed
- Renamed the project from `hstui` to `hyprsunset-tui`: Go module, binary, and the
  desktop notification application name (`notify-send -a`) now use `hyprsunset-tui`.
- The AUR package was renamed from `hstui` to `hyprsunset-tui`; the new package
  declares `replaces`/`conflicts`/`provides=('hstui')` so existing installs migrate
  automatically on upgrade.

## [1.1.0] - 2026-06-25

### Added
- Continuous integration workflow (`.github/workflows/ci.yml`) running build, vet, and lint checks.
- AUR packaging under `packaging/aur/` (`PKGBUILD`, `.SRCINFO`) for installing hyprsunset-tui as an Arch package.
- Linker/build flags in the `PKGBUILD` to strip symbols and produce a smaller binary.

### Changed
- Writing the config now preserves comments and other settings placed *between*
  `profile { ... }` blocks. Previously the whole span from the first block to the
  last was replaced as one unit, dropping any interleaved lines.
- Dependency check no longer requires `hyprctl`, and now requires `notify-send`
  (since desktop notifications are used). Checked binaries are `hyprsunset`,
  `uwsm`, and `notify-send`.
- `CheckDependencies` and `Notify` moved from `dependencies.go` into `main.go`;
  `dependencies.go` removed (no behavior change beyond the dependency list above).

### Fixed
- Corrected a mislabeled status error: a failed hyprsunset running-state check now
  reports `hyprsunset:` instead of `uwsm:`.

## [1.0.0] - 2026-06-24

- Initial release.

[1.1.1]: https://github.com/mplaczek99/hyprsunset-tui/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/mplaczek99/hyprsunset-tui/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/mplaczek99/hyprsunset-tui/releases/tag/v1.0.0
