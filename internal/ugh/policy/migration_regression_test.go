package policy

import (
	"bytes"
	"encoding/json"
	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationPreservesRawAndUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "# user comment\nunknown = 'keep'\n[routing] # routing comment\npick_model = 'table' # saved choice\npolicy_version = 0 # old marker\n[routing.tiers]\nhigh = 'mimo-v2.6-pro' # explicitly saved old default\n[future]\nvalue = 'keep me'\n"
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	if _, err := MigrateConfig(&cfg, path); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := strings.Replace(original, "policy_version = 0", "policy_version = 1", 1)
	if string(got) != want {
		t.Fatalf("unexpected migration:\n%s", got)
	}
	backup, _ := os.ReadFile(path + ".bak")
	if string(backup) != original {
		t.Fatal("backup does not preserve original")
	}
	if _, err := MigrateConfig(&cfg, path); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(path)
	if !bytes.Equal(got, again) {
		t.Fatal("migration is not idempotent")
	}
}

func TestMigrationDetectsAbsentVersionDespiteNewDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "[routing]\npick_model = 'table'\n[routing.tiers]\nhigh = 'codex/gpt-6-sol'\n"
	os.WriteFile(path, []byte(original), 0600)
	cfg := config.Default() // in-memory version is already current
	if _, err := MigrateConfig(&cfg, path); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "policy_version = 1") || !strings.Contains(string(got), "codex/gpt-6-sol") {
		t.Fatalf("%s", got)
	}
}

func TestScopeManifestDistinguishesEmptyDefaultAndUnrestricted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scope.json")
	eval := FuncAccessEvaluator(func(string, string) (AccessStatus, string) { return AccessGranted, "test" })
	for _, tc := range []struct {
		mode  string
		state ScopeState
		empty bool
	}{
		{"custom", ScopeStateCustom, true}, {"default", ScopeStateAbsent, false}, {"unrestricted", ScopeStateUnrestricted, false},
	} {
		b, _ := json.Marshal(ScopeDocument{Version: 1, Mode: tc.mode})
		os.WriteFile(path, b, 0600)
		got := ResolveScopedModels(path, eval)
		if got.State != tc.state || (len(got.Refs) == 0) != tc.empty {
			t.Fatalf("%s: %+v", tc.mode, got)
		}
	}
	for _, b := range []string{"[]", "null", "broken", `{"version":99,"mode":"default"}`} {
		os.WriteFile(path, []byte(b), 0600)
		got := ResolveScopedModels(path, eval)
		if len(got.Refs) != 0 || len(got.Warnings) == 0 {
			t.Fatalf("unsafe expansion for %s: %+v", b, got)
		}
	}
}

func TestEligibilityRejectsUnknownPurposeAndUnauthorizedSynthesis(t *testing.T) {
	for _, purpose := range []Purpose{"", "bogus"} {
		if got := CheckEligibility(EligibilityRequest{Purpose: purpose, Family: FamilyMiMoPro, Provider: "openrouter", Access: AccessGranted}); got.Eligible {
			t.Fatalf("unknown purpose accepted: %s", purpose)
		}
	}
	for _, provider := range []string{"openrouter", "opencode-zen"} {
		got := CheckEligibility(EligibilityRequest{Purpose: PurposeCouncilSynth, Family: FamilyGPTSol, Provider: provider, Access: AccessGranted})
		if got.Eligible {
			t.Fatalf("non-Codex Sol synthesis accepted: %s", provider)
		}
	}
	req := EligibilityRequest{Purpose: PurposeCouncilSynth, Family: FamilyGPTAstra, Provider: "codex", Transport: TransportResponses, Access: AccessGranted, OracleLed: true, ApprovalRequired: true}
	if CheckEligibility(req).Eligible {
		t.Fatal("missing approval accepted")
	}
	req.Approved = true
	if !CheckEligibility(req).Eligible {
		t.Fatal("trusted oracle-led synthesis rejected")
	}
	var untrusted EligibilityRequest
	raw, _ := json.Marshal(map[string]any{"family": FamilyGPTAstra, "provider": "codex", "purpose": PurposeCouncilSynth, "transport": TransportResponses, "access": AccessGranted, "OracleLed": true, "Approved": true})
	json.Unmarshal(raw, &untrusted)
	if untrusted.OracleLed || untrusted.Approved || CheckEligibility(untrusted).Eligible {
		t.Fatal("JSON granted workflow authority")
	}
}

func TestBackupDoesNotTrustPartialInterruptedBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(path+".bak", []byte("partial"), 0600)
	original := []byte("complete original")
	if err := BackupOriginal(path, original); err != nil {
		t.Fatal(err)
	}
	entries, _ := filepath.Glob(path + ".bak.*")
	if len(entries) != 1 {
		t.Fatalf("backups: %v", entries)
	}
	b, _ := os.ReadFile(entries[0])
	if !bytes.Equal(b, original) {
		t.Fatal("original was not backed up")
	}
	old, _ := os.ReadFile(path + ".bak")
	if string(old) != "partial" {
		t.Fatal("existing backup overwritten")
	}
}

// C1: a config built from normal [routing.tiers]/[routing.access] tables,
// quoted keys included, migrates silently. The old "inline or quoted routing
// table retained" warning never applies to plain tables.
func TestMigrationHandlesRoutingSubTablesSilently(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	const original = `[routing.tiers]
low = ["claude-haiku-5.5"]
high = ["mimo-v2.6-pro"]
oracle = ["gpt-astra"]

[routing.access]
codex = "granted"
"opencode-go" = "granted"
openrouter = "denied"
`
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	warnings, err := MigrateConfig(&cfg, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if cfg.Routing.PolicyVersion != CurrentPolicyVersion {
		t.Fatalf("policy version = %d, want %d", cfg.Routing.PolicyVersion, CurrentPolicyVersion)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "[routing]\npolicy_version = 1\n" + original
	if string(got) != want {
		t.Fatalf("migrated file = %q, want %q", got, want)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatalf("backup = %q, want the original", backup)
	}
	// A second run is a no-op and stays silent.
	cfg2, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	warnings, err = MigrateConfig(&cfg2, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("second-run warnings = %v, want none", warnings)
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(got) {
		t.Fatalf("second run rewrote the file: %q, want %q", again, got)
	}
}

// C1: a config already carrying policy_version = 1 is left completely alone
// and never warns.
func TestMigrationAlreadyCurrentNeverWarns(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	const original = `[routing]
policy_version = 1

[routing.tiers]
low = ["claude-haiku-5.5"]

[routing.access]
openrouter = "granted"
`
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	warnings, err := MigrateConfig(&cfg, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if cfg.Routing.PolicyVersion != CurrentPolicyVersion {
		t.Fatalf("policy version = %d, want %d", cfg.Routing.PolicyVersion, CurrentPolicyVersion)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("file changed: %q, want %q", got, original)
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Fatal("unexpected backup for an already-current config")
	}
}

// C1: quoted routing headers are still recognized as the routing table.
func TestMigrationQuotedRoutingHeaderSilent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	const original = `["routing"] # keep
pick_model = 'table'
["routing".tiers]
low = ['claude-haiku-5.5']
`
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	warnings, err := MigrateConfig(&cfg, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `["routing"] # keep
policy_version = 1
pick_model = 'table'
["routing".tiers]
low = ['claude-haiku-5.5']
`
	if string(got) != want {
		t.Fatalf("migrated file = %q, want %q", got, want)
	}
}

// C1: an inline or dotted routing block cannot be extended safely; the
// warning names the file and the exact line instead of a vague skip.
func TestMigrationInlineRoutingWarnsWithExactLine(t *testing.T) {
	for name, tc := range map[string]struct {
		original string
		line     string
	}{
		"inline table": {original: "name = 'x'\nrouting = { tiers = { low = 'claude-haiku-5.5' } }\n", line: ":2:"},
		"dotted keys":  {original: "routing.tiers = { low = 'claude-haiku-5.5' }\n", line: ":1:"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			path, err := config.Path()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.original), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			warnings, err := MigrateConfig(&cfg, path)
			if err != nil {
				t.Fatal(err)
			}
			if len(warnings) != 1 {
				t.Fatalf("warnings = %v, want exactly one", warnings)
			}
			if !strings.Contains(warnings[0], tc.line) || !strings.Contains(warnings[0], "policy_version") {
				t.Fatalf("warning = %q, want file:line %q and the policy_version fix", warnings[0], tc.line)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.original {
				t.Fatalf("file changed: %q, want %q", got, tc.original)
			}
		})
	}
}

// C1: an inline routing block that already carries policy_version is valid
// for the current policy and must not warn at all.
func TestMigrationInlineRoutingCurrentSilent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	const original = "routing = { policy_version = 1, tiers = { low = 'claude-haiku-5.5' } }\n"
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	warnings, err := MigrateConfig(&cfg, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("file changed: %q, want %q", got, original)
	}
}
