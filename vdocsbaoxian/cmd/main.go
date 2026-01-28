// Package main provides the entry point for Insurance Expert Agent.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwego/eino/vdocsbaoxian/config"
)

var (
	configPath  = flag.String("config", "config.yaml", "Path to configuration file")
	logLevel    = flag.String("log-level", "", "Log level override (debug, info, warn, error)")
	showVersion = flag.Bool("version", false, "Show version information")
)

// Version information (set by ldflags)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

func main() {
	flag.Parse()

	if *showVersion {
		printVersion()
		return
	}

	// Get command
	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override log level if specified
	if *logLevel != "" {
		cfg.Log.Level = *logLevel
	}

	// Execute command
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	switch command {
	case "serve":
		if err := runServer(ctx, cfg); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	case "index":
		if len(args) < 2 {
			fmt.Println("Usage: insurance-expert index [build|rebuild|status]")
			os.Exit(1)
		}
		if err := runIndex(ctx, cfg, args[1]); err != nil {
			log.Fatalf("Index error: %v", err)
		}
	case "query":
		if len(args) < 2 {
			fmt.Println("Usage: insurance-expert query <question>")
			os.Exit(1)
		}
		if err := runQuery(ctx, cfg, args[1:]); err != nil {
			log.Fatalf("Query error: %v", err)
		}
	case "chat":
		if err := runChat(ctx, cfg); err != nil {
			log.Fatalf("Chat error: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("Insurance Expert Agent\n")
	fmt.Printf("  Version:    %s\n", Version)
	fmt.Printf("  Git Commit: %s\n", GitCommit)
	fmt.Printf("  Build Time: %s\n", BuildTime)
	fmt.Printf("  Go Version: %s\n", GoVersion)
}

func printUsage() {
	fmt.Println("Insurance Expert Agent - 保险专家智能助手")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  insurance-expert [flags] <command> [arguments]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  serve       Start the API server")
	fmt.Println("  index       Manage the search index")
	fmt.Println("  query       Execute a single query")
	fmt.Println("  chat        Start interactive chat mode")
	fmt.Println("")
	fmt.Println("Flags:")
	flag.PrintDefaults()
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  insurance-expert serve --config=config.yaml")
	fmt.Println("  insurance-expert index build --config=config.yaml")
	fmt.Println("  insurance-expert query \"什么是保险等待期\"")
	fmt.Println("  insurance-expert chat")
}

func runServer(ctx context.Context, cfg *config.Config) error {
	fmt.Println("Starting Insurance Expert Server...")
	fmt.Printf("  REST API: http://localhost%s%s\n", cfg.Interaction.RESTServer.Address, cfg.Interaction.RESTServer.BasePath)
	
	if cfg.Stats.Prometheus.Enabled {
		fmt.Printf("  Metrics:  http://localhost:%d%s\n", cfg.Stats.Prometheus.Port, cfg.Stats.Prometheus.Path)
	}

	// TODO: Initialize and start server
	// 1. Initialize agent factory
	// 2. Initialize index system
	// 3. Initialize cache
	// 4. Start REST server
	// 5. Start MCP server if enabled
	// 6. Start metrics server if enabled

	// Wait for context cancellation
	<-ctx.Done()
	
	fmt.Println("Server stopped")
	return nil
}

func runIndex(ctx context.Context, cfg *config.Config, action string) error {
	switch action {
	case "build":
		fmt.Println("Building index...")
		// TODO: Implement index building
		fmt.Println("Index build completed")
		return nil
	case "rebuild":
		fmt.Println("Rebuilding index...")
		// TODO: Implement index rebuilding
		fmt.Println("Index rebuild completed")
		return nil
	case "status":
		fmt.Println("Index status:")
		// TODO: Implement index status
		return nil
	default:
		return fmt.Errorf("unknown index action: %s", action)
	}
}

func runQuery(ctx context.Context, cfg *config.Config, args []string) error {
	question := ""
	for i, arg := range args {
		if i > 0 {
			question += " "
		}
		question += arg
	}

	fmt.Printf("Question: %s\n\n", question)
	
	// TODO: Initialize agent and execute query
	// 1. Initialize agent factory
	// 2. Create master agent
	// 3. Execute query
	// 4. Print result

	fmt.Println("Answer: [Query execution not implemented yet]")
	return nil
}

func runChat(ctx context.Context, cfg *config.Config) error {
	fmt.Println("Starting interactive chat mode...")
	fmt.Println("Type 'exit' or 'quit' to end the session.")
	fmt.Println("")

	// TODO: Implement interactive chat
	// 1. Initialize agent factory
	// 2. Create master agent
	// 3. Read input loop
	// 4. Execute queries and print results

	fmt.Println("[Chat mode not implemented yet]")
	return nil
}
