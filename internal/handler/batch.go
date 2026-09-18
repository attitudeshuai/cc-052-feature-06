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

// RecordHarvest 补录/更正采收信息，成功后批次状态变为已采收。
func (h *BatchHandler) RecordHarvest(c *gin.Context) {
	batchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid batch id")
		return
	}

	var req model.RecordHarvestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	rec, batch, err := h.svc.RecordHarvest(batchID, &req)
	if err != nil {
		var confirmErr *service.ConfirmRequiredError
		switch {
		case errors.Is(err, service.ErrBatchNotFound):
			response.NotFound(c, "batch not found")
		case errors.Is(err, service.ErrBatchLocked):
			response.Error(c, http.StatusConflict, err.Error())
		case errors.As(err, &confirmErr):
			c.JSON(http.StatusConflict, response.APIResponse{
				Code:    http.StatusConflict,
				Message: confirmErr.Error(),
				Data:    gin.H{"affected_code_count": confirmErr.AffectedCodeCount},
			})
		case errors.Is(err, service.ErrInvalidInput):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.Success(c, gin.H{"batch": batch, "harvest_record": rec})
}

// ListHarvestRecords 查看批次的采收补录历史（含每次影响的溯源码清单）。
func (h *BatchHandler) ListHarvestRecords(c *gin.Context) {
	batchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid batch id")
		return
	}
	records, err := h.svc.ListHarvestRecords(batchID)
	if err != nil {
		if errors.Is(err, service.ErrBatchNotFound) {
			response.NotFound(c, "batch not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, records)
}
