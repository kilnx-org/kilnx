# Releasing Kilnx

This document describes the steps to cut a new release of Kilnx.

## Prerequisites

- Write access to the repository
- The `main` branch must be green (all CI checks passing)

## Release checklist

1. **Ensure CI is green**
   - Open the [Actions tab](https://github.com/kilnx-org/kilnx/actions) and verify the latest run on `main` passed.

2. **Update the changelog**
   - Open `CHANGELOG.md`.
   - Move items from `[Unreleased]` to a new section: `## [X.Y.Z] - YYYY-MM-DD`.
   - Add the new compare link at the bottom of the file.

3. **Commit the changelog**
   ```bash
   git add CHANGELOG.md
   git commit -m "chore(release): prepare vX.Y.Z"
   ```

4. **Create and push the tag**
   ```bash
   git tag -a vX.Y.Z -m "Release vX.Y.Z"
   git push origin main
   git push origin vX.Y.Z
   ```

5. **Verify the release pipeline**
   - The `Release` workflow triggers automatically on the tag push.
   - It builds binaries via GoReleaser, publishes a GitHub Release, updates the Homebrew tap, and pushes the Docker image to `ghcr.io/kilnx-org/kilnx`.

6. **Validate the artifacts**
   - Check the [Releases page](https://github.com/kilnx-org/kilnx/releases) for the new version.
   - Pull the Docker image: `docker pull ghcr.io/kilnx-org/kilnx:X.Y.Z`
   - Run `kilnx version` from the release binary and confirm it prints `kilnx X.Y.Z`.

## Versioning policy

Kilnx follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html):

- **Patch** (0.0.X): backwards-compatible bug fixes and security patches
- **Minor** (0.X.0): new features, backwards-compatible
- **Major** (X.0.0): breaking language or runtime changes

## Emergency fixes

If a critical bug is found in a released version:

1. Create a fix branch from the release tag if `main` has diverged with unrelated changes.
2. Apply the fix, open a PR, and merge.
3. Cut a new patch release following the checklist above.
