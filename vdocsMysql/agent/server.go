package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/vdocsMysql/config"
)

// RunServerMode runs the agent in server mode (for K8s).
func RunServerMode(ctx context.Context, masterAgent *MasterAgent, cfg *config.Config) error {
	log.Println("Starting MySQL Expert Agent in Server Mode...")

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if masterAgent != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Ready"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Agent not initialized"))
		}
	})

	mux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read body first
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		query := string(body)

		// Set headers for streaming
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			log.Println("Warning: ResponseWriter does not support Flusher")
		} else {
			flusher.Flush()
		}

		if masterAgent == nil {
			http.Error(w, "Agent not initialized", http.StatusServiceUnavailable)
			return
		}

		input := &adk.AgentInput{
			Messages: []*schema.Message{
				schema.UserMessage(query),
			},
			EnableStreaming: cfg.Agent.EnableStreaming,
		}

		iter := masterAgent.Run(ctx, input)

		// Collect output
		for {
			event, ok := iter.Next()
			if !ok {
				break
			}
			if event.Err != nil {
				fmt.Fprintf(w, "Error: %v\n", event.Err)
				if ok {
					flusher.Flush()
				}
				continue
			}
			if event.Output != nil && event.Output.MessageOutput != nil {
				msg, _ := event.Output.MessageOutput.GetMessage()
				if msg != nil {
					fmt.Fprintf(w, "%s", msg.Content)
					if ok {
						flusher.Flush()
					}
				}
			}
		}
	})

	port := "8080"
	log.Printf("Listening on :%s", port)
	return http.ListenAndServe(":"+port, mux)
}
