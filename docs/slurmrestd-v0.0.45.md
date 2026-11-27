# slurmrestd v0.0.45 facts

Source: SchedMD/slurm tag `slurm-26-05-5-1`, `src/plugins/data_parser/v0.0.45/parsers.c` (lines below) and `src/slurmrestd/plugins/openapi/slurmctld/`.

The replies in `testdata/` are recorded from Slurm 26.05.5. The OpenAPI spec is `testdata/openapi-slurmctld-v0.0.45.json`. `just openapi-spec` (flake app `openapi-spec`) makes it again with `slurmrestd -s slurmctld -d v0.0.45 --generate-openapi-spec` from the nixpkgs Slurm in `flake.lock` (26.05.5). The output is the same on each run. The spec from 26.05.6 is different only in `info.version` and `info.x-slurm` (the Slurm version). The v0.0.45 paths and schemas are the same.

## The spec check

`internal/openapi` checks a reply body against the spec. The fake slurmrestd checks each reply that it sends, and a mismatch fails the test with the field path (for example `nodes[0].cpus`). The check is stricter than OpenAPI:

- A field that is not in the spec is an error.
- An integer must fit its `format` (`int32` or `int64`).
- `required` entries with a `/` (for example `nodes/count`) are nested paths.

Real slurmrestd 26.05.5 replies are different from the spec in two ways. The check accepts only these two:

- `jobs[].job_resources` is `null` for a pending job.
- A reply with an error in `errors[]` can leave out the required fields, for example `nodes` and `last_update` when the token is bad.

A test that must send an incorrect reply on purpose (for example an unknown node state from a newer Slurm) calls `ExpectOffSpec` with the field path. The reply must then fail at that field. The expectation applies only to replies from the path whose last part starts the field path (`jobs[0].job_state[1]` applies to `GET /jobs`).

## Shapes

- `{set,infinite,number}`: `TIMESTAMP_NO_VAL`, `UINT32_NO_VAL`, `UINT64_NO_VAL`.
- `CSV_STRING`: array of strings, `[]` when empty.
- `NODE_STATES`, `JOB_STATE`, `WARN_FLAGS`: array of strings.
- `USER_ID`: user name string.

## GET /slurm/v0.0.45/nodes → `nodes[]` (9750)

| field | JSON |
|---|---|
| name, reason | string |
| reason_set_by_user | string (user name) |
| state | string[] (enum) |
| cpus, alloc_cpus | int32 |
| real_memory, alloc_memory | int64 |
| reason_changed_at, boot_time, last_busy, free_mem | {set,infinite,number}. `boot_time` and `last_busy` are `{set:true,number:0}` on a node that never registered |
| partitions, features | string[] |
| gres, specialized_cpus | string, `""` when empty |
| specialized_memory | int64, MiB |
| cpu_load | int32, OS load ×100 |

## GET /slurm/v0.0.45/partitions?flags=ALL → `partitions[]`

`flags=ALL` includes hidden partitions, because `GET /jobs` includes their jobs. Recorded reply: `testdata/partitions.json`.

| field | JSON |
|---|---|
| name | string |
| defaults.time | {set,infinite,number}, minutes (DefaultTime). `{set:false}` when not set |
| maximums.time | {set,infinite,number}, minutes (MaxTime). `{infinite:true}` is UNLIMITED |

## GET /slurm/v0.0.45/jobs → `jobs[]` (9944)

| field | JSON |
|---|---|
| job_id | int32 |
| name, user_name, partition, state_reason, state_description | string |
| array_task_string | string, `""` when not an array meta record |
| nodes | string (hostlist), `""` while pending |
| job_state | string[] |
| array_job_id, het_job_id, het_job_offset | {set,infinite,number}, `{set:true,number:0}` when not an array or het job |
| array_task_id | {set,infinite,number}, `{set:false}` when no task is set |
| node_count | {set,infinite,number} |
| start_time, end_time | {set,infinite,number}. For a pending or running job they are estimates; `end_time` can be far in the future |
| time_limit | {set,infinite,number}, minutes. `{infinite:true}` is UNLIMITED; `{set:false}` means the partition limit applies |
| submit_time | {set,infinite,number} |
| account, qos, current_working_directory, standard_output, command | string. `standard_output` keeps patterns such as `%j` unexpanded. `command` is `""` for a batch job |
| exit_code | `{status[], return_code{set,infinite,number}, signal{id{set,infinite,number}, name}}` |
| job_resources | object, `null` for a pending job (not in the spec) |

## Node update: POST /slurm/v0.0.45/node/{name} (10867)

- Body `{"state":["DRAIN"],"reason":"..."}`, or `{"state":["RESUME"]}`.
- `reason_uid` is still in the spec as a deprecated string, but 26.05 ignores it. Slurm records the Token's user as the one who set the reason.

## Job cancel: DELETE /slurm/v0.0.45/job/{job_id}

- `{job_id}` accepts `1234`, `1234_5`, `1234+1`.
- Query `signal` (default SIGKILL) and `flags` (default full job).
- Failure is per job in `status[].error{code,string,message}`; the reply is HTTP 200 with empty `errors[]`. Non-zero `code` is a failure. `status` is required.

## GET /slurm/v0.0.45/ping → `pings[]`

- Each item: `hostname`, `responding` (bool), `latency` (int64, microseconds), `primary` (bool), `status` (string). `responding` and `primary` are required (`testdata/ping.json`).
- A bad token does not get HTTP 511 here. The reply is HTTP 200 with empty `errors[]`, and each item has `responding: false` and `status: "Protocol authentication error"` (`testdata/ping-bad-token-reply.json`). Thus slurmtui treats a reply with no responding item as a failure.

## Auth

- Headers: `X-SLURM-USER-NAME`, `X-SLURM-USER-TOKEN`.
- Token: HS256 JWT. slurmctld needs `exp` and `sun` (auth_jwt.c). It checks only `exp < now`, with no leeway, and ignores `iat`.
- When `sun` is root or SlurmUser and the user header names someone else, the request runs as the header user (auth_jwt.c:409-450).
- A token that is expired or has a bad signature gets the same reply: HTTP 511 with error 1007 (except on `/ping`, see above) `Protocol authentication error` (`testdata/auth-bad-token-reply.json`). An `exp` far in the future is accepted.
- The key file is used as raw bytes, trailing newline included (auth_jwt.c:221).

## Envelope

- `errors[]`: `{description, error_number, error, source}`. Non-empty means failure, even on HTTP 200. slurmtui shows `description: error`, for example `Failure to update node ghost3: Invalid user id`, because `description` alone does not give the cause.
- `warnings[]`: `{description, source}`.
- `meta`: `plugin{type,name,data_parser,accounting_storage}`, `client{source,user,group}`, `command[]`, and `slurm{version{major,minor,micro},release,cluster}`. All values are strings.
- Nodes replies must have `nodes` and `last_update`. Jobs replies must have `jobs`, `last_update` and `last_backfill`. Each time is {set,infinite,number}.
- Collection paths in the spec end in `/` (`/slurm/v0.0.45/nodes/`). slurmrestd also accepts them without the `/`.
- `GET /nodes` takes the query parameters `update_time` and `flags`.
