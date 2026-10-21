package backend

import (
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
