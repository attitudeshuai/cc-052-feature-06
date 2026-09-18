package model

import "time"

type BatchStatus string

const (
	BatchStatusGrowing   BatchStatus = "growing"
	BatchStatusHarvested BatchStatus = "harvested"
	BatchStatusLocked    BatchStatus = "locked"
)

type CropBatch struct {
	ID              int64       `db:"id" json:"id"`
	PlotID          int64       `db:"plot_id" json:"plot_id"`
	CropID          string      `db:"crop_id" json:"crop_id"`
	SowingDate      time.Time   `db:"sowing_date" json:"sowing_date"`
	HarvestDate     *time.Time  `db:"harvest_date" json:"harvest_date,omitempty"`
	ExpectedYieldKg float64     `db:"expected_yield_kg" json:"expected_yield_kg"`
	ActualYieldKg   *float64    `db:"actual_yield_kg" json:"actual_yield_kg,omitempty"`
	Status          BatchStatus `db:"status" json:"status"`
	CreatedAt       time.Time   `db:"created_at" json:"created_at"`
}

type CreateBatchRequest struct {
	PlotID          int64   `json:"plot_id" binding:"required"`
	CropID          string  `json:"crop_id" binding:"required"`
	SowingDate      string  `json:"sowing_date" binding:"required"`
	HarvestDate     string  `json:"harvest_date"`
	ExpectedYieldKg float64 `json:"expected_yield_kg" binding:"required"`
}

// RecordHarvestRequest 采收信息补录/修改。
// 已发码批次修改采收日期时必须带 confirm=true，并建议填 reason / changed_by 留痕。
type RecordHarvestRequest struct {
	HarvestDate   string   `json:"harvest_date" binding:"required"`
	ActualYieldKg *float64 `json:"actual_yield_kg" binding:"required,gte=0"`
	Reason        string   `json:"reason"`
	ChangedBy     string   `json:"changed_by"`
	Confirm       bool     `json:"confirm"`
}

// HarvestChange 采收日期变更审计。
// 仅在批次已发码且采收日期发生变化时写入；
// 受影响码 = 变更时刻该批次 seq <= AffectedMaxSeq 的全部 trace_code。
type HarvestChange struct {
	ID                int64      `db:"id" json:"id"`
	BatchID           int64      `db:"batch_id" json:"batch_id"`
	OldHarvestDate    *time.Time `db:"old_harvest_date" json:"old_harvest_date,omitempty"`
	NewHarvestDate    time.Time  `db:"new_harvest_date" json:"new_harvest_date"`
	AffectedCodeCount int        `db:"affected_code_count" json:"affected_code_count"`
	AffectedMaxSeq    int        `db:"affected_max_seq" json:"affected_max_seq"`
	Reason            string     `db:"reason" json:"reason,omitempty"`
	ChangedBy         string     `db:"changed_by" json:"changed_by,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
}