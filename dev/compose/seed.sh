#!/usr/bin/env bash
set -euo pipefail
cd /tmp
sbatch -J sleep-a -n 1 --wrap 'sleep 3600'
sbatch -J sleep-gpu -p gpu -n 1 --wrap 'sleep 3600'
sbatch -J array -n 1 --array=1-20 --wrap 'sleep 3600'
sbatch -J held -H -n 1 --wrap 'sleep 60'
sbatch -J multi-part -p debug,gpu -n 2 --mem=900 --wrap 'sleep 60'
sbatch -J quick -n 1 --wrap 'true'
scontrol update nodename=c3 state=drain reason="bad DIMM slot 3"
