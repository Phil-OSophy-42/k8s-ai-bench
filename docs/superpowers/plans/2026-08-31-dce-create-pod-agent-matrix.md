# DCE Pod Creation Agent Matrix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Codex/OpenClaw matrix and a DCE task that creates, queries, verifies, and cleans up a temporary Pod through the configured DCE API.

**Architecture:** Keep the existing single-agent-per-matrix-run model. The matrix declares both connectors and selects one with `runs.agent`; the same task is run once per connector. Extend task lifecycle commands to inherit the selected agent/model environment so the verifier and cleanup can authenticate to the target DCE host. The verifier queries DCE directly, while Codex additionally uses CLI audit expectations.

**Tech Stack:** Go, YAML task/matrix definitions, Bash, Python 3, DCE generated CLI.

---

### Task 1: Pass selected connector environment to task lifecycle scripts

**Files:**
- Modify: `/Users/LAY/GolandProjects/daocloud/k8s-ai-bench/eval.go:620-700`
- Test: `/Users/LAY/GolandProjects/daocloud/k8s-ai-bench/eval_test.go`

- [ ] **Step 1: Write the failing test**

Add a test that constructs a `TaskExecution` with `agentConfig.Env["DCE_HOST"]` and `agentConfig.Env["DCE_TOKEN"]`, invokes the lifecycle environment helper, and asserts both values are present alongside `K8S_AI_BENCH_TASK_OUTPUT_DIR`.

- [ ] **Step 2: Run the focused test and verify it fails**

Run:

```bash
go test ./... -run TestTaskLifecycleEnvIncludesAgentEnv -count=1
```

Expected: FAIL because lifecycle commands currently only inherit `os.Environ()` and benchmark paths.

- [ ] **Step 3: Implement the minimal environment helper**

Add a helper on `TaskExecution` that starts from `os.Environ()`, overlays `x.agentConfig.Env` and `x.llmConfig.Env` using `os.ExpandEnv`, then sets `KUBECONFIG` and `K8S_AI_BENCH_TASK_OUTPUT_DIR`. Use it for setup, verifier, and cleanup commands. Do not add credentials to logs or command arguments.

- [ ] **Step 4: Run the focused test and the full Go tests**

Run:

```bash
go test ./... -run TestTaskLifecycleEnvIncludesAgentEnv -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add eval.go eval_test.go
git commit -m "test: pass agent env to task lifecycle"
```

### Task 2: Add the DCE Pod task and real API verifier

**Files:**
- Create: `/Users/LAY/GolandProjects/daocloud/k8s-ai-bench/tasks/agent-connectors-remote/dce-create-pod/task.yaml`
- Create: `/Users/LAY/GolandProjects/daocloud/k8s-ai-bench/tasks/agent-connectors-remote/dce-create-pod/verify.sh`
- Create: `/Users/LAY/GolandProjects/daocloud/k8s-ai-bench/tasks/agent-connectors-remote/dce-create-pod/cleanup.sh`

- [ ] **Step 1: Add the task prompt**

Use the DCE skill workflow and these exact resource values:

```text
cluster: kpanda-global-cluster
namespace: default
pod: k8s-ai-bench-dce-pod
image: nginx:stable
```

The prompt must discover and inspect `container-management apps create-workload-by-json`, `container-management core get-pod`, and `container-management core delete-pod`; authenticate with `DCE_HOST`; create the Pod with `--kind pods`; query it; and emit `DCE_POD_CREATED_OK` only after confirming name, namespace, and image. It must not create any other resource.

- [ ] **Step 2: Add the verifier**

The verifier must:

1. Require `DCE_HOST` and `DCE_TOKEN`.
2. Authenticate the local DCE CLI with `printf '%s' "$DCE_TOKEN" | dce --hostname "$DCE_HOST" auth login --auth-type bearer --with-token --skip-validate`.
3. Run `dce --hostname "$DCE_HOST" container-management core get-pod --cluster kpanda-global-cluster --namespace default --name k8s-ai-bench-dce-pod -o json`.
4. Parse JSON with Python and require the expected metadata name/namespace and container image `nginx:stable`.
5. Require the task log to contain `DCE_POD_CREATED_OK`.

Never print the token or the full authenticated command environment.

- [ ] **Step 3: Add cleanup**

Delete only `k8s-ai-bench-dce-pod` with the inspected DCE delete command, tolerate an already absent resource, and return a nonzero status for other failures.

- [ ] **Step 4: Validate shell syntax and task YAML**

Run:

```bash
bash -n tasks/agent-connectors-remote/dce-create-pod/verify.sh tasks/agent-connectors-remote/dce-create-pod/cleanup.sh
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tasks/agent-connectors-remote/dce-create-pod
git commit -m "test: add dce pod creation task"
```

### Task 3: Add the Codex/OpenClaw matrix

**Files:**
- Create: `/Users/LAY/GolandProjects/daocloud/k8s-ai-bench/eval-matrix-codex-openclaw-dce.yaml`

- [ ] **Step 1: Define both agents and the task selection**

Configure `codex` and `openclaw` with the existing bridge, use `generic-stdin`, set `DCE_HOST` and the target cluster metadata, and point OpenClaw at `http://10.0.6.152:30256/v1` with route `openclaw/mingtest`. Use `tasksDir: ./tasks/agent-connectors-remote`, `clusterCreationPolicy: DoNotCreate`, one iteration, serial execution, and task pattern `dce-create-pod`.

Set `runs.agent: codex` as the default. To run the second connector, change only that value to `openclaw` in a local copy or working-tree edit; the matrix still contains both agent definitions.

- [ ] **Step 2: Keep live credentials local**

Use the user-provided values only in the uncommitted working-tree matrix used for the live test. Do not commit them, add them to documentation, or print them in test output. The remote OpenClaw instance must already have its own DCE CLI authentication because local matrix environment variables do not cross the HTTP bridge.

- [ ] **Step 3: Validate matrix loading and selection**

Run:

```bash
go test ./... -count=1
go build -o .build/bin/k8s-ai-agent-bridge ./cmd/k8s-ai-agent-bridge
go build -o .build/bin/k8s-ai-bench .
```

Expected: PASS and both binaries exist.

- [ ] **Step 4: Commit the non-secret matrix template**

If the matrix uses placeholders for committed source, commit the template. Keep the live credential-filled copy untracked.

### Task 4: Execute and inspect both connector runs

**Files:**
- Inspect: `/Users/LAY/GolandProjects/daocloud/k8s-ai-bench/.build/codex-openclaw-dce`

- [ ] **Step 1: Run Codex**

Run with `runs.agent: codex` and the user-provided DCE host/token in the local matrix. Confirm the result is `success`, the verifier finds the Pod through DCE, and cleanup removes it.

- [ ] **Step 2: Run OpenClaw**

Run with `runs.agent: openclaw`, `OPENCLAW_MODEL=openclaw/mingtest`, and the user-provided gateway token. Confirm the remote instance can authenticate to DCE, create/query the Pod, and produce `DCE_POD_CREATED_OK`; the local verifier then queries and cleanup deletes the same Pod.

- [ ] **Step 3: Inspect outputs without exposing secrets**

Check only `results.yaml`, `log.txt`, and the final DCE lookup/deletion status. Do not print environment dumps or token-bearing config.
