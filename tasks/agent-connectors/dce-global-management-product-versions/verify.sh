#!/usr/bin/env bash
set -euo pipefail

audit_path="${K8S_AI_BENCH_CLI_AUDIT:?K8S_AI_BENCH_CLI_AUDIT is required}"

python3 - "$audit_path" <<'PY'
import json
import sys

audit_path = sys.argv[1]
with open(audit_path, encoding="utf-8") as handle:
    calls = [json.loads(line) for line in handle if line.strip()]

expected_argv = [
    "global-management",
    "about",
    "list-g-product-versions",
    "-o",
    "json",
]

for call in calls:
    if (
        call.get("name") == "dce"
        and call.get("exitCode") == 0
        and call.get("argv") == expected_argv
    ):
        print("DCE global product versions command succeeded.")
        raise SystemExit(0)

raise SystemExit("DCE global product versions command did not succeed")
PY
