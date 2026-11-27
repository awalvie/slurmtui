# Testing

Tests drive slurmtui through `app.Main`, as a user does, and check the screen and the requests that the fake slurmrestd receives (ADR 0003). Two other seams: hostlist expansion, and the redaction of a Token and the Cluster key (`internal/auth/redact_test.go`), because slurmtui never prints them.

## The harness

The harness is in `internal/app/harness_test.go`. The fake slurmrestd is in `internal/fakeslurm`. The fake clock is in `internal/clock`.

- Wait for screen text with `waitFor` or `waitUntil`. Never sleep for a fixed time.
- Use `advance` to move the fake clock. It waits until slurmtui has armed its two steady timers (refresh and the 1-second footer tick). A new timer changes `steadyTimers`.
- Find rows with `rowByName` and read cells by column header. `table()` reads the focused pane (the frame whose title name is blue), or the first frame when no frame has the focus. `pane(title)` reads one pane by its title (`Nodes` or `Jobs`), and `rowByNameIn(title, name)` finds a row in it. Each finds the frame by its title (` ┌ Nodes` or ` ┌ Jobs`) and the column header row under it, in column 3. A column is a run of text between gaps of 2 or more blank cells in all rows. The sort arrow is not part of the column name. In the jobs view, `rowByName` and `selected` use the ID column. `waitNames` waits until the rows show in a given order. `columnNames` gives the column names in screen order. `rightAligned` tells if each cell and the header of a column end at the right edge of the column.
- The default sorts put rows in severity or priority order, so the first row is not always the first node or job of the fixture. To put the cursor on a row, use `moveTo`, not a count of `j` presses.
- `selected` finds the row with `▌` in the lead cell (column 2) and reverse video in column 3, in either pane.
- slurmtui starts on the Overview, so both frames are on the screen. To test a full-screen view, call `fullScreen("jobs", "Jobs")`: it sends the command and waits until only that frame shows. Do not wait for a title only, because the Overview already shows it.
- A STATE cell is the glyph and the label. `stateLabel` returns only the label.
- Dialogs, help and the setup form: `dialogBox(title)` finds the frame (x, y, width, height) and `dialogRows` reads its inner rows. `waitButton(label)` waits until that button has the focus (reverse). `styleOf(text)` gives the style of the first cell of a text on the screen. `offsetOf(text)` gives its column and row. `checkMarginAround(box, bodyTop)` checks the blank 1-cell margin of a pop-up and that the pop-up stays in the body. `checkFadedUnder(box)` checks that all other cells above the bottom line are faint and have no colour.
- Read the style of a cell with `cellAt(x, y).Style`. Check a colour with `isANSI(fg, index)`. The default colour is a nil `Fg`. Use ANSI index literals in tests, not the constants of `internal/ui`.
- `startSized` starts slurmtui in a terminal of a given size. The default is 160x50.
- `run.output` holds all bytes that slurmtui wrote. `TestOutputUsesOnlyANSIColours` fails on a 256-colour or truecolor sequence.
- `start` sets `SLURMTUI_CONFIG_DIR` to an empty temporary directory, unless the test gives `SLURMTUI_CONFIG_DIR` or `XDG_CONFIG_HOME`. Thus a test never reads the real config file. Use `configDir` to write a `config.yaml`.
- A Provided token must be a JWT, because slurmtui reads `sun` and `exp` from it. Use `providedToken(sun, exp)` or `aliceToken`. The signature is fake, because slurmtui does not verify a Provided token.
- A Signed token test uses `fakeClusterKey`, a fake key, and writes it to a 0400 file with `keyFileIn`. `startSigned` makes a `keyFile` Context and calls `RequireSignedTokens` on the fake slurmrestd. Then the fake slurmrestd verifies each token with the key and the clock, as slurmctld does, and rejects a bad one with HTTP 511 and error 1007. On `/ping` it sends HTTP 200 with `responding: false`, as slurmrestd does. `SignedClaims` returns the claims of each accepted token.
- The fake slurmrestd takes `POST /node/{name}` and applies DRAIN or RESUME to its nodes. `RefuseNodeUpdates(status, reply)` makes it send a recorded reply instead, for example `testdata/slurm-26.05/node-drain-non-admin-reply.json`. Read a recorded reply with `recordedReply`.
- The fake slurmrestd takes `DELETE /job/{id}` (`1234` or `1234_5`) and sets the matching jobs to CANCELLED. An unknown ID gets error 2017 in `status[]`, as Slurm sends it. `RefuseJobCancels(reply)` sends a recorded reply with HTTP 200 instead, for example `testdata/slurm-26.05/job-cancel-not-owner-reply.json`. Each recorded `Request` has the `Query` string, so a test can check that slurmtui sends no query parameters.
- The fake slurmrestd takes `GET /partitions` and sends `Cluster.Partitions`. With no partitions in the Cluster, it sends `DebugPartition` (MaxTime UNLIMITED, as in the Compose cluster), so a job with no limit of its own shows `UNLIMITED`.
- `SetNodeField(index, field, value)` replaces one JSON field of a node in the reply, for example a string where Slurm sends a number. Call `ExpectOffSpec` for that field too.
- A timing test multiplies its limit by `raceSlowdown` (5 with `-race`, else 1), because `just check` runs the tests with the race detector.
- `columnNamesIn(title)` gives the column names of one pane. `waitEmptyRow(title, text)` waits for the faint empty row of a pane.
- `mustFrameTop(title)` gives the row of a frame title and fails the test if the frame is not there. `frameRows(title)` gives its top and bottom rows.
- `requestsWhere(cluster, keep)` gives the requests that the fake slurmrestd got and that `keep` accepts. `waitOneRequest(cluster, what, keep)` waits for exactly one such request.
- To give the fake slurmrestd the same clock as slurmtui, make a `clock.Fake` and pass it to `startWithClock`.
- `fakeslurm.StartTLS` starts the fake slurmrestd on https with a self-signed certificate. `CertPEM` holds that certificate, for a `caFile`.
- The fake slurmrestd checks each reply against the OpenAPI spec (`internal/openapi`). A mismatch fails the test and names the field. Fix the fixture, not the check. To send an incorrect reply on purpose, call `ExpectOffSpec` with the field path. See `docs/slurmrestd-v0.0.45.md`.

## Integration tests against real slurmrestd

Integration tests run slurmtui against the Compose cluster (`dev/compose/`). They have the build tag `integration`, so `just check` does not run them. `just check` lints them, so they must compile.

- Run: `just cluster-test` (add `-v` or `-run` as needed). The cluster must be up. In a new cluster, run `just cluster-wait` first.
- The recipe sets `SLURMTUI_TEST_URL`, `SLURMTUI_TEST_USER` (`root`) and `SLURM_JWT`. If one is missing, the test fails.
- CI runs `cluster-up`, `cluster-wait`, `cluster-seed` and `cluster-test`. It also makes the OpenAPI spec again and fails if the spec in the repo is different.
- Each test makes the state that it needs and removes it in `t.Cleanup`. Do not use the seed state: people also use the cluster by hand.

To add an integration test:

1. Make a file `internal/app/integration_<area>_test.go` with `//go:build integration` on line 1.
2. Start the test name with `TestRealCluster`. `just cluster-test` runs only these tests.
3. Call `connectRealCluster(t)`, make the state with the helpers in `realslurm_test.go`, then call `cluster.start()`. Its fake clock starts at the real time, because the times come from the cluster. Read the screen with the usual harness helpers.
4. To change cluster state, add a helper to `realslurm_test.go` that calls `c.call(method, path, body, reply)` and registers its undo with `t.Cleanup`. `drainUntilCleanup` and `submitUntilCleanup` are the examples. When slurmtui itself changes the state, register the undo first: `resumeAtCleanupIfDrained` resumes the node only if the test left it drained.

The seed jobs use all free CPUs, so a job that a test submits stays pending. Submit jobs with `hold: true` to get a known reason (`JobHeldUser`). To check running jobs, read them with `runningPlainJobs` and do not change them.

## Traps (each one cost a debug cycle)

- **Input:** Bubble Tea reads from an `io.Pipe`, not from the emulator. Bubble Tea cannot cancel a read on a reader that is not a file, so its input goroutine races with `Close` on the emulator. The harness copies the emulator's replies into the pipe and writes keys to the pipe.
- **Line endings:** the harness turns `\n` into `\r\n` before the emulator, like a TTY in cooked mode. Bubble Tea does not use raw mode when its output is not a TTY, so it relies on the terminal for this.
- **Colour:** the harness sets `CLICOLOR_FORCE=1`. Without it, Bubble Tea removes all colour from output that is not a TTY.
- **Locking:** `SafeEmulator.CellAt` returns a pointer into the screen buffer. The harness holds its own lock (`termMu`) across each write and across each full read of cells.
- **Escape:** a lone `\x1b` on the pipe never reaches slurmtui as `esc`, and it blocks the input after it (even ctrl-c). Send `escKey` (the kitty form `\x1b[27u`).
- **Timers:** slurmtui timers use the injected `clock.Clock`, never `tea.Tick` or `time.After`. `testing/synctest` does not work here, because goroutines that wait on httptest sockets are never "durably blocked".
