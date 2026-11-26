package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ObjectStore definise interfejs za cuvanje binarnog sadrzaja datoteka (zahtev 1.5)
type ObjectStore interface {
	PutObject(ctx context.Context, objectKey string, data []byte, contentType string) error
	GetObject(ctx context.Context, objectKey string) ([]byte, error)
	DeleteObject(ctx context.Context, objectKey string) error
}

// LocalStore cuva binarni sadrzaj datoteka u uploads/ direktorijumu
type LocalStore struct {
	baseDir string
}

func NewLocalStore(baseDir string) *LocalStore {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "uploads"
	}
	_ = os.MkdirAll(baseDir, 0755)
	return &LocalStore{baseDir: baseDir}
}

func (s *LocalStore) PutObject(_ context.Context, objectKey string, data []byte, _ string) error {
	cleanKey := strings.TrimLeft(objectKey, "/")
	localPath := filepath.Join(s.baseDir, filepath.FromSlash(cleanKey))
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("greska pri kreiranju direktorijuma za skladistenje: %w", err)
	}
	return os.WriteFile(localPath, data, 0644)
}

func (s *LocalStore) GetObject(_ context.Context, objectKey string) ([]byte, error) {
	cleanKey := strings.TrimLeft(objectKey, "/")
	localPath := filepath.Join(s.baseDir, filepath.FromSlash(cleanKey))
	return os.ReadFile(localPath)
}

func (s *LocalStore) DeleteObject(_ context.Context, objectKey string) error {
	cleanKey := strings.TrimLeft(objectKey, "/")
	localPath := filepath.Join(s.baseDir, filepath.FromSlash(cleanKey))
	_ = os.Remove(localPath)
	return nil
}
