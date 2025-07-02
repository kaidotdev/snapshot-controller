package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type fileStorage struct {
	config FileConfig
}

type FileConfig struct {
	Directory string
}

// NewFileStorage creates a new file storage backend
func NewFileStorage(ctx context.Context, f FileConfig) (Storage, error) {
	if f.Directory == "" {
		f.Directory = "."
	}

	return &fileStorage{
		config: f,
	}, nil
}

func (a *fileStorage) Put(ctx context.Context, key string, data []byte) (string, error) {
	filePath := filepath.Join(a.config.Directory, key)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filePath, nil
}

func (a *fileStorage) Get(ctx context.Context, url string) ([]byte, error) {
	data, err := os.ReadFile(url)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}
