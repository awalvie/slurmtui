# The setup form writes the config file

When slurmtui cannot make a complete Context from the flags, environment and config file, it opens a setup form in the UI. It does not exit with an error. The form tests the connection, then adds the Context to `config.yaml`, so that the next start needs no input. This replaces the earlier rule "slurmtui never writes the config file", because users expect the first run to set itself up, as in sofka.

- slurmtui saves only a URL, a CA file path, a user, and a key file path or an env var name. It never saves a Token or a key.
- A pasted Provided token is used for that session only. A Context that uses one is not saved.
- slurmtui adds a Context and keeps the rest of the file as it is, comments included. It never changes or removes an existing Context.
- A Context that is in the config file does not open the form. If it is not complete (for example its `tokenEnv` is not set), slurmtui stops with an error that names the missing value. The form cannot save a Context whose name exists, so it cannot fix that Context.
- A config file that exists but is incorrect (bad YAML, an unknown field, unsafe permissions) still stops slurmtui with an error. The form is only for missing settings, not for broken ones.
