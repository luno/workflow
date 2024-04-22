package sqlstore

import (
	"context"
	"database/sql"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"

	"github.com/luno/workflow"
)

func (s *SQLStore) create(ctx context.Context, tx *sql.Tx, workflowName, foreignID, runID string, status int, object []byte, runState int) (int64, error) {
	resp, err := tx.ExecContext(ctx, "insert into "+s.recordTableName+" set "+
		" workflow_name=?, foreign_id=?, run_id=?, run_state=?, status=?, object=?, created_at=now(), updated_at=now() ",
		workflowName,
		foreignID,
		runID,
		runState,
		status,
		object,
	)
	if err != nil {
		return 0, errors.Wrap(err, "failed to create entry", j.MKV{
			"workflowName": workflowName,
			"foreignID":    foreignID,
			"runID":        runID,
			"status":       status,
			"object":       object,
		})
	}

	return resp.LastInsertId()
}

func (s *SQLStore) update(ctx context.Context, tx *sql.Tx, workflowName, foreignID, runID string, status int, object []byte, runState int, id int64) error {
	_, err := tx.ExecContext(ctx, "update "+s.recordTableName+" set "+
		" workflow_name=?, foreign_id=?, run_id=?, run_state=?, status=?, object=?, updated_at=now() where id=?",
		workflowName,
		foreignID,
		runID,
		runState,
		status,
		object,
		id,
	)
	if err != nil {
		return errors.Wrap(err, "failed to create entry", j.MKV{
			"workflowName": workflowName,
			"foreignID":    foreignID,
			"runID":        runID,
			"status":       status,
			"object":       object,
		})
	}

	return nil
}

func (s *SQLStore) insertOutboxEvent(ctx context.Context, tx *sql.Tx, workflowName string, data []byte) (int64, error) {
	resp, err := tx.ExecContext(ctx, "insert into "+s.outboxTableName+" set "+
		" workflow_name=?, data=?, created_at=now() ",
		workflowName,
		data,
	)
	if err != nil {
		return 0, errors.Wrap(err, "failed to create outbox event", j.MKV{
			"workflowName": workflowName,
		})
	}

	return resp.LastInsertId()
}

func (s *SQLStore) lookupWhere(ctx context.Context, dbc *sql.DB, where string, args ...any) (*workflow.WireRecord, error) {
	return recordScan(dbc.QueryRowContext(ctx, s.recordSelectPrefix+where, args...))
}

// listWhere queries the table with the provided where clause, then scans
// and returns all the rows.
func (s *SQLStore) listWhere(ctx context.Context, dbc *sql.DB, where string, args ...any) ([]*workflow.WireRecord, error) {
	rows, err := dbc.QueryContext(ctx, s.recordSelectPrefix+where, args...)
	if err != nil {
		return nil, errors.Wrap(err, "listWhere")
	}
	defer rows.Close()

	var res []*workflow.WireRecord
	for rows.Next() {
		r, err := recordScan(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, r)
	}

	if rows.Err() != nil {
		return nil, errors.Wrap(rows.Err(), "rows")
	}

	return res, nil
}

func (s *SQLStore) listOutboxWhere(ctx context.Context, dbc *sql.DB, where string, args ...any) ([]workflow.OutboxEvent, error) {
	rows, err := dbc.QueryContext(ctx, s.outboxSelectPrefix+where, args...)
	if err != nil {
		return nil, errors.Wrap(err, "listOutboxWhere")
	}
	defer rows.Close()

	var res []workflow.OutboxEvent
	for rows.Next() {
		r, err := outboxScan(rows)
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

func recordScan(row row) (*workflow.WireRecord, error) {
	var r workflow.WireRecord
	err := row.Scan(
		&r.ID,
		&r.WorkflowName,
		&r.ForeignID,
		&r.RunID,
		&r.RunState,
		&r.Status,
		&r.Object,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.Wrap(workflow.ErrRecordNotFound, "")
	} else if err != nil {
		return nil, errors.Wrap(err, "recordScan")
	}

	return &r, nil
}

func outboxScan(row row) (*workflow.OutboxEvent, error) {
	var e workflow.OutboxEvent
	err := row.Scan(
		&e.ID,
		&e.WorkflowName,
		&e.Data,
		&e.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.Wrap(workflow.ErrOutboxRecordNotFound, "")
	} else if err != nil {
		return nil, errors.Wrap(err, "outboxScan")
	}

	return &e, nil
}

// row is a common interface for *sql.Rows and *sql.Row.
type row interface {
	Scan(dest ...any) error
}
