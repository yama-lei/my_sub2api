// Package handler provides HTTP request handlers for the application.
package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GroupStatusHandler 面向所有登录用户的分组状态页。
type GroupStatusHandler struct {
	groupStatusService *service.GroupStatusService
}

// NewGroupStatusHandler creates a new GroupStatusHandler.
func NewGroupStatusHandler(groupStatusService *service.GroupStatusService) *GroupStatusHandler {
	return &GroupStatusHandler{
		groupStatusService: groupStatusService,
	}
}

// GetGroupStatus 返回当前用户可用分组的 24h 状态报告
// （decode 速度 / TTFT 按 fast、normal 两种模式的 EMA 均值，uptime、缓存率等）。
// GET /api/v1/groups/status
func (h *GroupStatusHandler) GetGroupStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	report, err := h.groupStatusService.GetReport(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, report)
}
