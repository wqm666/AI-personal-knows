package pgstore

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/personal-know/internal/domain"
)

type PgStore struct {
	db *sql.DB
}

func New(dsn string, embeddingDim ...int) (*PgStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	dim := 0
	if len(embeddingDim) > 0 {
		dim = embeddingDim[0]
	}

	s := &PgStore{db: db}
	if err := s.initSchema(dim); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return s, nil
}

func (s *PgStore) initSchema(dim int) error {
	if _, err := s.db.Exec(buildSchemaSQL(dim)); err != nil {
		return err
	}
	_, err := s.db.Exec(migrationSQL)
	return err
}

func (s *PgStore) Close() error {
	return s.db.Close()
}

type txKey struct{}

func (s *PgStore) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	return tx.Commit()
}

type dbExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *PgStore) executor(ctx context.Context) dbExecutor {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return s.db
}

func (s *PgStore) Save(ctx context.Context, k *domain.Knowledge) error {
	query := `
		INSERT INTO knowledge (id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags, embedding,
			related_ids, superseded_by, status, consolidated_from, hit_count, useful_count, last_hit_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`

	knowledgeType := k.KnowledgeType
	if knowledgeType == "" {
		knowledgeType = domain.TypeGeneral
	}

	_, err := s.executor(ctx).ExecContext(ctx, query,
		k.ID, k.OwnerID, k.Title, k.Content, k.Summary,
		k.Source, k.SourceRef, knowledgeType, k.KnowledgeCategory,
		pq.Array(k.Tags),
		embeddingToString(k.Embedding),
		pq.Array(k.RelatedIDs),
		k.SupersededBy,
		k.Status,
		pq.Array(k.ConsolidatedFrom),
		k.HitCount, k.UsefulCount, k.LastHitAt,
		k.CreatedAt, k.UpdatedAt,
	)
	return err
}

func (s *PgStore) Get(ctx context.Context, ownerID, id string) (*domain.Knowledge, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from, hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 AND id = $2`

	k := &domain.Knowledge{}
	err := s.executor(ctx).QueryRowContext(ctx, query, ownerID, id).Scan(
		&k.ID, &k.OwnerID, &k.Title, &k.Content, &k.Summary,
		&k.Source, &k.SourceRef, &k.KnowledgeType, &k.KnowledgeCategory,
		pq.Array(&k.Tags),
		pq.Array(&k.RelatedIDs),
		&k.SupersededBy,
		&k.Status, pq.Array(&k.ConsolidatedFrom),
		&k.HitCount, &k.UsefulCount, &k.LastHitAt,
		&k.CreatedAt, &k.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return k, err
}

func (s *PgStore) Update(ctx context.Context, k *domain.Knowledge) error {
	query := `UPDATE knowledge SET title=$3, content=$4, summary=$5, source=$6, source_ref=$7,
		knowledge_type=$8, knowledge_category=$9, tags=$10, related_ids=$11, superseded_by=$12, status=$13, consolidated_from=$14,
		hit_count=$15, useful_count=$16, last_hit_at=$17, updated_at=$18
		WHERE owner_id=$1 AND id=$2`

	_, err := s.executor(ctx).ExecContext(ctx, query,
		k.OwnerID, k.ID, k.Title, k.Content, k.Summary,
		k.Source, k.SourceRef, k.KnowledgeType, k.KnowledgeCategory,
		pq.Array(k.Tags),
		pq.Array(k.RelatedIDs),
		k.SupersededBy,
		k.Status, pq.Array(k.ConsolidatedFrom),
		k.HitCount, k.UsefulCount, k.LastHitAt,
		k.UpdatedAt,
	)
	return err
}

func (s *PgStore) Delete(ctx context.Context, ownerID, id string) error {
	_, err := s.executor(ctx).ExecContext(ctx, `DELETE FROM knowledge WHERE owner_id = $1 AND id = $2`, ownerID, id)
	return err
}

func (s *PgStore) List(ctx context.Context, ownerID string, offset, limit int) ([]*domain.Knowledge, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from, hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	return s.scanMultiple(ctx, query, ownerID, limit, offset)
}

func (s *PgStore) ListByStatus(ctx context.Context, ownerID, status string, offset, limit int) ([]*domain.Knowledge, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from, hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 AND status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	return s.scanMultiple(ctx, query, ownerID, status, limit, offset)
}

func (s *PgStore) ListByIDs(ctx context.Context, ownerID string, ids []string) ([]*domain.Knowledge, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from, hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 AND id = ANY($2)`
	return s.scanMultiple(ctx, query, ownerID, pq.Array(ids))
}

func (s *PgStore) IncrementHitCount(ctx context.Context, ownerID, id string) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`UPDATE knowledge SET hit_count = hit_count + 1, last_hit_at = NOW(), updated_at = NOW() WHERE owner_id = $1 AND id = $2`, ownerID, id)
	return err
}

func (s *PgStore) IncrementUsefulCount(ctx context.Context, ownerID, id string) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`UPDATE knowledge SET useful_count = useful_count + 1, updated_at = NOW() WHERE owner_id = $1 AND id = $2`, ownerID, id)
	return err
}

func (s *PgStore) UpdateRelatedIDs(ctx context.Context, ownerID, id string, relatedIDs []string) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`UPDATE knowledge SET related_ids = $3, updated_at = NOW() WHERE owner_id = $1 AND id = $2`, ownerID, id, pq.Array(relatedIDs))
	return err
}

func (s *PgStore) UpdateSupersededBy(ctx context.Context, ownerID, id string, supersededBy string) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`UPDATE knowledge SET superseded_by = $3, status = $4, updated_at = NOW() WHERE owner_id = $1 AND id = $2`,
		ownerID, id, supersededBy, domain.StatusSuperseded)
	return err
}

func (s *PgStore) UpdateStatus(ctx context.Context, ownerID, id string, status string) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`UPDATE knowledge SET status = $3, updated_at = NOW() WHERE owner_id = $1 AND id = $2`, ownerID, id, status)
	return err
}

func (s *PgStore) UpdateTags(ctx context.Context, ownerID, id string, tags []string) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`UPDATE knowledge SET tags = $3, updated_at = NOW() WHERE owner_id = $1 AND id = $2`, ownerID, id, pq.Array(tags))
	return err
}

func (s *PgStore) UpdateKnowledgeCategory(ctx context.Context, ownerID, id string, category string) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`UPDATE knowledge SET knowledge_category = $3, updated_at = NOW() WHERE owner_id = $1 AND id = $2`, ownerID, id, category)
	return err
}

func (s *PgStore) UpdateEmbedding(ctx context.Context, ownerID, id string, embedding []float64) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`UPDATE knowledge SET embedding = $3, updated_at = NOW() WHERE owner_id = $1 AND id = $2`, ownerID, id, embeddingToString(embedding))
	return err
}

func (s *PgStore) Count(ctx context.Context, ownerID string) (int, error) {
	var count int
	err := s.executor(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge WHERE owner_id = $1`, ownerID).Scan(&count)
	return count, err
}

func (s *PgStore) CountByStatus(ctx context.Context, ownerID, status string) (int, error) {
	var count int
	err := s.executor(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge WHERE owner_id = $1 AND status = $2`, ownerID, status).Scan(&count)
	return count, err
}

func (s *PgStore) AllOwnerIDs(ctx context.Context) ([]string, error) {
	rows, err := s.executor(ctx).QueryContext(ctx, `SELECT DISTINCT owner_id FROM knowledge`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SearchByVector returns top-k items by cosine similarity within an owner's space.
func (s *PgStore) SearchByVector(ctx context.Context, ownerID string, embedding []float64, limit int, scoreThreshold float64) ([]VectorHit, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from, hit_count, useful_count, last_hit_at,
		created_at, updated_at,
		1 - (embedding <=> $1) AS score
		FROM knowledge
		WHERE owner_id = $2 AND embedding IS NOT NULL AND status = 'active'
		ORDER BY embedding <=> $1
		LIMIT $3`

	rows, err := s.executor(ctx).QueryContext(ctx, query, embeddingToString(embedding), ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []VectorHit
	for rows.Next() {
		var h VectorHit
		err := rows.Scan(
			&h.Item.ID, &h.Item.OwnerID, &h.Item.Title, &h.Item.Content, &h.Item.Summary,
			&h.Item.Source, &h.Item.SourceRef, &h.Item.KnowledgeType, &h.Item.KnowledgeCategory,
			pq.Array(&h.Item.Tags),
			pq.Array(&h.Item.RelatedIDs),
			&h.Item.SupersededBy,
			&h.Item.Status, pq.Array(&h.Item.ConsolidatedFrom),
			&h.Item.HitCount, &h.Item.UsefulCount, &h.Item.LastHitAt,
			&h.Item.CreatedAt, &h.Item.UpdatedAt,
			&h.Score,
		)
		if err != nil {
			return nil, err
		}
		if h.Score >= scoreThreshold {
			hits = append(hits, h)
		}
	}
	return hits, rows.Err()
}

// SearchByFTS runs full-text search within an owner's space.
func (s *PgStore) SearchByFTS(ctx context.Context, ownerID, queryText string, limit int) ([]VectorHit, error) {
	words := strings.Fields(queryText)
	if len(words) == 0 {
		return nil, nil
	}
	// Sanitize each word: remove tsquery special characters to prevent syntax errors
	sanitized := make([]string, 0, len(words))
	for _, w := range words {
		clean := sanitizeTSWord(w)
		if clean != "" {
			sanitized = append(sanitized, clean)
		}
	}
	if len(sanitized) == 0 {
		return nil, nil
	}
	tsQuery := strings.Join(sanitized, " | ")

	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from, hit_count, useful_count, last_hit_at,
		created_at, updated_at,
		ts_rank(to_tsvector('simple', title || ' ' || content), to_tsquery('simple', $1)) AS score
		FROM knowledge
		WHERE owner_id = $2
		AND to_tsvector('simple', title || ' ' || content) @@ to_tsquery('simple', $1)
		AND status = 'active'
		ORDER BY score DESC
		LIMIT $3`

	rows, err := s.executor(ctx).QueryContext(ctx, query, tsQuery, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []VectorHit
	for rows.Next() {
		var h VectorHit
		err := rows.Scan(
			&h.Item.ID, &h.Item.OwnerID, &h.Item.Title, &h.Item.Content, &h.Item.Summary,
			&h.Item.Source, &h.Item.SourceRef, &h.Item.KnowledgeType, &h.Item.KnowledgeCategory,
			pq.Array(&h.Item.Tags),
			pq.Array(&h.Item.RelatedIDs),
			&h.Item.SupersededBy,
			&h.Item.Status, pq.Array(&h.Item.ConsolidatedFrom),
			&h.Item.HitCount, &h.Item.UsefulCount, &h.Item.LastHitAt,
			&h.Item.CreatedAt, &h.Item.UpdatedAt,
			&h.Score,
		)
		if err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// FindSimilar finds items similar to the given embedding above a threshold within an owner's space.
func (s *PgStore) FindSimilar(ctx context.Context, ownerID string, embedding []float64, threshold float64, excludeID string, limit int) ([]VectorHit, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from, hit_count, useful_count, last_hit_at,
		created_at, updated_at,
		1 - (embedding <=> $1) AS score
		FROM knowledge
		WHERE owner_id = $2 AND embedding IS NOT NULL AND status = 'active' AND id != $3
		AND 1 - (embedding <=> $1) >= $4
		ORDER BY embedding <=> $1
		LIMIT $5`

	rows, err := s.executor(ctx).QueryContext(ctx, query, embeddingToString(embedding), ownerID, excludeID, threshold, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []VectorHit
	for rows.Next() {
		var h VectorHit
		err := rows.Scan(
			&h.Item.ID, &h.Item.OwnerID, &h.Item.Title, &h.Item.Content, &h.Item.Summary,
			&h.Item.Source, &h.Item.SourceRef, &h.Item.KnowledgeType, &h.Item.KnowledgeCategory,
			pq.Array(&h.Item.Tags),
			pq.Array(&h.Item.RelatedIDs),
			&h.Item.SupersededBy,
			&h.Item.Status, pq.Array(&h.Item.ConsolidatedFrom),
			&h.Item.HitCount, &h.Item.UsefulCount, &h.Item.LastHitAt,
			&h.Item.CreatedAt, &h.Item.UpdatedAt,
			&h.Score,
		)
		if err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// ListWithoutEmbedding returns items that need embedding generation within an owner's space.
func (s *PgStore) ListWithoutEmbedding(ctx context.Context, ownerID string, limit int) ([]*domain.Knowledge, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from, hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 AND embedding IS NULL LIMIT $2`
	return s.scanMultiple(ctx, query, ownerID, limit)
}

type VectorHit struct {
	Item  domain.Knowledge
	Score float64
}

func (s *PgStore) scanMultiple(ctx context.Context, query string, args ...any) ([]*domain.Knowledge, error) {
	rows, err := s.executor(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.Knowledge
	for rows.Next() {
		k := &domain.Knowledge{}
		err := rows.Scan(
			&k.ID, &k.OwnerID, &k.Title, &k.Content, &k.Summary,
			&k.Source, &k.SourceRef, &k.KnowledgeType, &k.KnowledgeCategory,
			pq.Array(&k.Tags),
			pq.Array(&k.RelatedIDs),
			&k.SupersededBy,
			&k.Status, pq.Array(&k.ConsolidatedFrom),
			&k.HitCount, &k.UsefulCount, &k.LastHitAt,
			&k.CreatedAt, &k.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, k)
	}
	return items, rows.Err()
}

func (s *PgStore) SaveSearchLog(ctx context.Context, log *domain.SearchLog) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`INSERT INTO search_log (id, owner_id, query, result_count, had_feedback, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		log.ID, log.OwnerID, log.Query, log.ResultCount, log.HadFeedback, log.CreatedAt)
	return err
}

func (s *PgStore) ListSearchLogs(ctx context.Context, ownerID string, offset, limit int) ([]*domain.SearchLog, error) {
	rows, err := s.executor(ctx).QueryContext(ctx,
		`SELECT id, owner_id, query, result_count, had_feedback, created_at
		FROM search_log WHERE owner_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		ownerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*domain.SearchLog
	for rows.Next() {
		l := &domain.SearchLog{}
		if err := rows.Scan(&l.ID, &l.OwnerID, &l.Query, &l.ResultCount, &l.HadFeedback, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

const (
	topQueriesLimit     = 10
	zeroResultTermLimit = 20
)

func (s *PgStore) SearchLogStats(ctx context.Context, ownerID string) (*domain.SearchLogStats, error) {
	stats := &domain.SearchLogStats{}

	err := s.executor(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN result_count > 0 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN result_count = 0 THEN 1 ELSE 0 END), 0),
		COALESCE(AVG(result_count), 0)
		FROM search_log WHERE owner_id = $1`, ownerID).Scan(
		&stats.TotalQueries, &stats.WithResults, &stats.ZeroResults, &stats.AvgResultCount)
	if err != nil {
		return nil, fmt.Errorf("search log stats: %w", err)
	}

	if stats.TotalQueries > 0 {
		stats.RecallRate = float64(stats.WithResults) / float64(stats.TotalQueries)
	}

	topRows, err := s.executor(ctx).QueryContext(ctx,
		`SELECT query, COUNT(*) as cnt FROM search_log WHERE owner_id = $1
		GROUP BY query ORDER BY cnt DESC LIMIT $2`, ownerID, topQueriesLimit)
	if err != nil {
		return stats, nil
	}
	defer topRows.Close()
	for topRows.Next() {
		var qc domain.QueryCount
		if err := topRows.Scan(&qc.Query, &qc.Count); err != nil {
			continue
		}
		stats.TopQueries = append(stats.TopQueries, qc)
	}

	zeroRows, err := s.executor(ctx).QueryContext(ctx,
		`SELECT query FROM search_log WHERE owner_id = $1 AND result_count = 0
		GROUP BY query ORDER BY MAX(created_at) DESC LIMIT $2`, ownerID, zeroResultTermLimit)
	if err != nil {
		return stats, nil
	}
	defer zeroRows.Close()
	for zeroRows.Next() {
		var q string
		if err := zeroRows.Scan(&q); err != nil {
			continue
		}
		stats.ZeroResultTerms = append(stats.ZeroResultTerms, q)
	}

	return stats, nil
}

func sanitizeTSWord(w string) string {
	var b strings.Builder
	for _, r := range w {
		if r == '\'' || r == '!' || r == '&' || r == '|' || r == '(' || r == ')' || r == ':' || r == '*' {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func embeddingToString(embedding []float64) *string {
	if len(embedding) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range embedding {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
	}
	b.WriteByte(']')
	s := b.String()
	return &s
}
