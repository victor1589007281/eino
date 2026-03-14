package dashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openclaw-collab/go-collab-service/internal/common"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/dashboard")
	g.GET("/overview", h.Overview)
	g.GET("/project/:id", h.ProjectDetail)
	g.GET("/agent/:id", h.AgentDetail)
}

func (h *Handler) Overview(c *gin.Context) {
	overview, err := h.svc.GetOverview()
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, overview)
}

func (h *Handler) ProjectDetail(c *gin.Context) {
	id := c.Param("id")
	detail, err := h.svc.GetProjectDetail(id)
	if err != nil {
		common.ErrorResponse(c, http.StatusNotFound, "project not found")
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *Handler) AgentDetail(c *gin.Context) {
	id := c.Param("id")
	detail, err := h.svc.GetAgentDetail(id)
	if err != nil {
		common.ErrorResponse(c, http.StatusNotFound, "agent not found")
		return
	}
	c.JSON(http.StatusOK, detail)
}
