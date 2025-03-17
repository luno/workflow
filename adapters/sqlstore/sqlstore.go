package sqlstore

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

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

	e.recordCols = " `workflow_name`, `foreign_id`, `run_id`, `run_state`, `status`, `object`, `created_at`, `updated_at`, `meta` "
	e.recordSelectPrefix = " select " + e.recordCols + " from " + e.recordTableName + " where "

	e.outboxCols = " `id`, `workflow_name`, `data`, `created_at` "
	e.outboxSelectPrefix = " select " + e.outboxCols + " from " + e.outboxTableName + " where "

	return e
}

var _ workflow.RecordStore = (*SQLStore)(nil)

func (s *SQLStore) Store(ctx context.Context, r *workflow.Record) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var mustCreate bool
	if r.RunID != "" {
		_, err := recordScan(tx.QueryRowContext(ctx, s.recordSelectPrefix+"run_id=?", r.RunID))
		if errors.Is(err, workflow.ErrRecordNotFound) {
			mustCreate = true
		} else if err != nil {
			return err
		}

	} else {
		mustCreate = true
	}

	if mustCreate {
		err := s.create(ctx, tx, r.WorkflowName, r.ForeignID, r.RunID, r.Status, r.Object, int(r.RunState), r.Meta)
		if err != nil {
			return err
		}
	} else {
		err := s.update(ctx, tx, r.RunID, r.Status, r.Object, int(r.RunState), r.Meta)
		if err != nil {
			return err
		}
	}

	eventData, err := workflow.MakeOutboxEventData(*r)
	if err != nil {
		return err
	}

	_, err = s.insertOutboxEvent(ctx, tx, eventData.ID, eventData.WorkflowName, eventData.Data)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLStore) Lookup(ctx context.Context, runID string) (*workflow.Record, error) {
	return s.lookupWhere(ctx, s.reader, "run_id=?", runID)
}

func (s *SQLStore) Latest(ctx context.Context, workflowName, foreignID string) (*workflow.Record, error) {
	ls, err := s.listWhere(
		ctx,
		s.reader,
		"workflow_name=? and foreign_id=? order by created_at desc limit 1",
		workflowName,
		foreignID,
	)
	if err != nil {
		return nil, err
	}

	if len(ls) < 1 {
		return nil, errors.Wrap(workflow.ErrRecordNotFound, "")
	}

	return &ls[0], nil
}

func (s *SQLStore) ListOutboxEvents(
	ctx context.Context,
	workflowName string,
	limit int64,
) ([]workflow.OutboxEvent, error) {
	return s.listOutboxWhere(ctx, s.reader, "workflow_name=? limit ?", workflowName, limit)
}

func (s *SQLStore) DeleteOutboxEvent(ctx context.Context, id string) error {
	_, err := s.writer.ExecContext(ctx, "delete from "+s.outboxTableName+" where id=?;", id)
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) List(
	ctx context.Context,
	workflowName string,
	offset int64,
	limit int,
	order workflow.OrderType,
	filters ...workflow.RecordFilter,
) ([]workflow.Record, error) {
	filter := workflow.MakeFilter(filters...)

	wb := new(whereBuilder)

	if workflowName != "" {
		wb.Where("workflow_name", workflowName)
	}

	if filter.ByForeignID().Enabled {
		if filter.ByForeignID().IsMultiMatch {
			wb.Where("foreign_id", filter.ByForeignID().MultiValues()...)
		} else {
			wb.Where("foreign_id", filter.ByForeignID().Value())
		}
	}

	if filter.ByStatus().Enabled {
		if filter.ByStatus().IsMultiMatch {
			wb.Where("status", filter.ByStatus().MultiValues()...)
		} else {
			wb.Where("status", filter.ByStatus().Value())
		}
	}

	if filter.ByRunState().Enabled {
		if filter.ByRunState().IsMultiMatch {
			wb.Where("run_state", filter.ByRunState().MultiValues()...)
		} else {
			wb.Where("run_state", filter.ByRunState().Value())
		}
	}

	if limit == 0 {
		limit = defaultListLimit
	}

	wb.WhereNotNull("run_id")
	wb.OrderBy("created_at", order)
	wb.Limit(limit)
	wb.Offset(offset)

	where, params := wb.Finalise()
	return s.listWhere(ctx, s.reader, where, params...)
}

type whereBuilder struct {
	conditions []string
	params     []any
	orderField string
	orderType  workflow.OrderType
	offset     int64
	limit      int
}

func (fq *whereBuilder) WhereNotNull(field string) {
	fq.conditions = append(fq.conditions, field+" is not null")
}

func (fq *whereBuilder) Where(field string, values ...string) {
	condition := " ( "
	for i, value := range values {
		condition += field + "=?"
		if i < len(values)-1 {
			condition += " OR "
		}

		fq.params = append(fq.params, value)
	}
	condition += " ) "

	fq.conditions = append(fq.conditions, condition)
}

func (fq *whereBuilder) OrderBy(field string, orderType workflow.OrderType) {
	fq.orderField = field
	fq.orderType = orderType
}

func (fq *whereBuilder) Offset(offset int64) {
	fq.offset = offset
}

func (fq *whereBuilder) Limit(limit int) {
	fq.limit = limit
}

func (fq *whereBuilder) Finalise() (condition string, params []any) {
	where := strings.Join(fq.conditions, " AND ")
	if fq.orderField != "" {
		where += " order by " + fq.orderField + " " + fq.orderType.String()
	}

	if fq.limit > 0 {
		where += " limit ?"
		fq.params = append(fq.params, strconv.Itoa(fq.limit))
	}

	if fq.offset > 0 {
		where += " offset ?"
		fq.params = append(fq.params, fq.offset)
	}

	return where, fq.params
}
