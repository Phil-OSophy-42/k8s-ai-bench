package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gke-labs/k8s-ai-bench/pkg/model"
)

func TestAgentProfileBuildArgs(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantBin   string
		wantStart []string
		wantNo    []string
	}{
		{
			name:      "kubectl-ai",
			model:     "gemini-2.5-pro",
			wantBin:   "kubectl-ai",
			wantStart: []string{"--kubeconfig", "kubeconfig.yaml", "--llm-provider", "gemini", "--model", "gemini-2.5-pro", "--trace-path", "trace.yaml", "--skip-permissions", "--show-tool-output"},
		},
		{
			name:      "codex",
			model:     "gpt-5.6",
			wantBin:   "codex",
			wantStart: []string{"exec", "--ephemeral", "--sandbox", "danger-full-access", "--model", "gpt-5.6", "-"},
			wantNo:    []string{"--kubeconfig", "--trace-path", "--llm-provider"},
		},
		{
			name:      "claude",
			model:     "claude-sonnet",
			wantBin:   "claude",
			wantStart: []string{"-p", "--dangerously-skip-permissions", "--model", "claude-sonnet"},
			wantNo:    []string{"--kubeconfig", "--trace-path", "--llm-provider"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, ok := agentProfileByName(tt.name)
			if !ok {
				t.Fatalf("agentProfileByName(%q) not found", tt.name)
			}

			args := profile.BuildArgs(model.LLMConfig{
				ProviderID: "gemini",
				ModelID:    tt.model,
				Quiet:      true,
			}, "kubeconfig.yaml", "trace.yaml")

			if profile.Binary != tt.wantBin {
				t.Fatalf("binary = %q, want %q", profile.Binary, tt.wantBin)
			}
			assertArgsContain(t, args, tt.wantStart)
			assertArgsNotContain(t, args, tt.wantNo)
		})
	}
}

func TestAgentProfileBuildArgsOmitDefaultModels(t *testing.T) {
	for _, name := range []string{"codex", "claude"} {
		t.Run(name, func(t *testing.T) {
			profile, ok := agentProfileByName(name)
			if !ok {
				t.Fatalf("agentProfileByName(%q) not found", name)
			}

			args := profile.BuildArgs(model.LLMConfig{ModelID: "default"}, "", "")
			assertArgsNotContain(t, args, []string{"--model"})
		})
	}
}

func TestAgentProfileNamesAreSortedAndComplete(t *testing.T) {
	got := supportedAgentNames()
	want := []string{"claude", "codex", "kubectl-ai"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("supportedAgentNames() = %#v, want %#v", got, want)
	}
}

func TestResolveAgentCustomBinary(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "custom-agent")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	profile, err := resolveAgent("", binary)
	if err != nil {
		t.Fatalf("resolve custom agent: %v", err)
	}
	if profile.Binary != binary {
		t.Fatalf("custom binary = %q, want %q", profile.Binary, binary)
	}
}

func TestResolveAgentRejectsConflict(t *testing.T) {
	_, err := resolveAgent("codex", "/tmp/custom-agent")
	if err == nil || !strings.Contains(err.Error(), "cannot use --agent and --agent-bin together") {
		t.Fatalf("unexpected conflict error: %v", err)
	}
}

func TestResolveAgentRejectsUnknownName(t *testing.T) {
	_, err := resolveAgent("unknown", "")
	if err == nil || !strings.Contains(err.Error(), "claude") ||
		!strings.Contains(err.Error(), "codex") ||
		!strings.Contains(err.Error(), "kubectl-ai") {
		t.Fatalf("unexpected unknown-agent error: %v", err)
	}
}

func assertArgsContain(t *testing.T, args, expected []string) {
	t.Helper()
	next := 0
	for _, arg := range args {
		if next < len(expected) && arg == expected[next] {
			next++
		}
	}
	if next != len(expected) {
		t.Fatalf("args %#v do not contain ordered sequence %#v", args, expected)
	}
}

func assertArgsNotContain(t *testing.T, args, forbidden []string) {
	t.Helper()
	for _, arg := range args {
		for _, forbiddenArg := range forbidden {
			if arg == forbiddenArg {
				t.Fatalf("args %#v contain forbidden flag %q", args, arg)
			}
		}
	}
}
