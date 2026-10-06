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
	Repo  string // "<owner>/<name>"
	Ref   string // branch, tag, sha or refs/pull/<n>/head; empty = default branch
	Path  string // directory inside the repo
	Token string // optional, for a private repo
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
// rendered output is stable and a reconcile does not churn the ManagedResource.
func (f *Fetcher) Fetch(ctx context.Context, src Source) ([]Document, error) {
	if src.Repo == "" {
		return nil, fmt.Errorf("profiles.repo is required")
	}
	if strings.Count(src.Repo, "/") != 1 {
		return nil, fmt.Errorf("profiles.repo %q must be <owner>/<name>", src.Repo)
	}
	url := fmt.Sprintf("%s/repos/%s/tarball/%s", f.baseURL, src.Repo, src.Ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if src.Token != "" {
		req.Header.Set("Authorization", "Bearer "+src.Token)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s@%s: %w", src.Repo, src.Ref, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s@%s: %s", src.Repo, src.Ref, resp.Status)
	}

	docs, err := documentsUnder(resp.Body, src.Path)
	if err != nil {
		return nil, fmt.Errorf("reading %s@%s: %w", src.Repo, src.Ref, err)
	}
	// An empty result is an error, not an empty stack: the Shoot asked for profiles from a
	// path, so a typo in the path must not quietly deploy nothing.
	if len(docs) == 0 {
		return nil, fmt.Errorf("no .yaml documents directly under %q in %s@%s", src.Path, src.Repo, src.Ref)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Name < docs[j].Name })
	return docs, nil
}

// documentsUnder walks the tarball. GitHub wraps everything in one generated top-level
// directory whose name includes the commit, so the first path element is dropped rather
// than matched. Only files directly inside dir are returned: a per-shoot directory must
// not be able to pull in a neighbouring shoot's profiles through a subdirectory.
func documentsUnder(body io.Reader, dir string) ([]Document, error) {
	gz, err := gzip.NewReader(body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	want := path.Clean("/" + dir)
	var docs []Document
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := h.Name
		if i := strings.IndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		if path.Dir(path.Clean("/"+name)) != want {
			continue
		}
		if ext := path.Ext(name); ext != ".yaml" && ext != ".yml" {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(tr, maxBytes+1))
		if err != nil {
			return nil, err
		}
		if len(content) > maxBytes {
			return nil, fmt.Errorf("%s is larger than %d bytes", name, maxBytes)
		}
		docs = append(docs, Document{Name: path.Base(name), Content: string(content)})
	}
	return docs, nil
}
