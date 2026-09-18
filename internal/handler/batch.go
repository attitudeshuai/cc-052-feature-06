package handler

import (
	"cc-052/internal/model"
	"cc-052/internal/service"
	"cc-052/pkg/response"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type BatchHandler struct {
	svc *service.BatchService
}

func NewBatchHandler(svc *service.BatchService) *BatchHandler {
	return &BatchHandler{svc: svc}
}

func (h *BatchHandler) Create(c *gin.Context) {
	var req model.CreateBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	batch, err := h.svc.Create(&req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, batch)
}

func (h *BatchHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	batch, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, "batch not found")
		return
	}
	response.Success(c, batch)
}

// RecordHarvest 采收信息补录/修改：POST /api/v1/batches/:id/harvest
func (h *BatchHandler) RecordHarvest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req model.RecordHarvestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	batch, change, err := h.svc.RecordHarvest(id, &req)
	if err != nil {
		var invalidDate *service.ErrInvalidHarvestDate
		var confirmRequired *service.ErrHarvestConfirmRequired
		switch {
		case errors.Is(err, service.ErrBatchNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrBatchLocked):
			response.Error(c, http.StatusConflict, "批次已锁定（检测不合格），不可补录采收信息")
		case errors.As(err, &invalidDate):
			response.BadRequest(c, invalidDate.Error())
		case errors.As(err, &confirmRequired):
			// 409 并带回受影响码数量，调用方确认后带 confirm=true 重发
			c.JSON(http.StatusConflict, response.APIResponse{
				Code:    http.StatusConflict,
				Message: confirmRequired.Error(),
				Data:    gin.H{"affected_codes": confirmRequired.AffectedCodes},
			})
		default:
			response.InternalError(c, err.Error())
		}
		return
	}

	resp := gin.H{"batch": batch}
	if change != nil {
		resp["harvest_change"] = change
	}
	response.Success(c, resp)
}

// ListHarvestChanges 采收日期变更历史：GET /api/v1/batches/:id/harvest-changes
func (h *BatchHandler) ListHarvestChanges(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	changes, err := h.svc.ListHarvestChanges(id)
	if err != nil {
		if errors.Is(err, service.ErrBatchNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, changes)
}

// ListHarvestChangeCodes 某次变更影响的码：GET /api/v1/batches/:id/harvest-changes/:changeId/codes
func (h *BatchHandler) ListHarvestChangeCodes(c *gin.Context) {
	batchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid batch id")
		return
	}
	changeID, err := strconv.ParseInt(c.Param("changeId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid change id")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	codes, total, err := h.svc.ListHarvestChangeCodes(batchID, changeID, limit, offset)
	if err != nil {
		if errors.Is(err, service.ErrHarvestChangeNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"codes": codes, "total": total, "limit": limit, "offset": offset})
}
