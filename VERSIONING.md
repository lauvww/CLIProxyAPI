# Versioning

This fork uses a simple repository-driven version policy.

## Source of truth

- Backend release version: `VERSION`
- Backend build metadata: compile-time ldflags (`main.Version`, `main.Commit`, `main.BuildDate`)
- Frontend management UI version: `VERSION` in the frontend repository, with `package.json` kept in sync

## Release format

- Stable release tag: `vX.Y.Z`
- Optional pre-release tag: `vX.Y.Z-rc.N`
- The coordinated major line for the current fork is `2.x`
- Backend and management UI major versions should move together unless a release note explicitly documents an exception

## Update check behavior

- The management UI `Check for updates` button calls `/v0/management/latest-version`
- The backend first checks GitHub `releases/latest`
- If no release is published yet, it falls back to the repository `VERSION` file

## Local build workflow

1. Update `VERSION`
2. Commit the version bump
3. Build locally with `scripts/build-local.ps1`
4. Publish the resulting `cli-proxy-api.exe`
5. When ready for a public release, create a Git tag such as `v2.0.0` and publish a GitHub Release

## Notes

- Do not use `BuildDate` in source code as a manual timestamp. Treat it as build output only.
- Local ad-hoc `go build` commands may show `BuildDate=local-build`. Use the build script when you need a stamped binary.
- `scripts/build-local.ps1` can target a custom output path via `-OutputPath` and prefers the workspace-local Go toolchain when `..\.tools\go1.26.2\go\bin\go.exe` is available.
- Binaries produced by `scripts/build-local.ps1` default to the embedded model catalog (`--local-model` behavior) so local desktop builds are not silently overwritten by remote model sources.
- The persisted config source for remote model refresh is `model-catalog.remote-refresh-enabled`.
- When the process is not force-overridden by CLI flags, toggling that setting in the management panel takes effect immediately without restarting CPA.
- `--remote-model` force-enables remote model refresh for the current process and overrides config.
- `--local-model` force-disables remote model refresh for the current process and overrides config.
- Major releases should update backend `VERSION`, frontend `VERSION`, frontend `package.json`, and the default source version strings used by local builds together.
