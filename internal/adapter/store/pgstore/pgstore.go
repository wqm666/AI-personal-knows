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

type StoreOpts struct {
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetimeSec int
}

func New(dsn string, embeddingDim int, opts ...StoreOpts) (*PgStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	maxOpen, maxIdle, lifetime := 20, 5, 5*time.Minute
	if len(opts) > 0 {
		o := opts[0]
		if o.MaxOpenConns > 0 {
			maxOpen = o.MaxOpenConns
		}
		if o.MaxIdleConns > 0 {
			maxIdle = o.MaxIdleConns
		}
		if o.ConnMaxLifetimeSec > 0 {
			lifetime = time.Duration(o.ConnMaxLifetimeSec) * time.Second
		}
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(lifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	s := &PgStore{db: db}
	if err := s.initSchema(embeddingDim); err != nil {
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
			related_ids, superseded_by, status, consolidated_from,
			review_status, confidence, review_reason, reviewed_at,
			hit_count, useful_count, last_hit_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)`

	knowledgeType := k.KnowledgeType
	if knowledgeType == "" {
		knowledgeType = domain.TypeGeneral
	}

	reviewStatus := k.ReviewStatus
	if reviewStatus == "" {
		reviewStatus = domain.ReviewPending
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
		reviewStatus, k.Confidence, k.ReviewReason, k.ReviewedAt,
		k.HitCount, k.UsefulCount, k.LastHitAt,
		k.CreatedAt, k.UpdatedAt,
	)
	return err
}

func (s *PgStore) Get(ctx context.Context, ownerID, id string) (*domain.Knowledge, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		embedding,
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 AND id = $2`

	k := &domain.Knowledge{}
	var embStr *string
	err := s.executor(ctx).QueryRowContext(ctx, query, ownerID, id).Scan(
		&k.ID, &k.OwnerID, &k.Title, &k.Content, &k.Summary,
		&k.Source, &k.SourceRef, &k.KnowledgeType, &k.KnowledgeCategory,
		pq.Array(&k.Tags),
		&embStr,
		pq.Array(&k.RelatedIDs),
		&k.SupersededBy,
		&k.Status, pq.Array(&k.ConsolidatedFrom),
		&k.ReviewStatus, &k.Confidence, &k.ReviewReason, &k.ReviewedAt,
		&k.HitCount, &k.UsefulCount, &k.LastHitAt,
		&k.CreatedAt, &k.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if embStr != nil {
		k.Embedding = parseEmbeddingString(*embStr)
	}
	return k, nil
}

func (s *PgStore) Update(ctx context.Context, k *domain.Knowledge) error {
	query := `UPDATE knowledge SET title=$3, content=$4, summary=$5, source=$6, source_ref=$7,
		knowledge_type=$8, knowledge_category=$9, tags=$10, related_ids=$11, superseded_by=$12, status=$13, consolidated_from=$14,
		review_status=$15, confidence=$16, review_reason=$17, reviewed_at=$18,
		hit_count=$19, useful_count=$20, last_hit_at=$21, updated_at=$22
		WHERE owner_id=$1 AND id=$2`

	_, err := s.executor(ctx).ExecContext(ctx, query,
		k.OwnerID, k.ID, k.Title, k.Content, k.Summary,
		k.Source, k.SourceRef, k.KnowledgeType, k.KnowledgeCategory,
		pq.Array(k.Tags),
		pq.Array(k.RelatedIDs),
		k.SupersededBy,
		k.Status, pq.Array(k.ConsolidatedFrom),
		k.ReviewStatus, k.Confidence, k.ReviewReason, k.ReviewedAt,
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
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	return s.scanMultiple(ctx, query, ownerID, limit, offset)
}

func (s *PgStore) ListByStatus(ctx context.Context, ownerID, status string, offset, limit int) ([]*domain.Knowledge, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 AND status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	return s.scanMultiple(ctx, query, ownerID, status, limit, offset)
}

func (s *PgStore) ListByIDs(ctx context.Context, ownerID string, ids []string) ([]*domain.Knowledge, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
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

func (s *PgStore) UpdateReviewStatus(ctx context.Context, ownerID, id, reviewStatus string, confidence int, reason string) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`UPDATE knowledge SET review_status = $3, confidence = $4, review_reason = $5, reviewed_at = NOW(), updated_at = NOW() WHERE owner_id = $1 AND id = $2`,
		ownerID, id, reviewStatus, confidence, reason)
	return err
}

func (s *PgStore) ListByReviewStatus(ctx context.Context, ownerID, reviewStatus string, offset, limit int) ([]*domain.Knowledge, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 AND review_status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	return s.scanMultiple(ctx, query, ownerID, reviewStatus, limit, offset)
}

func (s *PgStore) CountByReviewStatus(ctx context.Context, ownerID, reviewStatus string) (int, error) {
	var count int
	err := s.executor(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge WHERE owner_id = $1 AND review_status = $2`, ownerID, reviewStatus).Scan(&count)
	return count, err
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

func (s *PgStore) TagStats(ctx context.Context, ownerID, status string) (map[string]int, error) {
	rows, err := s.executor(ctx).QueryContext(ctx,
		`SELECT unnest(tags) AS tag, COUNT(*) FROM knowledge WHERE owner_id = $1 AND status = $2 GROUP BY tag ORDER BY count DESC`,
		ownerID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			return nil, err
		}
		result[tag] = count
	}
	return result, rows.Err()
}

// SearchByVector returns top-k items by cosine similarity within an owner's space.
func (s *PgStore) SearchByVector(ctx context.Context, ownerID string, embedding []float64, limit int, scoreThreshold float64) ([]VectorHit, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
		created_at, updated_at,
		1 - (embedding <=> $1) AS score
		FROM knowledge
		WHERE owner_id = $2 AND embedding IS NOT NULL AND status = 'active' AND review_status = 'approved'
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
			&h.Item.ReviewStatus, &h.Item.Confidence, &h.Item.ReviewReason, &h.Item.ReviewedAt,
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
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
		created_at, updated_at,
		ts_rank(to_tsvector('simple', title || ' ' || content), to_tsquery('simple', $1)) AS score
		FROM knowledge
		WHERE owner_id = $2
		AND to_tsvector('simple', title || ' ' || content) @@ to_tsquery('simple', $1)
		AND status = 'active' AND review_status = 'approved'
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
			&h.Item.ReviewStatus, &h.Item.Confidence, &h.Item.ReviewReason, &h.Item.ReviewedAt,
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

// SearchByKeyword performs database-side keyword matching using ILIKE.
// Each keyword is matched against title, content, and tags; a score is computed
// from the number of field hits (title=2, content=1, tags=1.5) normalised by max possible.
func (s *PgStore) SearchByKeyword(ctx context.Context, ownerID string, keywords []string, limit int) ([]VectorHit, error) {
	if len(keywords) == 0 {
		return nil, nil
	}

	// Build a CASE expression per keyword that sums field-weighted matches.
	// All keywords are passed via parameter placeholders to avoid injection.
	//
	// Parameters layout:  $1 = ownerID, $2..$N+1 = keywords, $N+2 = limit
	var sb strings.Builder
	sb.WriteString(`SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
		created_at, updated_at, (`)

	args := make([]any, 0, len(keywords)+2)
	args = append(args, ownerID) // $1

	for i, kw := range keywords {
		paramIdx := i + 2 // $2, $3, ...
		args = append(args, "%"+kw+"%")
		if i > 0 {
			sb.WriteString(" + ")
		}
		p := fmt.Sprintf("$%d", paramIdx)
		sb.WriteString(fmt.Sprintf(
			"(CASE WHEN title ILIKE %s THEN 2.0 ELSE 0 END + CASE WHEN content ILIKE %s THEN 1.0 ELSE 0 END + CASE WHEN array_to_string(tags,' ') ILIKE %s THEN 1.5 ELSE 0 END)",
			p, p, p))
	}

	maxScore := float64(len(keywords)) * 4.5
	sb.WriteString(fmt.Sprintf(") / %f AS score FROM knowledge WHERE owner_id = $1 AND status = 'active' AND review_status = 'approved' AND (", maxScore))

	// WHERE filter: at least one keyword matches any field
	for i := range keywords {
		paramIdx := i + 2
		if i > 0 {
			sb.WriteString(" OR ")
		}
		p := fmt.Sprintf("$%d", paramIdx)
		sb.WriteString(fmt.Sprintf("title ILIKE %s OR content ILIKE %s OR array_to_string(tags,' ') ILIKE %s", p, p, p))
	}

	limitIdx := len(keywords) + 2
	sb.WriteString(fmt.Sprintf(") ORDER BY score DESC LIMIT $%d", limitIdx))
	args = append(args, limit)

	rows, err := s.executor(ctx).QueryContext(ctx, sb.String(), args...)
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
			&h.Item.ReviewStatus, &h.Item.Confidence, &h.Item.ReviewReason, &h.Item.ReviewedAt,
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
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
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
			&h.Item.ReviewStatus, &h.Item.Confidence, &h.Item.ReviewReason, &h.Item.ReviewedAt,
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
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 AND embedding IS NULL LIMIT $2`
	return s.scanMultiple(ctx, query, ownerID, limit)
}

// ListActiveWithEmbedding returns active items that have embeddings, for use by maintenance tasks
// that need the embedding vector (e.g., link discovery).
func (s *PgStore) ListActiveWithEmbedding(ctx context.Context, ownerID string, limit int) ([]*domain.Knowledge, error) {
	query := `SELECT id, owner_id, title, content, summary, source, source_ref, knowledge_type, knowledge_category, tags,
		embedding,
		related_ids, superseded_by, status, consolidated_from,
		review_status, confidence, review_reason, reviewed_at,
		hit_count, useful_count, last_hit_at,
		created_at, updated_at FROM knowledge WHERE owner_id = $1 AND status = 'active' AND embedding IS NOT NULL LIMIT $2`

	rows, err := s.executor(ctx).QueryContext(ctx, query, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.Knowledge
	for rows.Next() {
		k := &domain.Knowledge{}
		var embStr *string
		err := rows.Scan(
			&k.ID, &k.OwnerID, &k.Title, &k.Content, &k.Summary,
			&k.Source, &k.SourceRef, &k.KnowledgeType, &k.KnowledgeCategory,
			pq.Array(&k.Tags),
			&embStr,
			pq.Array(&k.RelatedIDs),
			&k.SupersededBy,
			&k.Status, pq.Array(&k.ConsolidatedFrom),
			&k.ReviewStatus, &k.Confidence, &k.ReviewReason, &k.ReviewedAt,
			&k.HitCount, &k.UsefulCount, &k.LastHitAt,
			&k.CreatedAt, &k.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if embStr != nil {
			k.Embedding = parseEmbeddingString(*embStr)
		}
		items = append(items, k)
	}
	return items, rows.Err()
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
			&k.ReviewStatus, &k.Confidence, &k.ReviewReason, &k.ReviewedAt,
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
	source := log.Source
	if source == "" {
		source = domain.SearchSourceAPI
	}
	_, err := s.executor(ctx).ExecContext(ctx,
		`INSERT INTO search_log (id, owner_id, query, source, result_count, result_ids, had_feedback, feedback_bad_ids, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		log.ID, log.OwnerID, log.Query, source, log.ResultCount, pq.Array(log.ResultIDs), log.HadFeedback, pq.Array(log.FeedbackBadIDs), log.CreatedAt)
	return err
}

func (s *PgStore) ListSearchLogs(ctx context.Context, ownerID string, offset, limit int) ([]*domain.SearchLog, error) {
	rows, err := s.executor(ctx).QueryContext(ctx,
		`SELECT id, owner_id, query, source, result_count, result_ids, had_feedback, feedback_bad_ids, created_at
		FROM search_log WHERE owner_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		ownerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*domain.SearchLog
	for rows.Next() {
		l := &domain.SearchLog{}
		if err := rows.Scan(&l.ID, &l.OwnerID, &l.Query, &l.Source, &l.ResultCount, pq.Array(&l.ResultIDs), &l.HadFeedback, pq.Array(&l.FeedbackBadIDs), &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (s *PgStore) ListSearchLogsFiltered(ctx context.Context, ownerID, source string, offset, limit int) ([]*domain.SearchLog, error) {
	var rows *sql.Rows
	var err error
	if source != "" {
		rows, err = s.executor(ctx).QueryContext(ctx,
			`SELECT id, owner_id, query, source, result_count, result_ids, had_feedback, feedback_bad_ids, created_at
			FROM search_log WHERE owner_id = $1 AND source = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
			ownerID, source, limit, offset)
	} else {
		rows, err = s.executor(ctx).QueryContext(ctx,
			`SELECT id, owner_id, query, source, result_count, result_ids, had_feedback, feedback_bad_ids, created_at
			FROM search_log WHERE owner_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			ownerID, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*domain.SearchLog
	for rows.Next() {
		l := &domain.SearchLog{}
		if err := rows.Scan(&l.ID, &l.OwnerID, &l.Query, &l.Source, &l.ResultCount, pq.Array(&l.ResultIDs), &l.HadFeedback, pq.Array(&l.FeedbackBadIDs), &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (s *PgStore) CountSearchLogs(ctx context.Context, ownerID, source string) (int, error) {
	var count int
	var err error
	if source != "" {
		err = s.executor(ctx).QueryRowContext(ctx,
			`SELECT COUNT(*) FROM search_log WHERE owner_id = $1 AND source = $2`, ownerID, source).Scan(&count)
	} else {
		err = s.executor(ctx).QueryRowContext(ctx,
			`SELECT COUNT(*) FROM search_log WHERE owner_id = $1`, ownerID).Scan(&count)
	}
	return count, err
}

func (s *PgStore) MarkSearchBadFeedback(ctx context.Context, ownerID, logID, badItemID string) error {
	_, err := s.executor(ctx).ExecContext(ctx,
		`UPDATE search_log SET feedback_bad_ids = array_append(feedback_bad_ids, $3), had_feedback = TRUE
		WHERE owner_id = $1 AND id = $2 AND NOT ($3 = ANY(feedback_bad_ids))`,
		ownerID, logID, badItemID)
	return err
}

const defaultHitRankingLimit = 20

func (s *PgStore) KnowledgeHitRanking(ctx context.Context, ownerID string, limit int) ([]domain.KnowledgeHitRank, error) {
	if limit <= 0 {
		limit = defaultHitRankingLimit
	}
	rows, err := s.executor(ctx).QueryContext(ctx,
		`SELECT k.id, k.title, k.hit_count, k.useful_count, COALESCE(bc.bad_count, 0) AS bad_count
		FROM knowledge k
		LEFT JOIN (
			SELECT unnest(feedback_bad_ids) AS kid, COUNT(*) AS bad_count
			FROM search_log WHERE owner_id = $1
			GROUP BY kid
		) bc ON bc.kid = k.id
		WHERE k.owner_id = $1 AND k.hit_count > 0
		ORDER BY k.hit_count DESC
		LIMIT $2`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ranks []domain.KnowledgeHitRank
	for rows.Next() {
		var r domain.KnowledgeHitRank
		if err := rows.Scan(&r.ID, &r.Title, &r.HitCount, &r.UsefulCount, &r.BadCount); err != nil {
			return nil, err
		}
		if r.HitCount > 0 {
			r.UsefulRate = float64(r.UsefulCount) / float64(r.HitCount)
		}
		ranks = append(ranks, r)
	}
	return ranks, rows.Err()
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
		switch r {
		case '\'', '!', '&', '|', '(', ')', ':', '*', '<', '>', '\\', '"', '{', '}', '[', ']', ';', '$', '/':
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func parseEmbeddingString(s string) []float64 {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]float64, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			continue
		}
		result = append(result, v)
	}
	return result
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
