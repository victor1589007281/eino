/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package main provides the entry point for Linux Kernel Expert Agent.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/model"

	"kernel_expert/agent"
	"kernel_expert/indexer"
	"kernel_expert/llm"
	"kernel_expert/output"
)

var (
	sourcePath = flag.String("source", "/Users/huaquan.liang/Documents/GitHub/linux", "Path to Linux kernel source")
	indexPath  = flag.String("index", "./index", "Path to store index files")
	buildIndex = flag.Bool("build-index", false, "Build index before starting")
	outputType = flag.String("output", "document", "Output type: summary or document")
	query      = flag.String("query", "", "Query to run (non-interactive mode)")
)

func main() {
	flag.Parse()

	ctx := context.Background()

	// Build index if requested
	if *buildIndex {
		fmt.Println("Building index...")
		builder := indexer.NewIndexBuilder(*sourcePath, *indexPath, 4)
		if err := builder.Build(ctx); err != nil {
			fmt.Printf("Error building index: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Index built successfully!")
		if *query == "" {
			return
		}
	}

	// Create model (placeholder - in real usage, inject actual model)
	chatModel := createChatModel()
	if chatModel == nil {
		fmt.Println("Error: Chat model not configured. Please set up your LLM API.")
		fmt.Println("Usage: Set OPENAI_API_KEY or ANTHROPIC_API_KEY environment variable")
		os.Exit(1)
	}

	// Create expert agent
	expert, err := agent.NewLinuxKernelExpert(ctx, &agent.LinuxKernelExpertConfig{
		Model:      chatModel,
		SourcePath: *sourcePath,
		IndexPath:  *indexPath,
		MaxWorkers: 4,
	})
	if err != nil {
		fmt.Printf("Error creating expert agent: %v\n", err)
		os.Exit(1)
	}

	// Determine output type
	outType := output.OutputDocument
	if *outputType == "summary" {
		outType = output.OutputSummary
	}

	// Run in non-interactive mode if query provided
	if *query != "" {
		runQuery(ctx, expert, *query, outType)
		return
	}

	// Interactive mode
	runInteractive(ctx, expert, outType)
}

// createChatModel creates the chat model based on environment configuration.
func createChatModel() model.ToolCallingChatModel {
	// Check for DeepSeek API key first
	if apiKey := os.Getenv("DEEPSEEK_API_KEY"); apiKey != "" {
		fmt.Println("Using DeepSeek model...")
		m, err := llm.NewDeepSeekModel(&llm.DeepSeekConfig{
			APIKey: apiKey,
		})
		if err != nil {
			fmt.Printf("Warning: Failed to create DeepSeek model: %v\n", err)
			return nil
		}
		return m
	}

	// Check for OpenAI API key (can also use for DeepSeek with custom base URL)
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		baseURL := os.Getenv("OPENAI_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		modelName := os.Getenv("OPENAI_MODEL")
		if modelName == "" {
			modelName = "gpt-4-turbo"
		}
		fmt.Printf("Using OpenAI-compatible model: %s\n", modelName)
		m, err := llm.NewDeepSeekModel(&llm.DeepSeekConfig{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   modelName,
		})
		if err != nil {
			fmt.Printf("Warning: Failed to create model: %v\n", err)
			return nil
		}
		return m
	}

	return nil
}

// runQuery runs a single query.
func runQuery(ctx context.Context, expert *agent.LinuxKernelExpert, query string, outType output.OutputType) {
	fmt.Printf("Query: %s\n", query)
	fmt.Println("Analyzing...")

	result, err := expert.Run(ctx, query, outType)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println(result.Content)
}

// runInteractive runs the interactive REPL.
func runInteractive(ctx context.Context, expert *agent.LinuxKernelExpert, outType output.OutputType) {
	fmt.Println("Linux Kernel Expert Agent")
	fmt.Println("========================")
	fmt.Println("Ask questions about Linux kernel source code.")
	fmt.Println("Commands: /summary, /document, /quit")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("You: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nGoodbye!")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle commands
		switch input {
		case "/quit", "/exit", "/q":
			fmt.Println("Goodbye!")
			return
		case "/summary":
			outType = output.OutputSummary
			fmt.Println("Switched to summary output mode")
			continue
		case "/document":
			outType = output.OutputDocument
			fmt.Println("Switched to document output mode")
			continue
		case "/help":
			printHelp()
			continue
		}

		// Run query
		fmt.Println("\nAnalyzing...")
		result, err := expert.Run(ctx, input, outType)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		fmt.Println("\n" + strings.Repeat("-", 60))
		fmt.Println(result.Content)
		fmt.Println(strings.Repeat("-", 60) + "\n")
	}
}

func printHelp() {
	fmt.Print(`
Commands:
  /summary   - Switch to summary output mode
  /document  - Switch to document output mode (with diagrams)
  /quit      - Exit the program
  /help      - Show this help

Example queries:
  - "fork系统调用是如何实现的？"
  - "schedule函数的调用链是什么？"
  - "VFS子系统的架构是怎样的？"
  - "spin_lock和mutex有什么区别？"
`)
}
