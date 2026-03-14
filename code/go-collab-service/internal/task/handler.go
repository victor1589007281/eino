package task

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
	g := r.Group("/tasks")
	g.POST("", h.CreateTask)
	g.PATCH("/:id", h.UpdateTask)
	g.GET("/:id", h.GetTask)
	g.GET("/:id/tree", h.GetTree)
	g.GET("/:id/ancestors", h.GetAncestors)
	g.GET("", h.QueryTasks)
	g.POST("/:id/activities", h.AddActivity)
	g.GET("/:id/activities", h.GetActivities)
	g.POST("/:id/artifacts", h.SaveArtifact)
	g.GET("/:id/artifacts", h.GetArtifacts)
}

func (h *Handler) CreateTask(c *gin.Context) {
	var t common.Task
	if err := c.ShouldBindJSON(&t); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Create(&t); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h *Handler) UpdateTask(c *gin.Context) {
	id := c.Param("id")
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if status, ok := body["status"].(string); ok {
		delete(body, "status")
		if err := h.svc.UpdateStatus(id, status, body); err != nil {
			common.ErrorResponse(c, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		if err := h.svc.Update(id, body); err != nil {
			common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")
	t, err := h.svc.Get(id)
	if err != nil {
		common.ErrorResponse(c, http.StatusNotFound, "task not found")
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *Handler) GetTree(c *gin.Context) {
	id := c.Param("id")
	tasks, err := h.svc.GetTree(id)
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (h *Handler) GetAncestors(c *gin.Context) {
	id := c.Param("id")
	ancestors, err := h.svc.GetAncestors(id)
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ancestors": ancestors})
}

func (h *Handler) QueryTasks(c *gin.Context) {
	params := TaskQueryParams{
		ProjectID:      c.Query("project_id"),
		Assignee:       c.Query("assignee"),
		Status:         c.Query("status"),
		ParentTaskID:   c.Query("parent_task_id"),
		IncludeSubtree: c.Query("include_subtree") == "true",
	}
	tasks, err := h.svc.Query(params)
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (h *Handler) AddActivity(c *gin.Context) {
	taskID := c.Param("id")
	var body struct {
		Type    string `json:"type" binding:"required"`
		ActorID string `json:"actor_id"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.AddActivity(taskID, body.Type, body.ActorID, body.Content); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

func (h *Handler) GetActivities(c *gin.Context) {
	taskID := c.Param("id")
	activities, err := h.svc.GetActivities(taskID)
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"activities": activities})
}

func (h *Handler) SaveArtifact(c *gin.Context) {
	taskID := c.Param("id")
	var a common.Artifact
	if err := c.ShouldBindJSON(&a); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	a.TaskID = taskID
	if err := h.svc.SaveArtifact(&a); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, a)
}

func (h *Handler) GetArtifacts(c *gin.Context) {
	taskID := c.Param("id")
	artifacts, err := h.svc.GetArtifacts(taskID)
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"artifacts": artifacts})
}
