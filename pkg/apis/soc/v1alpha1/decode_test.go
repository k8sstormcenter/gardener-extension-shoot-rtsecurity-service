package v1alpha1

import "testing"

func TestDecodeValues(t *testing.T) {
	raw := []byte(`
apiVersion: rtsecurity.extensions.gardener.cloud/v1alpha1
kind: SOCConfig
mode: full
pixie:
  pemMemoryLimit: 2Gi
clickhouse:
  centralHost: ch.forensic.example
  retentionHours: 72
detection:
  mode: enforce
  rules: ["Exec to pod"]
components:
  vector: true
credentials:
  pixieApiKey: my-key-ref
images:
  nodeAgent: example/agent:1
`)
	cfg, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	v := cfg.Values()
	ch := v["clickhouse"].(map[string]any)
	if ch["mode"] != "central" || ch["retentionHours"] != 72 || ch["central"].(map[string]any)["host"] != "ch.forensic.example" {
		t.Fatalf("clickhouse values: %v", ch)
	}
	if v["pixie"].(map[string]any)["pemMemoryLimit"] != "2Gi" {
		t.Fatalf("pixie values: %v", v["pixie"])
	}
	ks := v["kubescape"].(map[string]any)
	if ks["mode"] != "enforce" || len(ks["rules"].([]string)) != 1 {
		t.Fatalf("kubescape values: %v", ks)
	}
	if v["vector"].(map[string]any)["enabled"] != true {
		t.Fatalf("components: %v", v["vector"])
	}
	if v["images"].(map[string]any)["nodeAgent"] != "example/agent:1" {
		t.Fatalf("images: %v", v["images"])
	}
	if *cfg.Credentials.PixieAPIKey != "my-key-ref" {
		t.Fatal("credentials")
	}
}

func TestDecodeRejects(t *testing.T) {
	for _, raw := range []string{
		"apiVersion: rtsecurity.extensions.gardener.cloud/v1alpha1\nkind: SOCConfig\nmode: full\n",
		"apiVersion: rtsecurity.extensions.gardener.cloud/v1alpha1\nkind: SOCConfig\ndetection: {mode: loud}\n",
		"apiVersion: rtsecurity.extensions.gardener.cloud/v1alpha1\nkind: SOCConfig\nbogus: 1\n",
	} {
		if _, err := Decode([]byte(raw)); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	if _, err := Decode([]byte("apiVersion: other.gardener.cloud/v1alpha1\nkind: SomethingElse\n")); err == nil {
		t.Error("accepted a providerConfig of another kind")
	}
}
