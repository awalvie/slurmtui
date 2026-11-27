default:
    @just --list

check:
    ./scripts/check

test *args:
    ./scripts/test {{args}}

lint:
    golangci-lint run ./...

fmt:
    gofmt -w .

build:
    nix build

run *args:
    go run ./cmd/slurmtui {{args}}

hooks:
    git config core.hooksPath .githooks

compose := "docker compose -f dev/compose/compose.yaml"

cluster-image:
    nix build .#slurm-image -o result-slurm-image
    docker load < result-slurm-image

cluster-up: cluster-image
    {{compose}} up -d

cluster-seed:
    {{compose}} exec -T ctld bash /conf/seed.sh

cluster-wait seconds="120":
    #!/usr/bin/env bash
    set -euo pipefail
    for _ in $(seq {{seconds}}); do
      if not_ready=$({{compose}} exec -T ctld sinfo -h -N -t unknown,no_respond -o %N 2>/dev/null) && [ -z "$not_ready" ] \
        && curl -sf -o /dev/null -H "X-SLURM-USER-NAME: root" -H "X-SLURM-USER-TOKEN: $(just cluster-token)" http://127.0.0.1:6820/slurm/v0.0.45/ping; then
        echo "cluster-wait: slurmctld, all slurmd nodes and slurmrestd are up"
        exit 0
      fi
      sleep 1
    done
    echo "cluster-wait: cluster not ready after {{seconds}}s" >&2
    {{compose}} ps >&2
    exit 1

cluster-down:
    {{compose}} down -v

cluster-token user="root" lifespan="3600":
    @{{compose}} exec -T ctld scontrol token username={{user}} lifespan={{lifespan}} | sed 's/^SLURM_JWT=//' | tr -d '\r\n'

cluster-run *args:
    SLURM_JWT=$(just cluster-token) go run ./cmd/slurmtui --url http://127.0.0.1:6820 --user root {{args}}

cluster-test *args:
    SLURM_JWT=$(just cluster-token) SLURMTUI_TEST_URL=http://127.0.0.1:6820 SLURMTUI_TEST_USER=root go test -race -timeout 300s -tags integration -run '^TestRealCluster' {{args}} ./internal/app/

openapi-spec:
    nix run .#openapi-spec
