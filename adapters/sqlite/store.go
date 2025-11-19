package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/luno/workflow"
)

const defaultListLimit = 25

type RecordStore struct {
	db *sql.DB
}

func NewRecordStore(db *sql.DB) *RecordStore {
	return &RecordStore{db: db}
}

var _ workflow.RecordStore = (*RecordStore)(nil)

func (s *RecordStore) Store(ctx context.Context, r *workflow.Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	var mustCreate bool
	if r.RunID != "" {
		_, err := recordScan(tx.QueryRowContext(ctx,
			"SELECT workflow_name, foreign_id, run_id, run_state, status, object, created_at, updated_at, meta FROM workflow_records WHERE run_id = ?",
			r.RunID))
		if err != nil {
			if err == workflow.ErrRecordNotFound {
				mustCreate = true
			} else {
				return err
			}
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
		return fmt.Errorf("make outbox event data: %w", err)
	}

	err = s.insertOutboxEvent(ctx, tx, eventData.ID, eventData.WorkflowName, eventData.Data)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *RecordStore) Lookup(ctx context.Context, runID string) (*workflow.Record, error) {
	return s.lookupWhere(ctx, "run_id = ?", runID)
}

func (s *RecordStore) Latest(ctx context.Context, workflowName, foreignID string) (*workflow.Record, error) {
	ls, err := s.listWhere(ctx, "workflow_name = ? AND foreign_id = ? ORDER BY created_at DESC LIMIT 1", workflowName, foreignID)
	if err != nil {
		return nil, err
	}

	if len(ls) < 1 {
		return nil, workflow.ErrRecordNotFound
	}

	return &ls[0], nil
}

func (s *RecordStore) ListOutboxEvents(ctx context.Context, workflowName string, limit int64) ([]workflow.OutboxEvent, error) {
	return s.listOutboxWhere(ctx, "workflow_name = ? LIMIT ?", workflowName, limit)
}

func (s *RecordStore) DeleteOutboxEvent(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM workflow_outbox WHERE id = ?", id)
	return err
}

func (s *RecordStore) List(
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

	if filter.ByCreatedAtAfter().Enabled {
		wb.AddCondition("created_at", ">", filter.ByCreatedAtAfter().Value())
	}

	if filter.ByCreatedAtBefore().Enabled {
		wb.AddCondition("created_at", "<", filter.ByCreatedAtBefore().Value())
	}

	if limit == 0 {
		limit = defaultListLimit
	}

	wb.WhereNotNull("run_id")
	wb.OrderBy("created_at", order)
	wb.Limit(limit)
	wb.Offset(offset)

	where, params := wb.Finalise()
	return s.listWhere(ctx, where, params...)
}

func (s *RecordStore) create(
	ctx context.Context,
	tx *sql.Tx,
	workflowName, foreignID, runID string,
	status int,
	object []byte,
	runState int,
	meta workflow.Meta,
) error {
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO workflow_records
		(workflow_name, foreign_id, run_id, run_state, status, object, meta)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		workflowName, foreignID, runID, runState, status, object, metaBytes,
	)
	if err != nil {
		return fmt.Errorf("insert record: %w", err)
	}

	return nil
}

func (s *RecordStore) update(
	ctx context.Context,
	tx *sql.Tx,
	runID string,
	status int,
	object []byte,
	runState int,
	meta workflow.Meta,
) error {
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE workflow_records
		SET run_state = ?, status = ?, object = ?, updated_at = CURRENT_TIMESTAMP, meta = ?
		WHERE run_id = ?`,
		runState, status, object, metaBytes, runID,
	)
	if err != nil {
		return fmt.Errorf("update record: %w", err)
	}

	return nil
}

func (s *RecordStore) insertOutboxEvent(
	ctx context.Context,
	tx *sql.Tx,
	id string,
	workflowName string,
	data []byte,
) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_outbox (id, workflow_name, data)
		VALUES (?, ?, ?)`,
		id, workflowName, data,
	)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	return nil
}

func (s *RecordStore) lookupWhere(ctx context.Context, where string, args ...any) (*workflow.Record, error) {
	query := "SELECT workflow_name, foreign_id, run_id, run_state, status, object, created_at, updated_at, meta FROM workflow_records WHERE " + where
	return recordScan(s.db.QueryRowContext(ctx, query, args...))
}

func (s *RecordStore) listWhere(ctx context.Context, where string, args ...any) ([]workflow.Record, error) {
	query := "SELECT workflow_name, foreign_id, run_id, run_state, status, object, created_at, updated_at, meta FROM workflow_records WHERE " + where
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query records: %w", err)
	}
	defer rows.Close()

	var res []workflow.Record
	for rows.Next() {
		r, err := recordScan(rows)
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

func (s *RecordStore) listOutboxWhere(ctx context.Context, where string, args ...any) ([]workflow.OutboxEvent, error) {
	query := "SELECT id, workflow_name, data, created_at FROM workflow_outbox WHERE " + where
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query outbox: %w", err)
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
		return nil, fmt.Errorf("rows error: %w", rows.Err())
	}

	return res, nil
}

func recordScan(row scannable) (*workflow.Record, error) {
	var r workflow.Record
	var meta []byte
	err := row.Scan(
		&r.WorkflowName,
		&r.ForeignID,
		&r.RunID,
		&r.RunState,
		&r.Status,
		&r.Object,
		&r.CreatedAt,
		&r.UpdatedAt,
		&meta,
	)
	if err == sql.ErrNoRows {
		return nil, workflow.ErrRecordNotFound
	} else if err != nil {
		return nil, fmt.Errorf("scan record: %w", err)
	}

	if len(meta) > 0 {
		err = json.Unmarshal(meta, &r.Meta)
		if err != nil {
			return nil, fmt.Errorf("unmarshal meta: %w", err)
		}
	}

	return &r, nil
}

func outboxScan(row scannable) (*workflow.OutboxEvent, error) {
	var e workflow.OutboxEvent
	err := row.Scan(
		&e.ID,
		&e.WorkflowName,
		&e.Data,
		&e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, workflow.ErrOutboxRecordNotFound
	} else if err != nil {
		return nil, fmt.Errorf("scan outbox: %w", err)
	}

	return &e, nil
}

type scannable interface {
	Scan(dest ...any) error
}

type whereBuilder struct {
	conditions []string
	params     []any
	orderField string
	orderType  workflow.OrderType
	offset     int64
	limit      int
}

func (wb *whereBuilder) WhereNotNull(field string) {
	wb.conditions = append(wb.conditions, field+" IS NOT NULL")
}

func (wb *whereBuilder) AddCondition(field string, comparison string, value any) {
	condition := fmt.Sprintf("(%s %s ?)", field, comparison)
	wb.conditions = append(wb.conditions, condition)
	wb.params = append(wb.params, value)
}

func (wb *whereBuilder) Where(field string, values ...string) {
	condition := " ( "
	for i, value := range values {
		condition += field + " = ?"
		if i < len(values)-1 {
			condition += " OR "
		}
		wb.params = append(wb.params, value)
	}
	condition += " ) "
	wb.conditions = append(wb.conditions, condition)
}

func (wb *whereBuilder) OrderBy(field string, orderType workflow.OrderType) {
	wb.orderField = field
	wb.orderType = orderType
}

func (wb *whereBuilder) Offset(offset int64) {
	wb.offset = offset
}

func (wb *whereBuilder) Limit(limit int) {
	wb.limit = limit
}

func (wb *whereBuilder) Finalise() (condition string, params []any) {
	where := strings.Join(wb.conditions, " AND ")
	if wb.orderField != "" {
		where += " ORDER BY " + wb.orderField + " " + wb.orderType.String()
	}

	if wb.limit > 0 {
		where += " LIMIT ?"
		wb.params = append(wb.params, strconv.Itoa(wb.limit))
	}

	if wb.offset > 0 {
		where += " OFFSET ?"
		wb.params = append(wb.params, wb.offset)
	}

	return where, wb.params
}
