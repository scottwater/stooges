package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaybeNotifyPrintsOncePerDay(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	cacheDir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v0.79"}`)
	}))
	defer server.Close()

	manager := testManager(now, cacheDir, server)
	var out bytes.Buffer
	if err := manager.MaybeNotify(context.Background(), &out, "0.78"); err != nil {
		t.Fatalf("MaybeNotify failed: %v", err)
	}
	if !strings.Contains(out.String(), "Run: stooges upgrade") {
		t.Fatalf("expected update notice, got %q", out.String())
	}

	out.Reset()
	if err := manager.MaybeNotify(context.Background(), &out, "0.78"); err != nil {
		t.Fatalf("MaybeNotify second call failed: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no repeated notice within 24h, got %q", out.String())
	}
}

func TestUpgradeReplacesExecutable(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	cacheDir := t.TempDir()
	execDir := t.TempDir()
	execPath := filepath.Join(execDir, binaryName)
	if err := os.WriteFile(execPath, []byte("old"), 0o755); err != nil {
		t.Fatalf("write old executable: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/scottwater/stooges/releases/latest":
			fmt.Fprint(w, `{"tag_name":"v0.79"}`)
		case "/scottwater/stooges/releases/download/v0.79/stooges_darwin_arm64.tar.gz":
			writeArchive(t, w, "new-binary")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager := testManager(now, cacheDir, server)
	manager.executable = func() (string, error) { return execPath, nil }

	result, err := manager.Upgrade(context.Background(), "0.78")
	if err != nil {
		t.Fatalf("Upgrade failed: %v", err)
	}
	if result.UpToDate {
		t.Fatalf("expected upgrade to run, got %#v", result)
	}
	if result.LatestVersion != "v0.79" {
		t.Fatalf("expected latest version v0.79, got %#v", result)
	}
	data, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("read upgraded executable: %v", err)
	}
	if string(data) != "new-binary" {
		t.Fatalf("expected upgraded binary, got %q", string(data))
	}
}

func TestUpgradeNoopsWhenCurrentVersionIsLatest(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	cacheDir := t.TempDir()
	execPath := filepath.Join(t.TempDir(), binaryName)
	if err := os.WriteFile(execPath, []byte("same"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v0.79"}`)
	}))
	defer server.Close()

	manager := testManager(now, cacheDir, server)
	manager.executable = func() (string, error) { return execPath, nil }

	result, err := manager.Upgrade(context.Background(), "v0.79")
	if err != nil {
		t.Fatalf("Upgrade failed: %v", err)
	}
	if !result.UpToDate {
		t.Fatalf("expected up-to-date result, got %#v", result)
	}
	data, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("read executable: %v", err)
	}
	if string(data) != "same" {
		t.Fatalf("expected executable to remain unchanged, got %q", string(data))
	}
}

func testManager(now time.Time, cacheDir string, server *httptest.Server) *Manager {
	client := server.Client()
	client.Transport = rewriteTransport{base: client.Transport, serverURL: server.URL}
	return &Manager{
		client:     client,
		now:        func() time.Time { return now },
		cacheDir:   func() (string, error) { return cacheDir, nil },
		executable: func() (string, error) { return filepath.Join(cacheDir, binaryName), nil },
		goos:       "darwin",
		goarch:     "arm64",
	}
}

type rewriteTransport struct {
	base      http.RoundTripper
	serverURL string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(t.serverURL, "http://")
	return t.base.RoundTrip(clone)
}

func writeArchive(tb testing.TB, w http.ResponseWriter, contents string) {
	tb.Helper()
	w.Header().Set("Content-Type", "application/gzip")
	gz := gzip.NewWriter(w)
	defer gz.Close()
	tr := tar.NewWriter(gz)
	defer tr.Close()
	data := []byte(contents)
	if err := tr.WriteHeader(&tar.Header{Name: binaryName, Mode: 0o755, Size: int64(len(data))}); err != nil {
		tb.Fatalf("write tar header: %v", err)
	}
	if _, err := tr.Write(data); err != nil {
		tb.Fatalf("write tar body: %v", err)
	}
}
