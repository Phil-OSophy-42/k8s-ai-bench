#!/usr/bin/env bash
set -euo pipefail

test -f "${K8S_AI_BENCH_CLI_AUDIT}"
grep -q '"name":"dce"' "${K8S_AI_BENCH_CLI_AUDIT}"
grep -q 'container-management' "${K8S_AI_BENCH_CLI_AUDIT}"
grep -q 'get-cluster' "${K8S_AI_BENCH_CLI_AUDIT}"
