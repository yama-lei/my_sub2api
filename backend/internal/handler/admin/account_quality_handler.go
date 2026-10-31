package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/modeltrace"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// AccountQualityHandler serves per-account quality metrics (adulteration rate,
// TTFT, TPS) and ModelTrace probe results.
type AccountQualityHandler struct {
	qualitySvc *service.AccountQualityService
}

func NewAccountQualityHandler(qualitySvc *service.AccountQualityService) *AccountQualityHandler {
	return &AccountQualityHandler{qualitySvc: qualitySvc}
}

const maxAccountQualityBatch = 500

// BatchMetrics GET /admin/accounts/quality?ids=1,2,3&hours=24
func (h *AccountQualityHandler) BatchMetrics(c *gin.Context) {
	ids, err := parseAccountQualityIDs(c.Query("ids"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	metrics, err := h.qualitySvc.GetMetrics(c.Request.Context(), ids, hours)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, metrics)
}

// GetMetrics GET /admin/accounts/:id/quality?hours=24
func (h *AccountQualityHandler) GetMetrics(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "invalid account id")
		return
	}
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	metrics, err := h.qualitySvc.GetMetrics(c.Request.Context(), []int64{accountID}, hours)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, metrics[accountID])
}

// ListEvents GET /admin/accounts/:id/quality/events?kind=probe&limit=20
func (h *AccountQualityHandler) ListEvents(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "invalid account id")
		return
	}
	kind := strings.TrimSpace(c.Query("kind"))
	if kind != "" && kind != service.AccountQualityEventDowngrade && kind != service.AccountQualityEventProbe {
		response.BadRequest(c, "invalid kind")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	events, err := h.qualitySvc.ListEvents(c.Request.Context(), accountID, kind, limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, events)
}

type runModelProbeRequest struct {
	Model         string `json:"model"`
	ExpectedModel string `json:"expected_model"`
}

// RunProbe POST /admin/accounts/:id/quality/probe
// Runs a ModelTrace probe synchronously (several upstream generations; may take minutes).
func (h *AccountQualityHandler) RunProbe(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "invalid account id")
		return
	}
	var req runModelProbeRequest
	_ = c.ShouldBindJSON(&req)
	// Detach from the HTTP request so a closed admin tab does not discard a
	// half-finished probe; the result is still stored.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 15*time.Minute)
	defer cancel()
	event, err := h.qualitySvc.RunProbeNow(ctx, accountID, req.Model, req.ExpectedModel)
	if err != nil {
		if errors.Is(err, service.ErrModelProbeBusy) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		if event == nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, event)
}

// KnownModels GET /admin/accounts/quality/probe-models
// Models the ModelTrace fingerprint bank can identify.
func (h *AccountQualityHandler) KnownModels(c *gin.Context) {
	response.Success(c, modeltrace.KnownModels())
}

func parseAccountQualityIDs(raw string) ([]int64, error) {
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("invalid account id: " + part)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) > maxAccountQualityBatch {
		return nil, errors.New("too many account ids")
	}
	return ids, nil
}
