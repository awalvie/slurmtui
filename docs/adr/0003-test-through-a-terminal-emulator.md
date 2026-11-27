# Test through the real entry point and a terminal emulator

Tests start slurmtui through its real entry point (args, env map, stdin, stdout, clock) against an in-process fake slurmrestd. They read the screen through a `charmbracelet/x/vt` emulator and check the requests the fake received. We do not use `x/exp/teatest/v2`: it takes a `tea.Model`, not the entry point, and greps raw output, which Bubble Tea v2's cell-diff renderer splits with cursor moves.

- The clock is a required input. Refresh and flash timers use it, not `tea.Tick`, which calls `time.NewTimer` directly. `testing/synctest` cannot replace it, because goroutines blocked on httptest sockets never count as durably blocked.
- Hostlist expansion is the one second seam. Its edge cases do not show through the screen.
- The fake server checks every response it serves against slurmrestd's v0.0.45 OpenAPI spec, generated with `slurmrestd --generate-openapi-spec` and committed. Fixtures are written by hand.
