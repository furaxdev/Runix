package backend

import (
	"context"
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
		{Name: "llama-b1-bin-ubuntu-x64.zip"},
		{Name: "llama-b1-bin-ubuntu-cuda-x64.zip"},
		{Name: "llama-b1-bin-win-x64.zip"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "llama-b1-bin-ubuntu-cuda-x64.zip" {
		t.Errorf("got %q, want the cuda build", got.Name)
	}
}

func TestPickAssetFallsBackToPlainCPUBuild(t *testing.T) {
	info := hwdetect.Info{OS: "linux", Arch: "amd64"}
	assets := []ghAsset{
		{Name: "llama-b1-bin-ubuntu-cuda-x64.zip"},
		{Name: "llama-b1-bin-ubuntu-x64.zip"},
		{Name: "llama-b1-bin-macos-arm64.zip"},
	}
	got, err := PickAsset(info, assets)
	if err != nil {
		t.Fatalf("PickAsset: %v", err)
	}
	if got.Name != "llama-b1-bin-ubuntu-x64.zip" {
		t.Errorf("got %q, want the plain CPU build", got.Name)
	}
}

func TestPickAssetErrorsWhenNoMatch(t *testing.T) {
	info := hwdetect.Info{OS: "windows", Arch: "arm64"}
	assets := []ghAsset{{Name: "llama-b1-bin-ubuntu-x64.zip"}}
	if _, err := PickAsset(info, assets); err == nil {
		t.Error("expected error when no asset matches os/arch")
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
