#!/usr/bin/env bash
set -euo pipefail

: "${DCE_HOST:?DCE_HOST is required}"
: "${DCE_TOKEN:?DCE_TOKEN is required}"

printf '%s' "$DCE_TOKEN" |
  dce --hostname "$DCE_HOST" auth login --auth-type bearer --with-token --skip-validate >/dev/null

set +e
if [[ "${DCE_INSECURE_SKIP_VERIFY:-false}" == "true" ]]; then
  delete_output="$({
    curl --noproxy '*' --fail-with-body -ksS -X DELETE \
      -H "Accept: application/json" \
      -H "Authorization: Bearer $DCE_TOKEN" \
      "${DCE_HOST%/}/apis/kpanda.io/v1alpha1/clusters/kpanda-global-cluster/namespaces/default/pods/k8s-ai-bench-dce-pod"
  } 2>&1)"
else
  delete_output="$({
    dce --hostname "$DCE_HOST" container-management core delete-pod \
      --cluster kpanda-global-cluster \
      --namespace default \
      --name k8s-ai-bench-dce-pod \
      -o json
  } 2>&1)"
fi
delete_status=$?
set -e

if [[ "$delete_status" -eq 0 ]]; then
  echo "Deleted DCE Pod k8s-ai-bench-dce-pod."
  exit 0
fi

if printf '%s' "$delete_output" | grep -Eiq '404|not found|does not exist'; then
  echo "DCE Pod k8s-ai-bench-dce-pod was already absent."
  exit 0
fi

printf '%s\n' "$delete_output" >&2
exit "$delete_status"
