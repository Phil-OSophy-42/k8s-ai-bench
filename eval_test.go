package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gke-labs/k8s-ai-bench/pkg/model"
)

func TestAgentCommandAdapters(t *testing.T) {
	x := &TaskExecution{
		kubeConfig: "/tmp/kubeconfig",
		agentConfig: AgentConfig{
			ID:      "kubectl-ai",
			Bin:     "/bin/kubectl-ai",
			Adapter: "kubectl-ai",
		},
		llmConfig: model.LLMConfig{
			ProviderID:        "gemini",
			ModelID:           "gemini-2.5-pro",
			EnableToolUseShim: true,
			Quiet:             true,
			McpClient:         true,
		},
	}

	bin, args, err := x.agentCommand("/tmp/trace.yaml")
	if err != nil {
		t.Fatalf("agentCommand returned error: %v", err)
	}
	if bin != "/bin/kubectl-ai" {
		t.Fatalf("bin = %q, want /bin/kubectl-ai", bin)
	}
	if !argvContainsAll(args, []string{"--kubeconfig", "/tmp/kubeconfig", "--model", "gemini-2.5-pro", "--mcp-client"}) {
		t.Fatalf("kubectl-ai args missing expected values: %#v", args)
	}

	x.agentConfig = AgentConfig{
		ID:      "generic",
		Bin:     "/bin/agent",
		Adapter: "generic-stdin",
		Args:    []string{"run", "--quiet"},
	}
	bin, args, err = x.agentCommand("/tmp/trace.yaml")
	if err != nil {
		t.Fatalf("generic agentCommand returned error: %v", err)
	}
	if bin != "/bin/agent" || !argvContainsAll(args, []string{"run", "--quiet", "--llm-provider", "gemini", "--model", "gemini-2.5-pro"}) {
		t.Fatalf("generic command = %q %#v, want provider/model args", bin, args)
	}

	x.agentConfig = AgentConfig{
		ID:      "dce",
		Bin:     "/bin/dce",
		Adapter: "direct-cli",
		Args:    []string{"container-management"},
	}
	bin, args, err = x.agentCommand("/tmp/trace.yaml")
	if err != nil {
		t.Fatalf("direct-cli agentCommand returned error: %v", err)
	}
	if bin != "/bin/dce" || len(args) != 1 || args[0] != "container-management" {
		t.Fatalf("direct-cli command = %q %#v, want /bin/dce [container-management]", bin, args)
	}
}

func TestComposePromptInjectsSkillAndCLI(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "kube-debug")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Use kube-inspector for pod diagnosis."), 0644); err != nil {
		t.Fatal(err)
	}

	x := &TaskExecution{
		skillsDir: filepath.Join(dir, "skills"),
		task: &Task{
			Skills: []string{"kube-debug/SKILL.md"},
			CLIs:   []CLIRef{{Name: "kube-inspector", Path: "kube-inspector"}},
		},
	}

	prompt, err := x.composePrompt("Diagnose pod app.")
	if err != nil {
		t.Fatalf("composePrompt returned error: %v", err)
	}
	for _, want := range []string{"<skill name=\"kube-debug\">", "Use kube-inspector", "Available CLI tools:", "- kube-inspector", "Task:\nDiagnose pod app."} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestCLIWrapperAuditsCalls(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for CLI wrapper test")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for CLI wrapper test")
	}

	dir := t.TempDir()
	realCLI := filepath.Join(dir, "clis", "kube-inspector")
	if err := os.MkdirAll(filepath.Dir(realCLI), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(realCLI, []byte("#!/usr/bin/env bash\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	x := &TaskExecution{
		clisDir:       filepath.Join(dir, "clis"),
		taskOutputDir: filepath.Join(dir, "out"),
		task: &Task{
			CLIs: []CLIRef{{Name: "kube-inspector", Path: "kube-inspector"}},
		},
	}
	wrapperDir := filepath.Join(dir, "wrappers")
	auditPath := filepath.Join(dir, "audit.jsonl")
	if err := x.createCLIWrappers(wrapperDir, auditPath); err != nil {
		t.Fatalf("createCLIWrappers returned error: %v", err)
	}

	cmd := exec.CommandContext(context.Background(), filepath.Join(wrapperDir, "kube-inspector"), "inspect", "pod", "app")
	cmd.Env = append(os.Environ(), "K8S_AI_BENCH_CLI_AUDIT="+auditPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("wrapper command failed: %v", err)
	}

	calls, err := readCLIAudit(auditPath)
	if err != nil {
		t.Fatalf("readCLIAudit returned error: %v", err)
	}
	if len(calls) != 1 || calls[0].Name != "kube-inspector" || !argvContainsAll(calls[0].Argv, []string{"inspect", "pod", "app"}) {
		t.Fatalf("unexpected audit calls: %#v", calls)
	}
}

func TestEvaluateTaskRequiresCLIsDirForLocalCLIs(t *testing.T) {
	config := EvalConfig{
		MatrixFile: "eval-matrix.yaml",
		Agents: map[string]AgentConfig{
			"generic": {
				ID:      "generic",
				Bin:     "/bin/true",
				Adapter: "generic-stdin",
			},
		},
		OutputDir: t.TempDir(),
	}
	task := Task{
		Agent: "generic",
		CLIs:  []CLIRef{{Name: "kube-inspector", Path: "kube-inspector"}},
		Script: []ScriptStep{{
			Prompt: "Diagnose pod app.",
		}},
	}

	result := evaluateTask(context.Background(), config, "local-cli-task", task, model.LLMConfig{ID: "test"}, 1, nil, nil)
	if result.Result != "error" {
		t.Fatalf("result = %q, want error", result.Result)
	}
	if !strings.Contains(result.Error, "declares local CLIs but matrix clisDir is not set") {
		t.Fatalf("unexpected error: %q", result.Error)
	}
}

func TestEvaluateTaskRequiresSkillsDirForLocalSkills(t *testing.T) {
	config := EvalConfig{
		MatrixFile: "eval-matrix.yaml",
		Agents: map[string]AgentConfig{
			"generic": {
				ID:      "generic",
				Bin:     "/bin/true",
				Adapter: "generic-stdin",
			},
		},
		OutputDir: t.TempDir(),
	}
	task := Task{
		Agent:  "generic",
		Skills: []string{"kube-debug/SKILL.md"},
		Script: []ScriptStep{{
			Prompt: "Diagnose pod app.",
		}},
	}

	result := evaluateTask(context.Background(), config, "local-skill-task", task, model.LLMConfig{ID: "test"}, 1, nil, nil)
	if result.Result != "error" {
		t.Fatalf("result = %q, want error", result.Result)
	}
	if !strings.Contains(result.Error, "declares local skills but matrix skillsDir is not set") {
		t.Fatalf("unexpected error: %q", result.Error)
	}
}

func TestResolveTaskAgentUsesRunAgentOverride(t *testing.T) {
	config := EvalConfig{Agent: "codex"}
	if got := resolveTaskAgent(config, Task{Agent: "generic"}); got != "codex" {
		t.Fatalf("resolveTaskAgent = %q, want codex", got)
	}
}

func TestTaskLifecycleEnvUsesRequiredEnvAndPrecedence(t *testing.T) {
	trueBin, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), "lifecycle path with spaces")
	taskDir := filepath.Join(dir, "tasks", "lifecycle-task")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	for name, output := range map[string]string{
		"setup.sh":    "setup.env",
		"verifier.sh": "verifier.env",
		"cleanup.sh":  "cleanup.env",
	} {
		outputEnv := strings.ToUpper(strings.TrimSuffix(name, ".sh")) + "_OUTPUT_PATH"
		t.Setenv(outputEnv, filepath.Join(dir, output))
		script := fmt.Sprintf(`#!/bin/sh
printf '%%s|%%s|%%s|%%s|%%s|%%s|%%s|%%s|%%s|%%s' \
  "${LIFECYCLE_SHARED-unset}" \
  "${LIFECYCLE_AGENT_ONLY-unset}" \
  "${LIFECYCLE_MODEL_ONLY-unset}" \
  "${DCE_HOST-unset}" \
  "${DCE_TOKEN-unset}" \
  "${DCE_HOSTNAME-unset}" \
  "${DCE_TOKEN_FILE-unset}" \
  "${KUBECONFIG-unset}" \
  "${K8S_AI_BENCH_TASK_OUTPUT_DIR-unset}" \
  "${K8S_AI_BENCH_CLI_AUDIT-unset}" > "${%s}"
`, outputEnv)
		if err := os.WriteFile(filepath.Join(taskDir, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("LIFECYCLE_SHARED", "process-value")
	t.Setenv("KUBECONFIG", "process-kubeconfig")
	t.Setenv("K8S_AI_BENCH_TASK_OUTPUT_DIR", "process-output")
	t.Setenv("K8S_AI_BENCH_CLI_AUDIT", "process-audit")

	config := EvalConfig{
		TasksDir:   filepath.Join(dir, "tasks"),
		OutputDir:  filepath.Join(dir, "output"),
		KubeConfig: filepath.Join(dir, "fixed-kubeconfig"),
		Agents: map[string]AgentConfig{
			"test-agent": {
				ID:      "test-agent",
				Bin:     trueBin,
				Adapter: "generic-stdin",
				Env: map[string]string{
					"LIFECYCLE_SHARED":             "agent-value",
					"LIFECYCLE_AGENT_ONLY":         "agent-only",
					"DCE_HOST":                     "agent-host",
					"DCE_TOKEN":                    "agent-token",
					"DCE_HOSTNAME":                 "agent-hostname",
					"DCE_TOKEN_FILE":               "agent-token-file",
					"KUBECONFIG":                   "agent-kubeconfig",
					"K8S_AI_BENCH_TASK_OUTPUT_DIR": "agent-output",
					"K8S_AI_BENCH_CLI_AUDIT":       "agent-audit",
				},
			},
		},
	}

	task := Task{
		Agent:    "test-agent",
		Setup:    "setup.sh",
		Verifier: "verifier.sh",
		Cleanup:  "cleanup.sh",
	}
	result := evaluateTask(context.Background(), config, "lifecycle-task", task, model.LLMConfig{
		ID: "test-model",
		Env: map[string]string{
			"LIFECYCLE_SHARED":             "model-value",
			"LIFECYCLE_MODEL_ONLY":         "model-only",
			"DCE_HOST":                     "model-host",
			"DCE_TOKEN":                    "model-token",
			"DCE_HOSTNAME":                 "model-hostname",
			"DCE_TOKEN_FILE":               "model-token-file",
			"KUBECONFIG":                   "model-kubeconfig",
			"K8S_AI_BENCH_TASK_OUTPUT_DIR": "model-output",
			"K8S_AI_BENCH_CLI_AUDIT":       "model-audit",
		},
	}, 1, nil, nil)
	if result.Result != "success" {
		t.Fatalf("result = %q, want success; error = %q; failures = %#v", result.Result, result.Error, result.Failures)
	}

	expectedTaskOutputDir := filepath.Join(config.OutputDir, "iteration-1", "lifecycle-task", "test-agent", "test-model", "task-skills")
	expectedAuditPath := filepath.Join(expectedTaskOutputDir, "cli-audit.jsonl")
	for _, testCase := range []struct {
		name          string
		expectedAudit string
	}{
		{name: "setup.env", expectedAudit: "model-audit"},
		{name: "verifier.env", expectedAudit: expectedAuditPath},
		{name: "cleanup.env", expectedAudit: "model-audit"},
	} {
		got, err := os.ReadFile(filepath.Join(dir, testCase.name))
		if err != nil {
			t.Fatalf("reading %s: %v", testCase.name, err)
		}
		expected := fmt.Sprintf("model-value|agent-only|model-only|model-host|model-token|model-hostname|model-token-file|%s|%s|%s", config.KubeConfig, expectedTaskOutputDir, testCase.expectedAudit)
		if string(got) != expected {
			t.Errorf("%s = %q, want %q", testCase.name, got, expected)
		}
	}
}

func TestEvaluateCLIExpectationsRequiredFailure(t *testing.T) {
	result := &model.TaskResult{}
	x := &TaskExecution{
		result:        result,
		taskOutputDir: t.TempDir(),
		task: &Task{
			CLIExpect: []CLIExpectation{{
				Name:         "kube-inspector",
				Required:     true,
				ArgvContains: []string{"inspect"},
			}},
		},
	}

	failures := x.evaluateCLIExpectations()
	if len(failures) != 1 {
		t.Fatalf("failures = %d, want 1", len(failures))
	}
	if len(result.CLIResults) != 1 || result.CLIResults[0].Called {
		t.Fatalf("unexpected CLIResults: %#v", result.CLIResults)
	}
}

func TestAppendCLIAudit(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "cli-audit.jsonl")
	if err := appendCLIAudit(auditPath, model.CLICall{
		Name:     "dce",
		Argv:     []string{"container-management", "cluster", "get-cluster"},
		ExitCode: 0,
	}); err != nil {
		t.Fatalf("appendCLIAudit returned error: %v", err)
	}
	calls, err := readCLIAudit(auditPath)
	if err != nil {
		t.Fatalf("readCLIAudit returned error: %v", err)
	}
	if len(calls) != 1 || calls[0].Name != "dce" || !argvContainsAll(calls[0].Argv, []string{"container-management", "get-cluster"}) {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}
