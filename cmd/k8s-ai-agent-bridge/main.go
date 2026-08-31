// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Minute

type config struct {
	Agent                string
	Provider             string
	BenchmarkModel       string
	Timeout              time.Duration
	CodexBin             string
	CodexModel           string
	ClaudeBin            string
	ClaudeModel          string
	OpenClawBaseURL      string
	OpenClawGatewayToken string
	OpenClawSessionID    string
	OpenClawInsecureTLS  bool
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, http.DefaultClient); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, client *http.Client) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	switch cfg.Agent {
	case "codex":
		return runCLI(callCtx, cfg.CodexBin, codexArgs(cfg.CodexModel), stdin, stdout, stderr, "CODEX_BIN", "codex")
	case "claude":
		return runCLI(callCtx, cfg.ClaudeBin, claudeArgs(cfg.ClaudeModel), stdin, stdout, stderr, "CLAUDE_BIN", "claude")
	case "openclaw":
		return runOpenClaw(callCtx, cfg, stdin, stdout, client)
	default:
		return fmt.Errorf("unsupported agent %q", cfg.Agent)
	}
}

func parseConfig(args []string) (config, error) {
	cfg := config{
		CodexBin:             os.Getenv("CODEX_BIN"),
		CodexModel:           os.Getenv("CODEX_MODEL"),
		ClaudeBin:            os.Getenv("CLAUDE_BIN"),
		ClaudeModel:          os.Getenv("CLAUDE_MODEL"),
		OpenClawBaseURL:      firstNonEmpty(os.Getenv("OPENCLAW_BASE_URL"), os.Getenv("OPENCLAW_API_URL")),
		OpenClawGatewayToken: firstNonEmpty(os.Getenv("OPENCLAW_GATEWAY_TOKEN"), os.Getenv("OPENCLAW_API_KEY")),
		OpenClawSessionID:    firstNonEmpty(os.Getenv("OPENCLAW_SESSION_ID"), os.Getenv("OPENCLAW_MODEL")),
		Timeout:              defaultTimeout,
	}
	if value := strings.TrimSpace(os.Getenv("OPENCLAW_INSECURE_SKIP_VERIFY")); value != "" {
		insecure, err := strconv.ParseBool(value)
		if err != nil {
			return cfg, fmt.Errorf("OPENCLAW_INSECURE_SKIP_VERIFY must be a boolean: %w", err)
		}
		cfg.OpenClawInsecureTLS = insecure
	}

	fs := flag.NewFlagSet("k8s-ai-agent-bridge", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.Agent, "agent", "", "agent connector: codex, claude, or openclaw")
	fs.StringVar(&cfg.Provider, "llm-provider", "", "benchmark provider metadata")
	fs.StringVar(&cfg.BenchmarkModel, "model", "", "benchmark model metadata")
	fs.DurationVar(&cfg.Timeout, "timeout", defaultTimeout, "maximum connector runtime")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	cfg.Agent = normalizeAgent(cfg.Agent)
	if cfg.Agent == "" {
		return cfg, errors.New("--agent is required")
	}
	if cfg.Timeout <= 0 {
		return cfg, errors.New("--timeout must be greater than zero")
	}
	if cfg.Agent == "openclaw" {
		if cfg.OpenClawBaseURL == "" {
			return cfg, errors.New("OPENCLAW_BASE_URL or OPENCLAW_API_URL is required for openclaw")
		}
		if cfg.OpenClawSessionID == "" {
			cfg.OpenClawSessionID = cfg.BenchmarkModel
		}
		if cfg.OpenClawSessionID == "" {
			return cfg, errors.New("OPENCLAW_SESSION_ID or --model is required for openclaw")
		}
	}
	return cfg, nil
}

func normalizeAgent(agent string) string {
	switch strings.ToLower(strings.TrimSpace(agent)) {
	case "codex", "codex-cli":
		return "codex"
	case "claude", "claude-code", "claude-cli":
		return "claude"
	case "openclaw", "openclaw-gateway":
		return "openclaw"
	default:
		return strings.ToLower(strings.TrimSpace(agent))
	}
}

func codexArgs(model string) []string {
	args := []string{"exec", "--ephemeral"}
	if model != "" {
		args = append(args, "--model", model)
	}
	return append(args, "-")
}

func claudeArgs(model string) []string {
	args := []string{"-p", "--output-format", "text"}
	if model != "" {
		args = append(args, "--model", model)
	}
	return args
}

func runCLI(ctx context.Context, configuredBin string, args []string, stdin io.Reader, stdout, stderr io.Writer, envName, defaultName string) error {
	bin, err := resolveBinary(configuredBin, envName, defaultName)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s connector timed out: %w", defaultName, ctx.Err())
		}
		return fmt.Errorf("running %s CLI: %w", defaultName, err)
	}
	return nil
}

func resolveBinary(configuredBin, envName, defaultName string) (string, error) {
	candidate := strings.TrimSpace(configuredBin)
	if candidate == "" {
		candidate = strings.TrimSpace(os.Getenv(envName))
	}
	if candidate == "" {
		candidate = defaultName
	}

	bin, err := exec.LookPath(candidate)
	if err != nil {
		return "", fmt.Errorf("%s CLI %q not found (set %s or add it to PATH): %w", defaultName, candidate, envName, err)
	}
	return bin, nil
}

func runOpenClaw(ctx context.Context, cfg config, stdin io.Reader, stdout io.Writer, client *http.Client) error {
	prompt, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("reading OpenClaw prompt: %w", err)
	}
	if strings.TrimSpace(string(prompt)) == "" {
		return errors.New("OpenClaw prompt is empty")
	}

	body, err := json.Marshal(chatRequest{
		Model: cfg.OpenClawSessionID,
		Messages: []chatMessage{{
			Role:    "user",
			Content: string(prompt),
		}},
		Temperature: 0,
	})
	if err != nil {
		return fmt.Errorf("encoding OpenClaw request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(cfg.OpenClawBaseURL), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating OpenClaw request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if cfg.OpenClawGatewayToken != "" {
		request.Header.Set("Authorization", "Bearer "+cfg.OpenClawGatewayToken)
	}

	response, err := openClawHTTPClient(client, cfg.OpenClawInsecureTLS).Do(request)
	if err != nil {
		return fmt.Errorf("calling OpenClaw gateway: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024))
	if err != nil {
		return fmt.Errorf("reading OpenClaw response: %w", err)
	}

	var parsed chatResponse
	_ = json.Unmarshal(data, &parsed)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(data))
		if parsed.Error != nil && parsed.Error.Message != "" {
			message = parsed.Error.Message
		}
		return fmt.Errorf("OpenClaw gateway returned HTTP %d: %s", response.StatusCode, message)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return fmt.Errorf("OpenClaw gateway error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return errors.New("OpenClaw response has no assistant message")
	}
	_, err = fmt.Fprintln(stdout, strings.TrimSpace(parsed.Choices[0].Message.Content))
	return err
}

func openClawHTTPClient(client *http.Client, insecureTLS bool) *http.Client {
	if !insecureTLS || client == nil {
		return client
	}

	baseTransport := client.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	transport, ok := baseTransport.(*http.Transport)
	if !ok {
		return client
	}
	transport = transport.Clone()
	tlsConfig := transport.TLSClientConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	} else {
		tlsConfig = tlsConfig.Clone()
	}
	tlsConfig.InsecureSkipVerify = true // #nosec G402 -- explicitly opted in for an internal gateway.
	transport.TLSClientConfig = tlsConfig

	configuredClient := *client
	configuredClient.Transport = transport
	return &configuredClient
}

func chatCompletionsURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/chat/completions"
	}
	return baseURL + "/v1/chat/completions"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
