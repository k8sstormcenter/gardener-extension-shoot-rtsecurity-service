package profiles

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func tarball(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: "repo-abc123/" + name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func serve(t *testing.T, body []byte, wantPath string) *Fetcher {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wantPath != "" && r.URL.Path != wantPath {
			t.Errorf("requested %q, want %q", r.URL.Path, wantPath)
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	f := NewFetcher()
	f.baseURL = srv.URL
	return f
}

func TestFetchReadsOnlyTheNamedDirectory(t *testing.T) {
	body := tarball(t, map[string]string{
		"profiles/mine/b.yaml":        "kind: B\n",
		"profiles/mine/a.yml":         "kind: A\n",
		"profiles/mine/notes.md":      "ignored",
		"profiles/mine/deeper/c.yaml": "kind: C\n",
		"profiles/other/d.yaml":       "kind: D\n",
		"README.md":                   "ignored",
	})
	f := serve(t, body, "/repos/k8sstormcenter/bob/tarball/refs/pull/7/head")

	res, err := f.Fetch(context.Background(), Source{Repo: "k8sstormcenter/bob", Ref: "refs/pull/7/head", Path: "profiles/mine"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range res.Documents {
		got = append(got, d.Name)
	}
	// Sorted, no subdirectory, no non-YAML, nothing from a sibling directory.
	if strings.Join(got, ",") != "a.yml,b.yaml" {
		t.Fatalf("got %v", got)
	}
	if res.Documents[1].Content != "kind: B\n" {
		t.Fatalf("content mangled: %q", res.Documents[1].Content)
	}
}

func TestFetchRejects(t *testing.T) {
	f := serve(t, tarball(t, map[string]string{"profiles/mine/a.yaml": "kind: A\n"}), "")
	if _, err := f.Fetch(context.Background(), Source{Repo: "k8sstormcenter/bob", Path: "profiles/typo"}); err == nil {
		t.Error("a path with no documents must fail rather than deploy nothing")
	}
	if _, err := f.Fetch(context.Background(), Source{Repo: "bob", Path: "profiles/mine"}); err == nil {
		t.Error("accepted a repo that is not <owner>/<name>")
	}
	if _, err := NewFetcher().Fetch(context.Background(), Source{Path: "x"}); err == nil {
		t.Error("accepted an empty repo")
	}
}

func TestFetchRejectsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	f := NewFetcher()
	f.baseURL = srv.URL
	if _, err := f.Fetch(context.Background(), Source{Repo: "k8sstormcenter/bob", Ref: "nope", Path: "p"}); err == nil {
		t.Error("accepted a 404")
	}
}

// The suite is read from the same tarball at the same ref as the profiles, which is what
// makes the attack suite and the profiles it is scored against unable to drift apart.
func TestFetchReadsTheSuiteFromTheSameRef(t *testing.T) {
	body := tarball(t, map[string]string{
		"example/github-runner/sbobs/runner.yaml": "kind: ContainerProfile\n",
		"example/github-runner-attacks.yaml":      "kind: AttackSuite\n",
		"example/other-attacks.yaml":              "kind: AttackSuite\nname: wrong\n",
	})
	f := serve(t, body, "/repos/k8sstormcenter/bob/tarball/v1")

	res, err := f.Fetch(context.Background(), Source{
		Repo:      "k8sstormcenter/bob",
		Ref:       "v1",
		Path:      "example/github-runner/sbobs",
		SuitePath: "example/github-runner-attacks.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Documents) != 1 || res.Documents[0].Name != "runner.yaml" {
		t.Fatalf("docs: %v", res.Documents)
	}
	// The named file, not the sibling that also matches the kind.
	if res.Suite != "kind: AttackSuite\n" {
		t.Fatalf("suite: %q", res.Suite)
	}
}

// A named suite that is not in the tarball is a typo, not an opt-out: the same rule the
// profiles directory already follows, so a mistake cannot quietly ship a stack with no
// suite to score against.
func TestFetchRejectsAMissingSuite(t *testing.T) {
	body := tarball(t, map[string]string{
		"example/github-runner/sbobs/runner.yaml": "kind: ContainerProfile\n",
	})
	f := serve(t, body, "/repos/k8sstormcenter/bob/tarball/v1")

	if _, err := f.Fetch(context.Background(), Source{
		Repo:      "k8sstormcenter/bob",
		Ref:       "v1",
		Path:      "example/github-runner/sbobs",
		SuitePath: "example/github-runner-attacks.yaml",
	}); err == nil {
		t.Fatal("a missing suite must be an error")
	}
}

// No suite requested means no suite returned, and the profiles still load: the chart's
// bundled proof-suite.yaml stays the fallback.
func TestFetchWithoutASuite(t *testing.T) {
	body := tarball(t, map[string]string{"p/a.yaml": "kind: A\n"})
	f := serve(t, body, "/repos/k8sstormcenter/bob/tarball/v1")

	res, err := f.Fetch(context.Background(), Source{Repo: "k8sstormcenter/bob", Ref: "v1", Path: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Documents) != 1 || res.Suite != "" {
		t.Fatalf("docs %v suite %q", res.Documents, res.Suite)
	}
}

// The rule set is read from the same tarball at the same ref as the profiles and the suite:
// one ref, so the rules, the profiles and the attacks scored against them cannot drift.
func TestFetchReadsTheRulesFromTheSameRef(t *testing.T) {
	body := tarball(t, map[string]string{
		"example/github-runner/sbobs/runner.yaml":        "kind: ContainerProfile\n",
		"example/github-runner-attacks.yaml":             "kind: AttackSuite\n",
		"example/github-runner/rules/default-rules.yaml": "kind: Rules\n",
	})
	f := serve(t, body, "/repos/k8sstormcenter/bob/tarball/v1")

	res, err := f.Fetch(context.Background(), Source{
		Repo:      "k8sstormcenter/bob",
		Ref:       "v1",
		Path:      "example/github-runner/sbobs",
		SuitePath: "example/github-runner-attacks.yaml",
		RulesPath: "example/github-runner/rules/default-rules.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	// All three out of one request, and the rules directory is NOT read as documents: it is
	// a subdirectory of the profiles path's parent, not of the profiles path.
	if len(res.Documents) != 1 || res.Suite != "kind: AttackSuite\n" || res.Rules != "kind: Rules\n" {
		t.Fatalf("docs %v suite %q rules %q", res.Documents, res.Suite, res.Rules)
	}
}

// The same rule as the suite and the directory. A missing rule set must not fall back to
// the chart's inline default under a signed-off ref: the shoot would report the ref it was
// told to run and run something else.
func TestFetchRejectsMissingRules(t *testing.T) {
	body := tarball(t, map[string]string{"p/a.yaml": "kind: A\n"})
	f := serve(t, body, "/repos/k8sstormcenter/bob/tarball/v1")

	if _, err := f.Fetch(context.Background(), Source{
		Repo:      "k8sstormcenter/bob",
		Ref:       "v1",
		Path:      "p",
		RulesPath: "p/rules/default-rules.yaml",
	}); err == nil {
		t.Fatal("a missing rule set must be an error")
	}
}
