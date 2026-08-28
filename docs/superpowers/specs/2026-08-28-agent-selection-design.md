# Built-in Agent Selection Design

## Goal

Allow users to select a well-known Kubernetes-capable coding agent with a stable `--agent <name>` option, while retaining `--agent-bin` for custom executables.

## Scope

The first release supports three built-in profiles:

- `kubectl-ai`: the existing native kubectl-ai invocation.
- `codex`: Codex CLI in non-interactive `exec` mode.
- `claude`: Claude Code in non-interactive print mode.

The option is a run flag, so both forms are valid because the current CLI defaults to `run` when no subcommand is supplied:

```text
k8s-ai-bench run --agent codex --output-dir .build/codex
k8s-ai-bench --agent codex --output-dir .build/codex
```

`--agent-bin` remains available for agents that are not in the built-in registry. A run may specify either `--agent` or `--agent-bin`, never both. Omitting both preserves the current custom-agent behavior and returns a clear missing-agent error before task execution.

## User-facing behavior

Supported names and fixed launch behavior:

| Profile | Executable | Launch mode | Prompt input |
|---|---|---|---|
| `kubectl-ai` | `kubectl-ai` or resolved custom path | Existing kubectl-ai flags | Existing stdin script steps |
| `codex` | `codex` | `codex exec --ephemeral --sandbox danger-full-access -` | Entire task script through stdin |
| `claude` | `claude` | `claude -p --dangerously-skip-permissions` | Entire task script through stdin |

The profile controls the executable-specific command shape. Common benchmark values such as kubeconfig, model, and trace path are translated only when the selected CLI supports them. `KUBECONFIG` continues to be inherited from the benchmark process so the selected agent's `kubectl` commands target the task cluster.

`--models` remains available. For `kubectl-ai` it is passed using the existing `--model` argument. For Codex and Claude it is passed using each CLI's model option when explicitly provided; otherwise the selected CLI's configured default model is used. The profile name is retained in the in-memory evaluation configuration so errors identify the selected agent.

When an unknown profile is provided, the command exits before creating a cluster and prints the supported names plus the `--agent-bin` escape hatch. When both selection mechanisms are provided, it exits with an actionable conflict error.

## Architecture

Add a small agent-profile registry in the main package. A profile has:

```go
type AgentProfile struct {
    Name        string
    Description string
    Binary      string
    BuildArgs   func(LLMConfig, string) []string
}
```

The registry resolves a profile name to a profile and exposes sorted metadata for errors and help text. `TaskExecution` receives the resolved profile, and `runAgent` delegates executable-specific argument construction to it. The existing task lifecycle remains unchanged:

```text
setup → build profile command → send script prompts → capture stdout → expect/verifier → cleanup
```

The custom `--agent-bin` path uses a compatibility profile that preserves the current kubectl-ai argument list. This keeps custom support backwards compatible without making every custom executable pretend to accept kubectl-ai-specific flags; custom users remain responsible for supplying a compatible binary or wrapper.

## Validation and errors

- Resolve and validate `--agent` before cluster creation.
- Resolve the selected executable with the platform's normal PATH lookup; absolute paths continue to work for `--agent-bin`.
- Reject empty or unknown names with the sorted supported list.
- Reject simultaneous `--agent` and `--agent-bin`.
- Preserve task-level timeout, stdout/stderr logging, trace output where supported, result writing, and cleanup behavior.
- Do not silently install any Agent. Errors should identify the missing executable and show the expected installation command or documentation hint.

## Testing

Tests must cover behavior rather than implementation details:

1. Resolving each built-in name returns the expected executable and launch mode.
2. Unknown names return an error containing all supported names.
3. `--agent` and `--agent-bin` together return a conflict error.
4. Codex and Claude command builders use non-interactive modes and do not pass kubectl-ai-only flags.
5. The kubectl-ai profile preserves the current argument contract.
6. Existing task loading and analysis tests remain green.

No live Kubernetes cluster, credentials, or external Agent process is required for these tests; command construction is tested through pure profile builders.

## Non-goals

- No automatic installation or authentication for Codex, Claude, or kubectl-ai.
- No simultaneous execution of multiple different Agent binaries in one `run` invocation.
- No generic plugin/configuration system for arbitrary Agent CLIs.
- No change to task discovery, verifier semantics, cluster lifecycle, or report formats beyond identifying the selected profile in runtime errors.
