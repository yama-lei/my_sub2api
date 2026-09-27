package admin

import (
	"log"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ModelTraceHandler 提供「模型真假检测」（ModelTrace 指纹归因）的 HTTP 接口。
type ModelTraceHandler struct {
	service *service.ModelTraceService
}

// NewModelTraceHandler 创建 ModelTraceHandler。
func NewModelTraceHandler(service *service.ModelTraceService) *ModelTraceHandler {
	return &ModelTraceHandler{service: service}
}

// Options 返回检测支持的模型与可用账号。
// GET /api/v1/admin/model-trace/options
func (h *ModelTraceHandler) Options(c *gin.Context) {
	options, err := h.service.Options(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, options)
}

// DetectRequest 是检测请求体。
type DetectRequest struct {
	AccountID int64  `json:"account_id" binding:"required"`
	Model     string `json:"model" binding:"required"`
}

// Detect 执行检测并以 SSE 流式推送进度与结果。
// POST /api/v1/admin/model-trace/detect
func (h *ModelTraceHandler) Detect(c *gin.Context) {
	var req DetectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.service.Detect(c, req.AccountID, req.Model); err != nil {
		// Detect 内部已通过 SSE 推送错误；仅基础设施级异常会走到这里。
		log.Printf("model trace detect error: %v", err)
		response.ErrorFrom(c, err)
		return
	}
}
