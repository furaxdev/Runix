// Package backend manages the llama.cpp backend: fetching the right
// prebuilt `llama-server` for the current machine, and supervising one
// subprocess per loaded model.
package backend

import (
	"archive/tar"
	"archive/zip"
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

	"github.com/furaxdev/runyx/internal/downloader"
	"github.com/furaxdev/runyx/internal/hwdetect"
)

// ReleasesAPI is the GitHub releases endpoint for upstream llama.cpp's
// latest release. Overridable in tests.
var ReleasesAPI = "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"

// releaseByTagAPI returns the GitHub API URL for one specific release
// tag. A separate var (rather than derived from ReleasesAPI) so tests can
// point it at a different test server independently of ReleasesAPI.
var releaseByTagAPI = func(tag string) string {
	return "https://api.github.com/repos/ggml-org/llama.cpp/releases/tags/" + tag
}

// nightlyTagAssetName is the asset llama.cpp's milestone releases (e.g.
// "v0.7.0") carry instead of prebuilt binaries: a one-line text file
// naming the continuous-build release tag (e.g. "b11669") that actually
// has the per-platform archives attached. "latest" resolves to whichever
// milestone tag is newest, which is frequently binary-less — this asset
// is upstream's own pointer to where the binaries really are.
const nightlyTagAssetName = "nightly-tag.txt"

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
// "llama-<tag>-bin-ubuntu-x64.tar.gz", "llama-<tag>-bin-win-cuda-x64.zip",
// "llama-<tag>-bin-macos-arm64.tar.gz". Linux/macOS builds ship as
// .tar.gz, Windows builds as .zip.
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
			if !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".tar.gz") {
				continue
			}
			// "cudart-llama-...-cuda-...-x64.tar.gz" ships only the CUDA
			// runtime libraries alongside the real "llama-...-cuda-..."
			// build — matching it instead would silently fetch an
			// archive with no llama-server binary in it at all.
			if !strings.HasPrefix(name, "llama-") {
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

	archivePath := filepath.Join(f.Dir, asset.Name)
	if err := downloader.Download(ctx, f.Client, asset.BrowserDownloadURL, archivePath, downloader.Options{
		Concurrency:  4,
		OnProgress:   onProgress,
		MinFreeBytes: asset.Size * 2,
	}); err != nil {
		return "", fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer os.Remove(archivePath)

	if err := extractServerBinary(archivePath, destDir); err != nil {
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

// latestRelease resolves the release that actually has prebuilt binary
// assets. llama.cpp's GitHub "latest" release is often a source-only
// milestone tag carrying just a nightlyTagAssetName pointer; when that's
// the case, follow it to the continuous-build release underneath.
func (f *Fetcher) latestRelease(ctx context.Context) (ghRelease, error) {
	rel, err := f.fetchRelease(ctx, ReleasesAPI)
	if err != nil {
		return ghRelease{}, err
	}
	nightlyURL, ok := nightlyTagAssetURL(rel.Assets)
	if !ok {
		return rel, nil
	}
	tag, err := f.fetchNightlyTag(ctx, nightlyURL)
	if err != nil {
		return ghRelease{}, fmt.Errorf("resolve nightly build tag: %w", err)
	}
	return f.fetchRelease(ctx, releaseByTagAPI(tag))
}

func nightlyTagAssetURL(assets []ghAsset) (string, bool) {
	for _, a := range assets {
		if a.Name == nightlyTagAssetName {
			return a.BrowserDownloadURL, true
		}
	}
	return "", false
}

func (f *Fetcher) fetchNightlyTag(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	tag := strings.TrimSpace(string(b))
	if tag == "" {
		return "", fmt.Errorf("%s was empty", nightlyTagAssetName)
	}
	return tag, nil
}

func (f *Fetcher) fetchRelease(ctx context.Context, url string) (ghRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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

// extractServerBinary extracts just the llama-server binary (and any
// sibling .dll/.so/.dylib it needs) from archive into destDir. archive
// may be a .zip (Windows builds) or a .tar.gz (Linux/macOS builds).
func extractServerBinary(archive, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(archive), ".tar.gz") {
		return extractServerBinaryFromTarGz(archive, destDir)
	}
	return extractServerBinaryFromZip(archive, destDir)
}

func wantedArchiveMembers() (wanted string, isWantedLib func(base string) bool) {
	wanted = serverBinaryName()
	isWantedLib = func(base string) bool {
		lower := strings.ToLower(base)
		if strings.HasSuffix(lower, ".dll") {
			return true
		}
		// Linux shared libraries commonly carry a version suffix after
		// ".so" (libllama-common.so.0, libggml.so.1.2.3, ...), and macOS
		// dylibs put the version before the extension
		// (libfoo.1.2.3.dylib) — a plain HasSuffix(".so"/".dylib") check
		// misses both, silently dropping a real runtime dependency
		// (this broke `runyx run` in CI: llama-server failed with
		// "error while loading shared libraries: libllama-common.so.0:
		// cannot open shared object file").
		return strings.Contains(lower, ".so") || strings.Contains(lower, ".dylib")
	}
	return
}

func extractServerBinaryFromZip(archive, destDir string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	wanted, isWantedLib := wantedArchiveMembers()
	found := false
	for _, zf := range r.File {
		base := filepath.Base(zf.Name)
		isServer := base == wanted
		if !isServer && !isWantedLib(base) {
			continue
		}
		if isServer {
			found = true
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		if zf.Mode()&os.ModeSymlink != 0 {
			// A zip stores a symlink's target path as the entry's
			// "content" rather than real file bytes.
			target, readErr := io.ReadAll(rc)
			rc.Close()
			if readErr != nil {
				return readErr
			}
			err = os.Symlink(string(target), filepath.Join(destDir, base))
		} else {
			err = writeExtractedFile(filepath.Join(destDir, base), rc)
			rc.Close()
		}
		if err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("archive %s did not contain %s", archive, wanted)
	}
	return nil
}

func extractServerBinaryFromTarGz(archive, destDir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	wanted, isWantedLib := wantedArchiveMembers()
	found := false
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Upstream ships each versioned shared library as a chain of
		// symlinks down to the real file (libfoo.so -> libfoo.so.0 ->
		// libfoo.so.1.2.3), and the binary dlopens it by one of the
		// symlinked names, not the real one — skipping TypeSymlink
		// entries (as if only TypeReg mattered) silently dropped the
		// exact filename llama-server actually needed, even though the
		// real file landed on disk under its own name. This is the
		// fix for a real CI failure: llama-server downloaded and
		// "extracted" fine, then failed at the dynamic linker with
		// "libllama-common.so.0: cannot open shared object file" —
		// that name was always a symlink, never a regular file.
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeSymlink {
			continue
		}
		base := filepath.Base(hdr.Name)
		isServer := base == wanted
		if !isServer && !isWantedLib(base) {
			continue
		}
		if isServer {
			found = true
		}
		if hdr.Typeflag == tar.TypeSymlink {
			if err := os.Symlink(hdr.Linkname, filepath.Join(destDir, base)); err != nil {
				return err
			}
			continue
		}
		if err := writeExtractedFile(filepath.Join(destDir, base), tr); err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("archive %s did not contain %s", archive, wanted)
	}
	return nil
}

func writeExtractedFile(dest string, src io.Reader) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}
