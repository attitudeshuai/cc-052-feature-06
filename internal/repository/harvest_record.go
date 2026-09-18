package repository

import (
	"cc-052/internal/model"

	"github.com/jmoiron/sqlx"
)

type HarvestRecordRepo struct {
	db *sqlx.DB
}

func NewHarvestRecordRepo(db *sqlx.DB) *HarvestRecordRepo {
	return &HarvestRecordRepo{db: db}
}

// RecordTx 在单个事务内：更新批次的采收日期/实际产量并置为已采收，同时插入一条采收审计记录。
func (r *HarvestRecordRepo) RecordTx(rec *model.HarvestRecord) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE crop_batch SET harvest_date = $1, actual_yield_kg = $2, status = 'harvested' WHERE id = $3`,
		rec.HarvestDate, rec.ActualYieldKg, rec.BatchID,
	); err != nil {
		return err
	}

	if err := tx.QueryRow(
		`INSERT INTO harvest_record (batch_id, harvest_date, actual_yield_kg, previous_harvest_date, affected_code_count, affected_codes)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		rec.BatchID, rec.HarvestDate, rec.ActualYieldKg, rec.PreviousHarvestDate, rec.AffectedCodeCount, rec.AffectedCodes,
	).Scan(&rec.ID, &rec.CreatedAt); err != nil {
		return err
	}

	return tx.Commit()
}

// ListByBatch 返回某批次的全部采收补录记录，最新的在前。
func (r *HarvestRecordRepo) ListByBatch(batchID int64) ([]model.HarvestRecord, error) {
	records := []model.HarvestRecord{}
	query := `SELECT id, batch_id, harvest_date, actual_yield_kg, previous_harvest_date, affected_code_count, affected_codes, created_at
	          FROM harvest_record WHERE batch_id = $1 ORDER BY created_at DESC, id DESC`
	if err := r.db.Select(&records, query, batchID); err != nil {
		return nil, err
	}
	return records, nil
}
