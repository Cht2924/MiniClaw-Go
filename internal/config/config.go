package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	LLM       LLMConfig
	Agent     AgentConfig
	Workspace RootConfig
	Memory    RootConfig
	Skills    RootConfig
	Sessions  RootConfig
	Traces    RootConfig
	Tools     ToolsConfig
	MCP       MCPConfig
}

type LLMConfig struct {
	Provider         string
	BaseURL          string
	APIKey           string
	Model            string
	MaxContextTokens int
	Temperature      float64
}

type AgentConfig struct {
	MaxSteps                  int
	SummaryKeepRecent         int
	SummarizeMessageThreshold int
	SummarizeTokenPercent     int
}

type RootConfig struct {
	Root string
}

type ToolsConfig struct {
	CommandTimeout time.Duration
}

type MCPConfig struct {
	Servers []MCPServerConfig
}

type MCPServerConfig struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

func Load(path string) (*Config, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := defaultConfig()
	if err := parseYAMLSubset(string(contents), cfg); err != nil {
		return nil, err
	}

	applyEnvOverrides(cfg)
	resolveRelativeRoots(path, cfg)

	if cfg.LLM.APIKey == "" {
		cfg.LLM.APIKey = os.Getenv("OPENAI_API_KEY")
	}
	if cfg.LLM.APIKey == "" {
		cfg.LLM.APIKey = os.Getenv("ARK_API_KEY")
	}
	if cfg.LLM.BaseURL == "" {
		cfg.LLM.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = "gpt-4o-mini"
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider:         "openai-compatible",
			BaseURL:          "https://api.openai.com/v1",
			Model:            "gpt-4o-mini",
			MaxContextTokens: 8192,
			Temperature:      0.2,
		},
		Agent: AgentConfig{
			MaxSteps:                  8,
			SummaryKeepRecent:         4,
			SummarizeMessageThreshold: 20,
			SummarizeTokenPercent:     75,
		},
		Workspace: RootConfig{Root: "../workspace"},
		Memory:    RootConfig{Root: "../memory"},
		Skills:    RootConfig{Root: "../skills"},
		Sessions:  RootConfig{Root: "../sessions"},
		Traces:    RootConfig{Root: "../traces"},
		Tools: ToolsConfig{
			CommandTimeout: 10 * time.Second,
		},
		MCP: MCPConfig{Servers: nil},
	}
}

func resolveRelativeRoots(configPath string, cfg *Config) {
	base := filepath.Dir(configPath)
	cfg.Workspace.Root = resolvePath(base, cfg.Workspace.Root)
	cfg.Memory.Root = resolvePath(base, cfg.Memory.Root)
	cfg.Skills.Root = resolvePath(base, cfg.Skills.Root)
	cfg.Sessions.Root = resolvePath(base, cfg.Sessions.Root)
	cfg.Traces.Root = resolvePath(base, cfg.Traces.Root)
}

func resolvePath(base, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(base, path))
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("MINICLAW_BASE_URL"); v != "" {
		cfg.LLM.BaseURL = v
	}
	if v := os.Getenv("MINICLAW_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
}

func parseYAMLSubset(contents string, cfg *Config) error {
	lines := strings.Split(contents, "\n")
	var section string
	var inMCPServers bool
	var currentServer *MCPServerConfig

	flushServer := func() {
		if currentServer == nil {
			return
		}
		if currentServer.Env == nil {
			currentServer.Env = map[string]string{}
		}
		cfg.MCP.Servers = append(cfg.MCP.Servers, *currentServer)
		currentServer = nil
	}

	for idx, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if comment := strings.Index(line, "#"); comment >= 0 {
			line = line[:comment]
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		indent := countLeadingSpaces(line)
		trimmed := strings.TrimSpace(line)

		switch {
		case indent == 0 && strings.HasSuffix(trimmed, ":"):
			flushServer()
			section = strings.TrimSuffix(trimmed, ":")
			inMCPServers = false
		case section == "mcp" && indent == 2 && trimmed == "servers:":
			inMCPServers = true
		case inMCPServers && indent >= 4 && strings.HasPrefix(trimmed, "- "):
			flushServer()
			currentServer = &MCPServerConfig{Env: map[string]string{}}
			if err := assignServerField(currentServer, strings.TrimPrefix(trimmed, "- ")); err != nil {
				return fmt.Errorf("config line %d: %w", idx+1, err)
			}
		case inMCPServers && indent >= 6:
			if currentServer == nil {
				return fmt.Errorf("config line %d: mcp server field without server item", idx+1)
			}
			if err := assignServerField(currentServer, trimmed); err != nil {
				return fmt.Errorf("config line %d: %w", idx+1, err)
			}
		case indent == 2:
			if err := assignSectionField(cfg, section, trimmed); err != nil {
				return fmt.Errorf("config line %d: %w", idx+1, err)
			}
		default:
			return fmt.Errorf("config line %d: unsupported structure", idx+1)
		}
	}

	flushServer()
	return nil
}

func assignSectionField(cfg *Config, section, line string) error {
	key, value, ok := splitKV(line)
	if !ok {
		return fmt.Errorf("invalid key/value pair: %s", line)
	}

	switch section {
	case "llm":
		switch key {
		case "provider":
			cfg.LLM.Provider = trimQuotes(value)
		case "base_url":
			cfg.LLM.BaseURL = trimQuotes(value)
		case "api_key":
			cfg.LLM.APIKey = trimQuotes(value)
		case "model":
			cfg.LLM.Model = trimQuotes(value)
		case "max_context_tokens":
			v, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			cfg.LLM.MaxContextTokens = v
		case "temperature":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			cfg.LLM.Temperature = v
		default:
			return fmt.Errorf("unknown llm key: %s", key)
		}
	case "agent":
		v, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		switch key {
		case "max_steps":
			cfg.Agent.MaxSteps = v
		case "summary_keep_recent":
			cfg.Agent.SummaryKeepRecent = v
		case "summarize_message_threshold":
			cfg.Agent.SummarizeMessageThreshold = v
		case "summarize_token_percent":
			cfg.Agent.SummarizeTokenPercent = v
		default:
			return fmt.Errorf("unknown agent key: %s", key)
		}
	case "workspace":
		if key != "root" {
			return fmt.Errorf("unknown workspace key: %s", key)
		}
		cfg.Workspace.Root = trimQuotes(value)
	case "memory":
		if key != "root" {
			return fmt.Errorf("unknown memory key: %s", key)
		}
		cfg.Memory.Root = trimQuotes(value)
	case "skills":
		if key != "root" {
			return fmt.Errorf("unknown skills key: %s", key)
		}
		cfg.Skills.Root = trimQuotes(value)
	case "sessions":
		if key != "root" {
			return fmt.Errorf("unknown sessions key: %s", key)
		}
		cfg.Sessions.Root = trimQuotes(value)
	case "traces":
		if key != "root" {
			return fmt.Errorf("unknown traces key: %s", key)
		}
		cfg.Traces.Root = trimQuotes(value)
	case "tools":
		if key != "command_timeout" {
			return fmt.Errorf("unknown tools key: %s", key)
		}
		d, err := time.ParseDuration(trimQuotes(value))
		if err != nil {
			return err
		}
		cfg.Tools.CommandTimeout = d
	default:
		return fmt.Errorf("unknown section: %s", section)
	}

	return nil
}

func assignServerField(server *MCPServerConfig, line string) error {
	key, value, ok := splitKV(line)
	if !ok {
		return fmt.Errorf("invalid mcp server field: %s", line)
	}

	switch key {
	case "name":
		server.Name = trimQuotes(value)
	case "command":
		server.Command = trimQuotes(value)
	case "args":
		var args []string
		if err := json.Unmarshal([]byte(value), &args); err != nil {
			return fmt.Errorf("parse args: %w", err)
		}
		server.Args = args
	case "env":
		var env map[string]string
		if err := json.Unmarshal([]byte(value), &env); err != nil {
			return fmt.Errorf("parse env: %w", err)
		}
		server.Env = env
	default:
		return fmt.Errorf("unknown mcp server key: %s", key)
	}

	return nil
}

func splitKV(line string) (string, string, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func trimQuotes(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, `"`)
	value = strings.TrimSuffix(value, `"`)
	value = strings.TrimPrefix(value, `'`)
	value = strings.TrimSuffix(value, `'`)
	return value
}

func countLeadingSpaces(s string) int {
	count := 0
	for _, ch := range s {
		if ch != ' ' {
			break
		}
		count++
	}
	return count
}
