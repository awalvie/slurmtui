# slurmtui

A terminal UI that shows a Slurm cluster and acts on it through slurmrestd.

## Language

### Auth

**Context**:
A named pairing of one slurmrestd endpoint, one user and one Token source.
_Avoid_: Profile, cluster, connection

**Token**:
The JWT that slurmtui sends to slurmrestd to prove who the caller is.
_Avoid_: Credential, auth token, session

**Cluster key**:
The shared HS256 secret from which slurmtui can sign a Token for any user, root included.
_Avoid_: JWT key, secret, signing key

**Provided token**:
A Token that the cluster made and that slurmtui receives as `SLURM_JWT`.
_Avoid_: Env token, user token

**Signed token**:
A Token that slurmtui makes itself from the Cluster key.
_Avoid_: Self-signed token, minted token

### Nodes

**Drain**:
To stop the scheduler from placing new jobs on a node, with a Reason. Running jobs keep running, except on a powered-down or powering-up node.
_Avoid_: Cordon, disable, offline

**Draining**:
A drained node that still has jobs or is completing them.
_Avoid_: Pending drain

**Drained**:
A drained node with no jobs left.
_Avoid_: Idle-drain, offline

**Resume**:
To return a drained, down or failed node to service.
_Avoid_: Undrain, uncordon, enable

**Reason**:
Free text that says why a node is drained, down or failed, with the user who set it and when.
_Avoid_: Note, comment, message

### Jobs

**Cancel**:
To end a job, one array task, or a whole array job, with SIGKILL.
_Avoid_: Kill, delete, scancel

**Array job**:
A set of jobs that share one ID and differ only by task ID, shown as `1234_[3-100]` while tasks are pending.
_Avoid_: Job array task list

### Screen

**Overview**:
The start screen: the Nodes pane on top and the Jobs pane under it.
_Avoid_: Dashboard, home, split view

**Pane**:
One table in a frame. The Overview has two panes. One pane has the focus at a time.
_Avoid_: Panel, tab, window

**Scope**:
A node that limits the Jobs pane to the jobs whose hostlist holds that node, ended jobs included.
_Avoid_: Drill-down filter

**Usable node**:
A node that can run jobs: not down, not powered down, and not drained. A draining node is usable.
_Avoid_: Healthy node, online node
