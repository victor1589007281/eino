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

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"kernel_expert/indexer"
)

// IndexSearchTool provides keyword-based search using inverted index.
type IndexSearchTool struct {
	indexManager *indexer.SimpleIndexManager
}

// IndexSearchInput represents input for index search.
type IndexSearchInput struct {
	Keywords []string `json:"keywords"` // Keywords to search
	Operator string   `json:"operator"` // AND or OR
	Limit    int      `json:"limit"`    // Maximum results
}

// IndexSearchResult represents index search results.
type IndexSearchResult struct {
	Files   []string `json:"files"`
	Total   int      `json:"total"`
	Keyword string   `json:"keyword"`
}

// NewIndexSearchTool creates a new index search tool.
func NewIndexSearchTool(manager *indexer.SimpleIndexManager) *IndexSearchTool {
	return &IndexSearchTool{
		indexManager: manager,
	}
}

// Info returns tool information.
func (t *IndexSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "index_search",
		Desc: `Search Linux kernel source files using keyword-based inverted index.
Faster than grep for finding files containing specific terms.
Use this for initial exploration to narrow down relevant files.

Good for:
- Finding files related to a subsystem
- Locating code by concept/keyword
- Initial broad search before precise grep`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"keywords": {
				Type:     schema.Array,
				Desc:     "Keywords to search for, e.g., ['scheduler', 'cfs', 'runqueue']",
				Required: true,
			},
			"operator": {
				Type:     schema.String,
				Desc:     "Search operator: AND (all keywords) or OR (any keyword). Default: AND",
				Required: false,
			},
			"limit": {
				Type:     schema.Integer,
				Desc:     "Maximum number of results. Default: 20",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun executes the index search.
func (t *IndexSearchTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input IndexSearchInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// Set defaults
	if input.Operator == "" {
		input.Operator = "AND"
	}
	if input.Limit <= 0 {
		input.Limit = 20
	}

	results := t.indexManager.Search(input.Keywords, input.Operator, input.Limit)

	files := make([]string, 0, len(results))
	for _, r := range results {
		files = append(files, r.File)
	}

	output := &IndexSearchResult{
		Files:   files,
		Total:   len(files),
		Keyword: fmt.Sprintf("%v (%s)", input.Keywords, input.Operator),
	}

	result, err := json.Marshal(output)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// Compile-time check
var _ tool.InvokableTool = (*IndexSearchTool)(nil)
