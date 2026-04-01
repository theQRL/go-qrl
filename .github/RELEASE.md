# Release Process

## Prerequisites

### GitHub Secrets

The following secrets must be configured in **Settings > Secrets and variables > Actions**:

| Secret | Required | Description |
|---|---|---|
| `GPG_KEY` | Optional | Base64-encoded GPG private key (fingerprint: `28580FDD99DA68203ABCE4205EA04E4E0FAC11B6`). If not set, binaries are released without GPG signatures. |
| `DOCKERHUB_USERNAME` | Yes | Docker Hub username for pushing images to `qrledger/go-qrl`. |
| `DOCKERHUB_TOKEN` | Yes | Docker Hub personal access token (Read/Write). Generate at Docker Hub > Account Settings > Security. |

`GITHUB_TOKEN` is provided automatically by GitHub Actions.

### Version Update

Before tagging a release, update the version constants in `version/version.go`:

```go
const (
    Major = 0
    Minor = 3
    Patch = 0
    Meta  = "stable"
)
```

Commit the version change before creating the tag.

## Creating a Release

### 1. Tag the release

```bash
git tag v0.3.0
git push origin v0.3.0
```

Pushing the tag automatically triggers the release workflow.

### 2. What happens automatically

The GitHub Actions workflow (`.github/workflows/release.yml`) runs the following jobs:

1. **create-release** - Creates a draft GitHub release with an empty body.
2. **publish-linux-amd64** - Builds `gqrl` for Linux x86_64 and uploads the binary + sha256 checksum (+ GPG signature if `GPG_KEY` is set).
3. **publish-linux-arm64** - Cross-compiles `gqrl` for Linux ARM64 and uploads artifacts.
4. **publish-darwin-amd64** - Cross-compiles `gqrl` for macOS x86_64 and uploads artifacts.
5. **build-and-deploy-docker-images** - Builds and pushes multi-arch Docker images (`linux/amd64`, `linux/arm64`) to Docker Hub (`qrledger/go-qrl`).

Jobs 2-5 run in parallel after the release is created.

### 3. Publish the release

The release is created as a **draft**. After all jobs complete:

1. Go to the [Releases page](https://github.com/theQRL/go-qrl/releases).
2. Review the draft and verify all artifacts are attached.
3. Add release notes.
4. Click **Publish release**.

## Release Artifacts

For each platform, the following files are uploaded:

| File | Description |
|---|---|
| `gqrl-<tag>-<platform>` | The `gqrl` binary |
| `gqrl-<tag>-<platform>.sha256` | SHA-256 checksum |
| `gqrl-<tag>-<platform>.sig` | GPG detached signature (only if `GPG_KEY` is configured) |

Platforms: `linux-amd64`, `linux-arm64`, `darwin-amd64`

## Re-running a Failed Release

If the workflow fails after the draft release was created:

1. Delete the draft release from the [Releases page](https://github.com/theQRL/go-qrl/releases).
2. Delete and recreate the tag:
   ```bash
   git tag -d v0.3.0
   git push origin :v0.3.0
   git tag v0.3.0
   git push origin v0.3.0
   ```

This triggers the workflow again. Note that Docker Hub image tags are overwritten on re-push.
