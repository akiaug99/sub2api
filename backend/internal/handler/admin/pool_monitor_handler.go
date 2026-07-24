package admin

import (
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PoolMonitorHandler struct {
	bridge *service.PoolMonitorBridge
	cfg    *config.Config
}

func NewPoolMonitorHandler(bridge *service.PoolMonitorBridge, cfg *config.Config) *PoolMonitorHandler {
	return &PoolMonitorHandler{bridge: bridge, cfg: cfg}
}

func (h *PoolMonitorHandler) Accounts(c *gin.Context) {
	if h.cfg == nil || !h.cfg.PoolMonitor.Enabled {
		c.Status(http.StatusNotFound)
		return
	}
	snapshot, err := h.bridge.Snapshot(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrPoolMonitorDisabled) {
			c.Status(http.StatusNotFound)
			return
		}
		response.InternalError(c, "pool monitor snapshot unavailable")
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

func (h *PoolMonitorHandler) CreateSSOTicket(c *gin.Context) {
	if h.cfg == nil || !h.cfg.PoolMonitor.Enabled {
		c.Status(http.StatusNotFound)
		return
	}
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	role, roleOK := servermiddleware.GetUserRoleFromContext(c)
	if !ok || subject.UserID <= 0 || !roleOK || role != "admin" {
		response.Unauthorized(c, "administrator context required")
		return
	}
	var request struct {
		Target string `json:"target"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "invalid target")
		return
	}
	result, err := h.bridge.CreateSSOTicket(subject.UserID, request.Target)
	if err != nil {
		response.BadRequest(c, "invalid target")
		return
	}
	response.Success(c, result)
}
