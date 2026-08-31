# DCE Pod Creation Agent Matrix

## Goal

Add a small, repeatable DCE integration task that creates a temporary Pod in
the `kpanda-global-cluster` cluster and `default` namespace, verifies the
created resource, and removes it afterward. Provide one matrix containing both
the Codex CLI connector and the remote OpenClaw gateway connector.

## Scope and execution model

The matrix configures both agents, but the current runner supports one selected
agent per invocation through `runs.agent`. The same matrix is therefore run
once with `codex` and once with `openclaw`; no runner behavior change is
required.

The task is agent-neutral and uses the DCE skill. Codex executes the local
`dce` binary. OpenClaw executes its own DCE CLI and credentials inside the
remote `mingtest` instance; local CLI wrappers and local environment variables
are not forwarded through the HTTP bridge.

## Task flow

The prompt directs the agent to:

1. Discover the create and list commands with `dce search`.
2. Inspect both exact command specifications with `dce commands show`.
3. Check authentication against the configured DCE host.
4. Create a Pod named `k8s-ai-bench-dce-pod` using image `nginx:stable`.
5. List/query the Pod and confirm its name, namespace, and image.
6. End with `DCE_POD_CREATED_OK` only after all checks succeed.

The task is intentionally limited to one Pod and does not create a namespace
or any other workload. Cleanup deletes the Pod through DCE when the local DCE
CLI and credentials are available; the agent prompt also instructs the remote
OpenClaw runtime to delete the same Pod after verification.

## Verification

`verify.sh` is run by the benchmark after the agent exits. It validates the
captured task log for the success sentinel and the expected resource
attributes. For Codex, `cliExpect` additionally validates that the local DCE
CLI was invoked with the create-workload operation and `--kind pods`. The
verifier does not claim to inspect remote OpenClaw tool calls because the HTTP
bridge currently exposes only the final response, not remote CLI audit data.

## Configuration

The new matrix uses:

- DCE host `http://10.0.6.152:30448`;
- cluster `kpanda-global-cluster`;
- OpenClaw base URL `http://10.0.6.152:30256/v1`;
- OpenClaw model route `openclaw/mingtest`;
- one iteration and serial execution;
- task pattern limited to the Pod creation task.

Credentials are represented by environment placeholders in the committed
example. A local ignored copy may substitute the user-provided values for a
live run. The remote OpenClaw DCE credential must already be configured in the
remote instance because the bridge does not forward local task environment
variables into that instance.

## Failure and cleanup behavior

Any failed discovery, command inspection, authentication check, creation,
query, or attribute check must prevent the success sentinel. Cleanup is
best-effort and ignores an already absent Pod. A failed verifier produces a
benchmark failure even if the agent produced a plausible prose response.

## Test plan

1. Add task and verifier fixture tests or shell-level validation as appropriate.
2. Run Go unit tests.
3. Build the bridge and benchmark binaries.
4. Validate the matrix parses and the task pattern selects only the new task.
5. Run Codex against the target DCE host.
6. Run OpenClaw against `mingtest` if its remote DCE authentication is ready.
7. Inspect result YAML, task log, and cleanup outcome without printing tokens.
