package project

import (
	"net/http"
	"strconv"

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
	g := r.Group("/projects")
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.GET("/by-group/:group_id", h.GetByGroup)
	g.GET("/:id/context", h.GetContext)
	g.PATCH("/:id", h.Update)
	g.GET("", h.List)

	// Iterations
	g.POST("/:id/iterations", h.CreateIteration)
	g.PATCH("/:id/iterations/:iid", h.UpdateIteration)
	g.GET("/:id/iterations", h.ListIterations)

	// Memory
	g.POST("/:id/memory", h.AddMemory)
	g.GET("/:id/memory", h.QueryMemory)
	g.PATCH("/:id/memory/:mid", h.UpdateMemory)
	g.POST("/:id/memory/sync", h.SyncMemories)

	// Experiences (on agents)
	r.POST("/agents/:id/experiences", h.AddExperience)
	r.GET("/agents/:id/experiences", h.QueryExperiences)
}

func (h *Handler) Create(c *gin.Context) {
	var p common.Project
	if err := c.ShouldBindJSON(&p); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Create(&p); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	p, err := h.svc.Get(id)
	if err != nil {
		common.ErrorResponse(c, http.StatusNotFound, "project not found")
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) GetByGroup(c *gin.Context) {
	gid := c.Param("group_id")
	p, err := h.svc.GetByGroup(gid)
	if err != nil {
		// 404 means no project bound, return empty response
		c.JSON(http.StatusOK, gin.H{"project": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"project": p})
}

func (h *Handler) GetContext(c *gin.Context) {
	id := c.Param("id")
	agentID := c.Query("agent_id")
	ctx, err := h.svc.GetContext(id, agentID)
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, ctx)
}

func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Update(id, body); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) List(c *gin.Context) {
	projects, err := h.svc.List()
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

// --- Iterations ---

func (h *Handler) CreateIteration(c *gin.Context) {
	projectID := c.Param("id")
	var iter common.Iteration
	if err := c.ShouldBindJSON(&iter); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	iter.ProjectID = projectID
	if err := h.svc.CreateIteration(&iter); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, iter)
}

func (h *Handler) UpdateIteration(c *gin.Context) {
	iid := c.Param("iid")
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.UpdateIteration(iid, body); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) ListIterations(c *gin.Context) {
	projectID := c.Param("id")
	iters, err := h.svc.ListIterations(projectID)
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"iterations": iters})
}

// --- Memory ---

func (h *Handler) AddMemory(c *gin.Context) {
	projectID := c.Param("id")
	var m common.ProjectMemory
	if err := c.ShouldBindJSON(&m); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	m.ProjectID = projectID
	if err := h.svc.AddMemory(&m); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *Handler) QueryMemory(c *gin.Context) {
	projectID := c.Param("id")
	category := c.Query("category")
	query := c.Query("query")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	memories, err := h.svc.QueryMemory(projectID, category, query, limit)
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

func (h *Handler) UpdateMemory(c *gin.Context) {
	mid := c.Param("mid")
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.UpdateMemory(mid, body); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) SyncMemories(c *gin.Context) {
	projectID := c.Param("id")
	var body struct {
		AgentID  string                 `json:"agent_id"`
		Memories []common.ProjectMemory `json:"memories"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.SyncMemories(projectID, body.AgentID, body.Memories); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "synced", "count": len(body.Memories)})
}

// --- Experiences ---

func (h *Handler) AddExperience(c *gin.Context) {
	agentID := c.Param("id")
	var exp common.AgentExperience
	if err := c.ShouldBindJSON(&exp); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	exp.AgentID = agentID
	if err := h.svc.AddExperience(&exp); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, exp)
}

func (h *Handler) QueryExperiences(c *gin.Context) {
	agentID := c.Param("id")
	keywords := c.QueryArray("keywords")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	exps, err := h.svc.QueryExperiences(agentID, keywords, limit)
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"experiences": exps})
}
