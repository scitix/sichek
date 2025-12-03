This directory contains GitHub Actions workflows related to releases.

Workflow file: `goreleaser.yml`

Purpose
- Use `goreleaser` to build and publish when pushing a tag (starting with `v`, e.g. `v1.2.3`) or when manually triggered:
  - Debian (`deb`) and RPM (`rpm`) packages.
  - Container images pushed to the GitHub Container Registry (GHCR).

Triggers
- Push tag: tags matching the `v*` pattern (for example `git tag v1.0.0 && git push origin v1.0.0`).
- Manual: open the Actions page, select the workflow and click `Run workflow`.

Required permissions and secrets
- `GITHUB_TOKEN`: automatically provided by GitHub and used by the workflow's `goreleaser` to publish to GHCR and create releases (no manual setup required).

Optional configuration (for other registries)
- If you also want to publish to Docker Hub or another private registry, add the corresponding credentials in the repository Settings → Secrets (for example `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN`) and update `goreleaser.yml` to enable the related login steps.

Recommendations
- Before creating a release, make sure the `goreleaser` config in the repository (for example `.goreleaser.yaml`) is correctly set up for packaging and images.
- For a quick local check (no remote publish), run `goreleaser release --snapshot --rm-dist`.

Troubleshooting
- If image push fails, verify that `GITHUB_TOKEN` has `packages:write` permission (the workflow sets this permission).
- To customize package names or repositories, edit `.goreleaser.yaml` and review `goreleaser` output in the Actions logs for debugging.

If you want this information copied to the repository root `README.md` or to `docs`, tell me and I will do it.
