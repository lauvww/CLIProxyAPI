# Versioning

This fork uses a simple repository-driven version policy.

## Source of truth

- Backend release version: `VERSION`
- Backend build metadata: compile-time ldflags (`main.Version`, `main.Commit`, `main.BuildDate`)
- Frontend management UI version: `VERSION` in the frontend repository, with `package.json` kept in sync

## Release format

- Stable release tag: `vX.Y.Z`
- Optional pre-release tag: `vX.Y.Z-rc.N`

## Update check behavior

- The management UI `Check for updates` button calls `/v0/management/latest-version`
- The backend first checks GitHub `releases/latest`
- If no release is published yet, it falls back to the repository `VERSION` file

## Local build workflow

1. Update `VERSION`
2. Commit the version bump
3. Build locally with `scripts/build-local.ps1`
4. Publish the resulting `cli-proxy-api.exe`
5. When ready for a public release, create a Git tag such as `v1.0.0` and publish a GitHub Release

## Notes

- Do not use `BuildDate` in source code as a manual timestamp. Treat it as build output only.
- Local ad-hoc `go build` commands may show `BuildDate=local-build`. Use the build script when you need a stamped binary.
