package monitor

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dingodb/dingocli/internal/task/scripts"
)

func TestNodeSyncPreservesOtherJobsAndIsIdempotent(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required")
	}
	root := filepath.Join(t.TempDir(), "config with spaces")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "sync.sh")
	config := filepath.Join(root, "prometheus.yml")
	if err := os.WriteFile(script, []byte(scripts.SYNC_PROMETHEUS), 0644); err != nil {
		t.Fatal(err)
	}
	original := "scrape_configs:\n  - job_name: 'node'\n    static_configs:\n    - targets: ['old:9100']\n  - job_name: 'mds'\n    static_configs:\n    - targets: ['meta:7801']\n"
	if err := os.WriteFile(config, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	groups := []map[string]interface{}{{"targets": []string{"worker:9100"}, "labels": map[string]string{
		"cluster": "test", "component": "host", "node": "worker"}}}
	data, _ := json.Marshal(groups)
	encoded := base64.StdEncoding.EncodeToString(data)
	run := func(value string) error { return exec.Command("bash", script, config, value, "monitor").Run() }
	if err := run(encoded); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(config)
	if !strings.Contains(string(first), "meta:7801") || strings.Contains(string(first), "old:9100") {
		t.Fatalf("other scrape job damaged: %s", first)
	}
	if err := run(encoded); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(config)
	if string(first) != string(second) || strings.Count(string(second), "job_name: 'dingofs-node'") != 1 {
		t.Fatalf("non-idempotent sync: %s", second)
	}
	targets, err := os.ReadFile(filepath.Join(root, "data", "node-targets.json"))
	if err != nil || !strings.Contains(string(targets), `"node": "worker"`) {
		t.Fatalf("missing node identity: %s %v", targets, err)
	}

	discovery, err := os.ReadFile(filepath.Join(root, "data", "discovery-targets.json"))
	if err != nil || !strings.Contains(string(discovery), `"node": "monitor"`) || !strings.Contains(string(discovery), `"cluster": "test"`) {
		t.Fatalf("missing discovery collector identity: %s %v", discovery, err)
	}
	if err := run("not base64!"); err == nil {
		t.Fatal("accepted malformed input")
	}
	final, _ := os.ReadFile(config)
	if string(final) != string(first) {
		t.Fatal("invalid input modified configuration")
	}
}
