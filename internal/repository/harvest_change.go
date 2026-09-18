package repository

import (
	"cc-052/internal/model"

	"github.com/jmoiron/sqlx"
)

type HarvestChangeRepo struct {
	db *sqlx.DB
}

func NewHarvestChangeRepo(db *sqlx.DB) *HarvestChangeRepo {
	return &HarvestChangeRepo{db: db}
}

const harvestChangeColumns = `id, batch_id, old_harvest_date, new_harvest_date, affected_code_count, affected_max_seq, reason, changed_by, created_at`

// insertHarvestChange 在既有事务内插入审计行（由 BatchRepo.RecordHarvest 调用，保证与批次更新同生共死）
func insertHarvestChange(tx *sqlx.Tx, c *model.HarvestChange) error {
	query := `INSERT INTO harvest_change (batch_id, old_harvest_date, new_harvest_date, affected_code_count, affected_max_seq, reason, changed_by)
	          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`
	return tx.QueryRow(query, c.BatchID, c.OldHarvestDate, c.NewHarvestDate,
		c.AffectedCodeCount, c.AffectedMaxSeq, c.Reason, c.ChangedBy).
		Scan(&c.ID, &c.CreatedAt)
}

func (r *HarvestChangeRepo) ListByBatch(batchID int64) ([]model.HarvestChange, error) {
	changes := []model.HarvestChange{}
	query := `SELECT ` + harvestChangeColumns + ` FROM harvest_change WHERE batch_id = $1 ORDER BY id DESC`
	if err := r.db.Select(&changes, query, batchID); err != nil {
		return nil, err
	}
	return changes, nil
}

func (r *HarvestChangeRepo) GetByID(id int64) (*model.HarvestChange, error) {
	var c model.HarvestChange
	query := `SELECT ` + harvestChangeColumns + ` FROM harvest_change WHERE id = $1`
	if err := r.db.Get(&c, query, id); err != nil {
		return nil, err
	}
	return &c, nil
}
