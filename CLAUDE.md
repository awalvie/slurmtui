# slurmtui

A k9s-style terminal UI that shows a Slurm cluster and acts on it through slurmrestd with a JWT.

## Commands

- Dev shell: `nix develop`, or `direnv allow` once for `.envrc`.
- Recipes: `just --list`. `just check` runs format, vet, no comments, lint and tests; the pre-commit hook and CI run the same script. Enable the hook once with `just hooks`.
- Build: `nix build`. After a change to `go.mod`, set `vendorHash` in `flake.nix` to `pkgs.lib.fakeHash`, run `nix build`, then use the hash it reports. Nix reuses the old vendor directory while the hash stays the same, so a stale hash builds from stale modules.
- Run: `just run --url <url>` with `SLURM_JWT` set (the user comes from the token), or `just run --context <name>` with a config file.
- Real Slurm 26.05 cluster in Docker (3 nodes, slurmrestd on `127.0.0.1:6820`): `just cluster-up`, `just cluster-wait`, `just cluster-seed`, `just cluster-run`, `just cluster-down`. `just cluster-test` runs the integration tests (build tag `integration`) against it. Config is in `dev/compose/`. Real replies recorded from Slurm are in `testdata/slurm-26.05/`. `just openapi-spec` makes the OpenAPI spec there again.

## Where to look

- Work to do: the spec and tickets in `.scratch/slurmtui-v1/` (local only, not in git). Read the ticket and its blockers before you start.
- Before you render or send a Slurm field: `docs/slurm-domain.md` (behaviour) and `docs/slurmrestd-v0.0.45.md` (JSON shapes).
- Before you change layout, keys, dialogs or config: `docs/ux.md`.
- Before you change a test or the test harness: `docs/testing.md`.
- Terms: `GLOSSARY.md`. Decisions: `docs/adr/`.
- Review rules: `CODING_STANDARDS.md`.

## Rules

- Commit only when asked.
- Use Conventional Commits: `type(scope): summary`.
- After a change, update the docs it touches. Leave other sections alone.

## Writing

For docs, comments, commit messages and replies:

- Use ASD-STE100 Simplified Technical English (the `simple-english` skill).
- Use Orwell's rules: cut every word you can, use the short plain word, use the active voice, no jargon or stock phrases when an everyday word works.

## Agent skills

### Issue tracker

Issues live as local markdown files under `.scratch/<feature>/` (not in git). See `docs/agents/issue-tracker.md` (local only).

### Triage labels

Default labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
