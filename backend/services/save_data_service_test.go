package services

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"appdeck/models"
)

func TestComputeChecksum_SameContentSameChecksum(t *testing.T) {
	a := computeChecksum([]byte("cuphead save v1"))
	b := computeChecksum([]byte("cuphead save v1"))
	if a != b {
		t.Errorf("computeChecksum() gave different results for identical content: %q vs %q", a, b)
	}
}

func TestComputeChecksum_DifferentContentDifferentChecksum(t *testing.T) {
	a := computeChecksum([]byte("cuphead save v1"))
	b := computeChecksum([]byte("cuphead save v2"))
	if a == b {
		t.Errorf("computeChecksum() gave same result for different content: %q", a)
	}
}

func TestLocalSaveStore_PutThenGet_ReturnsSameData(t *testing.T) {
	store := NewLocalSaveStore(t.TempDir())
	want := []byte("save-blob-contents")

	if err := store.Put("user1/app1.zip", bytes.NewReader(want)); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	rc, err := store.Get("user1/app1.zip")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Get() = %q, want %q", got, want)
	}
}

func TestLocalSaveStore_Get_NonexistentKey_ReturnsError(t *testing.T) {
	store := NewLocalSaveStore(t.TempDir())

	_, err := store.Get("nobody/nothing.zip")
	if err == nil {
		t.Error("Get() on nonexistent key: expected error, got nil")
	}
}

func TestBuildSaveStore_LocalType_ReturnsLocalSaveStore(t *testing.T) {
	cfg := &models.SaveStorageConfig{StorageType: "local", LocalPath: t.TempDir()}

	store := buildSaveStore(cfg)

	if _, ok := store.(*LocalSaveStore); !ok {
		t.Errorf("buildSaveStore() with type=local = %T, want *LocalSaveStore", store)
	}
}

func TestBuildSaveStore_S3TypeNotImplemented_FallsBackToLocal(t *testing.T) {
	cfg := &models.SaveStorageConfig{StorageType: "s3", LocalPath: t.TempDir()}

	store := buildSaveStore(cfg)

	if _, ok := store.(*LocalSaveStore); !ok {
		t.Errorf("buildSaveStore() with type=s3 (unimplemented) = %T, want fallback *LocalSaveStore", store)
	}
}

func TestLocalSaveStore_Delete_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	store := NewLocalSaveStore(dir)
	if err := store.Put("user1/app1.zip", bytes.NewReader([]byte("data"))); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if err := store.Delete("user1/app1.zip"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "user1/app1.zip")); !os.IsNotExist(err) {
		t.Errorf("expected file removed after Delete(), stat err = %v", err)
	}
}
