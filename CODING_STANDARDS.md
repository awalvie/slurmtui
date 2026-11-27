# Coding standards

The review step reads this file. `scripts/check` enforces the mechanical rules (format, vet, lint, tests). This file holds the rules that need judgement.

## Fail loud

- Fail early, with a message that names the bad value and where it came from.
- Catch an error only to retry or to add context. Every other error goes up to the caller.
- No fallbacks: no default value that hides a missing or bad input, no `_` for an error, no "best effort" branch. If Slurm sends data that slurmtui cannot read, show an error that names the field and the value.

## Comments

- Comments say why, not what. Exported names get standard Go doc comments.

## Domain language

- Use the terms in `GLOSSARY.md` for types, functions and test names.
- Slurm behaviour follows `docs/slurm-domain.md`. A change that contradicts an ADR must say so.

## Secrets

- A Token or the Cluster key never goes into output, logs, error text or test fixtures. Errors name the URL, never headers.
