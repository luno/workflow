package sqltimeout

import (
	"context"
	"database/sql"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"

	"github.com/andrewwormald/workflow"
)

func New(writer, reader *sql.DB, tableName string) *Store {
	var s Store
	s.writer = writer
	s.reader = reader

	s.timeoutTableName = tableName
	s.timeoutCols = " `id`, `workflow_name`, `foreign_id`, `run_id`, `status`, `completed`, `expire_at`, `created_at` "
	s.timeoutSelectPrefix = " select " + s.timeoutCols + " from " + s.timeoutTableName + " where "

	return &s
}

type Store struct {
	writer *sql.DB
	reader *sql.DB

	timeoutTableName    string
	timeoutCols         string
	timeoutSelectPrefix string
}

func (s *Store) Create(ctx context.Context, workflowName, foreignID, runID string, status int, expireAt time.Time) error {
	_, err := s.writer.ExecContext(ctx, "insert into "+s.timeoutTableName+" set "+
		" workflow_name=?, foreign_id=?, run_id=?, status=?, completed=?, expire_at=?, created_at=now() ",
		workflowName,
		foreignID,
		runID,
		status,
		false,
		expireAt,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create timeout", j.MKV{
			"workflowName": workflowName,
			"foreignID":    foreignID,
			"runID":        runID,
			"status":       status,
			"expireAt":     expireAt,
		})
	}

	return nil
}

func (s *Store) Complete(ctx context.Context, workflowName, foreignID, runID string, status int) error {
	_, err := s.writer.ExecContext(ctx, "update "+s.timeoutTableName+" set completed=true where workflow_name=? and foreign_id=? and run_id=? and status=?", workflowName, foreignID, runID, status)
	if err != nil {
		return errors.Wrap(err, "failed to complete timeout", j.MKV{
			"workflowName": workflowName,
			"foreignID":    foreignID,
			"runID":        runID,
			"status":       status,
		})
	}

	return nil
}

func (s *Store) Cancel(ctx context.Context, workflowName, foreignID, runID string, status int) error {
	_, err := s.writer.ExecContext(ctx, "delete from "+s.timeoutTableName+" where workflow_name=? and foreign_id=? and run_id=? and status=?", workflowName, foreignID, runID, status)
	if err != nil {
		return errors.Wrap(err, "failed to cancel / delete timeout", j.MKV{
			"workflowName": workflowName,
			"foreignID":    foreignID,
			"runID":        runID,
			"status":       status,
		})
	}

	return nil
}

func (s *Store) List(ctx context.Context, workflowName string) ([]workflow.Timeout, error) {
	rows, err := s.reader.QueryContext(ctx, s.timeoutSelectPrefix+" workflow_name=? and completed=false", workflowName)
	if err != nil {
		return nil, errors.Wrap(err, "list timeouts")
	}
	defer rows.Close()

	var res []workflow.Timeout
	for rows.Next() {
		r, err := timeoutScan(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *r)
	}

	if rows.Err() != nil {
		return nil, errors.Wrap(rows.Err(), "rows")
	}

	return res, nil
}

func (s *Store) ListValid(ctx context.Context, workflowName string, status int, now time.Time) ([]workflow.Timeout, error) {
	rows, err := s.reader.QueryContext(ctx, s.timeoutSelectPrefix+" workflow_name=? and status=? and expire_at<? and completed=false", workflowName, status, now)
	if err != nil {
		return nil, errors.Wrap(err, "list valid timeouts")
	}
	defer rows.Close()

	var res []workflow.Timeout
	for rows.Next() {
		r, err := timeoutScan(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *r)
	}

	if rows.Err() != nil {
		return nil, errors.Wrap(rows.Err(), "rows")
	}

	return res, nil
}

var _ workflow.TimeoutStore = (*Store)(nil)

func timeoutScan(row row) (*workflow.Timeout, error) {
	var t workflow.Timeout
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
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.Wrap(workflow.ErrTimeoutNotFound, "")
	} else if err != nil {
		return nil, errors.Wrap(err, "scan timeout")
	}

	return &t, nil
}

type row interface {
	Scan(dest ...any) error
}
