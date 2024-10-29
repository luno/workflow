package sqlstore

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/luno/jettison/errors"
	"github.com/luno/workflow"
)

const defaultListLimit = 25

type SQLStore struct {
	writer *sql.DB
	reader *sql.DB

	recordTableName    string
	recordCols         string
	recordSelectPrefix string

	outboxTableName    string
	outboxCols         string
	outboxSelectPrefix string
}

func New(writer *sql.DB, reader *sql.DB, recordTableName string, outboxTableName string) *SQLStore {
	e := &SQLStore{
		writer:          writer,
		reader:          reader,
		recordTableName: recordTableName,
		outboxTableName: outboxTableName,
	}

	e.recordCols = " `id`, `workflow_name`, `foreign_id`, `run_id`, `run_state`, `status`, `object`, `created_at`, `updated_at` "
	e.recordSelectPrefix = " select " + e.recordCols + " from " + e.recordTableName + " where "

	e.outboxCols = " `id`, `workflow_name`, `data`, `created_at` "
	e.outboxSelectPrefix = " select " + e.outboxCols + " from " + e.outboxTableName + " where "

	return e
}

var _ workflow.RecordStore = (*SQLStore)(nil)

func (s *SQLStore) Store(ctx context.Context, r *workflow.Record, maker workflow.OutboxEventDataMaker) error {
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

	var recordID int64
	if mustCreate {
		recordID, err = s.create(ctx, tx, r.WorkflowName, r.ForeignID, r.RunID, r.Status, r.Object, int(r.RunState))
		if err != nil {
			return err
		}
	} else {
		err := s.update(ctx, tx, r.WorkflowName, r.ForeignID, r.RunID, r.Status, r.Object, int(r.RunState), r.ID)
		if err != nil {
			return err
		}

		// Set so that outbox event maker below receives the recordID in a consistent manner.
		recordID = r.ID
	}

	eventData, err := maker(recordID)
	if err != nil {
		return err
	}

	_, err = s.insertOutboxEvent(ctx, tx, eventData.WorkflowName, eventData.Data)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLStore) Lookup(ctx context.Context, id int64) (*workflow.Record, error) {
	return s.lookupWhere(ctx, s.reader, "id=?", id)
}

func (s *SQLStore) Latest(ctx context.Context, workflowName, foreignID string) (*workflow.Record, error) {
	ls, err := s.listWhere(ctx, s.reader, "workflow_name=? and foreign_id=? order by id desc limit 1", workflowName, foreignID)
	if err != nil {
		return nil, err
	}

	if len(ls) < 1 {
		return nil, errors.Wrap(workflow.ErrRecordNotFound, "")
	}

	return &ls[0], nil
}

func (s *SQLStore) ListOutboxEvents(ctx context.Context, workflowName string, limit int64) ([]workflow.OutboxEvent, error) {
	return s.listOutboxWhere(ctx, s.reader, "workflow_name=? limit ?", workflowName, limit)
}

func (s *SQLStore) DeleteOutboxEvent(ctx context.Context, id int64) error {
	_, err := s.writer.ExecContext(ctx, "delete from "+s.outboxTableName+" where id=?;", id)
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) List(ctx context.Context, workflowName string, offsetID int64, limit int, order workflow.OrderType, filters ...workflow.RecordFilter) ([]workflow.Record, error) {
	filter := workflow.MakeFilter(filters...)

	var filterStr string
	if filter.ByForeignID().Enabled {
		filterStr += fmt.Sprintf(" and foreign_id='%v' ", filter.ByForeignID().Value)
	}

	if filter.ByStatus().Enabled {
		filterStr += fmt.Sprintf(" and status=%v ", filter.ByStatus().Value)
	}

	if filter.ByRunState().Enabled {
		filterStr += fmt.Sprintf(" and run_state=%v ", filter.ByRunState().Value)
	}

	if limit == 0 {
		limit = defaultListLimit
	}

	if workflowName == "" {
		return s.listWhere(ctx, s.reader, "id>? "+filterStr+"order by id "+order.String()+" limit ?", offsetID, limit)
	}

	return s.listWhere(ctx, s.reader, "workflow_name=? and id>? "+filterStr+"order by id "+order.String()+" limit ?", workflowName, offsetID, limit)
}
