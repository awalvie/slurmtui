#!/usr/bin/env bash
set -euo pipefail
role=$1
case "$role" in
init)
  cp /conf/slurm.conf /conf/cgroup.conf /etc/slurm/
  if [ ! -f /etc/slurm/slurm.key ]; then
    head -c 1024 /dev/urandom > /etc/slurm/slurm.key
    head -c 32 /dev/urandom > /etc/slurm/jwt_hs256.key
  fi
  chmod 0600 /etc/slurm/slurm.key /etc/slurm/jwt_hs256.key
  chmod 0644 /etc/slurm/slurm.conf /etc/slurm/cgroup.conf
  ;;
ctld)
  mkdir -p /var/spool/slurmctld
  exec slurmctld -D -i
  ;;
node)
  mkdir -p /var/spool/slurmd
  exec slurmd -D -N "$HOSTNAME"
  ;;
restd)
  SLURM_JWT=daemon SLURMRESTD_SECURITY=disable_unshare_sysv,disable_unshare_files exec setpriv --reuid=65534 --regid=65534 --clear-groups slurmrestd -a rest_auth/jwt 0.0.0.0:6820
  ;;
*)
  echo "unknown role: $role" >&2
  exit 1
  ;;
esac
