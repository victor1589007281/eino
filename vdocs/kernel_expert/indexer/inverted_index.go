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

package indexer

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// InvertedIndex provides efficient keyword-based search.
type InvertedIndex struct {
	Terms     map[string]*PostingList `json:"terms"`
	Documents map[int]*Document       `json:"documents"`
	DocCount  int                     `json:"doc_count"`
	mu        sync.RWMutex            // unexported, not serialized
}

// NewInvertedIndex creates a new inverted index.
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		Terms:     make(map[string]*PostingList),
		Documents: make(map[int]*Document),
	}
}

// AddDocument adds a document to the index.
func (idx *InvertedIndex) AddDocument(path string, content string, size int64) int {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	docID := idx.DocCount
	idx.DocCount++

	// Tokenize content
	tokens := tokenize(content)
	termFreq := make(map[string]int)
	termPositions := make(map[string][]int)

	for i, token := range tokens {
		term := strings.ToLower(token.Term)
		termFreq[term]++
		termPositions[term] = append(termPositions[term], i)
	}

	// Create document
	doc := &Document{
		ID:    docID,
		Path:  path,
		Size:  size,
		Terms: termFreq,
	}
	idx.Documents[docID] = doc

	// Update inverted index
	for term, freq := range termFreq {
		posting := &Posting{
			DocID:     docID,
			Frequency: freq,
			Positions: termPositions[term],
		}

		if pl, exists := idx.Terms[term]; exists {
			pl.Postings = append(pl.Postings, posting)
			pl.DocFreq++
		} else {
			idx.Terms[term] = &PostingList{
				Term:     term,
				DocFreq:  1,
				Postings: []*Posting{posting},
			}
		}
	}

	return docID
}

// Search searches for documents containing the given keywords.
func (idx *InvertedIndex) Search(keywords []string, operator string, limit int) []*SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(keywords) == 0 {
		return nil
	}

	// Normalize keywords
	normalizedKeywords := make([]string, len(keywords))
	for i, kw := range keywords {
		normalizedKeywords[i] = strings.ToLower(strings.TrimSpace(kw))
	}

	// Get posting lists for all keywords
	postingLists := make([]*PostingList, 0, len(normalizedKeywords))
	for _, kw := range normalizedKeywords {
		if pl, exists := idx.Terms[kw]; exists {
			postingLists = append(postingLists, pl)
		}
	}

	if len(postingLists) == 0 {
		return nil
	}

	// Calculate document scores
	docScores := make(map[int]float64)

	if operator == "AND" {
		// AND: document must contain all keywords
		docScores = idx.intersectPostings(postingLists)
	} else {
		// OR: document can contain any keyword
		docScores = idx.unionPostings(postingLists)
	}

	// Sort by score
	type docScore struct {
		docID int
		score float64
	}
	scores := make([]docScore, 0, len(docScores))
	for docID, score := range docScores {
		scores = append(scores, docScore{docID, score})
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Build results
	results := make([]*SearchResult, 0, limit)
	for i, ds := range scores {
		if i >= limit {
			break
		}
		doc := idx.Documents[ds.docID]
		results = append(results, &SearchResult{
			File:  doc.Path,
			Score: ds.score,
		})
	}

	return results
}

// intersectPostings performs AND operation on posting lists.
func (idx *InvertedIndex) intersectPostings(lists []*PostingList) map[int]float64 {
	if len(lists) == 0 {
		return nil
	}

	// Start with the shortest list
	sort.Slice(lists, func(i, j int) bool {
		return lists[i].DocFreq < lists[j].DocFreq
	})

	// Get document IDs from the shortest list
	result := make(map[int]float64)
	for _, posting := range lists[0].Postings {
		result[posting.DocID] = float64(posting.Frequency)
	}

	// Intersect with other lists
	for i := 1; i < len(lists); i++ {
		docIDs := make(map[int]int)
		for _, posting := range lists[i].Postings {
			docIDs[posting.DocID] = posting.Frequency
		}

		for docID := range result {
			if freq, exists := docIDs[docID]; exists {
				result[docID] += float64(freq)
			} else {
				delete(result, docID)
			}
		}
	}

	return result
}

// unionPostings performs OR operation on posting lists.
func (idx *InvertedIndex) unionPostings(lists []*PostingList) map[int]float64 {
	result := make(map[int]float64)

	for _, pl := range lists {
		for _, posting := range pl.Postings {
			result[posting.DocID] += float64(posting.Frequency)
		}
	}

	return result
}

// GetDocument returns a document by ID.
func (idx *InvertedIndex) GetDocument(docID int) *Document {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.Documents[docID]
}

// GetStats returns index statistics.
func (idx *InvertedIndex) GetStats() (docCount, termCount int) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.DocCount, len(idx.Terms)
}

// CTokenizer provides C-language aware tokenization.
type CTokenizer struct {
	stopWords map[string]bool
}

// NewCTokenizer creates a new C tokenizer.
func NewCTokenizer() *CTokenizer {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"if": true, "else": true, "for": true, "while": true, "do": true,
		"return": true, "int": true, "char": true, "void": true, "long": true,
		"unsigned": true, "signed": true, "static": true, "const": true,
		"struct": true, "union": true, "enum": true, "typedef": true,
	}
	return &CTokenizer{stopWords: stopWords}
}

// tokenize tokenizes source code content.
func tokenize(content string) []Token {
	tokens := make([]Token, 0)

	// Match identifiers
	identifierRe := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`)
	matches := identifierRe.FindAllStringIndex(content, -1)

	for i, match := range matches {
		term := content[match[0]:match[1]]
		// Skip very short terms
		if len(term) < 2 {
			continue
		}
		// Split camelCase and snake_case
		subTokens := splitIdentifier(term)
		for _, subToken := range subTokens {
			tokens = append(tokens, Token{
				Term:     subToken,
				Position: i,
			})
		}
	}

	return tokens
}

// splitIdentifier splits an identifier into sub-tokens.
func splitIdentifier(identifier string) []string {
	result := make([]string, 0)

	// Add the full identifier
	result = append(result, identifier)

	// Split by underscore
	if strings.Contains(identifier, "_") {
		parts := strings.Split(identifier, "_")
		for _, part := range parts {
			if len(part) >= 2 {
				result = append(result, part)
			}
		}
	}

	// Split camelCase
	var current strings.Builder
	for i, r := range identifier {
		if i > 0 && unicode.IsUpper(r) {
			if current.Len() >= 2 {
				result = append(result, strings.ToLower(current.String()))
			}
			current.Reset()
		}
		current.WriteRune(r)
	}
	if current.Len() >= 2 {
		result = append(result, strings.ToLower(current.String()))
	}

	return result
}
