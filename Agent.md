# Agent.md

This file defines mandatory rules for preparing and publishing this Traefik plugin so it can be indexed by [plugins.traefik.io](https://plugins.traefik.io/plugins).

## Goal

Ensure this repository always satisfies Traefik plugin catalog requirements before any public release.

## Mandatory Publishing Rules

1. Repository hosting
- Must be a **public** repository on **GitHub.com**.
- Must **not** be a fork.
- Must include GitHub topic: `traefik-plugin`.

2. Required files at repository root
- `.traefik.yml` (valid metadata + valid `testData`).
- `go.mod` (valid module path).
- Plugin source file(s) implementing Traefik plugin entry points.
- `README.md`.
- `LICENSE`.

3. Module/import consistency
- `go.mod` `module` value must match the GitHub repository path exactly.
- `.traefik.yml` `import` value must match `go.mod` `module` exactly.
- Code package and exported plugin functions must be valid for Traefik plugin loading.

4. Middleware plugin interface requirements (Go)
- Must define:
  - `type Config struct { ... }`
  - `func CreateConfig() *Config`
  - `func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error)`
- Config JSON tags must match documented runtime configuration fields.

5. Versioning and release
- Must have at least one semantic version Git tag (example: `v0.1.0`).
- Tag must be pushed to origin.
- Catalog indexing depends on versioned sources available through Go module proxy.

6. Dependencies
- If external Go dependencies are used, vendor them (`vendor/`) when required by Traefik plugin expectations.
- Keep dependency surface minimal.

## Required `.traefik.yml` Fields

Minimum required fields:
- `displayName`
- `type` (`middleware` or `provider`)
- `import`
- `summary`
- `testData` (mandatory for catalog validation)

Optional fields:
- `runtime`, `wasmPath`, `compatibility`, `iconPath`, `bannerPath`

## Pre-Publish Checklist (Must Pass)

- [ ] `go test ./...` passes.
- [ ] `go.mod` module path equals GitHub repo path.
- [ ] `.traefik.yml` `import` equals `go.mod` module path.
- [ ] `.traefik.yml` contains valid `testData`.
- [ ] Repo is public, not forked.
- [ ] GitHub topic `traefik-plugin` is set.
- [ ] Release tag created and pushed (for example `v0.1.0`).
- [ ] No secrets/API keys committed in repository files.

## Standard Release Procedure

```bash
git init
git add .
git commit -m "Initial Traefik plugin"
git branch -M main
git remote add origin git@github.com:<org-or-user>/<repo>.git
git push -u origin main
git tag v0.1.0
git push origin v0.1.0
```

Then on GitHub:
- Set repository visibility to public.
- Add topic `traefik-plugin`.

## Local Testing Guidance (Before First Tag)

Use Traefik local plugin mode:
- Place plugin in `plugins-local/src/github.com/...`
- Configure:
  - `experimental.localPlugins.<plugin-name>.moduleName`
- Reference plugin in dynamic middleware config with realistic test configuration.

## Common Blocking Errors

- `.traefik.yml` missing or invalid.
- Missing `testData` in `.traefik.yml`.
- `import` and `go.mod module` mismatch.
- Missing Git tag.
- Missing GitHub topic `traefik-plugin`.
- Private repository or non-GitHub hosting.
- Forked repository.
- Unvendored dependencies when required.

## Security Rules

- Never hardcode secrets.
- Prefer environment variable references for sensitive values.
- Validate plugin configuration strictly.

## Notes

- Catalog discovery is automatic and not immediate.
- If indexing fails, Traefik may open a GitHub issue on the repository with the failure reason.
