// Package backend manages the llama.cpp backend: fetching the right
// prebuilt `llama-server` for the current machine, and supervising one
// subprocess per loaded model.
package backend

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/furaxdev/runyx/internal/downloader"
	"github.com/furaxdev/runyx/internal/hwdetect"
)

// ReleasesAPI is the GitHub releases endpoint for upstream llama.cpp.
// Overridable in tests.
var ReleasesAPI = "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"

// ghAsset mirrors the subset of GitHub's release-asset JSON we need.
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// Fetcher downloads and caches llama-server builds under a backends
// directory, one subdirectory per (OS, arch, variant, release tag).
type Fetcher struct {
	Dir    string
	Client *http.Client
}

// NewFetcher returns a Fetcher rooted at dir.
func NewFetcher(dir string) *Fetcher {
	return &Fetcher{Dir: dir, Client: http.DefaultClient}
}

// serverBinaryName is "llama-server" or "llama-server.exe" depending on OS.
func serverBinaryName() string {
	if runtime.GOOS == "windows" {
		return "llama-server.exe"
	}
	return "llama-server"
}

// PickAsset selects the best release asset name fragment for the given
// hardware, matching upstream llama.cpp's own release-asset naming
// convention (documented in its release notes), e.g.
// "llama-<tag>-bin-ubuntu-x64.zip", "llama-<tag>-bin-win-cuda-x64.zip",
// "llama-<tag>-bin-macos-arm64.zip".
func PickAsset(info hwdetect.Info, assets []ghAsset) (ghAsset, error) {
	osTag := map[string]string{"linux": "ubuntu", "windows": "win", "darwin": "macos"}[info.OS]
	archTag := map[string]string{"amd64": "x64", "arm64": "arm64"}[info.Arch]
	backend := info.BestBackend()

	var variantPriority []string
	switch backend {
	case "cuda":
		variantPriority = []string{"cuda", "vulkan", ""}
	case "rocm":
		variantPriority = []string{"hip", "rocm", "vulkan", ""}
	case "vulkan":
		variantPriority = []string{"vulkan", ""}
	case "metal":
		variantPriority = []string{""} // macOS builds bundle Metal support
	default:
		variantPriority = []string{""}
	}

	for _, variant := range variantPriority {
		for _, a := range assets {
			name := strings.ToLower(a.Name)
			if !strings.HasSuffix(name, ".zip") {
				continue
			}
			if !strings.Contains(name, osTag) || !strings.Contains(name, archTag) {
				continue
			}
			if variant == "" {
				// Prefer an asset with no extra GPU-vendor tag.
				if !containsAny(name, "cuda", "vulkan", "hip", "rocm") {
					return a, nil
				}
				continue
			}
			if strings.Contains(name, variant) {
				return a, nil
			}
		}
	}
	return ghAsset{}, fmt.Errorf("no matching release asset for os=%s arch=%s backend=%s", info.OS, info.Arch, backend)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// Ensure returns the path to a usable llama-server binary for info,
// downloading and extracting it if not already cached.
func (f *Fetcher) Ensure(ctx context.Context, info hwdetect.Info, onProgress downloader.ProgressFunc) (string, error) {
	release, err := f.latestRelease(ctx)
	if err != nil {
		// GitHub's releases API is unauthenticated here and rate-limited
		// per source IP (60 req/hour) — easy to exhaust on a shared CI
		// runner pool, or offline. Resolving "latest" always requires
		// the network even when a perfectly usable build is already on
		// disk from a previous run, so before failing outright, fall
		// back to whatever release tag is already cached for this
		// os/arch rather than refusing to start at all.
		if cached, ok := f.anyCachedBinary(info); ok {
			return cached, nil
		}
		return "", fmt.Errorf("fetch latest llama.cpp release: %w", err)
	}
	asset, err := PickAsset(info, release.Assets)
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(f.Dir, fmt.Sprintf("%s-%s-%s", release.TagName, info.OS, info.Arch))
	binPath := filepath.Join(destDir, serverBinaryName())
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil // already cached
	}

	zipPath := filepath.Join(f.Dir, asset.Name)
	if err := downloader.Download(ctx, f.Client, asset.BrowserDownloadURL, zipPath, downloader.Options{
		Concurrency:  4,
		OnProgress:   onProgress,
		MinFreeBytes: asset.Size * 2,
	}); err != nil {
		return "", fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer os.Remove(zipPath)

	if err := extractServerBinary(zipPath, destDir); err != nil {
		return "", fmt.Errorf("extract %s: %w", asset.Name, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binPath, 0o755); err != nil {
			return "", fmt.Errorf("chmod server binary: %w", err)
		}
	}
	return binPath, nil
}

// anyCachedBinary looks for a previously downloaded llama-server under
// f.Dir matching info's os/arch, regardless of release tag, picking the
// most recently extracted one if more than one is cached.
func (f *Fetcher) anyCachedBinary(info hwdetect.Info) (string, bool) {
	entries, err := os.ReadDir(f.Dir)
	if err != nil {
		return "", false
	}
	suffix := fmt.Sprintf("-%s-%s", info.OS, info.Arch)
	var best string
	var bestMod int64
	for _, e := range entries {
		if !e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		binPath := filepath.Join(f.Dir, e.Name(), serverBinaryName())
		fi, err := os.Stat(binPath)
		if err != nil {
			continue
		}
		if mod := fi.ModTime().Unix(); best == "" || mod > bestMod {
			best, bestMod = binPath, mod
		}
	}
	return best, best != ""
}

func (f *Fetcher) latestRelease(ctx context.Context) (ghRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ReleasesAPI, nil)
	if err != nil {
		return ghRelease{}, err
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return ghRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ghRelease{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return ghRelease{}, err
	}
	return rel, nil
}

// extractServerBinary unzips just the llama-server (and any sibling .dll
// /.so it needs) from archive into destDir.
func extractServerBinary(archive, destDir string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	wanted := serverBinaryName()
	found := false
	for _, zf := range r.File {
		base := filepath.Base(zf.Name)
		isServer := base == wanted
		isLib := strings.HasSuffix(base, ".dll") || strings.HasSuffix(base, ".so") || strings.HasSuffix(base, ".dylib")
		if !isServer && !isLib {
			continue
		}
		if isServer {
			found = true
		}
		if err := extractOne(zf, filepath.Join(destDir, base)); err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("archive %s did not contain %s", archive, wanted)
	}
	return nil
}

func extractOne(zf *zip.File, dest string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
