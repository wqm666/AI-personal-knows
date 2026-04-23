package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

type Parallel struct {
	retrievers []port.Retriever
	merger     port.Merger
}

func New(merger port.Merger) *Parallel {
	return &Parallel{merger: merger}
}

func (p *Parallel) Register(r port.Retriever) {
	p.retrievers = append(p.retrievers, r)
}

func (p *Parallel) Search(ctx context.Context, query domain.SearchQuery) ([]domain.SearchHit, error) {
	type result struct {
		hits []domain.SearchHit
		err  error
	}

	ch := make(chan result, len(p.retrievers))
	var wg sync.WaitGroup

	for _, r := range p.retrievers {
		wg.Add(1)
		go func(ret port.Retriever) {
			defer wg.Done()
			hits, err := ret.Search(ctx, query)
			ch <- result{hits: hits, err: err}
		}(r)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var allHits []domain.SearchHit
	var lastErr error
	successCount := 0
	for res := range ch {
		if res.err != nil {
			lastErr = res.err
			continue
		}
		successCount++
		allHits = append(allHits, res.hits...)
	}

	if successCount == 0 && lastErr != nil {
		return nil, fmt.Errorf("all retrievers failed, last error: %w", lastErr)
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 5
	}
	return p.merger.Merge(allHits, limit), nil
}
