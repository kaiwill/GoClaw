package agent

import (
	"context"
	"errors"
	"sync"
)

type NoneMemoryBackend struct {
	mu sync.RWMutex
}

func NewNoneMemoryBackend() *NoneMemoryBackend {
	return &NoneMemoryBackend{}
}

func (b *NoneMemoryBackend) Recall(ctx context.Context, query string, limit int, category *string) ([]MemoryEntry, error) {
	return []MemoryEntry{}, nil
}

func (b *NoneMemoryBackend) Store(ctx context.Context, key, content string, category *string, metadata map[string]string) error {
	return nil
}

func (b *NoneMemoryBackend) Delete(ctx context.Context, key string) error {
	return nil
}

func (b *NoneMemoryBackend) Clear(ctx context.Context) error {
	return nil
}

func (b *NoneMemoryBackend) Close() error {
	return nil
}

func (b *NoneMemoryBackend) List(ctx context.Context, category *string) ([]MemoryEntry, error) {
	return []MemoryEntry{}, nil
}

func (b *NoneMemoryBackend) Count(ctx context.Context, category *string) (int, error) {
	return 0, nil
}

func (b *NoneMemoryBackend) Compact(ctx context.Context) error {
	return nil
}

func (b *NoneMemoryBackend) Export(ctx context.Context, path string) error {
	return errors.New("export not supported")
}

func (b *NoneMemoryBackend) Import(ctx context.Context, path string) error {
	return errors.New("import not supported")
}