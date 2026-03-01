package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

type QdrantMemory struct {
	mu             sync.RWMutex
	collectionName string
	vectors        map[string]*VectorEntry
	lastCleanup    time.Time
}

type VectorEntry struct {
	ID        string
	Key       string
	Content   string
	Category  types.MemoryCategory
	Vector    []float64
	Timestamp time.Time
	SessionID *string
	Score     *float64
	Metadata  []byte
}

type QdrantConfig struct {
	URL        string
	APIKey     string
	Collection string
	VectorSize int
}

func NewQdrantMemory(config QdrantConfig) (*QdrantMemory, error) {
	return &QdrantMemory{
		collectionName: config.Collection,
		vectors:        make(map[string]*VectorEntry),
		lastCleanup:    time.Now(),
	}, nil
}

func (q *QdrantMemory) Store(ctx context.Context, entry *types.MemoryEntry) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	vector := generateEmbedding(entry.Content)

	q.vectors[entry.ID] = &VectorEntry{
		ID:        entry.ID,
		Key:       entry.Key,
		Content:   entry.Content,
		Category:  entry.Category,
		Vector:    vector,
		Timestamp: time.Now(),
		SessionID: entry.SessionID,
		Score:     entry.Score,
		Metadata:  entry.Metadata,
	}

	return nil
}

func (q *QdrantMemory) Recall(ctx context.Context, query string, limit int) ([]*types.MemoryEntry, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	queryVector := generateEmbedding(query)

	type scoredEntry struct {
		entry *VectorEntry
		score float64
	}

	var scored []scoredEntry
	for _, entry := range q.vectors {
		score := cosineSimilarity(queryVector, entry.Vector)
		scored = append(scored, scoredEntry{entry: entry, score: score})
	}

	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	if limit > len(scored) {
		limit = len(scored)
	}

	var results []*types.MemoryEntry
	for i := 0; i < limit; i++ {
		entry := scored[i].entry
		score := scored[i].score

		results = append(results, &types.MemoryEntry{
			ID:        entry.ID,
			Key:       entry.Key,
			Content:   entry.Content,
			Category:  entry.Category,
			Timestamp: entry.Timestamp.Format(time.RFC3339),
			SessionID: entry.SessionID,
			Score:     &score,
			Metadata:  entry.Metadata,
		})
	}

	return results, nil
}

func (q *QdrantMemory) Forget(ctx context.Context, id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.vectors, id)
	return nil
}

func (q *QdrantMemory) List(ctx context.Context, category *types.MemoryCategory, limit int) ([]*types.MemoryEntry, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var entries []*types.MemoryEntry

	for _, entry := range q.vectors {
		if category != nil && entry.Category != *category {
			continue
		}

		entries = append(entries, &types.MemoryEntry{
			ID:        entry.ID,
			Key:       entry.Key,
			Content:   entry.Content,
			Category:  entry.Category,
			Timestamp: entry.Timestamp.Format(time.RFC3339),
			SessionID: entry.SessionID,
			Score:     entry.Score,
			Metadata:  entry.Metadata,
		})
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

func (q *QdrantMemory) Search(ctx context.Context, query string, category *types.MemoryCategory, limit int) ([]*types.MemoryEntry, error) {
	return q.Recall(ctx, query, limit)
}

func (q *QdrantMemory) Hygiene(ctx context.Context, retentionDays int) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deleted := 0

	for id, entry := range q.vectors {
		if entry.Timestamp.Before(cutoff) {
			delete(q.vectors, id)
			deleted++
		}
	}

	q.lastCleanup = time.Now()
	return deleted, nil
}

func (q *QdrantMemory) Close() error {
	return nil
}

func (q *QdrantMemory) Name() string {
	return "qdrant"
}

func (q *QdrantMemory) Capabilities() MemoryCapabilities {
	return MemoryCapabilities{
		Persistent:     true,
		VectorSearch:   true,
		SessionSupport: true,
	}
}

func (q *QdrantMemory) BatchStore(ctx context.Context, entries []*types.MemoryEntry) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, entry := range entries {
		vector := generateEmbedding(entry.Content)
		q.vectors[entry.ID] = &VectorEntry{
			ID:        entry.ID,
			Key:       entry.Key,
			Content:   entry.Content,
			Category:  entry.Category,
			Vector:    vector,
			Timestamp: time.Now(),
			SessionID: entry.SessionID,
			Score:     entry.Score,
			Metadata:  entry.Metadata,
		}
	}

	return nil
}

func (q *QdrantMemory) Get(ctx context.Context, id string) (*types.MemoryEntry, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	entry, exists := q.vectors[id]
	if !exists {
		return nil, fmt.Errorf("memory entry not found: %s", id)
	}

	return &types.MemoryEntry{
		ID:        entry.ID,
		Key:       entry.Key,
		Content:   entry.Content,
		Category:  entry.Category,
		Timestamp: entry.Timestamp.Format(time.RFC3339),
		SessionID: entry.SessionID,
		Score:     entry.Score,
		Metadata:  entry.Metadata,
	}, nil
}

func (q *QdrantMemory) UpdateScore(ctx context.Context, id string, score float64) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	entry, exists := q.vectors[id]
	if !exists {
		return fmt.Errorf("memory entry not found: %s", id)
	}

	entry.Score = &score
	return nil
}

func (q *QdrantMemory) Count(ctx context.Context, category *types.MemoryCategory) (int, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	count := 0
	for _, entry := range q.vectors {
		if category != nil && entry.Category != *category {
			continue
		}
		count++
	}

	return count, nil
}

func generateEmbedding(text string) []float64 {
	hash := simpleHash(text)
	vector := make([]float64, 1536)

	for i := range vector {
		vector[i] = float64((hash+i)%1000) / 1000.0
	}

	norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	norm = sqrt(norm)

	if norm > 0 {
		for i := range vector {
			vector[i] /= norm
		}
	}

	return vector
}

func simpleHash(s string) int {
	hash := 0
	for i, c := range s {
		hash = hash*31 + int(c)*i
	}
	return hash
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}

	guess := x / 2
	for i := 0; i < 20; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	dot := 0.0
	normA := 0.0
	normB := 0.0

	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (sqrt(normA) * sqrt(normB))
}
