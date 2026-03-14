package registry

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
	g := r.Group("/agents")
	g.GET("", h.ListAgents)
	g.GET("/:id", h.GetAgent)
	g.GET("/by-session", h.GetBySession)
	g.POST("/register", h.RegisterAgent)
	g.DELETE("/:id", h.DeleteAgent)
	g.PATCH("/:id/status", h.UpdateStatus)
	g.PATCH("/:id/load", h.UpdateLoad)
}

func (h *Handler) ListAgents(c *gin.Context) {
	projectID := c.Query("project_id")
	status := c.Query("status")
	agents, err := h.svc.List(projectID, status)
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

func (h *Handler) GetAgent(c *gin.Context) {
	id := c.Param("id")
	agent, err := h.svc.Get(id)
	if err != nil {
		common.ErrorResponse(c, http.StatusNotFound, "agent not found")
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (h *Handler) GetBySession(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		common.ErrorResponse(c, http.StatusBadRequest, "key required")
		return
	}
	agent, err := h.svc.GetBySessionKey(key)
	if err != nil {
		common.ErrorResponse(c, http.StatusNotFound, "agent not found for session key")
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (h *Handler) RegisterAgent(c *gin.Context) {
	var agent common.AgentRecord
	if err := c.ShouldBindJSON(&agent); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Register(&agent); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, agent)
}

func (h *Handler) DeleteAgent(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.UpdateStatus(id, body.Status); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) UpdateLoad(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Load int `json:"load"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.UpdateLoad(id, body.Load); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
