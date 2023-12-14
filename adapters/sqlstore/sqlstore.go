package sqlstore

import (
	"context"
	"database/sql"

	"github.com/luno/jettison/errors"

	"github.com/andrewwormald/workflow"
)

type SQLStore struct {
	writer *sql.DB
	reader *sql.DB

	recordTableName    string
	recordCols         string
	recordSelectPrefix string
}

func New(writer *sql.DB, reader *sql.DB, tableName string) *SQLStore {
	e := &SQLStore{
		writer:          writer,
		reader:          reader,
		recordTableName: tableName,
	}

	e.recordCols = " `id`, `workflow_name`, `foreign_id`, `run_id`, `status`, `object`, `is_start`, `is_end`, `created_at` "
	e.recordSelectPrefix = " select " + e.recordCols + " from " + e.recordTableName + " where "

	return e
}

var _ workflow.RecordStore = (*SQLStore)(nil)

func (s *SQLStore) Store(ctx context.Context, r *workflow.WireRecord, eventEmitter workflow.EventEmitter) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var mustCreate bool
	if r.ID != 0 {
		_, err := recordScan(tx.QueryRowContext(ctx, s.recordSelectPrefix+"id=?", r.ID))
		if errors.Is(err, workflow.ErrRecordNotFound) {
			mustCreate = true
		} else if err != nil {
			return err
		}
	} else {
		mustCreate = true
	}

	var id int64
	if mustCreate {
		id, err = s.create(ctx, tx, r.WorkflowName, r.ForeignID, r.RunID, r.Status, r.Object, r.IsStart, r.IsEnd)
		if err != nil {
			return err
		}
	} else {
		err := s.update(ctx, tx, r.WorkflowName, r.ForeignID, r.RunID, r.Status, r.Object, r.IsStart, r.IsEnd, r.ID)
		if err != nil {
			return err
		}

		// For updates
		id = r.ID
	}

	err = eventEmitter(id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLStore) Lookup(ctx context.Context, id int64) (*workflow.WireRecord, error) {
	return s.lookupWhere(ctx, s.reader, "id=?", id)
}

func (s *SQLStore) Latest(ctx context.Context, workflowName, foreignID string) (*workflow.WireRecord, error) {
	ls, err := s.listWhere(ctx, s.reader, "workflow_name=? and foreign_id=? order by id desc limit 1", workflowName, foreignID)
	if err != nil {
		return nil, err
	}

	if len(ls) < 1 {
		return nil, errors.Wrap(workflow.ErrRecordNotFound, "")
	}

	return ls[0], nil
}
