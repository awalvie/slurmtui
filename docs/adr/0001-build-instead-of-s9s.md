# Build slurmtui instead of using s9s

s9s (github.com/jontk/s9s) is an MIT, k9s-style Slurm TUI over slurmrestd that already lists nodes and jobs and drains and resumes nodes. It cannot sign a token from the cluster key. We build our own tool instead of extending s9s or wrapping it with a token script, so we own a small codebase that follows our rules and shows the fields we need.
