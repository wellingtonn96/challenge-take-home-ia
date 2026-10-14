package config

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrefersProcessEnv(t *testing.T) {
	t.Setenv("API_HOST", "127.0.0.1")
	t.Setenv("API_PORT", "9090")
	t.Setenv("POSTGRES_HOST", "dbhost")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "from-env")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_DB", "knowledge_assistant")
	t.Setenv("RAG_SERVICE_URL", "http://rag:8000/")
	t.Setenv("RAG_CLIENT_TIMEOUT_SECONDS", "15")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr() != "127.0.0.1:9090" {
		t.Fatalf("addr = %s", cfg.HTTPAddr())
	}
	if cfg.RAGServiceURL != "http://rag:8000" {
		t.Fatalf("rag url = %s", cfg.RAGServiceURL)
	}
	if cfg.RAGClientTimeout.Seconds() != 15 {
		t.Fatalf("timeout = %s", cfg.RAGClientTimeout)
	}

	parsed, err := url.Parse(cfg.DatabaseURL())
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if parsed.Hostname() != "dbhost" || parsed.Port() != "5433" {
		t.Fatalf("host = %s", parsed.Host)
	}
	if parsed.User.Username() != "from-env" {
		t.Fatalf("user = %s", parsed.User.Username())
	}
	password, _ := parsed.User.Password()
	if password != "secret" {
		t.Fatal("password was not taken from the environment")
	}
	if parsed.Query().Get("sslmode") != "disable" {
		t.Fatalf("sslmode = %s", parsed.RawQuery)
	}
}

func TestLoadRejectsBadPort(t *testing.T) {
	t.Setenv("API_PORT", "nope")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseEnvFileQuotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := []byte("# comment\n\nFOO=bar\nBAZ=\"qux\"\r\nEMPTY=\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	got := parseEnvFile(path)
	if got["FOO"] != "bar" || got["BAZ"] != "qux" || got["EMPTY"] != "" {
		t.Fatalf("parsed = %#v", got)
	}
}
