package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultFWSBaseURL = "http://localhost:8000"
	DefaultStatePath  = "data/codex_sessions.json"
	DefaultCodexModel = "gpt-5.1"
	DefaultReasoning  = "minimal"
	DefaultAlarmEvent = "discord_alarm"
)

// Config represents runtime configuration from environment variables.
type Config struct {
	DiscordToken        string
	TranscriptChannelID string
	FWSBaseURL          string
	StatePath           string
	GeminiAPIKey        string
	CodexModel          string
	ReasoningEffort     string
	CodexWorkdir        string
	HomeAssistantToken  string
	HomeAssistantBase   string
	HomeAssistantEvent  string
}

// Load reads configuration from environment variables and validates it.
func Load() (Config, error) {
	cfg := Config{
		DiscordToken:        os.Getenv("DISCORD_TOKEN"),
		TranscriptChannelID: os.Getenv("TRANSCRIPT_CHANNEL_ID"),
		FWSBaseURL:          os.Getenv("FWS_BASE_URL"),
		StatePath:           os.Getenv("CODEX_STATE_PATH"),
		GeminiAPIKey:        os.Getenv("GEMINI_API_KEY"),
		CodexModel:          os.Getenv("CODEX_MODEL"),
		ReasoningEffort:     os.Getenv("CODEX_REASONING_EFFORT"),
		CodexWorkdir:        os.Getenv("CODEX_WORKDIR"),
		HomeAssistantToken:  os.Getenv("HOME_ASSISTANT_TOKEN"),
		HomeAssistantBase:   os.Getenv("HOME_ASSISTANT_BASE_URL"),
		HomeAssistantEvent:  os.Getenv("HOME_ASSISTANT_ALARM_EVENT"),
	}

	if cfg.FWSBaseURL == "" {
		cfg.FWSBaseURL = DefaultFWSBaseURL
	}
	if cfg.StatePath == "" {
		cfg.StatePath = DefaultStatePath
	}
	if cfg.CodexModel == "" {
		cfg.CodexModel = DefaultCodexModel
	}
	if cfg.ReasoningEffort == "" {
		cfg.ReasoningEffort = DefaultReasoning
	}
	if cfg.HomeAssistantEvent == "" {
		cfg.HomeAssistantEvent = DefaultAlarmEvent
	}

	if cfg.CodexWorkdir != "" {
		workdir, err := expandPath(cfg.CodexWorkdir)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CODEX_WORKDIR: %w", err)
		}
		cfg.CodexWorkdir = workdir
		if err := os.MkdirAll(cfg.CodexWorkdir, 0o755); err != nil {
			return Config{}, fmt.Errorf("failed to create CODEX_WORKDIR %q: %w", cfg.CodexWorkdir, err)
		}
	}

	var missing []string
	if cfg.DiscordToken == "" {
		missing = append(missing, "DISCORD_TOKEN")
	}
	if cfg.TranscriptChannelID == "" {
		missing = append(missing, "TRANSCRIPT_CHANNEL_ID")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}

func expandPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	path = os.ExpandEnv(path)

	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return filepath.Abs(path)
}
