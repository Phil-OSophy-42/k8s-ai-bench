# Agent Connectors

`k8s-ai-agent-bridge` adapts command-line agents and OpenAI-compatible agent
gateways to the `generic-stdin` contract used by matrix mode:

```text
k8s-ai-bench run --matrix-file matrix.yaml
        |
        v
k8s-ai-agent-bridge --agent <name>
        |
        +-- codex exec -
        +-- claude -p
        `-- POST <OpenClaw base URL>/v1/chat/completions
```

## Build

```sh
go build -o k8s-ai-agent-bridge ./cmd/k8s-ai-agent-bridge
```

The bridge reads the complete benchmark prompt from stdin, writes the agent
result to stdout, and writes failures to stderr. It exits with the underlying
CLI exit code or a non-zero error status.

## Codex CLI

The bridge invokes the non-interactive `codex exec` command, not the default
TUI. It resolves the executable in this order:

1. `CODEX_BIN`, if set;
2. the `codex` executable found in `PATH`.

```sh
export CODEX_BIN=/opt/homebrew/bin/codex
go build -o k8s-ai-agent-bridge ./cmd/k8s-ai-agent-bridge
```

Matrix configuration:

```yaml
agents:
  - id: codex
    bin: ./k8s-ai-agent-bridge
    adapter: generic-stdin
    args: [--agent, codex]
```

The matrix `models[].model` value is metadata for CLI connectors; it is not
automatically passed to Codex. Set `CODEX_MODEL` when the local Codex CLI
should use a specific model. If it is unset, Codex uses the model selected by
its own account/configuration. Codex's own sandbox and approval settings
should be configured explicitly for the CI environment.

## Claude Code

The bridge invokes Claude Code print mode (`claude -p`) and requests plain text
output. It resolves the executable in this order:

1. `CLAUDE_BIN`, if set;
2. the `claude` executable found in `PATH`.

```sh
export CLAUDE_BIN=/usr/local/bin/claude
```

Matrix configuration:

```yaml
agents:
  - id: claude
    bin: ./k8s-ai-agent-bridge
    adapter: generic-stdin
    args: [--agent, claude]
```

The matrix `models[].model` value is metadata for Claude Code. Set
`CLAUDE_MODEL` to pass `--model <value>`; if it is unset, Claude Code uses its
own account/configuration default.

## OpenClaw gateway

OpenClaw is called through an OpenAI-compatible chat completions endpoint. The
bridge accepts either a base URL or a full `/chat/completions` URL:

```sh
export OPENCLAW_BASE_URL=http://127.0.0.1:30145/v1
export OPENCLAW_API_KEY=replace-me
export OPENCLAW_MODEL=openclaw-agent
```

The request is:

```http
POST {OPENCLAW_BASE_URL}/chat/completions
Authorization: Bearer {OPENCLAW_API_KEY}
Content-Type: application/json
```

The bridge sends the benchmark prompt as a single `user` message and writes
`choices[0].message.content` to stdout. OpenClaw's own skills, tools, and
cluster access remain on the gateway side; local benchmark CLI wrappers are
not transferred into the gateway process.

For a remote OpenClaw Control UI deployment that exposes the OpenAI-compatible
HTTP route, use the gateway's `/v1` base URL rather than the Control UI chat
page URL:

```sh
export OPENCLAW_BASE_URL=http://10.0.6.152:32516/v1
export OPENCLAW_API_KEY=replace-me
export OPENCLAW_MODEL=openclaw/skilldemo-af6zc
```

The gateway URL and API key are used by the local bridge only. The DCE CLI,
DCE skill, DCE credentials, `DCE_HOST`, and TLS settings must be configured in
the remote OpenClaw runtime. A remote task should inject the skill content but
must not declare local `clis`, because benchmark-generated CLI wrappers cannot
be reached by a remote gateway. Use `eval-matrix-openclaw-dce.yaml` as the
single-task example.

Matrix configuration:

```yaml
agents:
  - id: openclaw
    bin: ./k8s-ai-agent-bridge
    adapter: generic-stdin
    args: [--agent, openclaw]
    env:
      OPENCLAW_BASE_URL: ${OPENCLAW_BASE_URL}
      OPENCLAW_API_KEY: ${OPENCLAW_API_KEY}
      OPENCLAW_MODEL: ${OPENCLAW_MODEL}
```

## Selecting one connector

`runs.agent` selects one configured agent for the current matrix run. This is
useful when the same task directory should be evaluated against different
connectors without editing every task's `agent` field:

```yaml
runs:
  agent: claude
```

If `runs.agent` is omitted, matrix mode keeps the existing task-level behavior
and requires `agent: <id>` in each task. A task-level agent remains supported,
but the run-level value takes precedence when both are present.
