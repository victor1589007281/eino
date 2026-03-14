package feishu

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
	g := r.Group("/feishu")
	g.POST("/send", h.Send)
	g.POST("/select-bot", h.SelectBot)
	g.GET("/bot-mapping", h.ListBotMappings)
	g.POST("/bot-mapping", h.SetBotMapping)
}

func (h *Handler) Send(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Send(req); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

func (h *Handler) SelectBot(c *gin.Context) {
	var req BotSelectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	botID := h.svc.SelectBot(req)
	c.JSON(http.StatusOK, BotSelectResponse{BotAppID: botID})
}

func (h *Handler) ListBotMappings(c *gin.Context) {
	mappings, err := h.svc.ListBotMappings()
	if err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"mappings": mappings})
}

func (h *Handler) SetBotMapping(c *gin.Context) {
	var m common.BotMapping
	if err := c.ShouldBindJSON(&m); err != nil {
		common.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.SetBotMapping(&m); err != nil {
		common.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "saved"})
}
