// SPDX-License-Identifier: Apache-2.0

// Package profiles reads signed-off profiles and rules out of a git repository.
//
// It speaks GitHub's tarball endpoint over plain HTTP rather than using a git client: the
// extension image is distroless and carries no git binary, one request is cheaper than a
// clone, and the endpoint accepts any ref including refs/pull/<n>/head, which is what lets
// a shoot run the content of an open pull request.
package profiles

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"
)

// maxBytes caps one decompressed document. The whole set lands in a ManagedResource
// Secret, which Kubernetes caps at 1 MiB, so a run-away file has to fail here with a
// readable message rather than at apply time.
const maxBytes = 512 << 10

// Source identifies what to read.
type Source struct {
	Repo string // "<owner>/<name>"
	Ref  string // branch, tag, sha or refs/pull/<n>/head; empty = default branch
	Path string // directory inside the repo
	// SuitePath and RulesPath are single files inside the repo, read from the SAME tarball at
	// the SAME ref as Path. Keeping them in one fetch is what makes the attack suite, the rule
	// set and the profiles they are scored against structurally unable to drift: there is one
	// ref, so there is one answer. A named file is consumed as that file and not also as a
	// document, so neither may sit directly inside Path.
	SuitePath string
	RulesPath string
	Token     string // optional, for a private repo
}

// Result is what one fetch yields. A struct rather than positional returns: each named file
// added here would otherwise widen every call site and every test.
type Result struct {
	Documents []Document
	Suite     string
	Rules     string
}

// Document is one YAML file from the repository.
type Document struct {
	Name    string
	Content string
}

type Fetcher struct {
	client  *http.Client
	baseURL string
}

func NewFetcher() *Fetcher {
	return &Fetcher{client: &http.Client{Timeout: 60 * time.Second}, baseURL: "https://api.github.com"}
}

// Fetch returns the YAML documents directly inside src.Path, sorted by name so the
// rendered output is stable and a reconcile does not churn the ManagedResource, plus the
// contents of each named file that is asked for.
func (f *Fetcher) Fetch(ctx context.Context, src Source) (Result, error) {
	if src.Repo == "" {
		return Result{}, fmt.Errorf("profiles.repo is required")
	}
	if strings.Count(src.Repo, "/") != 1 {
		return Result{}, fmt.Errorf("profiles.repo %q must be <owner>/<name>", src.Repo)
	}
	url := fmt.Sprintf("%s/repos/%s/tarball/%s", f.baseURL, src.Repo, src.Ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if src.Token != "" {
		req.Header.Set("Authorization", "Bearer "+src.Token)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("fetching %s@%s: %w", src.Repo, src.Ref, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("fetching %s@%s: %s", src.Repo, src.Ref, resp.Status)
	}

	named := []string{src.SuitePath, src.RulesPath}
	docs, files, err := extract(resp.Body, src.Path, named)
	if err != nil {
		return Result{}, fmt.Errorf("reading %s@%s: %w", src.Repo, src.Ref, err)
	}
	// An empty result is an error, not an empty stack: the Shoot asked for profiles from a
	// path, so a typo in the path must not quietly deploy nothing.
	if len(docs) == 0 {
		return Result{}, fmt.Errorf("no .yaml documents directly under %q in %s@%s", src.Path, src.Repo, src.Ref)
	}
	// Same rule as the directory: a named file that is not there is a typo, not an opt-out.
	// Silently falling back would ship the chart's defaults under a signed-off ref.
	for _, n := range named {
		if n != "" && files[n] == "" {
			return Result{}, fmt.Errorf("%q not found in %s@%s", n, src.Repo, src.Ref)
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Name < docs[j].Name })
	return Result{Documents: docs, Suite: files[src.SuitePath], Rules: files[src.RulesPath]}, nil
}

// extract walks the tarball. GitHub wraps everything in one generated top-level directory
// whose name includes the commit, so the first path element is dropped rather than matched.
// Only files directly inside dir are returned as documents: a per-shoot directory must not
// be able to pull in a neighbouring shoot's profiles through a subdirectory. Each path in
// named is returned separately, keyed by the string the caller asked under.
func extract(body io.Reader, dir string, named []string) ([]Document, map[string]string, error) {
	gz, err := gzip.NewReader(body)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = gz.Close() }()

	want := path.Clean("/" + dir)
	wanted := map[string]string{}
	for _, n := range named {
		if n != "" {
			wanted[path.Clean("/"+n)] = n
		}
	}
	var docs []Document
	files := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := h.Name
		if i := strings.IndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		clean := path.Clean("/" + name)
		if key, ok := wanted[clean]; ok {
			content, err := read(tr, name)
			if err != nil {
				return nil, nil, err
			}
			files[key] = content
			continue
		}
		if path.Dir(clean) != want {
			continue
		}
		if ext := path.Ext(name); ext != ".yaml" && ext != ".yml" {
			continue
		}
		content, err := read(tr, name)
		if err != nil {
			return nil, nil, err
		}
		docs = append(docs, Document{Name: path.Base(name), Content: content})
	}
	return docs, files, nil
}

// read reads one document, refusing one too large to survive the ManagedResource Secret.
func read(r io.Reader, name string) (string, error) {
	content, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return "", err
	}
	if len(content) > maxBytes {
		return "", fmt.Errorf("%s is larger than %d bytes", name, maxBytes)
	}
	return string(content), nil
}
