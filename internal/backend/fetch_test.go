package backend

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/furaxdev/runyx/internal/hwdetect"
)

func TestPickAssetPrefersCUDAOnNvidia(t *testing.T) {
	info := hwdetect.Info{OS: "linux", Arch: "amd64", GPUs: []hwdetect.GPU{{Backend: "cuda"}}}
	assets := []ghAsset{
		{Name: "llama-b1-bin-ubuntu-x64.tar.gz"},
		{Name: "llama-b1-bin-ubuntu-cuda-x64.tar.gz"},
		{Name: "llama-b1-bin-win-x64.zip"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "llama-b1-bin-ubuntu-cuda-x64.tar.gz" {
		t.Errorf("got %q, want the cuda build", got.Name)
	}
}

func TestPickAssetFallsBackToPlainCPUBuild(t *testing.T) {
	info := hwdetect.Info{OS: "linux", Arch: "amd64"}
	assets := []ghAsset{
		{Name: "llama-b1-bin-ubuntu-cuda-x64.tar.gz"},
		{Name: "llama-b1-bin-ubuntu-x64.tar.gz"},
		{Name: "llama-b1-bin-macos-arm64.tar.gz"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "llama-b1-bin-ubuntu-x64.tar.gz" {
		t.Errorf("got %q, want the plain CPU build", got.Name)
	}
}

func TestPickAssetErrorsWhenNoMatch(t *testing.T) {
	info := hwdetect.Info{OS: "windows", Arch: "arm64"}
	assets := []ghAsset{{Name: "llama-b1-bin-ubuntu-x64.tar.gz"}}
	if _, err := PickAsset(info, assets); err == nil {
		t.Error("expected error when no asset matches os/arch")
	}
}

// TestPickAssetSkipsCudartRuntimeOnlyPackage guards against a real
// upstream naming collision: llama.cpp's CUDA releases ship both
// "llama-<tag>-bin-ubuntu-cuda-<ver>-x64.tar.gz" (the actual
// llama-server build) and "cudart-llama-<tag>-bin-ubuntu-cuda-<ver>-x64.tar.gz"
// (just the CUDA runtime libraries, no llama-server binary at all) —
// both contain "ubuntu", "x64" and "cuda", so without excluding the
// cudart-prefixed one explicitly, alphabetical/JSON ordering could pick
// an archive with no server binary in it.
func TestPickAssetSkipsCudartRuntimeOnlyPackage(t *testing.T) {
	info := hwdetect.Info{OS: "linux", Arch: "amd64", GPUs: []hwdetect.GPU{{Backend: "cuda"}}}
	assets := []ghAsset{
		{Name: "cudart-llama-b1-bin-ubuntu-cuda-12.8-x64.tar.gz"},
		{Name: "llama-b1-bin-ubuntu-cuda-12.8-x64.tar.gz"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "llama-b1-bin-ubuntu-cuda-12.8-x64.tar.gz" {
		t.Errorf("got %q, want the real build, not the cudart-only package", got.Name)
	}
}

// TestEnsureFallsBackToCachedBinaryWhenReleasesAPIFails guards against a
// real CI failure: GitHub's unauthenticated releases API is rate-limited
// (60 req/hour per source IP), easily exhausted on a shared runner pool.
// Ensure() always resolves "latest" before it can even look at what's on
// disk, so a 403 there used to hard-fail `runyx run` even with a fully
// usable llama-server already cached from a previous run.
func TestEnsureFallsBackToCachedBinaryWhenReleasesAPIFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	old := ReleasesAPI
	ReleasesAPI = srv.URL
	defer func() { ReleasesAPI = old }()

	dir := t.TempDir()
	info := hwdetect.Info{OS: "linux", Arch: "amd64"}
	cachedDir := filepath.Join(dir, "b1234-linux-amd64")
	if err := os.MkdirAll(cachedDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	binPath := filepath.Join(cachedDir, serverBinaryName())
	if err := os.WriteFile(binPath, []byte("fake binary"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	f := NewFetcher(dir)
	got, err := f.Ensure(context.Background(), info, nil)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got != binPath {
		t.Errorf("Ensure() = %q, want cached binary %q", got, binPath)
	}
}

// TestEnsureFailsWhenReleasesAPIFailsAndNothingCached makes sure the
// fallback above doesn't paper over the case where there really is
// nothing usable on disk: Ensure() must still report the original error.
func TestEnsureFailsWhenReleasesAPIFailsAndNothingCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	old := ReleasesAPI
	ReleasesAPI = srv.URL
	defer func() { ReleasesAPI = old }()

	f := NewFetcher(t.TempDir())
	info := hwdetect.Info{OS: "linux", Arch: "amd64"}
	if _, err := f.Ensure(context.Background(), info, nil); err == nil {
		t.Error("expected an error when the releases API fails and nothing is cached")
	}
}

// TestLatestReleaseFollowsNightlyTagPointer guards against a real CI
// failure: llama.cpp's "latest" GitHub release is sometimes a
// source-only milestone tag with no prebuilt binaries, carrying only a
// nightly-tag.txt asset that names the continuous-build release (e.g.
// "b11669") which actually has them. latestRelease() must follow that
// pointer instead of returning the binary-less milestone release as-is.
func TestLatestReleaseFollowsNightlyTagPointer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/nightly-tag.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("b11669\n"))
	})
	mux.HandleFunc("/tags/b11669", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghRelease{
			TagName: "b11669",
			Assets:  []ghAsset{{Name: "llama-b11669-bin-ubuntu-x64.tar.gz"}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghRelease{
			TagName: "v0.7.0",
			Assets: []ghAsset{
				{Name: "v0.7.0.zip"},
				{Name: "v0.7.0.tar.gz"},
				{Name: nightlyTagAssetName, BrowserDownloadURL: srv.URL + "/nightly-tag.txt"},
			},
		})
	})

	oldReleasesAPI, oldByTag := ReleasesAPI, releaseByTagAPI
	ReleasesAPI = srv.URL + "/latest"
	releaseByTagAPI = func(tag string) string { return srv.URL + "/tags/" + tag }
	defer func() { ReleasesAPI, releaseByTagAPI = oldReleasesAPI, oldByTag }()

	f := NewFetcher(t.TempDir())
	rel, err := f.latestRelease(context.Background())
	if err != nil {
		t.Fatalf("latestRelease: %v", err)
	}
	if rel.TagName != "b11669" {
		t.Errorf("TagName = %q, want the nightly tag, not the milestone release", rel.TagName)
	}
	if len(rel.Assets) != 1 || rel.Assets[0].Name != "llama-b11669-bin-ubuntu-x64.tar.gz" {
		t.Errorf("Assets = %+v, want the nightly release's binary assets", rel.Assets)
	}
}

// TestExtractServerBinaryFromTarGz guards the .tar.gz extraction path
// (used on Linux and macOS, where llama.cpp ships tar.gz rather than
// zip) end-to-end: build a real tar.gz containing a server binary plus
// an unrelated file, extract it, and check only the wanted file lands.
func TestExtractServerBinaryFromTarGz(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "llama-b1-bin-ubuntu-x64.tar.gz")
	writeTestTarGz(t, archive, map[string]string{
		"build/bin/" + serverBinaryName(): "fake server binary",
		"build/bin/README.md":             "not wanted",
	})

	destDir := filepath.Join(dir, "out")
	if err := extractServerBinary(archive, destDir); err != nil {
		t.Fatalf("extractServerBinary: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, serverBinaryName()))
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(got) != "fake server binary" {
		t.Errorf("extracted binary content = %q", got)
	}
	if _, err := os.Stat(filepath.Join(destDir, "README.md")); !os.IsNotExist(err) {
		t.Error("expected README.md to NOT be extracted")
	}
}

func writeTestTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
}
