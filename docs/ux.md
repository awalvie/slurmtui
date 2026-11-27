# UX conventions

slurmtui copies k9s and sofka. Each choice names its source. Slurm semantics are in `slurm-domain.md`.

The prototype on branch `prototype/visual-system`, commit `1944627` (`cmd/proto-visual/`, header style H2 "k9s"), is the reference for this design. Run it with `go run ./cmd/proto-visual` and press `F2` once to get H2. Where this file and the prototype do not agree, this file is correct.

## Layout

The start screen is the Overview. It has four bands, from top to bottom: the header (5 rows), the Nodes pane, the Jobs pane and the bottom line (1 row). Example at 100x32:

```
   Context  prod                        j/k   move         U     resume
   Cluster  lamp                        enter jobs         /     filter
   User     alice                       d     describe     tab   jobs pane
   Slurm    26.05.5                     D     drain        ?     more
   Expires  18:00

 ┌ Nodes 40 · ◆ 2 draining · ◆ 2 drained · ✕ 2 down ─────────────────────────── cpu 56% · mem 54% ┐
 │ NAME  STATE ↑          CPU  MEM(GiB)  GRES  BY     AGE   REASON                                │
 │▌n07   ✕ down*         0/64     0/256        slurm  5m    Not responding                        │
 │ n23   ✕ down*         0/64     0/256        slurm  12m   Not responding                        │
 │ n05   ◆ draining     64/64   250/256   4/4  alice  2h3m  bad DIMM slot 3, ticket INC-20417, w… │
 │ n06   ◆ drained       0/64     0/256   0/4  root   3d4h  GPU 2 fell off the bus (Xid 79); nvi… │
 │ n21   ◆ drained       0/64     0/256        bob    6h    kernel panic after OFED update, wait… │
 │ n30   ◆ draining     48/64   180/256   1/4  alice  40m   nvlink errors on GPU 1 and 3 (ECC un… │
 └────────────────────────────────────────────────────────────────────────────────────────────────┘

 ┌ Jobs 120 · 69 running · 31 pending ────────────────────────────────────────────────────────────┐
 │ ID    PARTITION  NAME     USER   STATE ↑         TIME  TIME_LEFT  NODES  NODELIST(REASON)      │
 │▌4729  cpu        ml-hpo   heidi  ◷ PENDING      5h49m                 1  (AssocGrpCPULimit)    │
 │ 4854  gpu        eval-s…  erin   ◷ PENDING      5h54m                 2  (ReqNodeNotAvail, Un… │
 │ 4738  cpu        md-run   ci     ◷ PENDING      4h58m                 4  (BeginTime)           │
 │ 4730  cpu        render…  grace  ◷ PENDING      1h50m                 4  (ReqNodeNotAvail, Un… │
 │ 4855  bigmem     genome…  frank  ◷ PENDING      1h55m                 1  (AssocGrpGRESRunMinu… │
 │ 4739  bigmem     nwchem…  judy   ◷ PENDING      5h59m                 3  (JobHeldUser)         │
 │ 4731  bigmem     blast-…  frank  ◷ PENDING      2h51m                 3  (QOSMaxCPUPerUserLim… │
 │ 4856  cpu        report   grace  ◷ PENDING      2h56m                 1  (QOSMaxJobsPerUserLi… │
 │ 4740  gpu        lammps…  ivan   ◷ PENDING         1h                 2  (AssocGrpCPULimit)    │
 │ 4732  gpu        gromac…  erin   ◷ PENDING      3h52m                 2  (Dependency)          │
 │ 4857  cpu        backup…  root   ◷ PENDING      3h57m                 1  (BeginTime)           │
 └────────────────────────────────────────────────────────────────────────────────────────────────┘
                                                                                              ● 3s
```

- Header: the Context information at the left, the key hints of the focused pane in the middle, and the logo at the right (k9s). Header gives the rules.
- Nodes pane: on top. Its height comes from its rows, to a maximum of 2/5 of the body (Height).
- Jobs pane: under the Nodes pane. It takes all other rows of the body.
- Bottom line: only the flash and the freshness, at the right. While the user types a filter or a command, the prompt is at the left (Bottom line).
- At 30 rows or more, one blank row is under the header and one blank row is between the two panes. Below 30 rows, there are no blank rows. The 80x24 screen thus has no blank row.
- `:nodes` and `:jobs` show one pane at full body height, with no Overview split (Navigation). Describe and raw JSON also use the full body. Help is a pop-up (Describe, raw JSON and help).

## Visual system

### Colours

slurmtui uses only the default foreground and background of the terminal, and the 16 ANSI colours (0 to 15). It uses no hex colour and no 256-colour index. Thus the terminal theme of the user sets the real colours. The attributes are bold, faint and reverse. slurmtui never paints a background, except with reverse.

Colour is never the only signal. Each state has a glyph and a word. Focus has the `▌` bar and reverse video. A dialog type has a word in its title.

| Role | Colour | Used for |
|---|---|---|
| `text` | default | cell text, info values, dialog text |
| `dim` | default, faint | unfocused frames, info labels, hint labels, counts in titles, ` · ` separators, ended jobs, the TIME of a pending job, `—` for an empty describe value |
| `key` | default, bold | hint keys, column header row, group names in describe and help |
| `focus` | 4 (blue) | the border of the focused frame, the name in its title (bold), the `▌` of the focused pane (bold), Resume dialog, help, describe, raw JSON, setup form |
| `ok` | 2 (green) | running jobs, allocated and mixed nodes, info flash, the `●` of freshness, JSON strings |
| `warn` | 3 (yellow) | drained and draining nodes (bold), Drain dialog, `[read-only]` (bold), warning flash, `○` of stale freshness, JSON numbers and booleans |
| `bad` | 1 (red) | down nodes (bold), failed jobs, Cancel dialog, error flash, field errors, `User root` (bold) |
| `accent` | 6 (cyan) | completing jobs, filter text in a title, the `/` and `:` prompt, describe keys, JSON keys, the `▸` of the setup form, `↓ more` |
| `cursor` | reverse, bold | the cursor row of the focused pane, the focused button |

`internal/ui/colors.go` holds one constant for each role, as an ANSI index. The `logo*` constants colour the can logo. No other file has a colour value.

### State glyphs

The STATE cell is the glyph, 1 space, then the label. Only the STATE cell has the state colour. The other cells of the row are in `text`, or in `dim` for an ended job (k9s dims completed pods).

| Node class | Glyph | Colour | Rule (`state[]`) |
|---|---|---|---|
| down | `✕` | `bad` bold | base DOWN, or flag FAIL or NOT_RESPONDING |
| drain | `◆` | `warn` bold | flag DRAIN, and not down |
| mixed | `◐` | `ok` | base MIXED |
| allocated | `●` | `ok` | base ALLOCATED, or base COMPLETING |
| idle | `○` | `text` | base IDLE, and no flag of the other row |
| other | `◌` | `dim` | all other states: POWERED_DOWN, POWERING_UP, MAINTENANCE, REBOOT_*, RESERVED, UNKNOWN and more |

The first matching row wins. Thus `drain*` on a DOWN node is down, not drain.

| Job class | Glyph | Colour | Rule (`job_state[]`) |
|---|---|---|---|
| pending | `◷` | `text` | base PENDING, no COMPLETING or STAGE_OUT flag |
| running | `▶` | `ok` | base RUNNING or SUSPENDED, no COMPLETING or STAGE_OUT flag |
| completing | `◼` | `accent` | flag COMPLETING or STAGE_OUT |
| completed | `✓` | `dim` | base COMPLETED |
| failed | `✕` | `bad` bold | base FAILED, NODE_FAIL, BOOT_FAIL or OUT_OF_MEMORY |
| stopped | `✕` | `bad` | base CANCELLED, TIMEOUT, PREEMPTED or DEADLINE |

A job is ended when its class is completed, failed or stopped.

### Grid

- All text starts in screen column 3 (0-based): the header, the column header row, the cells, the dialog hints and the prompt. This is the left edge of the grid.
- Frames start in column 1 and end 1 column before the right edge of the screen. Thus each frame is the terminal width minus 2.
- In a table row, column 2 is the lead cell: `▌`, `▸` or a space. The first cell starts in column 3.

### Frames

- All frames use the square border `┌ ┐ └ ┘ ─ │`.
- The title starts after the top-left corner, with 1 space on each side: `┌ Nodes 40 ─…┐`.
- A frame can have a text at the right of the top border: `─ cpu 56% · mem 54% ┐`. If the title and the right text do not fit with 3 border cells between them, the right text is not shown.
- A title that is wider than the frame is cut with `…`. The top border always ends with `┐`.
- One frame has the focus at a time. Its border is in `focus`, and the name in its title is in `focus` bold. The other frames have a `dim` border and a `dim` bold name.

### Frame titles

The title is the name, then facts separated by ` · ` in `dim` (k9s `[count]`, in words).

- Nodes: `Nodes 40 · ◆ 2 draining · ◆ 2 drained · ✕ 2 down`. The number after the name is the count of rows in the pane, in `dim`. The problem counts are for all nodes, in the node class colour, bold. A count of 0 is not shown. Draining and drained use the sinfo label (`slurm-domain.md` §1).
- Nodes right text: `cpu 56% · mem 54%` in `dim`. This is allocation over usable nodes (`slurm-domain.md` §5).
- Jobs: `Jobs 120 · on n02 · 69 running · 31 pending`. The number is the count of rows in the pane. `on n02` is in `text` and shows only when the pane has a node scope. `running` counts the running, suspended and completing rows of the pane: the group that sorts with the running jobs (Jobs view). `pending` counts the pending rows. A count of 0 is not shown.
- A filter adds ` · /<text>` in `accent` at the end of the title.
- Describe: `Describe · n02`, `JSON · 4851`. Help: `Help`.

### Height

- Body: the terminal height minus the header (5 rows), minus the bottom line (1 row), minus the blank row under the header (Layout). The body is at least 7 rows.
- Nodes pane height: `min(N + 3, max(floor(body × 2 / 5), 5))`. N is the count of rows in the Nodes pane, and at least 1. The 3 rows are the two borders and the column header row.
- Jobs pane height: all other rows of the body, minus the blank row between the panes (Layout). At 160x50 with 40 nodes, the body is 43 rows, the Nodes pane is 17 rows and the Jobs pane is 25 rows.
- The heights do not change when the focus moves, when the cursor moves, or when the Jobs pane gets or loses a scope. A pane with fewer rows than its height has blank rows at the bottom.
- When the rows do not fit, the pane shows a window that keeps the cursor row visible (sofka). The column header row is always the first row in the frame.

### Table rows

- Lead cell: `▌` in `focus` bold on the cursor row of the focused pane. `▌` in `dim` on the cursor row of a pane without focus. `▸` in `dim` on the node that scopes the Jobs pane. A space on other rows.
- The cursor row of the focused pane is in reverse and bold, across the full inner width. All its cells, the STATE cell too, take the reversed default colours. The state colour does not show on this row. The glyph and the word still show the state.
- The cursor row of a pane without focus has no reverse.
- Columns are separated by 2 spaces. The last column ends 1 cell before the right border.
- No row is wider than its frame.
- The column header row is in `key`. The sort arrow ` ↑` (default order) or ` ↓` (reversed) follows the name of the sorted column, in the same style.
- An empty pane shows one row in `dim` under the column header row: `no nodes`, `no jobs`, `no jobs on n02`, `no nodes match /x`, `no jobs on n02 match /x`. A row that is too long is cut with `…` and keeps the 1-cell right margin.
- Age format (AGE, TIME, TIME_LEFT, the times in describe and in dialogs), as k9s with hours from 1 hour: under 2 minutes `90s`; under 10 minutes `3m20s`; under 1 hour `40m`; under 8 hours `2h2m` (`1h` when the minutes are 0); under 2 days `6h`; under 8 days `3d4h`; under 2 years `40d`; under 8 years `2y30d`; then `9y`.

### Column widths

The table divides its inner width between the columns (sofka `distribute_column_widths`). The budget is the frame width minus 2 (border), minus 1 (lead cell), minus 1 (right margin), minus 2 for each gap between columns.

Each column has a kind:

- Fixed: always the same width. STATE is Fixed, at the width of the longest glyph and label that the view can show (sinfo labels for nodes, squeue state names for jobs, for example `✕ OUT_OF_MEMORY` at 15 cells). Thus a state change does not move the other columns.
- Fit: the width of its widest cell or header, but not more than its cap. A cell that is longer than the cap is cut.
- Flex: shares the remaining budget by weight.

| View | Fixed | Fit (cap) | Flex (weight) |
|---|---|---|---|
| Nodes | STATE | NAME (20), CPU, MEM(GiB), GRES, BY (12), AGE | REASON (1) |
| Jobs | STATE | ID (no cap), PARTITION (10), USER (12), TIME, TIME_LEFT, NODES | NAME (1), NODELIST(REASON) (3) |

- ID is the width of the widest ID in the rows that the pane shows in its window, or of the header `ID`. It has no other minimum. Thus a long array ID makes ID wide only while that row is on the screen.
- CPU, MEM(GiB), GRES, TIME, TIME_LEFT and NODES are right-aligned. All other columns are left-aligned.

The budget that remains after the Fixed and Fit columns goes to the Flex columns (sofka waterfall):

1. If the share by weight of a Flex column holds its widest cell, the column gets that width. The other Flex columns share the remainder. This step repeats until no other column gets its full width.
2. If all Flex columns got their full width, the surplus goes to them by weight. Thus the table always fills the frame width, and the columns do not pack to the left.
3. If some Flex columns did not get their full width, they share the remainder by weight. The last Flex column also gets the cells that the division by weight leaves. Thus the table fills the frame. No Flex column is narrower than its header.

- A cell that is wider than its column is cut and ends with `…` (U+2026). No space goes before the `…`: `required…`, not `required …`.
- ID is never cut (squeue form, see Jobs view).
- If the budget cannot hold each Fixed and Fit column at its width and each Flex column at its header width, slurmtui removes columns in the order that States gives.

### Header

```
   Context  prod  [read-only]           j/k   move           U     resume       <logo>
   Cluster  lamp                        enter jobs           /     filter
   User     alice                       d     describe       tab   jobs pane
   Slurm    26.05.5                     D     edit reason    ?     more
   Expires  18:00
```

- The header has no frame. It is 5 rows high.
- Info rows: `Context`, `Cluster` (`meta.slurm.cluster`), `User`, `Slurm` (version), `Expires`. The label is in `dim`, padded to 9 cells. The value is in `text`. The Context value is bold. `User root` has the value in `bad` bold.
- `Expires` shows only with a Provided token: the token `exp` in local time, `HH:MM`. With a Signed token, the row is blank.
- In read-only mode, `[read-only]` in `warn` bold follows the Context value, after 2 spaces (sofka).
- Key hints: a grid that starts in one column on every screen: column 40, or 6 cells after the widest info row that the Context can show (with `[read-only]`) when that is further right. Thus the grid does not move when `[read-only]` shows or not. The grid fills column by column (k9s). It has at most one row for each info row (4 rows when `Expires` is not shown), and the columns are balanced: with 6 hints and 5 rows, each column has 3 hints. `? more` is the last hint. Grid columns are 4 spaces apart.
- Each hint is the key in `key`, padded to the widest key of the grid, 1 space, then the label in `dim`, padded to the widest label of the grid. Thus all grid columns have the same width.
- The logo is the pixel soda can (`canLogo` in `internal/ui/logo.go`): 5 rows, 7 cells, in half blocks (`▀ ▄ █`), on rows 1 to 5. It has a solid body, one label band and a tab in the centre of the lid. Each cell has 1 `logo*` colour and no background. The lid, the right edge and the bottom are in `BrightBlack`, so the can also shows on a light background. The right edge of the logo is in the column of the right edge of the frames. The pictures are in `assets/`.
- The grid ends at or before the right edge of the frames. If the grid does not fit, slurmtui hides the grid and the logo. If the logo does not fit with 4 cells after the grid, slurmtui hides only the logo. The info rows always show.

### Key hints

The hints show the keys of the focused pane, in this order. They change with the row under the cursor (contextual hints).

- Nodes pane: `j/k move`, `enter jobs`, `d describe`, then `D edit reason` and `U resume` when the node has the DRAIN flag, `D drain` and `U resume` when the node is down without DRAIN, or `D drain` for all other nodes. Then `/ filter`, or `esc clear filter` while the pane has a filter. Then `tab jobs pane`, `? more`.
- Jobs pane: `j/k move`, `d describe`, `ctrl-d cancel`, then `esc clear filter` while the pane has a filter, `esc all jobs` while it has a node scope, or `/ filter`. Then `tab nodes pane`, `? more`.
- Full-screen `:nodes` and `:jobs`: the same hints, without the `tab` hint.
- Describe and raw JSON: `j/k scroll`, `esc back`, `? more`. Help: `j/k scroll`, `esc back`.
- Dialog: `tab next button`, `enter confirm`, `esc keep`. Drain dialog: `type reason` first.
- Read-only: no `D`, `U` or `ctrl-d` hint.
- `y`, `N`, `S`, `g/G`, `:` commands and `ctrl-r` are only in help.

### Bottom line

- The bottom line is the last screen row. It has no frame.
- At other times than typing, it holds only the flash and the freshness, at the right: the flash, 2 spaces, the freshness, 2 spaces.
- Freshness: `● 3s` while the last refresh worked (`●` in `ok`, the age in `dim`). `○ stale 47s` after a failed refresh (`○` in `warn`, the text in `dim`). `○ no data` in `dim` before the first load completes (sofka `sync_indicator`).
- Flash: info in `ok`, warning in `warn`, error in `bad`. A warning is a key that does nothing in the current place. An error is a refused action or a failed refresh. If the flash does not fit, it is cut with `…`.
- While the user types a filter: `/` in `accent` bold at column 3, the text in `text`, a `█` cursor in `accent`, 2 spaces, then `enter keep · esc undo · filtering Nodes` in `dim`. A command uses `:` in the same form, with `enter run · esc cancel`.
- While a dialog is open, the header is faded (Dialogs). Thus the bottom line shows the dialog hints at column 3, in the hint form, separated by ` · `. If they do not fit, slurmtui removes hints from the end, but keeps `enter confirm` and `esc keep`.

### Dialogs

```
 ┌ Drain n02 on prod (lamp) ────────────────────────────────────┐
 │                                                              │
 │  No new jobs start on n02. Running jobs finish.              │
 │                                                              │
 │  7 jobs run here                                             │
 │    4850     it-k3x9q2        ci     1d0h left                │
 │    4830+1   het-workers      carol  23h48m left              │
 │    4820_3   sweep            dave   23h53m left              │
 │    4798     cfd-solve        bob    18h43m left              │
 │    and 3 more                                                │
 │  Empty by ~Fri 12:00 (latest time limit)                     │
 │                                                              │
 │  ▸ Reason  █                                                 │
 │                                                              │
 │                       Drain      Keep                        │
 │                                                              │
 └──────────────────────────────────────────────────────────────┘
```

- A dialog is a pop-up: a frame in the centre of the body. Its width is half the screen width minus 2, but at least 64 cells, and never wider than the screen minus 2. Its height fits its content, but never more than the body height minus 2.
- Pop-up fade: under each pop-up (dialog, help, setup form), all rows above the bottom line are drawn again in `dim`, with their colours removed. Text and pane borders keep their shape. The bottom line keeps its colours.
- Pop-up margin: a blank margin of 1 cell goes around the frame on all 4 sides. Thus no faded character touches the border. The frame and its margin stay in the body. They never cover the header or the bottom line.
- The title is bold, in the border colour. It names the action, the target, the Context and the cluster: ` Drain n02 on prod (lamp) `. When Slurm sends no cluster name, the title has only the Context: ` Drain n02 on prod `. The colour shows the type: Drain and Edit reason in `warn`, Resume in `focus`, Cancel in `bad`.
- Body rows start 2 cells inside the border. A blank row is above the first body row.
- Drain body: `No new jobs start on n02. Running jobs finish.`, then the jobs on the node:
  - `N jobs run here` in `key`, then one row for each running or completing job whose hostlist holds the node: ID, name, user and time left (`slurm-domain.md` §3, TIME_LEFT). The ID column is 8 cells, the name column 16 cells (cut with `…`), the user column 6 cells. Rows are newest ID first. A job with no limit of its own uses its partition's limit. A job with no limit at all shows `no time limit`. One job shows `1 job runs here`.
  - After 4 jobs, `and N more` in `dim`. When the dialog does not fit in the body, it lists fewer jobs. Thus the field, the buttons and the error row always show.
  - `Empty by ~Fri 12:00 (latest time limit)` in `dim`: the clock time plus the largest time left, as `Mon 15:04`. If one of the jobs has no time limit, the row is `Empty by: unknown (a job has no time limit)`.
  - With no job: `No jobs run on n02.` in `dim`.
- Edit reason body (`D` on a node with the DRAIN flag): `The node stays drained. Only the reason changes.`, the jobs on the node, then the Reason field with the current reason. Title ` Edit reason of n05 on prod (lamp) `. Buttons `Save` / `Keep`.
- Resume body: `n02 takes new jobs again. Its reason is cleared.`, then `Reason: <reason>` and `Set by alice, 2h3m ago` in `dim`. The age uses the AGE format. A blank BY shows `an unknown user`. With no reason: `No reason is set.` in `dim`.
- Cancel body: `Slurm stops job 4851. Its work is lost.` For an array record or a het job, the row that follows names the scope: `This stops all tasks of array 1234.` or `This stops all components of het job 1234.`
- Input field: `▸ ` in `warn`, the label, 2 spaces, the value in `text`, and a `█` cursor in `warn`. Without the focus, `▸` and `█` do not show. The Drain field starts empty. A paste goes into the focused field, with each line break as a space.
- Buttons are on one row, in the centre, 4 spaces apart. A button is its label with 1 space on each side, and no brackets. The focused button is in `cursor`. Other buttons are in `dim` (k9s dialog).
- An error from the dialog (for example `Reason is required`) is on a row under the buttons, in `bad`.
- `esc` does the same as the `Keep` button (k9s).

### Forms

```
 ┌ Connect to Slurm ────────────────────────────────────────────┐
 │                                                              │
 │  ▸ Context name  prod█                                       │
 │    URL           https://slurm.example:6820                  │
 │    CA file                                                   │
 │    User          alice                                       │
 │    Token source  ‹ Key file ›                                │
 │    Key file      ~/.config/slurmtui/keys/prod.key            │
 │                                                              │
 │                   Test and save      Quit                    │
 │                                                              │
 │  Key file: no such file                                      │
 │                                                              │
 └──────────────────────────────────────────────────────────────┘
```

The setup form uses the dialog rules, plus these rules (sofka `draw_plugin_form`):

- The frame is 64 cells wide and in `focus`, with the title in `focus` bold. Only the form and the bottom line show. There is no header and no pane.
- Rows start 2 cells inside the border, as in a dialog.
- The top row of the form does not move. It is where the form without a note or an error is in the centre. A note or an error makes the form grow down.
- The focused field has `▸ ` in `accent`, the label in `text`, and the value with a `█` cursor in `accent`. Other fields have the label in `dim` and the value in `text`.
- Labels are padded to the longest label plus 2 spaces, so all values start in one column.
- A choice field shows `‹ value ›`, with `‹` and `›` in `dim`. `←` and `→` change the value.
- A check error or a test error is on a row under the buttons, in `bad`. It starts with the field name (Setup form).
- The bottom line shows the hints of the form: `tab next · shift-tab previous · ←/→ change · enter select`.

### Describe, raw JSON and help

- Describe and raw JSON use the full body in one frame with a `focus` border. The header and the bottom line stay. Titles: `Describe · n02`, `JSON · n02`, with `Describe` or `JSON` in `focus` bold, ` · ` in `dim` and the name in `text`.
- Describe: one row for each field. The key is in `accent`, padded to 16 cells. The value is in `text`. An empty value is `—` in `dim`. The state value has its glyph and its class colour.
- A group name (`CPU`, `Memory`, `Times`, `Files`) is in `key`, with a blank row above it.
- Raw JSON: keys in `accent`, strings in `ok`, numbers, `true` and `false` in `warn`, `null`, brackets, colons and commas in `dim`. Indent is 2 spaces.
- Help is a pop-up in the centre of the body, with the dialog fade and margin (Dialogs). Its frame is in `focus` with the title `Help`. It is as wide as its content plus 2 cells at the right, but never wider than the screen minus 2. It has one group for each place: `Nodes pane`, `Jobs pane`, `Both panes`, `Dialogs`, `Describe, JSON`, `Commands`. The group name is in `key`, 2 cells inside the border. Each key row is indented 2 more cells: the key in `key`, padded to the widest key of all groups plus 2, then the action in `text`. Thus all groups have the same key column. The groups go into 3 columns, 2 columns or 1 column: as many as the width holds. There is no blank row above the first group, and one blank row above the bottom border. `esc` or `?` goes back to the same view and cursor. The bottom line shows `j/k scroll · esc back`.
- After the key groups, help shows the group `Legend` in `dim`, with items separated by ` · ` and packed into the width of the columns, under the columns:
  - The glyphs: `✕ down or failed · ◆ drain · ◐ mixed · ● allocated · ○ idle · ◌ other`, and `◷ pending · ▶ running · ◼ completing · ✓ completed`.
  - The state suffixes: `* not responding · ~ powered off · # powering up · ! power down pending · % powering down · $ maintenance · @ reboot requested · ^ reboot issued`.
- These views do not wrap. A line that is too long is cut with `…`. `j` and `k` scroll. Help is never taller than the body minus 2. When help has more rows below, its last visible row above the blank row is `↓ more` in `accent`.

## States

- No complete Context (no config file, or flags and env that leave a value out): the setup form opens (see Setup form).
- First load fails: empty panes with the error in the flash, retrying each tick.
- A config file that exists but is incorrect (bad YAML, unknown field, unsafe key file, `http` to a host that is not loopback) stops slurmtui before the UI opens, with a non-zero exit code and a message that names the field.
- Empty pane: `Nodes 0` and the row `no nodes`. Filter with no match: `Nodes 0 · /x` and the row `no nodes match /x`.
- Expired Provided token: refresh stops and the flash holds `SLURM_JWT expired at <time>; get a new token and restart`.
- Data from Slurm that slurmtui cannot parse (bad hostlist, bad GRES string, unknown shape) is an error in the flash naming the field and value, never a guess. A field of the wrong JSON type gives `node "n1": cpus: "sixty-four" is not int`.
- Terminal smaller than 80x24: the screen shows only `terminal too small (80x24 minimum)` in `bad`, on the top row from column 3 (Grid). No frame.
- Narrow terminals: first the logo goes, then the hint grid (Header). Then each pane removes columns in its order, until the budget of Column widths holds:
  - Nodes: BY, AGE, GRES. Then REASON is cut.
  - Jobs: TIME_LEFT, PARTITION, NODES, USER, TIME. Then NAME and NODELIST(REASON) are cut.
- The right text of the Nodes title (`cpu 56% · mem 54%`) goes when it does not fit (Frames).

## Nodes view

The Nodes pane of the Overview and the full-screen `:nodes` view use the same columns: NAME, STATE, CPU, MEM(GiB), GRES, BY, AGE, REASON.

- Default sort: by severity, then natural by name (`n2` before `n10`). The severity order is the node class order of State glyphs: down, drain, mixed, allocated, idle, other.
- STATE: the glyph, then the sinfo long label, lowercase, with one suffix character (`draining`, `drained`, `idle*`, `idle~`). Rules in `slurm-domain.md` §1.
- CPU: `alloc_cpus/cpus`. MEM(GiB): `alloc_memory/real_memory` in GiB (MiB/1024), one decimal below 10. This is allocation, not load.
- GRES: GPUs in use over GPUs configured, for example `1/4`. Blank on a node with no GPU. Rule in `slurm-domain.md` §5.
- BY (`reason_set_by_user`), AGE (`reason_changed_at`, k9s age format `2d3h`), REASON. A reason with a blank BY means slurmrestd could not resolve the user.
- Partitions are not a column. They are in describe.

## Jobs view

The Jobs pane of the Overview and the full-screen `:jobs` view use the same columns: ID, PARTITION, NAME, USER, STATE, TIME, TIME_LEFT, NODES, NODELIST(REASON). These are the squeue default columns plus TIME_LEFT.

- Shows every job `/jobs` returns, ended ones included (k9s shows completed pods). Ended rows are in `dim`, except the STATE cell. Ended jobs drop out after MinJobAge.
- Default sort: pending jobs first, by `priority` from high to low. Then running and completing jobs. Then ended jobs. In each group with equal priority, the newest ID is first (ID descending in natural order). A completing or suspended job sorts with the running jobs.
- ID: squeue form: `1234_[3-100]`, `1234_5`, `1234+0`, `1234`. No merging, no truncation.
- STATE: the glyph, then the squeue rule, flag over base, long name in capitals as Slurm sends it (`RUNNING`, `COMPLETING`).
- TIME: for a running job, elapsed since `start_time`, in k9s age format. For an ended job, `end_time` minus `start_time`, so its TIME does not grow. For a pending job, the wait since `submit_time`, in `dim`.
- TIME_LEFT: the time limit minus TIME for a running, suspended or completing job. `UNLIMITED` when the job has no limit. Blank for pending and ended jobs, and when the partition limit applies. Rule in `slurm-domain.md` §3.
- NODES: `node_count`.
- NODELIST(REASON): squeue rule (`slurm-domain.md` §4): `(reason)` for pending and failed-type jobs, the hostlist otherwise. `state_description` replaces the reason when it is set. A reason of `None` shows a blank cell.
- An unknown `job_state` value is an error in the flash that names the job and the value.

## Describe (`d`)

- Node fields, in groups: Name, State (full `state[]` joined with `+`), Reason, Reason by, Reason at, Partitions. `CPU`: Allocated, Total, Specialized, Load (`cpu_load`/100). `Memory`: Allocated MiB, Total MiB, Free MiB. Then Features, Gres (the `gres` string), Reboot pending (`true` when `state[]` holds REBOOT_REQUESTED). `Times`: Boot time, Last busy.
- Job fields, in groups: ID, Name, User, Account, QOS, State (with `state_description`), Reason, Priority, Dependency, Partition, Nodes, Node count, Requested TRES (`tres_req_str`). `Times`: Submit, Waiting (pending only), Expected start (`start_time` of a pending job), Start, End, Elapsed, Time limit, Time left. `Files`: Exit code, Work dir, StdOut, Command.
- Absolute times in local time with the offset.
- Time limit uses scontrol's form: `D-HH:MM:SS`, `UNLIMITED`, or `Partition_Limit` when the job has no limit of its own. Exit code is `return code:signal`, as in scontrol.
- `y` shows the raw JSON object of the node or job, indented.
- Describe and raw JSON read the row again at each refresh. If the row is gone, they say so.
- `j`/`k`, arrows, `g`/`G` scroll. `esc` goes back to the Overview or the full-screen view, with the focus and the cursor on the same row.

## Navigation

- slurmtui starts on the Overview.
- `tab` (and `shift-tab`) moves the focus between the Nodes pane and the Jobs pane. The Nodes pane has the focus at start. Each pane keeps its own cursor, filter and sort.
- `enter` on a node scopes the Jobs pane to that node, and moves the focus to the Jobs pane with the cursor on the first row. The scope holds each job whose `nodes` hostlist contains the node, ended jobs included (k9s node → pods). The flash says `Jobs shows n02 only · esc shows all jobs`.
- `enter` on a job: describe.
- In a scoped pane, a job hostlist that does not expand is an error in the flash that names the job and the value, for example `job 9: nodes "n[1-": ...`.
- `esc` in a pane: first it clears the filter of the focused pane. If there is no filter, `esc` in the Jobs pane removes the node scope. If there is nothing to clear, the flash shows a warning.
- `:nodes` (`:no`) and `:jobs` (`:j`) show one full-screen pane. `:overview` (`:ov`) goes back to the Overview. `:q` and `ctrl-c` quit.
- In the full-screen `:nodes` view, `enter` on a node opens the full-screen jobs view scoped to that node. In the Jobs pane and in the full-screen `:jobs` view, `o` on a job opens the full-screen nodes view filtered to its hostlist (sofka, spec story 41), with the title `Nodes 2 · of 1234`. `esc` with nothing to clear goes back one step. At the first full-screen view, it goes back to the Overview.
- `d` describe, `y` raw JSON of the response object (Slurm's stand-in for YAML).
- `/` filters the focused pane by regex. `! x` inverts (k9s). The regex matches the row's cells joined by spaces, without regard to case (k9s). The rows change as you type. `enter` keeps the filter. `esc` while typing puts back the filter from before. A bad regex shows no rows and the error in the flash.
- A key that does nothing in the focused pane is a warning in the flash that names the right place, for example `ctrl-d cancels a job · press tab to focus Jobs`, or `x does nothing here · ? lists the keys`. An unknown command is a warning too: `:xyz is not a command · ? lists the commands`.
- `ctrl-r` refreshes now (k9s, sofka).
- `?` help. `esc` or `?` closes it. A ticket that adds a pane key adds one entry to `bindings` in `internal/ui/keymap.go`. The entry gives the key, its pane, its help row, its hint and its action, so the key, the hints and Help come from one table. Read-only mode hides the hints and help rows with `writes`.
- `j`/`k`, arrows, `g` (top), `G` (bottom).
- The cursor stays on the same node name or job ID across refreshes. If that row is gone, the cursor keeps its index, clamped (sofka).

## Sorting

- `shift-n` name, `shift-s` state (k9s). They sort the focused pane. Pressing the same key again reverses.
- Name is NAME in both panes: the node name in natural order, the job name for jobs, then the newest ID first. State is the default order of the pane: severity for nodes, the job groups of Jobs view for jobs.

## Actions

| key | pane | action | dialog |
|---|---|---|---|
| `D` | nodes | drain | `Drain n02 on prod (lamp)`, Reason field focused and required, buttons `Drain` / `Keep` |
| `D` | nodes, node with DRAIN | edit reason | `Edit reason of n05 on prod (lamp)`, Reason field focused, prefilled with the current reason, buttons `Save` / `Keep` |
| `U` | nodes | resume | `Resume n02 on prod (lamp)` with the current reason, its user and age, buttons `Resume` / `Keep`, focus on `Keep` |
| `ctrl-d` | jobs | cancel | `Cancel job 1234 on prod (lamp)`, buttons `Cancel job` / `Keep`, focus on `Keep` |

- Node keys follow sofka (`D`/`U`). k9s uses `r`/`u`. Cancel uses delete's key, since cancel is the job's delete.
- Edit reason sends the same request as Drain. Slurm only updates the reason on a drained node (`slurm-domain.md` §2).
- The dialog fixes its target when it opens. Refresh pauses while a dialog is open (k9s).
- In a dialog: `tab` / `shift-tab` move the focus, `left` / `right` move between buttons, `enter` pushes the focused button (`enter` in the Reason field pushes the action), `esc` is `Keep`. Edit reason opens with the focus in the Reason field, so an accidental `enter` there must cause no change (spec story 63): `enter` while the reason is unchanged sends nothing, closes the dialog and flashes `reason unchanged`.
- An empty Reason is refused in the dialog with `Reason is required`. Slurm itself accepts DRAIN without a reason (`slurm-domain.md` §2), so this copies scontrol.
- An accepted action flashes `drain of n02 accepted`, `reason of n05 updated`, `resume of n02 accepted` or `cancel of job 1234 accepted`, in `ok`. `Keep` flashes `kept n02 as it was`. Action flashes clear after 6 s of clock time. A refresh error stays until a refresh works.
- Help (`?`) lists the action keys.
- Cancel target: `A_T` when `array_task_id` is set; `job_id` with "all tasks of array 1234" in the dialog when `array_task_string` is set; `het_job_id` with "all components of het job 1234" for any het component; `job_id` otherwise. The dialog title names the target that slurmtui sends, for example `Cancel job 1300 on prod (lamp)` for the het component `1300+1`.
- Slurm decides validity. slurmtui does not pre-check permissions or state. `U` on an idle node and `ctrl-d` on an ended job open the dialog, and Slurm's reply goes in the flash. A rejected action puts Slurm's error in the flash in `bad`. A cancel fails when `status[].error.code` is non-zero.
- After an action, refresh at once.

## Refresh

- One refresh in flight at a time. A tick that fires during a refresh is dropped.
- Nodes, jobs and partitions are fetched together each tick. No `update_time`. Partitions give the time limit of a job with no limit of its own (`slurm-domain.md` §3).

## Read-only

- `--readonly`, `--write`, per-context `readOnly`, global `readOnly`. Precedence: flag, then context, then global (k9s). The default is off. `--readonly` and `--write` together are a startup error.
- A Context from the flags or the setup form has no `readOnly` of its own, so the flag or the global setting applies.
- `D`, `U`, `ctrl-d` are unbound and hidden in the hints and in Help. Pressing one in either Overview pane opens no dialog and flashes `read-only mode — action disabled` (sofka). It never suggests `tab` to the other pane.
- As a second guard, the client refuses node updates and job cancels in read-only mode. slurmtui sends only GET requests.

## Setup form

The form opens when slurmtui cannot make a complete Context from the flags and env (ADR 0004). Values from flags and env are filled in. A Context that is in the config file never opens the form: a missing value there (for example its `tokenEnv` is not set) is a startup error that names the value, because the form cannot save a Context with a name that exists.

The form follows the Dialogs and Forms rules in Visual system.

- `tab` / `shift-tab` (or `down` / `up`) move between fields. `enter` on a field moves to the next field. `enter` on `Test and save` starts the test. `enter` on `Quit` exits.
- In a text field, type or paste to add text, `backspace` removes one character, and `ctrl-u` clears the field.
- On `Token source`, `left` / `right` (or `h` / `l`, `space`) change the source. The last field is `Key file:`, `Env var:` (default `SLURM_JWT`) or `Token:`.
- The token source starts on `Env var` when the env var is set, and on `Key file` when it is not.
- The test sends `GET /slurm/v0.0.45/ping` with the Token. A failure shows its error under the form, and the form stays open.
- Each field gets the same checks as the config file (ADR 0002 for the key file, the URL rules in Config). An error starts with the field name, for example `URL: http://slurm.example:6820: http is allowed only to a loopback host`. A Context name that is already in the config file is refused.
- After a successful test with a key file or env var, slurmtui adds the Context to `config.yaml` and opens the Overview. It creates the directory with mode 0700 and the file with mode 0600.
- With Paste, the token field shows only `[hidden, N characters]`. After a successful test, slurmtui opens the Overview and saves nothing. The form says "A pasted token is used for this session only."
- `Quit` or `ctrl-c` exits with code 0.

## Config

```yaml
slurmtui:
  refreshRate: 5
  readOnly: false
  contexts:
    prod:
      url: https://slurm.example:6820
      caFile: ~/.config/slurmtui/prod-ca.pem
      user: alice
      keyFile: keys/prod.key
    lab:
      url: http://127.0.0.1:6820
      tokenEnv: SLURM_JWT_LAB
```

- Path: `$XDG_CONFIG_HOME/slurmtui/config.yaml`, override with `SLURMTUI_CONFIG_DIR`. Contexts live here because Slurm has no kubeconfig. Only the setup form writes the file, and only to add a Context (ADR 0004).
- A context has exactly one of `keyFile` or `tokenEnv`.
- `keyFile`: `~` and `~/` expand (`~user` is an error), and relative paths resolve against the config dir. Checks in ADR 0002. `user` is required. The header shows no `Expires:`, because each request gets a new token. When slurmctld refuses the token, the flash says to check the clock and the key.
- `tokenEnv`: names the env var holding a Provided token. slurmtui reads `sun` and `exp` from the token and does not verify the signature. The user is `sun`, so `user` is optional; a `user` or `--user` that differs is a startup error. A value that is not a JWT with `sun` and `exp` is a startup error that names the variable, never the value.
- `url`: `https` is verified against system roots plus `caFile`. There is no skip-verify. `http` is allowed only to a loopback host (`127.0.0.1`, `::1`, `localhost`, for an SSH tunnel). Other schemes and other `http` hosts are a startup error. Redirects are refused: a 3xx reply is an error that names the target, and the token headers do not go to it.
- `caFile`: PEM file with one or more CA certificates, added to the system roots. `~` expanded, relative paths resolve against the config dir. A file that slurmtui cannot read, or that has no PEM certificate, is a startup error.
- Context selection: `--context` selects a context. Without it, `--url` makes a context from the flags, named after the URL host (for example `127.0.0.1:6820`), with the token in `SLURM_JWT`. Without both, a file with one context uses that context, and a file with more than one context is a startup error. `--url` and `--user` replace the values of a selected context.
- No Context selected from the config file: `--url` and `SLURM_JWT` are enough (`--user` is optional, see `tokenEnv`). If one is missing, the setup form opens with the others filled in.
- Config errors start with the file path, the line and the field path, for example `line 7: slurmtui.contexts.prod.password: unknown field`.
- `refreshRate`: whole seconds, at least 2. `-r/--refresh` has priority.
- Flags: `--context` (required when there are several contexts), `-r/--refresh` (whole seconds, at least 2, as in k9s), `--readonly`, `--write`. `-h/--help` prints the flags and exits with code 0.

## Secrets

- Tokens and the key never appear in output, logs, errors or panics. Errors name the URL, never headers.
- Test fixtures use a fake key.

## Not in v1

Marks and bulk actions, `:ctx` switching, faults-only toggle, skins, aliases, `keyCommand`, logs. Each has a k9s/sofka model to copy later.
