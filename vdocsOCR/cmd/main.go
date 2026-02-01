// Package main provides the entry point for OCR Agent.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/eino/vdocsOCR/agent"
	"github.com/cloudwego/eino/vdocsOCR/config"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("OCR Agent %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// Load configuration
	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.LoadConfig(*configPath)
	} else {
		cfg, err = config.LoadConfigFromEnv()
	}
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create agent factory
	factory, err := agent.NewAgentFactory(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent factory: %v", err)
	}
	defer factory.Close()

	// Create OCR agent
	ocrAgent, err := factory.CreateOCRAgent(ctx)
	if err != nil {
		log.Fatalf("Failed to create OCR agent: %v", err)
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
	})

	// Version endpoint
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version":"%s","build_time":"%s"}`, Version, BuildTime)
	})

	// Capabilities endpoint
	mux.HandleFunc("/api/capabilities", func(w http.ResponseWriter, r *http.Request) {
		capabilities := ocrAgent.GetCapabilities()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"capabilities":%q}`, capabilities)
	})

	// OCR endpoint
	mux.HandleFunc("/api/ocr", handleOCR(ocrAgent, cfg))

	// Invoice recognition endpoint
	mux.HandleFunc("/api/invoice", handleInvoice(ocrAgent, cfg))

	// PDF processing endpoint
	mux.HandleFunc("/api/pdf", handlePDF(ocrAgent, cfg))

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting OCR Agent server on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down...", sig)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

// handleOCR handles OCR requests.
func handleOCR(ocrAgent *agent.OCRAgentImpl, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit body size
		r.Body = http.MaxBytesReader(w, r.Body, cfg.Server.MaxBodySize)

		// Parse multipart form
		if err := r.ParseMultipartForm(cfg.Server.MaxBodySize); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
			return
		}

		// Get uploaded file
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get file: %v", err), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Read file content
		var buf []byte
		buf, err = readAll(file, cfg.Server.MaxBodySize)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusBadRequest)
			return
		}

		// Get options from form
		opts := agent.DefaultProcessOptions()
		if lang := r.FormValue("language"); lang != "" {
			opts.Language = lang
		}
		if format := r.FormValue("format"); format != "" {
			opts.OutputFormat = format
		}

		// Process image
		ctx, cancel := context.WithTimeout(r.Context(), opts.Timeout)
		defer cancel()

		result, err := ocrAgent.ProcessImage(ctx, buf, opts)
		if err != nil {
			http.Error(w, fmt.Sprintf("OCR failed: %v", err), http.StatusInternalServerError)
			return
		}

		// Return result
		output, err := result.ToJSON()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to serialize result: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(output))
	}
}

// handleInvoice handles invoice recognition requests.
func handleInvoice(ocrAgent *agent.OCRAgentImpl, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, cfg.Server.MaxBodySize)

		if err := r.ParseMultipartForm(cfg.Server.MaxBodySize); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get file: %v", err), http.StatusBadRequest)
			return
		}
		defer file.Close()

		buf, err := readAll(file, cfg.Server.MaxBodySize)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusBadRequest)
			return
		}

		opts := agent.DefaultProcessOptions()
		opts.TaskType = agent.TaskTypeInvoiceRecognition
		opts.ExtractItems = true

		ctx, cancel := context.WithTimeout(r.Context(), opts.Timeout)
		defer cancel()

		result, err := ocrAgent.ProcessInvoice(ctx, buf, opts)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invoice recognition failed: %v", err), http.StatusInternalServerError)
			return
		}

		output, err := result.ToJSON()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to serialize result: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(output))
	}
}

// handlePDF handles PDF processing requests.
func handlePDF(ocrAgent *agent.OCRAgentImpl, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, cfg.Server.MaxBodySize)

		if err := r.ParseMultipartForm(cfg.Server.MaxBodySize); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get file: %v", err), http.StatusBadRequest)
			return
		}
		defer file.Close()

		buf, err := readAll(file, cfg.Server.MaxBodySize)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusBadRequest)
			return
		}

		opts := agent.DefaultProcessOptions()
		opts.TaskType = agent.TaskTypePDFOCR
		if pageRange := r.FormValue("page_range"); pageRange != "" {
			opts.PageRange = pageRange
		}

		ctx, cancel := context.WithTimeout(r.Context(), opts.Timeout)
		defer cancel()

		result, err := ocrAgent.ProcessPDF(ctx, buf, opts)
		if err != nil {
			http.Error(w, fmt.Sprintf("PDF processing failed: %v", err), http.StatusInternalServerError)
			return
		}

		output, err := result.ToJSON()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to serialize result: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(output))
	}
}

// readAll reads all data from reader with size limit.
func readAll(r interface{ Read([]byte) (int, error) }, limit int64) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	for {
		if len(buf) >= int(limit) {
			return nil, fmt.Errorf("file too large")
		}
		n, err := r.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if err != nil {
			if err.Error() == "EOF" {
				return buf, nil
			}
			return buf, err
		}
		if len(buf) == cap(buf) {
			buf = append(buf, 0)[:len(buf)]
		}
	}
}
