package service

import (
	"cc-052/internal/model"
	"cc-052/internal/repository"
	"errors"
	"fmt"
	"time"
)

// 采收补录的可预期错误，handler 据此映射 HTTP 状态码
var (
	ErrBatchNotFound         = errors.New("batch not found")
	ErrBatchLocked           = errors.New("batch is locked")
	ErrHarvestChangeNotFound = errors.New("harvest change not found")
)

// ErrInvalidHarvestDate 采收日期未通过时序校验（早于播种或早于最后一条农事记录）
type ErrInvalidHarvestDate struct{ Reason string }

func (e *ErrInvalidHarvestDate) Error() string { return e.Reason }

// ErrHarvestConfirmRequired 批次已发码且采收日期将发生变化，需调用方确认后重发
type ErrHarvestConfirmRequired struct{ AffectedCodes int }

func (e *ErrHarvestConfirmRequired) Error() string {
	return fmt.Sprintf("批次已发码 %d 个，修改采收日期将影响这些码，请确认影响后带 confirm=true 重试", e.AffectedCodes)
}

type BatchService struct {
	repo              *repository.BatchRepo
	plotRepo          *repository.PlotRepo
	farmRepo          *repository.FarmRepo
	activityRepo      *repository.ActivityRepo
	traceCodeRepo     *repository.TraceCodeRepo
	harvestChangeRepo *repository.HarvestChangeRepo
}

func NewBatchService(
	repo *repository.BatchRepo,
	plotRepo *repository.PlotRepo,
	farmRepo *repository.FarmRepo,
	activityRepo *repository.ActivityRepo,
	traceCodeRepo *repository.TraceCodeRepo,
	harvestChangeRepo *repository.HarvestChangeRepo,
) *BatchService {
	return &BatchService{
		repo:              repo,
		plotRepo:          plotRepo,
		farmRepo:          farmRepo,
		activityRepo:      activityRepo,
		traceCodeRepo:     traceCodeRepo,
		harvestChangeRepo: harvestChangeRepo,
	}
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

// RecordHarvest 采收信息补录/修改：校验日期时序 → 已发码批次改日期需确认 → 事务落库（含审计）。
// 返回更新后的批次与审计记录（未触发审计时为 nil）。
func (s *BatchService) RecordHarvest(batchID int64, req *model.RecordHarvestRequest) (*model.CropBatch, *model.HarvestChange, error) {
	harvestDate, err := time.Parse("2006-01-02", req.HarvestDate)
	if err != nil {
		return nil, nil, &ErrInvalidHarvestDate{Reason: "harvest_date 格式错误，应为 YYYY-MM-DD"}
	}

	batch, err := s.repo.GetByID(batchID)
	if err != nil {
		return nil, nil, ErrBatchNotFound
	}

	if batch.Status == model.BatchStatusLocked {
		return nil, nil, ErrBatchLocked
	}

	// 时序校验一：采收日期不早于播种日期
	if harvestDate.Before(batch.SowingDate) {
		return nil, nil, &ErrInvalidHarvestDate{Reason: fmt.Sprintf(
			"采收日期 %s 早于播种日期 %s",
			harvestDate.Format("2006-01-02"), batch.SowingDate.Format("2006-01-02"))}
	}

	// 时序校验二：采收日期不早于最后一条农事记录（农事不会发生在采收之后，按天比较）
	lastActivity, err := s.activityRepo.GetLastHappenedAt(batchID)
	if err != nil {
		return nil, nil, err
	}
	if lastActivity != nil {
		lastActivityDay := time.Date(lastActivity.Year(), lastActivity.Month(), lastActivity.Day(), 0, 0, 0, 0, time.UTC)
		if harvestDate.Before(lastActivityDay) {
			return nil, nil, &ErrInvalidHarvestDate{Reason: fmt.Sprintf(
				"采收日期 %s 早于最后一条农事记录日期 %s",
				harvestDate.Format("2006-01-02"), lastActivityDay.Format("2006-01-02"))}
		}
	}

	// 已发码批次修改采收日期（含首次补录前已发码的异常情形）：需 confirm，并落审计
	dateChanged := batch.HarvestDate == nil || !batch.HarvestDate.Equal(harvestDate)
	var audit *model.HarvestChange
	if dateChanged {
		codeCount, err := s.traceCodeRepo.CountByBatch(batchID)
		if err != nil {
			return nil, nil, err
		}
		if codeCount > 0 {
			if !req.Confirm {
				return nil, nil, &ErrHarvestConfirmRequired{AffectedCodes: codeCount}
			}
			maxSeq, err := s.traceCodeRepo.GetMaxSeqByBatch(batchID)
			if err != nil {
				return nil, nil, err
			}
			audit = &model.HarvestChange{
				BatchID:           batchID,
				OldHarvestDate:    batch.HarvestDate,
				NewHarvestDate:    harvestDate,
				AffectedCodeCount: codeCount,
				AffectedMaxSeq:    maxSeq,
				Reason:            req.Reason,
				ChangedBy:         req.ChangedBy,
			}
		}
	}

	if err := s.repo.RecordHarvest(batchID, harvestDate, *req.ActualYieldKg, audit); err != nil {
		return nil, nil, err
	}

	updated, err := s.repo.GetByID(batchID)
	if err != nil {
		return nil, nil, err
	}
	return updated, audit, nil
}

// ListHarvestChanges 某批次的采收日期变更历史（新→旧）
func (s *BatchService) ListHarvestChanges(batchID int64) ([]model.HarvestChange, error) {
	if _, err := s.repo.GetByID(batchID); err != nil {
		return nil, ErrBatchNotFound
	}
	return s.harvestChangeRepo.ListByBatch(batchID)
}

// ListHarvestChangeCodes 某次变更影响的码（变更时刻已生成的全部码），分页返回
func (s *BatchService) ListHarvestChangeCodes(batchID, changeID int64, limit, offset int) ([]model.TraceCode, int, error) {
	change, err := s.harvestChangeRepo.GetByID(changeID)
	if err != nil {
		return nil, 0, ErrHarvestChangeNotFound
	}
	if change.BatchID != batchID {
		return nil, 0, ErrHarvestChangeNotFound
	}
	codes, err := s.traceCodeRepo.ListByBatchUpToSeq(batchID, change.AffectedMaxSeq, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return codes, change.AffectedCodeCount, nil
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
