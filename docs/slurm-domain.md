# Slurm domain semantics (26.05, tag `slurm-26-05-5-1`)

Citation key: `pd`=src/common/slurm_protocol_defs.c, `pdh`=…/slurm_protocol_defs.h, `h`=slurm/slurm.h, `nm`=src/slurmctld/node_mgr.c, `jm`=src/slurmctld/job_mgr.c, `pr`=src/slurmctld/proc_req.c, `P`=src/plugins/data_parser/v0.0.45/parsers.c, `rn`/`rj`=src/slurmrestd/plugins/openapi/slurmctld/{nodes,jobs}.c, `sq`=src/squeue/print.c, `ni`=src/api/node_info.c.

## 1. Node state

- `uint32`: low 4 bits = base, rest = flags (h:999-1000). Base: UNKNOWN, DOWN, IDLE, ALLOCATED, ERROR(unused), MIXED, FUTURE (h:990-996).
- REST `state[]` = one base string, then set flags in table order (P:9695-9736). Flag strings: EXTERNAL, RESERVED, UNDRAIN, CLOUD, RESUME, DRAIN, COMPLETING, NOT_RESPONDING, POWERED_DOWN, FAIL, POWERING_UP, MAINTENANCE, REBOOT_REQUESTED, REBOOT_CANCELED, POWERING_DOWN, DYNAMIC_FUTURE, REBOOT_ISSUED, PLANNED, INVALID_REG, POWER_DOWN, POWER_UP, POWER_DRAIN, DYNAMIC_NORM, BLOCKED; composite POWER_DOWN_ASAP / POWER_DOWN_FORCE may also appear. "INVALID" if NO_VAL.
- MIXED is computed client-side by libslurm, not slurmctld: if `0 < idle < effective_cpus` (idle = effective_cpus − alloc_cpus), or no CPUs allocated but some TRES (e.g. memory, GPU) is, base becomes MIXED (ni:517-537). REST asks for it by default (`SHOW_ALL|SHOW_DETAIL|SHOW_MIXED`, rn:113-114).
- DRAINING = DRAIN and base ALLOCATED or MIXED (pdh:161-163). The string functions also count COMPLETING.

### Rendering: `node_state_string` (pd:3676) / `node_state_string_compact` (pd:3933)
sinfo lowercases them: `%T` long, `%t` compact (sinfo/print.c:1075-1140). `scontrol show node` prints `BASE+FLAG+FLAG` instead, e.g. `IDLE+DRAIN` (`node_state_string_complete`, pd:3648; ni:341).

The first matching rule wins:
1. INVALID_REG → `INVAL`.
2. MAINT, and not DRAIN, not ALLOC/DOWN/MIXED → `MAINT` / `MAINT*`.
3. REBOOT_REQUESTED or REBOOT_ISSUED, base not ALLOC/MIXED → `REBOOT^` (issued), `REBOOT*`, `REBOOT`. Compact: `BOOT…`.
4. DRAIN: COMPLETING or ALLOC/MIXED → `DRAINING`/`DRNG`, otherwise `DRAINED`/`DRAIN`. That includes DOWN+DRAIN → drained. Takes the suffix S.
5. FAIL: same split → `FAILING`/`FAILG` or `FAIL`. Only `*` is used as a suffix.
6. The exact-value cases (REBOOT_ISSUED, CLOUD, POWER_*, UNDRAIN…) match only when no base or other flag is set. In practice they are rare.
7. DOWN+S; ALLOCATED+S (`ALLOCATED+`/`ALLOC+` if COMPLETING); COMPLETING+S (`COMP`); IDLE+S, otherwise `RESERVED`/`RESV`, `BLOCKED`/`BLOCK`, `PLANNED`/`PLND`; MIXED+S, otherwise `MIXED-`/`MIX-` when PLANNED; FUTURE+S (`FUTR`); RESUME → `RESM`; UNKNOWN (`UNK`), with `*` if not responding.
- The suffix S is a single character, chosen in this order: `$` MAINT, `^` REBOOT_ISSUED, `@` REBOOT_REQUESTED, `#` POWERING_UP, `%` POWERING_DOWN, `~` POWERED_DOWN, `!` POWER_DOWN (pending), `*` NOT_RESPONDING. Meanings: sinfo.1:967-1005.
- Examples: MIXED+DRAIN→`draining`/`drng`; IDLE+DRAIN→`drained`/`drain`; IDLE+DRAIN+NOT_RESPONDING→`drain*`; IDLE+POWERED_DOWN→`idle~`; DOWN+NOT_RESPONDING→`down*`.

## 2. Node update (`update_node`, nm:1641)

- Who may do it: root, SlurmUser, or AdminLevel=**Admin**. Operator is not enough (pr:4494, pr:582-590).
- A reason is required only by scontrol on the client side (scontrol/update_node.c:265-272). slurmctld and REST accept DRAIN with no reason.
- A non-empty `reason` sets reason, reason_time=now, and reason_uid=the authenticated user (nm:2051-2058). This happens **before** the transition is validated, so the reason sticks even if the state change is rejected.
- DRAIN is valid from any state (nm:3110-3114). It sets the DRAIN flag and clears FAIL (nm:2229-2250). **Exception:** DRAIN on an ALLOCATED node that is POWERED_DOWN or POWERING_UP kills its jobs (nm:2231-2236). DRAIN on a node that is already drained only updates the reason (nm:1482-1493, 2021-2036).
- RESUME is valid only if the node is DOWN, FUTURE, DRAIN, FAIL, REBOOT_REQUESTED, or POWERING_DOWN (nm:3129-3140). INVALID_REG blocks it, but POWERING_DOWN is checked first, so a node with both can be resumed. Otherwise slurmctld returns `ESLURM_INVALID_NODE_STATE_TRANSITION` (nm:2064-2073).
  - RESUME clears DRAIN, FAIL, REBOOT_REQUESTED and REBOOT_ISSUED (nm:2091-2096). DOWN becomes IDLE (nm:2124-2126).
  - For DOWN, FUTURE or DRAIN nodes it sets **NOT_RESPONDING** until slurmd re-registers (`_require_node_reg`, nm:1622-1639, 2124-2142), except on EXTERNAL, POWERED_DOWN or POWERING_DOWN nodes. Expect `idle*` after resuming a drained node, `mix*`/`alloc*` after resuming a draining one. Resuming a node with only FAIL or REBOOT_REQUESTED sets nothing. Resume also sets boot_time to 0.
  - It does not clear a NOT_RESPONDING flag that is caused by the node itself.
- Any update that leaves a node not DOWN, DRAIN or FAIL clears reason, reason_time and reason_uid (nm:2454-2461; accounting_storage.c:1041-1043).
- UNDRAIN clears only DRAIN and keeps the base state (e.g. stays DOWN). It also requires re-registration (nm:2142-2153; scontrol.1:1926).
- slurmctld writes its own reasons with reason_uid=SlurmUser, e.g. "Not responding", "Node unexpectedly rebooted" (nm:3928-3936, 5496-5520).
- REST `reason_set_by_user` is `""` when there is no reason (P:2236-2245), and also when slurmrestd's host cannot resolve the uid to a name (P:2152-2163). Reasons set by slurmctld show the SlurmUser name, which may be `root`.
- `GET /nodes` hides FUTURE nodes (nm:960-962, rn:113-114), so RESUME from FUTURE is not reachable from the TUI.

## 3. Jobs

- Base states: PENDING, RUNNING, SUSPENDED, COMPLETED, CANCELLED, FAILED, TIMEOUT, NODE_FAIL, PREEMPTED, BOOT_FAIL, DEADLINE, OUT_OF_MEMORY.
- Flags: LAUNCH_FAILED, REQUEUED, REQUEUE_HOLD, SPECIAL_EXIT, RESIZING, CONFIGURING, COMPLETING, STOPPED, RECONFIG_FAIL, POWER_UP_NODE, REVOKED, REQUEUE_FED, RESV_DEL_HOLD, SIGNALING, STAGE_OUT, EXPEDITING (P:11339-11368).
- squeue shows a single state, and **a flag takes priority over the base**. Order: COMPLETING(CG) > STAGE_OUT(SO) > CONFIGURING(CF) > EXPEDITING > RESIZING(RS) > REQUEUED(RQ) > REQUEUE_FED(RF) > REQUEUE_HOLD(RH) > SPECIAL_EXIT(SE) > STOPPED(ST) > REVOKED(RV) > RESV_DEL_HOLD(RD) > SIGNALING(SI), then the base: PD R S CD CA F TO NF PR BF DL OOM (pd:2891-2950, 2952-3006). A completing job has a terminal base, e.g. `["CANCELLED","COMPLETING"]`.
- `GET /jobs` returns every job slurmctld still holds in memory. Finished jobs are purged once `end_time` is older than MinJobAge (**default 300 s**; 0 = never purge), and never while still COMPLETING (jm:10201-10250; read_config.h:128; slurm.conf.5:2870).
- The REST default is `SHOW_ALL|SHOW_DETAIL`, which includes jobs in hidden partitions (rj:128-129).
- squeue by default shows only PENDING, RUNNING, SUSPENDED, COMPLETING and STAGE_OUT (sq:3205-3211; squeue.1:1544). `--start` defaults to PD (squeue/opts.c:537-539).
- Default format: `%.18i %.9P %.8j %.8u %.2t %.10M %.6D %R` (squeue.1:76).

### Arrays and het jobs
- Fields: `array_job_id`, `array_task_id`, `array_task_string`, `het_job_id`, `het_job_offset` (P:9951-9954, 10003-10005). These are UINT32_NO_VAL, so `{set:false}` when not applicable.
- Pending array tasks that have not started stay in **one meta record**: `job_id == array_job_id`, `array_task_string` set (e.g. "3-100"), `array_task_id` unset. A task that has started or been modified becomes its own record with its own `job_id` (jm:3513-3530).
- **The last task to start reuses the meta record**: `job_id == array_job_id`, `array_task_id` set, no `array_task_string`. A record is the array meta record only when `array_task_string` is set. To cancel one task, always send `A_T` when `array_task_id` is set; `DELETE /job/{job_id}` on the last task cancels the whole array.
- squeue merges pending task records that were split out but share name and partition back into the meta record. `-r` turns this off. squeue truncates the bitmap string at 64 chars unless `SLURM_BITSTR_LEN` is set (sq:259-320).
- JOBID rendering (sq:715-746): `A_[str]` if array_task_string is set; otherwise `A_T` if a task id is set; otherwise `H+O` if het_job_id; otherwise `job_id`.
- Cancelling:
  - `DELETE /job/{id}` parses `{id}` with `unfmt_job_id_string`. It accepts `1234`, `1234_5`, `1234+1`, and sluid forms (P:5758-5769; pd:1039; rj:208-227). Defaults are SIGKILL and KILL_FULL_JOB.
  - `1234` (the array master) cancels **all tasks**. `1234_5` cancels only that task: pending tasks are removed from the meta record's bitmap (jm:5104-5140; scancel.1:14-19).
  - Het jobs: offset 0, or the leader id, cancels the whole job. A non-leader component cannot be cancelled while pending (`ESLURM_NOT_WHOLE_HET_JOB`), nor while running when slurm.conf sets `whole_hetjob` (jm:4938-4954; scancel.1:20-26).
  - **Errors come back per job, not in `errors[]`.** `job_mgr_signal_jobs` always returns success (jm:5975-6058). Each job's failure (ACCESS_DENIED, ALREADY_DONE, INVALID_JOB_ID, NOT_WHOLE_HET_JOB) goes into `status[].error{code,string,message}` (jm:4605-4628; P:11770-11775, 12957). The reply is HTTP 200 with empty `errors[]`; a non-zero `status[].error.code` is the failure.
- `GET /job/1234_5` warns and returns the whole array (rj:178-179). `POST /job/1234_5` is rejected (rj:289-291).

### Times and scope in the jobs view
- `nodes` keeps the hostlist after a job ends, for example on a FAILED job (`testdata/slurm-26.05/jobs.json`). Thus the node scope of the Jobs pane matches ended jobs by `nodes`, not by the NODELIST(REASON) text.
- TIME_LEFT is `time_limit` minus the elapsed time, for a job that is running, suspended or completing. A value less than 0 shows `0s`. squeue `%L` gives the same value (squeue.1).
  - `time_limit` with `infinite: true` shows `UNLIMITED`. REST sends `{set:false, infinite:true}` for a job with no limit (`testdata/slurm-26.05/jobs.json`).
  - `{set:false, infinite:false}` means the partition limit applies. slurmtui reads the partitions each refresh and uses the job's partition: DefaultTime if it is set, else MaxTime. An unset MaxTime is UNLIMITED. A partition that is not in the list is an error that names the job and the partition.
- slurmtui does not subtract suspended time from the elapsed time. squeue does (`suspend_time`). The difference shows only on a job that was suspended.
- TIME of a pending job is the wait since `submit_time`. squeue shows `0:00` here. slurmtui shows the wait, in `dim`, because it tells an admin more.
- `priority` orders pending jobs. A held job has priority 0 (`testdata/slurm-26.05/jobs.json`, job 7), so it sorts last in the pending group.

## 4. state_reason
- REST gives the string from `job_state_reason_string` (P:2349-2356). The value is **"None"** when no reason is set, which includes most running jobs (job_state_reason.c:53-55). `state_description` holds optional detail.
- squeue `%R` (NODELIST(REASON)): if the job is not COMPLETING and is PENDING, STAGE_OUT, TIMEOUT, OUT_OF_MEMORY, DEADLINE or FAILED, it prints `(state_description or reason)`. Otherwise it prints the `nodes` hostlist (sq:1178-1203).

## 5. CPU and memory
| field | meaning |
|---|---|
| cpus | configured CPUs (threads) |
| effective_cpus | cpus minus specialized cores; the MIXED test uses this (P:9764; ni:526) |
| alloc_cpus | CPUs allocated to jobs |
| alloc_idle_cpus | cpus − alloc_cpus (P:4174) |
| cpu_load | OS load **×100** (h:2454); sinfo and scontrol divide by 100 (sinfo/print.c:306; ni:234) |
| real_memory | configured MB (slurm.conf RealMemory) |
| alloc_memory | MB allocated to jobs (scontrol `AllocMem`) |
| free_mem | MB free as reported by the OS, for information only, not used by the scheduler; NO_VAL → N/A (sinfo.1:188) |
| specialized_memory | MemSpecLimit |

- sinfo `%C` A/I/O/T (sinfo/sinfo.c:1094-1118): A=alloc_cpus. The remaining CPUs count as **O**ther when the node is DRAIN or DOWN, otherwise as **I**dle. T=cpus.
- A sinfo-like "used" display should show alloc_cpus/cpus and alloc_memory/real_memory. These reflect scheduler allocation, not actual utilisation.
- Usable nodes: a node is usable when it is not down (base DOWN, or flag FAIL or NOT_RESPONDING), not POWERED_DOWN, and not drained (DRAIN with no allocated CPU). A draining node is usable, because its jobs still run. The `cpu %` in the Nodes title is the sum of `alloc_cpus` over the sum of `cpus` of the usable nodes. The `mem %` is the sum of `alloc_memory` over the sum of `real_memory`. Both round down to a whole number.
- GRES column: `gres` is a comma list of `name[:type][:count][(...)]`, for example `gpu:a100:4(S:0-1)`. `gres_used` has the same form, for example `gpu:a100:2(IDX:0-1)`. The text in brackets can hold commas (`IDX:0,2`), so slurmtui splits the list only at commas outside brackets. A missing count is 1. The column shows the sum of the `gpu` counts in `gres_used`, `/`, the sum of the `gpu` counts in `gres`. Other GRES names show only in describe. An empty `gres` or no `gpu` entry gives a blank cell. A string that slurmtui cannot parse is an error that names the node and the value.
- Memory MB are MiB. `real_memory` includes `specialized_memory` (P:9778), so a full node stays below 100%. Without a `CR_*_Memory` SelectTypeParameters, `alloc_memory` stays 0 (unverified in source).

## 6. Partitions
- A node can belong to several partitions (slurm.conf.5:7695). REST `partitions[]` is built client-side (rn:146-161).
- A pending job submitted to several partitions has `partition` = a comma list such as "a,b". Once it runs, `partition` is the single partition it runs in (jm:10755-10758). squeue `-P` lists such a job once per partition (squeue.1:1465).

## 7. Hostlist (src/common/hostlist.c)
- Tokens are separated by `,`, space, tab or newline. A comma inside brackets is not a separator (hostlist.c:357-395, 1813).
- `prefix[ranges]suffix`, where ranges are a comma list of `N` or `lo-hi`. lo ≤ hi, no negatives (hostlist.c:1542-1606, 1615-1650).
- **Zero padding width is the length of `lo`**: `n[01-10]` → n01…n10, but `n[1-10]` → n1…n10 (`%0*lu`, hostlist.c:1567, 1687).
- Multiple brackets give the Cartesian product, with the left bracket varying slowest: `r[1-2]n[1-3]` (hostlist.c:1653-1720). A suffix after `]` is allowed (hostlist.c:1748-1752).
- Limits: 64K hosts per range (hostlist.h:54), 256K ranges.

## 8. Permissions
- PrivateData unset: **all users see all jobs, nodes and partitions** (slurm.conf.5:3559-3562). PrivateData=jobs hides other users' jobs except from account coordinators (jm:10316-10329).
- Signal or cancel: the job owner, an Operator or Admin, or a coordinator of the job's account (jm:5005-5020; scancel.1:47, 386-388).
- Node drain or resume: Admin only (§2).

## 9. Surprises
- Nodes `?update_time` with no change → **HTTP 304** (rn:120-128). Jobs with no change → **200** with a warning and no `jobs` (rj:134-140).
- The REST base state can be MIXED even though slurmctld stores ALLOCATED or IDLE (§1). MIXED can also mean that CPUs are idle while a GPU or memory is allocated.
- REST fills `nodes[].partitions` and MIXED, which come from libslurm. Don't expect them in raw RPC state.
- Re-draining a drained node with a new reason overwrites the reason, its time and its user.
- Allocating a node clears its reason (nm:4452-4455).
