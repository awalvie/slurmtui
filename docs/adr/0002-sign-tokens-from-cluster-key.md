# Sign tokens from the Cluster key on the client

Clusters with `disable_token_creation` give users no way to get a Provided token, so slurmtui signs its own HS256 Token from the Cluster key. The key can sign as any user, root included, so it is treated as a root password.

- The key comes only from a file, never from an env var or flag value. slurmtui opens it (following symlinks, for agenix and sops-nix), then checks the open file: regular, owned by the current user, `mode & 077 == 0`, not empty. The bytes are used as they are, as slurmctld does.
- A Context names either a key file or a token env var, never both.
- slurmtui signs a fresh Token for every request, with `sun` equal to the user header and `exp` 5 minutes ahead. `iat` is sent to match `scontrol token`, but slurmctld does not check it. A fresh token per request removes renewal state. slurmctld checks `exp` with no leeway, so a laptop clock more than 5 minutes slow fails every call; that error says to check the clock. A fast clock does no harm, because slurmctld ignores `iat`.
- Rotating the cluster key breaks every user of a Signed token at once.
