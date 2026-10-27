package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://lethe:lethe_password@localhost:5432/lethe_test?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	batchSize := 2
	runStart := time.Now().UTC()
	cutoff := runStart.AddDate(-3, 0, 0)

	fmt.Printf("Starting spike run at %s (cutoff: %s, batch size: %d)\n",
		runStart.Format(time.RFC3339), cutoff.Format(time.RFC3339), batchSize)

	totalDeleted := 0
	batchNo := 0

	for {
		batchNo++

		done, count, err := processBatch(ctx, pool, batchNo, cutoff, runStart, batchSize)
		if err != nil {
			log.Fatalf("batch %d failed: %v", batchNo, err)
		}

		totalDeleted += count

		if done {
			break
		}
	}

	fmt.Printf("\nDone! Total users deleted: %d\n", totalDeleted)
}

// processBatch executes a single batch in an isolated transaction.
// Deferring tx.Rollback here ensures immediate cleanup per batch iteration.
func processBatch(
	ctx context.Context,
	pool *pgxpool.Pool,
	batchNo int,
	cutoff, runStart time.Time,
	batchSize int,
) (done bool, deletedCount int, err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id FROM users 
		WHERE last_activity_at < $1 
		  AND NOT (kyc_verified = true) 
		  AND NOT EXISTS (
		    SELECT 1 FROM lethe_holds h 
		    WHERE h.entity = 'users' 
		      AND h.record_id = users.id::text 
		      AND h.released_at IS NULL 
		      AND (h.until IS NULL OR h.until > $2)
		  )
		ORDER BY id
		LIMIT $3
		FOR UPDATE SKIP LOCKED;
	`

	rows, err := tx.Query(ctx, query, cutoff, runStart, batchSize)
	if err != nil {
		return false, 0, fmt.Errorf("candidate query failed: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return false, 0, fmt.Errorf("row scan failed: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return false, 0, fmt.Errorf("row iteration error: %w", err)
	}

	if len(ids) == 0 {
		fmt.Printf("Batch %d: No more eligible records found. Exiting.\n", batchNo)
		return true, 0, nil
	}

	delChildrenQuery := `DELETE FROM user_sessions WHERE user_id = ANY($1);`
	delParentsQuery := `DELETE FROM users WHERE id = ANY($1);`

	cmdTagChildren, err := tx.Exec(ctx, delChildrenQuery, ids)
	if err != nil {
		return false, 0, fmt.Errorf("child delete failed: %w", err)
	}

	cmdTagParents, err := tx.Exec(ctx, delParentsQuery, ids)
	if err != nil {
		return false, 0, fmt.Errorf("parent delete failed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, 0, fmt.Errorf("failed to commit tx: %w", err)
	}

	fmt.Printf("Batch %d: locked %v | deleted %d sessions, %d users\n",
		batchNo, ids, cmdTagChildren.RowsAffected(), cmdTagParents.RowsAffected())

	isDone := len(ids) < batchSize
	if isDone {
		fmt.Printf("Batch %d returned %d (< %d). Completed.\n", batchNo, len(ids), batchSize)
	}

	return isDone, len(ids), nil
}
