package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/luno/workflow"
)

type TimeoutStore struct {
	db *sql.DB
}

func NewTimeoutStore(db *sql.DB) *TimeoutStore {
	return &TimeoutStore{db: db}
}

var _ workflow.TimeoutStore = (*TimeoutStore)(nil)

func (s *TimeoutStore) Create(ctx context.Context, workflowName, foreignID, runID string, status int, expireAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO workflow_timeouts
		(workflow_name, foreign_id, run_id, status, completed, expire_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		workflowName, foreignID, runID, status, false, expireAt,
	)
	if err != nil {
		return fmt.Errorf("create timeout: %w", err)
	}

	return nil
}

func (s *TimeoutStore) Complete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE workflow_timeouts SET completed = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("complete timeout: %w", err)
	}

	return nil
}

func (s *TimeoutStore) Cancel(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM workflow_timeouts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("cancel timeout: %w", err)
	}

	return nil
}

func (s *TimeoutStore) List(ctx context.Context, workflowName string) ([]workflow.TimeoutRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workflow_name, foreign_id, run_id, status, completed, expire_at, created_at
		FROM workflow_timeouts
		WHERE workflow_name = ? AND completed = 0`,
		workflowName,
	)
	if err != nil {
		return nil, fmt.Errorf("list timeouts: %w", err)
	}
	defer rows.Close()

	var res []workflow.TimeoutRecord
	for rows.Next() {
		r, err := timeoutScan(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *r)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("rows error: %w", rows.Err())
	}

	return res, nil
}

func (s *TimeoutStore) ListValid(ctx context.Context, workflowName string, status int, now time.Time) ([]workflow.TimeoutRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workflow_name, foreign_id, run_id, status, completed, expire_at, created_at
		FROM workflow_timeouts
		WHERE workflow_name = ? AND status = ? AND expire_at < ? AND completed = 0`,
		workflowName, status, now,
	)
	if err != nil {
		return nil, fmt.Errorf("list valid timeouts: %w", err)
	}
	defer rows.Close()

	var res []workflow.TimeoutRecord
	for rows.Next() {
		r, err := timeoutScan(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *r)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("rows error: %w", rows.Err())
	}

	return res, nil
}

func timeoutScan(row scannable) (*workflow.TimeoutRecord, error) {
	var t workflow.TimeoutRecord
	err := row.Scan(
		&t.ID,
		&t.WorkflowName,
		&t.ForeignID,
		&t.RunID,
		&t.Status,
		&t.Completed,
		&t.ExpireAt,
		&t.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, workflow.ErrTimeoutNotFound
	} else if err != nil {
		return nil, fmt.Errorf("scan timeout: %w", err)
	}

	return &t, nil
}
