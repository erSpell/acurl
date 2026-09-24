package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func flags() (*pflag.FlagSet, *time.Duration, *[]string, *[]string, *string) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	timeout := fs.Duration("timeout", 30*time.Second, "")
	agents := fs.StringSlice("agents", nil, "")
	headers := fs.StringArray("header", nil, "")
	format := fs.String("format", "table", "")
	return fs, timeout, agents, headers, format
}

func TestApplyPrecedence(t *testing.T) {
	fs, timeout, agents, headers, format := flags()
	if err := fs.Parse([]string{"--format", "markdown"}); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		"timeout": "10s",
		"agents":  []any{"GPTBot", "ClaudeBot"},
		"header":  []any{"Cookie: a=b", "X-Test: 1"},
		"format":  "json",
	}
	env := map[string]string{"ACURL_TIMEOUT": "5s"}
	if err := Apply(fs, cfg, func(k string) string { return env[k] }); err != nil {
		t.Fatal(err)
	}
	if *timeout != 5*time.Second {
		t.Errorf("env should beat config: timeout = %v", *timeout)
	}
	if strings.Join(*agents, ",") != "GPTBot,ClaudeBot" {
		t.Errorf("agents = %v", *agents)
	}
	if len(*headers) != 2 || (*headers)[1] != "X-Test: 1" {
		t.Errorf("headers = %v", *headers)
	}
	if *format != "markdown" {
		t.Errorf("flag should beat config: format = %q", *format)
	}
}

func TestApplyBadValue(t *testing.T) {
	fs, _, _, _, _ := flags()
	err := Apply(fs, Config{"timeout": "soon"}, func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), `config "timeout"`) {
		t.Errorf("err = %v", err)
	}
}

func TestLoadAndFind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "acurl.yaml")
	if err := os.WriteFile(path, []byte("timeout: 10s\ncategory: [ai-assistant]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := Find(path)
	if err != nil || found != path {
		t.Fatalf("Find = %q, %v", found, err)
	}
	c, err := Load(found)
	if err != nil || c["timeout"] != "10s" {
		t.Errorf("Load = %v, %v", c, err)
	}
	if _, err := Find(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("explicit missing config should error")
	}
	if got := Unknown(Config{"timeout": 1, "timeuot": 2}, map[string]bool{"timeout": true}); len(got) != 1 || got[0] != "timeuot" {
		t.Errorf("Unknown = %v", got)
	}
}

func TestEnvName(t *testing.T) {
	if got := EnvName("cloak-threshold"); got != "ACURL_CLOAK_THRESHOLD" {
		t.Errorf("EnvName = %q", got)
	}
}
