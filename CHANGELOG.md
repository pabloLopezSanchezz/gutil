# Changelog

All notable changes follow semantic versioning.

## [v0.1.2] - 2026-07-29

### Fixed

- `gutil conflict --abort` now clears a stale gUtil resolving workflow when a merge was already aborted outside gUtil.
- `gutil conflict --continue` now explains how to clear that stale workflow safely.

### Tests

- Added unit and end-to-end regression coverage for stale workflow cleanup after a manual Git merge abort.

## [v0.1.1] - 2026-07-15

### Fixed

- Prevented `gutil conflict --abort` from aborting manual merges that were not started by gUtil.
- Corrected macOS/Linux README examples for `GUTIL_VERSION` and `GUTIL_INSTALL_DIR` when installing through `curl | sh`.

### Tests

- Added regression coverage for manual merges, stale gUtil state, missing branches, dirty working trees, wrong-branch continuation, outside-repository execution, and mismatched abort state.

## [v0.1.0] - 2026-07-15

### Added

- Cross-platform `gutil` executable.
- Conflict preparation, status, and abort commands.
- Safe branch synchronization through `origin`.
- Conditional Visual Studio Code launch.
- Staged conflict continuation with a custom commit message and safe push-only retry.
- Dated conflict-resolution branches for protected source branches.
- macOS, Linux, and Windows installers.
