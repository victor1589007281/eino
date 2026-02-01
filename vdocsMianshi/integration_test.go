package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/eino/vdocsMianshi/config"
	"github.com/cloudwego/eino/vdocsMianshi/interaction"
)

func TestIntegration_HealthEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := interaction.NewHandler(cfg, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, "/api/v1")

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.NotEmpty(t, response["timestamp"])
}

func TestIntegration_AnalyzeJDEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := interaction.NewHandler(cfg, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, "/api/v1")

	// Test with valid JD content
	reqBody := `{"jd_content": "高级Go开发工程师\n要求：3年以上Go开发经验"}`
	req := httptest.NewRequest("POST", "/api/v1/interview/analyze-jd", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response interaction.AnalyzeJDResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response.SessionID)
	assert.NotNil(t, response.JobDescription)
}

func TestIntegration_AnalyzeJD_MissingContent(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := interaction.NewHandler(cfg, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, "/api/v1")

	// Test with missing JD content
	reqBody := `{"jd_content": ""}`
	req := httptest.NewRequest("POST", "/api/v1/interview/analyze-jd", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestIntegration_AnalyzeJD_InvalidMethod(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := interaction.NewHandler(cfg, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, "/api/v1")

	req := httptest.NewRequest("GET", "/api/v1/interview/analyze-jd", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestIntegration_CORS(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := interaction.NewHandler(cfg, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, "/api/v1")

	// Test OPTIONS request
	req := httptest.NewRequest("OPTIONS", "/api/v1/interview/analyze-jd", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestIntegration_ListSessions(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := interaction.NewHandler(cfg, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, "/api/v1")

	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "sessions")
	assert.Contains(t, response, "count")
}

func TestIntegration_FullWorkflow(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := interaction.NewHandler(cfg, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, "/api/v1")

	// Step 1: Analyze JD
	analyzeBody := `{"jd_content": "高级Go开发工程师\n要求：3年以上Go开发经验，熟悉MySQL、Redis"}`
	analyzeReq := httptest.NewRequest("POST", "/api/v1/interview/analyze-jd", strings.NewReader(analyzeBody))
	analyzeReq.Header.Set("Content-Type", "application/json")
	analyzeW := httptest.NewRecorder()

	mux.ServeHTTP(analyzeW, analyzeReq)
	assert.Equal(t, http.StatusOK, analyzeW.Code)

	var analyzeResp interaction.AnalyzeJDResponse
	json.Unmarshal(analyzeW.Body.Bytes(), &analyzeResp)
	sessionID := analyzeResp.SessionID

	// Step 2: Design Plan
	designBody := `{"session_id": "` + sessionID + `"}`
	designReq := httptest.NewRequest("POST", "/api/v1/interview/design-plan", strings.NewReader(designBody))
	designReq.Header.Set("Content-Type", "application/json")
	designW := httptest.NewRecorder()

	mux.ServeHTTP(designW, designReq)
	assert.Equal(t, http.StatusOK, designW.Code)

	// Step 3: Start Interview
	startBody := `{"session_id": "` + sessionID + `", "candidate_name": "张三"}`
	startReq := httptest.NewRequest("POST", "/api/v1/interview/start", strings.NewReader(startBody))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()

	mux.ServeHTTP(startW, startReq)
	assert.Equal(t, http.StatusOK, startW.Code)

	// Step 4: Evaluate
	evalBody := `{"session_id": "` + sessionID + `"}`
	evalReq := httptest.NewRequest("POST", "/api/v1/interview/evaluate", strings.NewReader(evalBody))
	evalReq.Header.Set("Content-Type", "application/json")
	evalW := httptest.NewRecorder()

	mux.ServeHTTP(evalW, evalReq)
	assert.Equal(t, http.StatusOK, evalW.Code)

	// Step 5: Generate Report
	reportBody := `{"session_id": "` + sessionID + `", "format": "markdown"}`
	reportReq := httptest.NewRequest("POST", "/api/v1/interview/report", strings.NewReader(reportBody))
	reportReq.Header.Set("Content-Type", "application/json")
	reportW := httptest.NewRecorder()

	mux.ServeHTTP(reportW, reportReq)
	assert.Equal(t, http.StatusOK, reportW.Code)
}

func TestIntegration_SessionNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := interaction.NewHandler(cfg, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, "/api/v1")

	// Try to design plan for non-existent session
	body := `{"session_id": "non-existent-session"}`
	req := httptest.NewRequest("POST", "/api/v1/interview/design-plan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := interaction.NewHandler(cfg, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, "/api/v1")

	// Send multiple concurrent requests
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			reqBody := `{"jd_content": "测试JD内容"}`
			req := httptest.NewRequest("POST", "/api/v1/interview/analyze-jd", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}
}

func TestIntegration_ServerGracefulShutdown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Interaction.RESTServer.Address = ":0" // Random port

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in goroutine
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- interaction.StartServer(ctx, cfg, nil)
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for server to stop
	select {
	case err := <-serverDone:
		// Server should stop with http.ErrServerClosed or nil
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not shut down in time")
	}
}
