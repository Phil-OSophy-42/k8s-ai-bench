package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/gke-labs/k8s-ai-bench/pkg/model"
)

type AgentProfile struct {
	Name         string
	Description  string
	Binary       string
	ProviderID   string
	DefaultModel string
	BuildArgs    func(model.LLMConfig, string, string) []string
}

var builtInAgentProfiles = map[string]AgentProfile{
	"kubectl-ai": {
		Name:         "kubectl-ai",
		Description:  "Kubernetes-focused AI agent",
		Binary:       "kubectl-ai",
		ProviderID:   "gemini",
		DefaultModel: "gemini-2.5-pro",
		BuildArgs:    buildKubectlAIArgs,
	},
	"codex": {
		Name:         "codex",
		Description:  "OpenAI Codex CLI",
		Binary:       "codex",
		ProviderID:   "codex",
		DefaultModel: "default",
		BuildArgs:    buildCodexArgs,
	},
	"claude": {
		Name:         "claude",
		Description:  "Anthropic Claude Code CLI",
		Binary:       "claude",
		ProviderID:   "claude",
		DefaultModel: "default",
		BuildArgs:    buildClaudeArgs,
	},
}

func agentProfileByName(name string) (AgentProfile, bool) {
	profile, ok := builtInAgentProfiles[name]
	return profile, ok
}

func supportedAgentNames() []string {
	names := make([]string, 0, len(builtInAgentProfiles))
	for name := range builtInAgentProfiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func customAgentProfile(binary string) AgentProfile {
	return AgentProfile{
		Name:         "custom",
		Description:  "Custom kubectl-ai-compatible agent",
		Binary:       binary,
		ProviderID:   "gemini",
		DefaultModel: "gemini-2.5-pro",
		BuildArgs:    buildKubectlAIArgs,
	}
}

func formatSupportedAgents() string {
	return strings.Join(supportedAgentNames(), ", ")
}

func resolveAgent(name, customBinary string) (AgentProfile, error) {
	if name != "" && customBinary != "" {
		return AgentProfile{}, fmt.Errorf("cannot use --agent and --agent-bin together")
	}
	if name == "" {
		if customBinary == "" {
			return AgentProfile{}, fmt.Errorf("must set --agent or --agent-bin")
		}
		return customAgentProfile(customBinary), nil
	}

	profile, ok := agentProfileByName(name)
	if !ok {
		return AgentProfile{}, fmt.Errorf("unknown agent %q; supported agents: %s; use --agent-bin for a custom agent", name, formatSupportedAgents())
	}
	return profile, nil
}

func validateAgentExecutable(profile AgentProfile) error {
	if strings.ContainsRune(profile.Binary, os.PathSeparator) {
		info, err := os.Stat(profile.Binary)
		if err != nil {
			return fmt.Errorf("agent %q is not available at %q: %w", profile.Name, profile.Binary, err)
		}
		if info.IsDir() || info.Mode()&0111 == 0 {
			return fmt.Errorf("agent %q at %q is not executable", profile.Name, profile.Binary)
		}
		return nil
	}

	if _, err := exec.LookPath(profile.Binary); err != nil {
		return fmt.Errorf("agent %q executable %q was not found in PATH; install it or use --agent-bin with a compatible executable", profile.Name, profile.Binary)
	}
	return nil
}

func buildKubectlAIArgs(config model.LLMConfig, kubeconfig, tracePath string) []string {
	args := []string{
		"--kubeconfig", kubeconfig,
		"--llm-provider", config.ProviderID,
		fmt.Sprintf("--enable-tool-use-shim=%t", config.EnableToolUseShim),
		fmt.Sprintf("--quiet=%t", config.Quiet),
		"--model", config.ModelID,
		"--trace-path", tracePath,
		"--skip-permissions",
		"--show-tool-output",
	}
	if config.McpClient {
		args = append(args, "--mcp-client")
	}
	return args
}

func buildCodexArgs(config model.LLMConfig, _, _ string) []string {
	args := []string{"exec", "--ephemeral", "--sandbox", "danger-full-access"}
	if config.ModelID != "" && config.ModelID != "default" {
		args = append(args, "--model", config.ModelID)
	}
	return append(args, "-")
}

func buildClaudeArgs(config model.LLMConfig, _, _ string) []string {
	args := []string{"-p", "--dangerously-skip-permissions"}
	if config.ModelID != "" && config.ModelID != "default" {
		args = append(args, "--model", config.ModelID)
	}
	return args
}
