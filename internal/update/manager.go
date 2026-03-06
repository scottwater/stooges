package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	apperrors "github.com/scottwater/stooges/internal/errors"
)

const (
	repoOwner      = "scottwater"
	repoName       = "stooges"
	binaryName     = "stooges"
	checkInterval  = 24 * time.Hour
	requestTimeout = 2 * time.Second
)

type Manager struct {
	client     *http.Client
	now        func() time.Time
	cacheDir   func() (string, error)
	executable func() (string, error)
	goos       string
	goarch     string
}

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

type UpgradeResult struct {
	CurrentVersion string
	LatestVersion  string
	ExecutablePath string
	UpToDate       bool
}

func NewManager() *Manager {
	return &Manager{
		client:     &http.Client{Timeout: requestTimeout},
		now:        time.Now,
		cacheDir:   os.UserCacheDir,
		executable: os.Executable,
		goos:       runtime.GOOS,
		goarch:     runtime.GOARCH,
	}
}

func (m *Manager) MaybeNotify(ctx context.Context, out io.Writer, currentVersion string) error {
	state, err := m.state(ctx)
	if err != nil {
		return err
	}
	if compareVersions(state.LatestVersion, currentVersion) <= 0 {
		return nil
	}
	now := m.now()
	if state.NotifiedVersion == state.LatestVersion && now.Sub(state.NotifiedAt) < checkInterval {
		return nil
	}
	if _, err := fmt.Fprintf(out, "Update available: %s (installed %s). Run: stooges upgrade\n", displayVersion(state.LatestVersion), displayVersion(currentVersion)); err != nil {
		return err
	}
	state.NotifiedVersion = state.LatestVersion
	state.NotifiedAt = now
	return m.writeCache(state)
}

func (m *Manager) Upgrade(ctx context.Context, currentVersion string) (UpgradeResult, error) {
	if !supportedPlatform(m.goos, m.goarch) {
		return UpgradeResult{}, apperrors.New(apperrors.KindUnsupportedPlatform, fmt.Sprintf("upgrade is unavailable on %s/%s", m.goos, m.goarch))
	}
	latest, err := m.fetchLatestVersion(ctx)
	if err != nil {
		return UpgradeResult{}, err
	}
	path, err := m.targetExecutablePath()
	if err != nil {
		return UpgradeResult{}, apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve executable path", err)
	}
	result := UpgradeResult{
		CurrentVersion: currentVersion,
		LatestVersion:  latest,
		ExecutablePath: path,
		UpToDate:       compareVersions(latest, currentVersion) <= 0,
	}
	if result.UpToDate {
		return result, nil
	}
	if err := m.replaceExecutable(ctx, path, latest); err != nil {
		return UpgradeResult{}, err
	}
	_ = m.writeCache(cacheState{CheckedAt: m.now(), LatestVersion: latest})
	return result, nil
}

func (m *Manager) state(ctx context.Context) (cacheState, error) {
	path, err := m.cachePath()
	if err != nil {
		return cacheState{}, err
	}
	state, err := loadCache(path)
	if err != nil {
		state = cacheState{}
	}
	if state.LatestVersion != "" && m.now().Sub(state.CheckedAt) < checkInterval {
		return state, nil
	}
	latest, fetchErr := m.fetchLatestVersion(ctx)
	if fetchErr != nil {
		if state.LatestVersion != "" {
			return state, nil
		}
		return cacheState{}, fetchErr
	}
	state.CheckedAt = m.now()
	state.LatestVersion = latest
	_ = saveCache(path, state)
	return state, nil
}

func (m *Manager) fetchLatestVersion(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := m.client.Do(req)
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindFilesystemFailure, "fetch latest release", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", apperrors.New(apperrors.KindFilesystemFailure, fmt.Sprintf("fetch latest release: GitHub returned %s: %s", resp.Status, strings.TrimSpace(string(body))))
	}
	var release releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", apperrors.Wrap(apperrors.KindFilesystemFailure, "decode latest release", err)
	}
	if normalizeVersion(release.TagName) == "" {
		return "", apperrors.New(apperrors.KindFilesystemFailure, "latest release is missing tag_name")
	}
	return release.TagName, nil
}

func (m *Manager) replaceExecutable(ctx context.Context, targetPath, version string) error {
	downloadURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s_%s_%s.tar.gz", repoOwner, repoName, version, binaryName, m.goos, m.goarch)
	tmpDir, err := os.MkdirTemp(filepath.Dir(targetPath), ".stooges-upgrade-")
	if err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, "create temporary upgrade directory", err)
	}
	defer os.RemoveAll(tmpDir)

	mode := os.FileMode(0o755)
	if info, err := os.Stat(targetPath); err == nil {
		mode = info.Mode().Perm()
	}
	tmpBinary := filepath.Join(tmpDir, binaryName)
	if err := m.downloadBinary(ctx, downloadURL, tmpBinary, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpBinary, targetPath); err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("replace executable at %s", targetPath), err)
	}
	return nil
}

func (m *Manager) downloadBinary(ctx context.Context, url, targetPath string, mode os.FileMode) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, "download release archive", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return apperrors.New(apperrors.KindFilesystemFailure, fmt.Sprintf("download release archive: GitHub returned %s: %s", resp.Status, strings.TrimSpace(string(body))))
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return apperrors.Wrap(apperrors.KindFilesystemFailure, "open release archive", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return apperrors.Wrap(apperrors.KindFilesystemFailure, "read release archive", err)
		}
		if header.FileInfo().IsDir() || filepath.Base(header.Name) != binaryName {
			continue
		}
		file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("create upgraded binary at %s", targetPath), err)
		}
		if _, err := io.Copy(file, tr); err != nil {
			file.Close()
			return apperrors.Wrap(apperrors.KindFilesystemFailure, "extract upgraded binary", err)
		}
		if err := file.Close(); err != nil {
			return apperrors.Wrap(apperrors.KindFilesystemFailure, "close upgraded binary", err)
		}
		if err := os.Chmod(targetPath, mode); err != nil {
			return apperrors.Wrap(apperrors.KindFilesystemFailure, fmt.Sprintf("chmod upgraded binary at %s", targetPath), err)
		}
		return nil
	}
	return apperrors.New(apperrors.KindFilesystemFailure, "release archive did not contain stooges binary")
}

func (m *Manager) writeCache(state cacheState) error {
	path, err := m.cachePath()
	if err != nil {
		return err
	}
	return saveCache(path, state)
}

func (m *Manager) cachePath() (string, error) {
	dir, err := m.cacheDir()
	if err != nil {
		return "", apperrors.Wrap(apperrors.KindFilesystemFailure, "resolve user cache directory", err)
	}
	return filepath.Join(dir, repoName, cacheFileName), nil
}

func (m *Manager) targetExecutablePath() (string, error) {
	path, err := m.executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil && strings.TrimSpace(resolved) != "" {
		return resolved, nil
	}
	return path, nil
}

func supportedPlatform(goos, goarch string) bool {
	if goos != "darwin" && goos != "linux" {
		return false
	}
	return goarch == "amd64" || goarch == "arm64"
}
