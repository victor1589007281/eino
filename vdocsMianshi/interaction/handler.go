// Package interaction provides HTTP handlers and API endpoints.
package interaction

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/cloudwego/eino/vdocsMianshi/agent"
	"github.com/cloudwego/eino/vdocsMianshi/config"
)

// Handler handles HTTP requests for the interview system.
type Handler struct {
	config       *config.Config
	agentFactory *agent.AgentFactory
	sessions     map[string]*SessionState
	sessionMu    sync.RWMutex
}

// SessionState represents the state of an interview session.
type SessionState struct {
	ID             string                  `json:"id"`
	JD             *agent.JobDescription   `json:"job_description,omitempty"`
	Plan           *agent.InterviewPlan    `json:"interview_plan,omitempty"`
	Session        *agent.InterviewSession `json:"interview_session,omitempty"`
	Status         string                  `json:"status"` // created, jd_analyzed, plan_designed, interviewing, completed
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

// NewHandler creates a new HTTP handler.
func NewHandler(cfg *config.Config, factory *agent.AgentFactory) *Handler {
	return &Handler{
		config:       cfg,
		agentFactory: factory,
		sessions:     make(map[string]*SessionState),
	}
}

// RegisterRoutes registers all HTTP routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, basePath string) {
	// Interview workflow endpoints
	mux.HandleFunc(basePath+"/interview/analyze-jd", h.handleCORS(h.handleAnalyzeJD))
	mux.HandleFunc(basePath+"/interview/design-plan", h.handleCORS(h.handleDesignPlan))
	mux.HandleFunc(basePath+"/interview/start", h.handleCORS(h.handleStartInterview))
	mux.HandleFunc(basePath+"/interview/answer", h.handleCORS(h.handleSubmitAnswer))
	mux.HandleFunc(basePath+"/interview/evaluate", h.handleCORS(h.handleEvaluate))
	mux.HandleFunc(basePath+"/interview/report", h.handleCORS(h.handleGenerateReport))

	// Session management
	mux.HandleFunc(basePath+"/sessions", h.handleCORS(h.handleListSessions))
	mux.HandleFunc(basePath+"/sessions/", h.handleCORS(h.handleGetSession))

	// Health check
	mux.HandleFunc(basePath+"/health", h.handleHealth)
}

// handleCORS adds CORS headers to responses.
func (h *Handler) handleCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// handleHealth handles health check requests.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
	}
	writeJSON(w, http.StatusOK, response)
}

// AnalyzeJDRequest represents the request for JD analysis.
type AnalyzeJDRequest struct {
	JDContent string `json:"jd_content"`
	SessionID string `json:"session_id,omitempty"`
}

// AnalyzeJDResponse represents the response for JD analysis.
type AnalyzeJDResponse struct {
	SessionID      string                `json:"session_id"`
	JobDescription *agent.JobDescription `json:"job_description"`
	Message        string                `json:"message"`
}

// handleAnalyzeJD handles JD analysis requests.
func (h *Handler) handleAnalyzeJD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req AnalyzeJDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.JDContent == "" {
		writeError(w, http.StatusBadRequest, "JD content is required")
		return
	}

	// Create or get session
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Create session state
	state := &SessionState{
		ID:        sessionID,
		Status:    "created",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Analyze JD (simplified - in production, this would call the agent)
	jd := &agent.JobDescription{
		ID:         uuid.New().String(),
		RawContent: req.JDContent,
	}

	// TODO: Call JD analyzer agent
	// For now, return a basic response
	state.JD = jd
	state.Status = "jd_analyzed"
	state.UpdatedAt = time.Now()

	h.sessionMu.Lock()
	h.sessions[sessionID] = state
	h.sessionMu.Unlock()

	response := AnalyzeJDResponse{
		SessionID:      sessionID,
		JobDescription: jd,
		Message:        "JD分析完成",
	}

	writeJSON(w, http.StatusOK, response)
}

// DesignPlanRequest represents the request for plan design.
type DesignPlanRequest struct {
	SessionID string `json:"session_id"`
}

// DesignPlanResponse represents the response for plan design.
type DesignPlanResponse struct {
	SessionID     string                `json:"session_id"`
	InterviewPlan *agent.InterviewPlan  `json:"interview_plan"`
	Message       string                `json:"message"`
}

// handleDesignPlan handles interview plan design requests.
func (h *Handler) handleDesignPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req DesignPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.sessionMu.RLock()
	state, ok := h.sessions[req.SessionID]
	h.sessionMu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	if state.JD == nil {
		writeError(w, http.StatusBadRequest, "JD not analyzed yet")
		return
	}

	// TODO: Call question designer agent
	// For now, return a basic plan
	plan := &agent.InterviewPlan{
		ID:               uuid.New().String(),
		JobDescriptionID: state.JD.ID,
		TotalDuration:    60,
		Rounds: []agent.InterviewRound{
			{
				ID:       uuid.New().String(),
				Name:     "技术面试",
				Type:     "technical",
				Duration: 45,
			},
		},
	}

	state.Plan = plan
	state.Status = "plan_designed"
	state.UpdatedAt = time.Now()

	h.sessionMu.Lock()
	h.sessions[req.SessionID] = state
	h.sessionMu.Unlock()

	response := DesignPlanResponse{
		SessionID:     req.SessionID,
		InterviewPlan: plan,
		Message:       "面试方案设计完成",
	}

	writeJSON(w, http.StatusOK, response)
}

// StartInterviewRequest represents the request to start interview.
type StartInterviewRequest struct {
	SessionID     string           `json:"session_id"`
	CandidateName string           `json:"candidate_name"`
	CandidateEmail string          `json:"candidate_email,omitempty"`
}

// StartInterviewResponse represents the response for starting interview.
type StartInterviewResponse struct {
	SessionID   string            `json:"session_id"`
	Question    *agent.Question   `json:"question"`
	RoundInfo   string            `json:"round_info"`
	Message     string            `json:"message"`
}

// handleStartInterview handles interview start requests.
func (h *Handler) handleStartInterview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req StartInterviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.sessionMu.RLock()
	state, ok := h.sessions[req.SessionID]
	h.sessionMu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	if state.Plan == nil {
		writeError(w, http.StatusBadRequest, "Interview plan not designed yet")
		return
	}

	// Create interview session
	candidate := &agent.Candidate{
		ID:    uuid.New().String(),
		Name:  req.CandidateName,
		Email: req.CandidateEmail,
	}

	session := &agent.InterviewSession{
		ID:              uuid.New().String(),
		Candidate:       candidate,
		Plan:            state.Plan,
		CurrentRound:    0,
		CurrentQuestion: 0,
		Answers:         make([]agent.Answer, 0),
		StartTime:       time.Now().Format(time.RFC3339),
		Status:          "in_progress",
	}

	state.Session = session
	state.Status = "interviewing"
	state.UpdatedAt = time.Now()

	h.sessionMu.Lock()
	h.sessions[req.SessionID] = state
	h.sessionMu.Unlock()

	// Get first question
	var firstQuestion *agent.Question
	if len(state.Plan.Rounds) > 0 && len(state.Plan.Rounds[0].Questions) > 0 {
		firstQuestion = &state.Plan.Rounds[0].Questions[0]
	}

	response := StartInterviewResponse{
		SessionID:   req.SessionID,
		Question:    firstQuestion,
		RoundInfo:   fmt.Sprintf("第1轮: %s", state.Plan.Rounds[0].Name),
		Message:     "面试开始",
	}

	writeJSON(w, http.StatusOK, response)
}

// SubmitAnswerRequest represents the request to submit an answer.
type SubmitAnswerRequest struct {
	SessionID  string `json:"session_id"`
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

// SubmitAnswerResponse represents the response for submitting answer.
type SubmitAnswerResponse struct {
	SessionID     string          `json:"session_id"`
	NextQuestion  *agent.Question `json:"next_question,omitempty"`
	RoundComplete bool            `json:"round_complete"`
	Message       string          `json:"message"`
}

// handleSubmitAnswer handles answer submission requests.
func (h *Handler) handleSubmitAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req SubmitAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.sessionMu.RLock()
	state, ok := h.sessions[req.SessionID]
	h.sessionMu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	if state.Session == nil {
		writeError(w, http.StatusBadRequest, "Interview not started")
		return
	}

	// Record answer
	answer := agent.Answer{
		QuestionID: req.QuestionID,
		Content:    req.Answer,
		Timestamp:  time.Now().Format(time.RFC3339),
	}
	state.Session.Answers = append(state.Session.Answers, answer)
	state.Session.CurrentQuestion++
	state.UpdatedAt = time.Now()

	// Check if round is complete
	currentRound := state.Session.Plan.Rounds[state.Session.CurrentRound]
	roundComplete := state.Session.CurrentQuestion >= len(currentRound.Questions)

	var nextQuestion *agent.Question
	if !roundComplete {
		nextQuestion = &currentRound.Questions[state.Session.CurrentQuestion]
	} else {
		// Move to next round
		state.Session.CurrentRound++
		state.Session.CurrentQuestion = 0
		if state.Session.CurrentRound < len(state.Session.Plan.Rounds) {
			nextRound := state.Session.Plan.Rounds[state.Session.CurrentRound]
			if len(nextRound.Questions) > 0 {
				nextQuestion = &nextRound.Questions[0]
			}
		} else {
			state.Session.Status = "completed"
			state.Status = "completed"
		}
	}

	h.sessionMu.Lock()
	h.sessions[req.SessionID] = state
	h.sessionMu.Unlock()

	response := SubmitAnswerResponse{
		SessionID:     req.SessionID,
		NextQuestion:  nextQuestion,
		RoundComplete: roundComplete,
		Message:       "答案已记录",
	}

	writeJSON(w, http.StatusOK, response)
}

// handleEvaluate handles skill evaluation requests.
func (h *Handler) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.sessionMu.RLock()
	state, ok := h.sessions[req.SessionID]
	h.sessionMu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	// TODO: Call skill evaluator agent
	// For now, return a placeholder response
	response := map[string]any{
		"session_id": req.SessionID,
		"skill_profile": map[string]any{
			"overall_level": "intermediate",
			"strengths":     []string{"技术基础扎实", "沟通能力良好"},
			"weaknesses":    []string{"系统设计经验不足"},
		},
		"match_result": map[string]any{
			"overall_match":  75.5,
			"recommendation": "hire",
		},
		"message": "技能评估完成",
	}

	writeJSON(w, http.StatusOK, response)
}

// handleGenerateReport handles report generation requests.
func (h *Handler) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Format    string `json:"format"` // markdown, json, html
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.sessionMu.RLock()
	state, ok := h.sessions[req.SessionID]
	h.sessionMu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	// TODO: Call report generator agent
	// For now, return a placeholder report
	report := map[string]any{
		"session_id":        req.SessionID,
		"executive_summary": "该候选人展现了扎实的技术基础和良好的学习能力，建议进入下一轮面试。",
		"overall_score":     75.5,
		"recommendation":    "hire",
		"generated_at":      time.Now().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, report)
}

// handleListSessions handles session list requests.
func (h *Handler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	h.sessionMu.RLock()
	sessions := make([]*SessionState, 0, len(h.sessions))
	for _, state := range h.sessions {
		sessions = append(sessions, state)
	}
	h.sessionMu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// handleGetSession handles single session retrieval.
func (h *Handler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract session ID from URL path
	// Expected: /api/v1/sessions/{session_id}
	sessionID := r.URL.Path[len("/api/v1/sessions/"):]

	h.sessionMu.RLock()
	state, ok := h.sessions[sessionID]
	h.sessionMu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	writeJSON(w, http.StatusOK, state)
}

// Utility functions

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error":   message,
		"status":  http.StatusText(status),
	})
}

// StartServer starts the HTTP server.
func StartServer(ctx context.Context, cfg *config.Config, factory *agent.AgentFactory) error {
	handler := NewHandler(cfg, factory)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, cfg.Interaction.RESTServer.BasePath)

	server := &http.Server{
		Addr:    cfg.Interaction.RESTServer.Address,
		Handler: mux,
	}

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	fmt.Printf("Starting server on %s\n", cfg.Interaction.RESTServer.Address)
	return server.ListenAndServe()
}

// StreamResponse writes a streaming response.
func StreamResponse(w http.ResponseWriter, r *http.Request, eventChan <-chan string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// ReadRequestBody reads and returns the request body.
func ReadRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	return body, nil
}
