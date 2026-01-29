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
	"bufio"
	"os"
	"regexp"
	"strings"
	"sync"
)

// FunctionSummaryDB provides fast function information lookup.
type FunctionSummaryDB struct {
	Functions map[string][]*FunctionInfo // name -> list of functions (exported for gob)
	ByFile    map[string][]*FunctionInfo // file -> list of functions (exported for gob)
	mu        sync.RWMutex
}

// NewFunctionSummaryDB creates a new function summary database.
func NewFunctionSummaryDB() *FunctionSummaryDB {
	return &FunctionSummaryDB{
		Functions: make(map[string][]*FunctionInfo),
		ByFile:    make(map[string][]*FunctionInfo),
	}
}

// AddFunction adds a function to the database.
func (db *FunctionSummaryDB) AddFunction(info *FunctionInfo) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.Functions[info.Name] = append(db.Functions[info.Name], info)
	db.ByFile[info.File] = append(db.ByFile[info.File], info)
}

// GetFunction returns function information by name.
func (db *FunctionSummaryDB) GetFunction(name string) []*FunctionInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.Functions[name]
}

// GetFunctionByFile returns function information by name and file.
func (db *FunctionSummaryDB) GetFunctionByFile(name, file string) *FunctionInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()

	funcs := db.Functions[name]
	for _, f := range funcs {
		if f.File == file {
			return f
		}
	}
	return nil
}

// GetFunctionsInFile returns all functions in a file.
func (db *FunctionSummaryDB) GetFunctionsInFile(file string) []*FunctionInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.ByFile[file]
}

// SearchFunctions searches for functions by name pattern.
func (db *FunctionSummaryDB) SearchFunctions(pattern string, limit int) []*FunctionInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		// Treat as prefix search
		results := make([]*FunctionInfo, 0, limit)
		for name, funcs := range db.Functions {
			if strings.HasPrefix(name, pattern) {
				results = append(results, funcs...)
				if len(results) >= limit {
					break
				}
			}
		}
		return results
	}

	results := make([]*FunctionInfo, 0, limit)
	for name, funcs := range db.Functions {
		if re.MatchString(name) {
			results = append(results, funcs...)
			if len(results) >= limit {
				break
			}
		}
	}
	return results
}

// GetStats returns database statistics.
func (db *FunctionSummaryDB) GetStats() (funcCount, fileCount int) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	totalFuncs := 0
	for _, funcs := range db.Functions {
		totalFuncs += len(funcs)
	}
	return totalFuncs, len(db.ByFile)
}

// ParseCTagsFile parses a ctags output file.
func ParseCTagsFile(path string) ([]*Symbol, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	symbols := make([]*Symbol, 0)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		// Skip comments
		if strings.HasPrefix(line, "!_") {
			continue
		}

		symbol := parseCTagsLine(line)
		if symbol != nil {
			symbols = append(symbols, symbol)
		}
	}

	return symbols, scanner.Err()
}

// parseCTagsLine parses a single ctags line.
// Format: name<TAB>file<TAB>pattern;"<TAB>kind<TAB>extras
func parseCTagsLine(line string) *Symbol {
	parts := strings.Split(line, "\t")
	if len(parts) < 3 {
		return nil
	}

	symbol := &Symbol{
		Name:   parts[0],
		File:   parts[1],
		Extras: make(map[string]string),
	}

	// Parse pattern and kind
	for i := 2; i < len(parts); i++ {
		part := parts[i]

		// Kind field
		if len(part) == 1 {
			symbol.Kind = ctagsKindToSymbolKind(part)
			continue
		}

		// Pattern field (ends with ;")
		if strings.HasSuffix(part, ";\"") {
			symbol.Pattern = strings.TrimSuffix(part, ";\"")
			continue
		}

		// Extra fields (key:value)
		if strings.Contains(part, ":") {
			kv := strings.SplitN(part, ":", 2)
			if len(kv) == 2 {
				symbol.Extras[kv[0]] = kv[1]
			}
		}
	}

	// Extract line number from extras
	if lineStr, ok := symbol.Extras["line"]; ok {
		var lineNum int
		if _, err := parseLineNumber(lineStr, &lineNum); err == nil {
			symbol.Line = lineNum
		}
	}

	return symbol
}

func parseLineNumber(s string, line *int) (int, error) {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	*line = n
	return n, nil
}

// ctagsKindToSymbolKind converts ctags kind to SymbolKind.
func ctagsKindToSymbolKind(kind string) SymbolKind {
	switch kind {
	case "f":
		return SymbolFunction
	case "s":
		return SymbolStruct
	case "d":
		return SymbolMacro
	case "t":
		return SymbolTypedef
	case "e":
		return SymbolEnum
	case "v":
		return SymbolVariable
	case "p":
		return SymbolPrototype
	default:
		return SymbolKind(kind)
	}
}

// SymbolTable provides symbol lookup.
type SymbolTable struct {
	Symbols map[string][]*Symbol      // name -> symbols (exported for gob)
	ByFile  map[string][]*Symbol      // file -> symbols (exported for gob)
	ByKind  map[SymbolKind][]*Symbol  // kind -> symbols (exported for gob)
	mu      sync.RWMutex
}

// NewSymbolTable creates a new symbol table.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		Symbols: make(map[string][]*Symbol),
		ByFile:  make(map[string][]*Symbol),
		ByKind:  make(map[SymbolKind][]*Symbol),
	}
}

// AddSymbol adds a symbol to the table.
func (st *SymbolTable) AddSymbol(symbol *Symbol) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.Symbols[symbol.Name] = append(st.Symbols[symbol.Name], symbol)
	st.ByFile[symbol.File] = append(st.ByFile[symbol.File], symbol)
	st.ByKind[symbol.Kind] = append(st.ByKind[symbol.Kind], symbol)
}

// GetSymbol returns symbols by name.
func (st *SymbolTable) GetSymbol(name string) []*Symbol {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.Symbols[name]
}

// GetSymbolsByFile returns symbols in a file.
func (st *SymbolTable) GetSymbolsByFile(file string) []*Symbol {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.ByFile[file]
}

// GetSymbolsByKind returns symbols of a specific kind.
func (st *SymbolTable) GetSymbolsByKind(kind SymbolKind) []*Symbol {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.ByKind[kind]
}

// SearchSymbols searches for symbols by name pattern.
func (st *SymbolTable) SearchSymbols(pattern string, kind SymbolKind, limit int) []*Symbol {
	st.mu.RLock()
	defer st.mu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	results := make([]*Symbol, 0, limit)
	for name, syms := range st.Symbols {
		if re.MatchString(name) {
			for _, sym := range syms {
				if kind == "" || sym.Kind == kind {
					results = append(results, sym)
					if len(results) >= limit {
						return results
					}
				}
			}
		}
	}
	return results
}

// GetStats returns symbol table statistics.
func (st *SymbolTable) GetStats() (total int, byKind map[SymbolKind]int) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	byKind = make(map[SymbolKind]int)
	for kind, syms := range st.ByKind {
		byKind[kind] = len(syms)
		total += len(syms)
	}
	return
}
