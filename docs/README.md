# Docs

| Doc | Read before you |
|---|---|
| [`ux.md`](ux.md) | Change layout, keys, dialogs, the setup form or config |
| [`slurm-domain.md`](slurm-domain.md) | Show or send a Slurm field (state rules, reasons, times, hostlists) |
| [`slurmrestd-v0.0.45.md`](slurmrestd-v0.0.45.md) | Read or send JSON (shapes, the OpenAPI spec check) |
| [`testing.md`](testing.md) | Change a test or the test harness |
| [`../GLOSSARY.md`](../GLOSSARY.md) | Name a thing (Context, Token, Cluster key, Drain, Scope) |
| [`../CODING_STANDARDS.md`](../CODING_STANDARDS.md) | Review code |
| [`agents/`](agents/) | Use the issue tracker, triage labels and domain docs as an agent |

## ADRs

| ADR | Decision |
|---|---|
| [0001](adr/0001-build-instead-of-s9s.md) | Build slurmtui, do not extend s9s |
| [0002](adr/0002-sign-tokens-from-cluster-key.md) | Sign tokens from the Cluster key, with key file rules |
| [0003](adr/0003-test-through-a-terminal-emulator.md) | Test through the real entry point and a terminal emulator |
| [0004](adr/0004-setup-form-writes-config.md) | The setup form writes the config file |

## Other places

- `testdata/slurm-26.05/`: real replies recorded from Slurm, and the OpenAPI spec.
- `dev/compose/`: the Docker Slurm cluster.
