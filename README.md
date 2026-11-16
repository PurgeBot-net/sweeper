# template

GitHub template repository for [PurgeBot-net](https://github.com/PurgeBot-net) — provides consistent CI/CD, linting, and repo hygiene across all services.

## Usage

When creating a new repository under `PurgeBot-net`, select this as the template. Then:

1. Open `.github/workflows/ci.yml` and replace `CHANGE_ME` with the repo name (e.g. `interactions`).
2. For **library repos**: swap `build-service.yml` → `build-library.yml` and delete the `with: package_name:` block — `build-library.yml` takes no inputs.

That's it. The reusable workflows live in [PurgeBot-net/workflows](https://github.com/PurgeBot-net/workflows).

## What you get

- **CI/CD** — vet + test on push to `main`, pull requests, and manual runs; on push to `main` or a manual run: build + push Docker image to ghcr.io tagged as `latest` and the commit SHA
- **Auto-labelling** — PRs are labelled by type (bug, feature, improvement, etc.)
- **`.editorconfig`** — consistent formatting across editors
- **`.gitignore`** — Go-specific, ignores `.env` but keeps `.env.example`
- **PR template** — type checklist, testing notes, and `go.mod` reminder
- **CODEOWNERS** — auto-requests review from `@PurgeBot-net/devs`
- **SECURITY.md** — responsible disclosure policy
- **LICENSE** — MIT
