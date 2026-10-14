package config

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the process environment for the public API.
// DATABASE_URL in .env keeps shell placeholders (${POSTGRES_USER}); this
// process does not expand them, so the DSN is built from POSTGRES_* parts.
type Config struct {
	Host               string
	Port               int
	LogLevel           string
	Environment        string
	PostgresHost       string
	PostgresPort       int
	PostgresUser       string
	PostgresPassword   string
	PostgresDB         string
	RAGServiceURL      string
	RAGClientTimeout   time.Duration
	CORSAllowedOrigins string
}

func (c Config) CORSOrigins() []string {
	parts := strings.Split(c.CORSAllowedOrigins, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (c Config) HTTPAddr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func (c Config) DatabaseURL() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.PostgresUser, c.PostgresPassword),
		Host:   net.JoinHostPort(c.PostgresHost, strconv.Itoa(c.PostgresPort)),
		Path:   "/" + c.PostgresDB,
	}
	q := u.Query()
	// Compose Postgres is local and does not terminate TLS.
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return u.String()
}

func Load() (Config, error) {
	file := map[string]string{}
	if root := findRepoRoot(); root != "" {
		file = parseEnvFile(filepath.Join(root, ".env"))
	}
	process := processEnv()
	get := func(key, fallback string) string {
		if value, ok := process[key]; ok {
			return value
		}
		if value, ok := file[key]; ok {
			return value
		}
		return fallback
	}

	port, err := parsePort(get("API_PORT", "8080"), "API_PORT")
	if err != nil {
		return Config{}, err
	}
	pgPort, err := parsePort(get("POSTGRES_PORT", "5432"), "POSTGRES_PORT")
	if err != nil {
		return Config{}, err
	}
	timeout, err := parseTimeout(get("RAG_CLIENT_TIMEOUT_SECONDS", "180"))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Host:               get("API_HOST", "0.0.0.0"),
		Port:               port,
		LogLevel:           get("LOG_LEVEL", "info"),
		Environment:        get("ENVIRONMENT", "development"),
		PostgresHost:       get("POSTGRES_HOST", "localhost"),
		PostgresPort:       pgPort,
		PostgresUser:       get("POSTGRES_USER", "knowledge"),
		PostgresPassword:   get("POSTGRES_PASSWORD", "change_me"),
		PostgresDB:         get("POSTGRES_DB", "knowledge_assistant"),
		RAGServiceURL:      strings.TrimRight(get("RAG_SERVICE_URL", "http://localhost:8000"), "/"),
		RAGClientTimeout:   timeout,
		CORSAllowedOrigins: get("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000"),
	}
	if cfg.PostgresUser == "" || cfg.PostgresDB == "" {
		return Config{}, fmt.Errorf("POSTGRES_USER and POSTGRES_DB are required")
	}
	if _, err := url.ParseRequestURI(cfg.RAGServiceURL); err != nil {
		return Config{}, fmt.Errorf("RAG_SERVICE_URL: %w", err)
	}
	return cfg, nil
}

func parsePort(raw, name string) (int, error) {
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s must be a TCP port", name)
	}
	return port, nil
}

func parseTimeout(raw string) (time.Duration, error) {
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 1 {
		return 0, fmt.Errorf("RAG_CLIENT_TIMEOUT_SECONDS must be a positive number of seconds")
	}
	return time.Duration(seconds) * time.Second, nil
}

func processEnv() map[string]string {
	out := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			out[key] = value
		}
	}
	return out
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func parseEnvFile(path string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		return map[string]string{}
	}
	defer file.Close()

	out := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if key != "" {
			out[key] = value
		}
	}
	return out
}
