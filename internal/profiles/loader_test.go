package profiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tragent-io/acurl/pkg/types"
)

func TestDefault(t *testing.T) {
	ps, err := Default()
	if err != nil {
		t.Fatalf("Default() error: %v", err)
	}
	if len(ps) < 5 {
		t.Fatalf("expected at least 5 default profiles, got %d", len(ps))
	}
	if !strings.Contains(strings.ToLower(ps[0].Name), "chrome") {
		t.Errorf("first profile should be the browser baseline, got %q", ps[0].Name)
	}
	for _, p := range ps {
		if p.UserAgent == "" && !p.RobotsOnly {
			t.Errorf("profile %q has empty user agent", p.Name)
		}
	}
}

func TestLoadCustomFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	content := `profiles:
  - name: MyBot
    user_agent: "MyBot/1.0"
    robots_key: MyBot
    headers:
      X-Test: "1"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ps, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(ps) != 1 || ps[0].Name != "MyBot" || ps[0].Headers["X-Test"] != "1" {
		t.Errorf("unexpected profiles: %+v", ps)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseValidation(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"empty", "profiles: []", "no profiles"},
		{"invalid yaml", "profiles: [", "parse profiles"},
		{"missing name", "profiles:\n  - user_agent: x", "name is required"},
		{"missing ua", "profiles:\n  - name: x", "user_agent is required"},
		{"robots only without key", "profiles:\n  - {name: X, robots_only: true}", "robots_key"},
		{"duplicate", "profiles:\n  - {name: A, user_agent: a}\n  - {name: a, user_agent: b}", "duplicate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Parse() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	ps := []types.Profile{{Name: "Chrome (Browser)"}, {Name: "ChatGPT-User"}, {Name: "Googlebot"}}

	if got := Filter(ps, nil); len(got) != 3 {
		t.Errorf("nil filter should return all, got %d", len(got))
	}
	got := Filter(ps, []string{"chatgpt-user", " Googlebot "})
	if len(got) != 2 || got[0].Name != "ChatGPT-User" || got[1].Name != "Googlebot" {
		t.Errorf("unexpected filter result: %+v", got)
	}
	if got := Filter(ps, []string{"unknown"}); len(got) != 0 {
		t.Errorf("expected no matches, got %d", len(got))
	}
}

func TestDefaultProfilesHaveValidCategories(t *testing.T) {
	valid := map[string]bool{
		types.CategoryBrowser: true, types.CategoryAIAssistant: true, types.CategoryAISearch: true,
		types.CategoryAITraining: true, types.CategorySearch: true, types.CategoryOther: true,
	}
	ps, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		if !valid[p.Category] {
			t.Errorf("profile %q has invalid category %q", p.Name, p.Category)
		}
	}
}
