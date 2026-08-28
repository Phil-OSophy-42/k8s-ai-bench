# k8s-ai-bench

`k8s-ai-bench` is a benchmark for assessing the performance of LLM models for Kubernetes related tasks. It evaluates AI agents (like `kubectl-ai`) on their ability to perform real-world Kubernetes operations such as creating deployments, debugging crash loops, and scaling applications.

## 📊 Live Dashboard

See [k8s-ai-bench live leaderboard](https://gke-labs.github.io/k8s-ai-bench/) for the latest benchmark results.

The leaderboard shows run results for widely used proprietary and open models over a few run types:

* **Pass@1**: Can the agent solve the task on the first try? This measures raw capability and immediate correctness.
* **Pass@5**: Can the agent solve the task at least once in 5 attempts? This shows if the agent can eventually find a solution.
* **Pass^5**: Does the agent solve the task every single time? This measures reliability and consistency, which is crucial for autonomous usage.



## 🚀 Quick Start

### 1. Build the Binary
```sh
go build
```

### 2. Run an Evaluation
Run the benchmark against your agent binary. Results will be saved to the `.build` directory.
```sh
# Basic usage
./k8s-ai-bench run --agent-bin <path/to/kubectl-ai> --output-dir .build/k8s-ai-bench

# Run a specific task type (e.g., scaling tasks)
./k8s-ai-bench run --agent-bin <path/to/kubectl-ai> --task-pattern "scale" --output-dir .build/k8s-ai-bench
```

### Selecting an Agent

Use `--agent` to select a built-in Agent profile. The profile controls how the
Agent CLI is launched and how task prompts are sent:

~~~sh
# Existing kubectl-ai integration
./k8s-ai-bench run --agent kubectl-ai --output-dir .build/k8s-ai-bench

# Codex CLI non-interactive execution
./k8s-ai-bench run --agent codex --output-dir .build/codex

# Claude Code non-interactive execution
./k8s-ai-bench run --agent claude --output-dir .build/claude
~~~

The same flag can be used without the run subcommand:

~~~sh
./k8s-ai-bench --agent codex --output-dir .build/codex
~~~

The supported built-in profiles are `kubectl-ai`, `codex`, and `claude`.
Install and authenticate the selected Agent before running an evaluation.
The benchmark passes the task cluster through KUBECONFIG and runs Codex and
Claude in non-interactive mode. Use `--models` to override the selected
Agent's default model.

For an Agent that is not built in, use `--agent-bin` with a compatible
executable or wrapper. Do not provide `--agent` and `--agent-bin` together.

## 🛠 Usage Guide

### `run` Subcommand
The `run` subcommand executes the benchmark evaluations. It creates ephemeral clusters to ensure test isolation. We support two platforms for the test environment: **Kind** (default) and **vCluster**.

**vCluster Prerequisites:**
For a detailed step-by-step guide on setting up and using vCluster, see [vCluster Guide](docs/vcluster.md).

To use `vcluster`, you must have:
* The `vcluster` [CLI](https://www.vcluster.com/docs/vcluster/) installed.
* A running host Kubernetes cluster.
* A kubecontext to connect to the host cluster (passed via `--host-cluster-context`).


```sh
# Run with specific LLM provider and model
./k8s-ai-bench run \
  --agent-bin <path/to/kubectl-ai> \
  --llm-provider gemini \
  --models gemini-2.5-pro-preview-03-25 \
  --task-pattern fix \
  --output-dir .build/k8s-ai-bench
```

**Common Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `--agent` | Built-in Agent profile (kubectl-ai, codex, or claude) | - |
| `--agent-bin` | Path to a custom compatible Agent executable | - |
| `--output-dir` | Directory to write results (Required) | - |
| `--task-pattern` | RegEx pattern to filter tasks (e.g. 'pod', 'fix') | - |
| `--llm-provider` | LLM provider ID (e.g. 'gemini', 'openai') | gemini |
| `--models` | Comma-separated list of models | gemini-2.5-pro... |
| `--concurrency` | Number of parallel tasks (0 = auto) | 0 |
| `--cluster-provider` | Cluster provider to use (`kind` or `vcluster`) | kind |
| `--host-cluster-context` | Host cluster context for vcluster (Required if provider is vcluster) | - |

### `analyze` Subcommand
Process and summarize results from previous runs.

```sh
# Generate a Markdown report
./k8s-ai-bench analyze --input-dir .build/k8s-ai-bench --results-filepath report.md

# Generate JSONL for visualization
./k8s-ai-bench analyze --input-dir .build/k8s-ai-bench --output-format jsonl --results-filepath site/combined_results.jsonl
```

## 💻 Development Scripts
For a streamlined development loop, use the scripts in `dev/ci/periodics/`:

- **Run Evaluation Loop**: Runs evaluations multiple times to test consistency.
  ```sh
  ./dev/ci/periodics/run-eval-loop.sh --iterations 5 --task-pattern "create"
  ```
- **Run Single Evaluation**:
  ```sh
  TEST_ARGS="--task-pattern=fix-probes" ./dev/ci/periodics/run-evals.sh
  ```
- **Analyze Results**:
  ```sh
  ./dev/ci/periodics/analyze-evals.sh --show-failures
  ```

## 📈 Visualizing Results Locally

The `site` directory contains a static website (Vue.js based) for visualizing benchmark results.

1.  **Generate Data**:
    ```sh
    ./k8s-ai-bench analyze --input-dir .build/k8s-ai-bench --output-format jsonl --results-filepath site/combined_results.jsonl
    ```
2.  **Serve Locally**:
    ```sh
    cd site
    python3 -m http.server
    ```
3.  **View**: Open [http://localhost:8000](http://localhost:8000)

## 🤝 Contributions
We welcome contributions! Please check out the [contributions guide](contributing.md).
