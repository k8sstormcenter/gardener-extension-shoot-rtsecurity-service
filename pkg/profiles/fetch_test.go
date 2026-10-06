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

	docs, err := f.Fetch(context.Background(), Source{Repo: "k8sstormcenter/bob", Ref: "refs/pull/7/head", Path: "profiles/mine"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range docs {
		got = append(got, d.Name)
	}
	// Sorted, no subdirectory, no non-YAML, nothing from a sibling directory.
	if strings.Join(got, ",") != "a.yml,b.yaml" {
		t.Fatalf("got %v", got)
	}
	if docs[1].Content != "kind: B\n" {
		t.Fatalf("content mangled: %q", docs[1].Content)
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
