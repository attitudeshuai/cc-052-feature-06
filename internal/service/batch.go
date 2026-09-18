package service

import (
	"cc-052/internal/model"
	"cc-052/internal/repository"
	"errors"
	"fmt"
	"time"
)

var (
	ErrBatchNotFound = errors.New("batch not found")
	ErrBatchLocked   = errors.New("batch is locked")
	ErrInvalidInput  = errors.New("invalid input")
)

// ConfirmRequiredError 已发码批次修改采收日期时必须显式确认，携带受影响的溯源码数量。
type ConfirmRequiredError struct {
	AffectedCodeCount int
}

func (e *ConfirmRequiredError) Error() string {
	return fmt.Sprintf("批次已发出 %d 个溯源码，修改采收日期将影响这些码的溯源页，请带 confirm=true 确认", e.AffectedCodeCount)
}

type BatchService struct {
	repo        *repository.BatchRepo
	plotRepo    *repository.PlotRepo
	farmRepo    *repository.FarmRepo
	harvestRepo *repository.HarvestRecordRepo
	codeRepo    *repository.TraceCodeRepo
}

func NewBatchService(repo *repository.BatchRepo, plotRepo *repository.PlotRepo, farmRepo *repository.FarmRepo, harvestRepo *repository.HarvestRecordRepo, codeRepo *repository.TraceCodeRepo) *BatchService {
	return &BatchService{repo: repo, plotRepo: plotRepo, farmRepo: farmRepo, harvestRepo: harvestRepo, codeRepo: codeRepo}
}

func (s *BatchService) Create(req *model.CreateBatchRequest) (*model.CropBatch, error) {
	sowingDate, err := time.Parse("2006-01-02", req.SowingDate)
	if err != nil {
		return nil, err
	}
	var harvestDate *time.Time
	if req.HarvestDate != "" {
		t, err := time.Parse("2006-01-02", req.HarvestDate)
		if err != nil {
			return nil, err
		}
		harvestDate = &t
	}
	b := &model.CropBatch{
		PlotID:          req.PlotID,
		CropID:          req.CropID,
		SowingDate:      sowingDate,
		HarvestDate:     harvestDate,
		ExpectedYieldKg: req.ExpectedYieldKg,
		Status:          model.BatchStatusGrowing,
	}
	if err := s.repo.Create(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *BatchService) GetByID(id int64) (*model.CropBatch, error) {
	return s.repo.GetByID(id)
}

// RecordHarvest 补录/更正采收信息：校验时序后更新批次为已采收，并留审计记录。
// 已发码批次修改采收日期必须 confirm=true，否则返回 ConfirmRequiredError。
func (s *BatchService) RecordHarvest(batchID int64, req *model.RecordHarvestRequest) (*model.HarvestRecord, *model.CropBatch, error) {
	batch, err := s.repo.GetByID(batchID)
	if err != nil {
		return nil, nil, ErrBatchNotFound
	}
	if batch.Status == model.BatchStatusLocked {
		return nil, nil, fmt.Errorf("批次已锁定（检测不合格），不能补录采收: %w", ErrBatchLocked)
	}

	harvestDate, err := time.Parse("2006-01-02", req.HarvestDate)
	if err != nil {
		return nil, nil, fmt.Errorf("harvest_date 格式应为 YYYY-MM-DD: %w", ErrInvalidInput)
	}
	if req.ActualYieldKg == nil || *req.ActualYieldKg < 0 {
		return nil, nil, fmt.Errorf("actual_yield_kg 不能为负数: %w", ErrInvalidInput)
	}

	// 时序校验：采收日期不早于播种日期
	if harvestDate.Before(batch.SowingDate) {
		return nil, nil, fmt.Errorf("采收日期 %s 早于播种日期 %s: %w",
			harvestDate.Format("2006-01-02"), batch.SowingDate.Format("2006-01-02"), ErrInvalidInput)
	}

	// 先看地里最后一条农事记录发生在哪天：采收日期早于它则拒掉，
	// 否则已有农事会变成「晚于采收」的非法数据（按日期比较，同一天的农事与采收允许共存）
	lastActivity, err := s.repo.GetLastActivityDate(batchID)
	if err != nil {
		return nil, nil, err
	}
	if lastActivity != nil {
		lastActivityDate := time.Date(lastActivity.Year(), lastActivity.Month(), lastActivity.Day(), 0, 0, 0, 0, time.UTC)
		if harvestDate.Before(lastActivityDate) {
			return nil, nil, fmt.Errorf("采收日期 %s 早于最后一条农事记录日期 %s: %w",
				harvestDate.Format("2006-01-02"), lastActivityDate.Format("2006-01-02"), ErrInvalidInput)
		}
	}

	codes, err := s.codeRepo.ListCodesByBatch(batchID)
	if err != nil {
		return nil, nil, err
	}

	// 已发码批次再改采收日期要格外小心：必须显式确认
	dateChanged := batch.HarvestDate != nil && !batch.HarvestDate.Equal(harvestDate)
	if dateChanged && len(codes) > 0 && !req.Confirm {
		return nil, nil, &ConfirmRequiredError{AffectedCodeCount: len(codes)}
	}

	rec := &model.HarvestRecord{
		BatchID:             batchID,
		HarvestDate:         harvestDate,
		ActualYieldKg:       *req.ActualYieldKg,
		PreviousHarvestDate: batch.HarvestDate,
		AffectedCodeCount:   len(codes),
		AffectedCodes:       model.StringSlice(codes),
	}
	if err := s.harvestRepo.RecordTx(rec); err != nil {
		return nil, nil, err
	}

	updated, err := s.repo.GetByID(batchID)
	if err != nil {
		return nil, nil, err
	}
	return rec, updated, nil
}

// ListHarvestRecords 返回批次的采收补录历史（含每次影响的溯源码）。
func (s *BatchService) ListHarvestRecords(batchID int64) ([]model.HarvestRecord, error) {
	if _, err := s.repo.GetByID(batchID); err != nil {
		return nil, ErrBatchNotFound
	}
	return s.harvestRepo.ListByBatch(batchID)
}

func (s *BatchService) CheckSafetyInterval(batchID int64) (bool, string) {
	batch, err := s.repo.GetByID(batchID)
	if err != nil || batch.HarvestDate == nil {
		return true, ""
	}

	lastPesticideDate, err := s.repo.GetLastPesticideDate(batchID)
	if err != nil || lastPesticideDate == nil {
		return true, ""
	}

	maxInterval, err := s.repo.GetMaxSafeInterval(batchID)
	if err != nil || maxInterval == 0 {
		return true, ""
	}

	daysSincePesticide := int(batch.HarvestDate.Sub(*lastPesticideDate).Hours() / 24)
	if daysSincePesticide < maxInterval {
		return false, "距上次施药不足安全间隔期"
	}
	return true, ""
}
