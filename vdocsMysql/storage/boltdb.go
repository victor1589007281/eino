package storage

import (
	"context"
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// Bucket names for BoltDB
var (
	bucketNodes       = []byte("nodes")
	bucketEdges       = []byte("edges")
	bucketCallerIndex = []byte("caller_index")
	bucketCalleeIndex = []byte("callee_index")
	bucketModuleIndex = []byte("module_index")
	bucketFileIndex   = []byte("file_index")
	bucketNameIndex   = []byte("name_index")
)

// BoltDBStorage implements call graph storage using BoltDB.
type BoltDBStorage struct {
	db     *bolt.DB
	dbPath string
}

// BoltDBConfig contains BoltDB configuration.
type BoltDBConfig struct {
	Path string
}

// NewBoltDBStorage creates a new BoltDB storage instance.
func NewBoltDBStorage(config *BoltDBConfig) (*BoltDBStorage, error) {
	db, err := bolt.Open(config.Path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open BoltDB: %w", err)
	}

	storage := &BoltDBStorage{
		db:     db,
		dbPath: config.Path,
	}

	// Initialize buckets
	if err := storage.initBuckets(); err != nil {
		db.Close()
		return nil, err
	}

	return storage, nil
}

func (s *BoltDBStorage) initBuckets() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			bucketNodes,
			bucketEdges,
			bucketCallerIndex,
			bucketCalleeIndex,
			bucketModuleIndex,
			bucketFileIndex,
			bucketNameIndex,
		}

		for _, name := range buckets {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", name, err)
			}
		}
		return nil
	})
}

// Close closes the database.
func (s *BoltDBStorage) Close() error {
	return s.db.Close()
}

// CreateNode creates a new call graph node.
func (s *BoltDBStorage) CreateNode(ctx context.Context, node *CallGraphNode) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Store node
		data, err := json.Marshal(node)
		if err != nil {
			return err
		}

		if err := tx.Bucket(bucketNodes).Put([]byte(node.ID), data); err != nil {
			return err
		}

		// Update indexes
		if err := s.addToIndex(tx, bucketModuleIndex, node.Module, node.ID); err != nil {
			return err
		}
		if err := s.addToIndex(tx, bucketFileIndex, node.FilePath, node.ID); err != nil {
			return err
		}
		if err := s.addToIndex(tx, bucketNameIndex, node.Name, node.ID); err != nil {
			return err
		}

		return nil
	})
}

// GetNode retrieves a call graph node by ID.
func (s *BoltDBStorage) GetNode(ctx context.Context, id string) (*CallGraphNode, error) {
	var node *CallGraphNode

	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketNodes).Get([]byte(id))
		if data == nil {
			return nil
		}

		node = &CallGraphNode{}
		return json.Unmarshal(data, node)
	})

	return node, err
}

// UpdateNode updates an existing call graph node.
func (s *BoltDBStorage) UpdateNode(ctx context.Context, node *CallGraphNode) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Get existing node to update indexes if needed
		existing := &CallGraphNode{}
		if data := tx.Bucket(bucketNodes).Get([]byte(node.ID)); data != nil {
			json.Unmarshal(data, existing)

			// Remove from old indexes if changed
			if existing.Module != node.Module {
				s.removeFromIndex(tx, bucketModuleIndex, existing.Module, node.ID)
				s.addToIndex(tx, bucketModuleIndex, node.Module, node.ID)
			}
			if existing.FilePath != node.FilePath {
				s.removeFromIndex(tx, bucketFileIndex, existing.FilePath, node.ID)
				s.addToIndex(tx, bucketFileIndex, node.FilePath, node.ID)
			}
			if existing.Name != node.Name {
				s.removeFromIndex(tx, bucketNameIndex, existing.Name, node.ID)
				s.addToIndex(tx, bucketNameIndex, node.Name, node.ID)
			}
		}

		data, err := json.Marshal(node)
		if err != nil {
			return err
		}

		return tx.Bucket(bucketNodes).Put([]byte(node.ID), data)
	})
}

// DeleteNode deletes a call graph node.
func (s *BoltDBStorage) DeleteNode(ctx context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Get node to remove from indexes
		if data := tx.Bucket(bucketNodes).Get([]byte(id)); data != nil {
			node := &CallGraphNode{}
			if err := json.Unmarshal(data, node); err == nil {
				s.removeFromIndex(tx, bucketModuleIndex, node.Module, id)
				s.removeFromIndex(tx, bucketFileIndex, node.FilePath, id)
				s.removeFromIndex(tx, bucketNameIndex, node.Name, id)
			}
		}

		// Delete associated edges
		edgesToDelete := make([]string, 0)
		tx.Bucket(bucketEdges).ForEach(func(k, v []byte) error {
			edge := &CallGraphEdge{}
			if err := json.Unmarshal(v, edge); err == nil {
				if edge.FromNodeID == id || edge.ToNodeID == id {
					edgesToDelete = append(edgesToDelete, edge.ID)
				}
			}
			return nil
		})

		for _, edgeID := range edgesToDelete {
			s.deleteEdgeInternal(tx, edgeID)
		}

		return tx.Bucket(bucketNodes).Delete([]byte(id))
	})
}

// CreateEdge creates a new call graph edge.
func (s *BoltDBStorage) CreateEdge(ctx context.Context, edge *CallGraphEdge) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		data, err := json.Marshal(edge)
		if err != nil {
			return err
		}

		if err := tx.Bucket(bucketEdges).Put([]byte(edge.ID), data); err != nil {
			return err
		}

		// Update caller/callee indexes
		if err := s.addToIndex(tx, bucketCallerIndex, edge.ToNodeID, edge.ID); err != nil {
			return err
		}
		if err := s.addToIndex(tx, bucketCalleeIndex, edge.FromNodeID, edge.ID); err != nil {
			return err
		}

		return nil
	})
}

// GetEdge retrieves a call graph edge by ID.
func (s *BoltDBStorage) GetEdge(ctx context.Context, id string) (*CallGraphEdge, error) {
	var edge *CallGraphEdge

	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketEdges).Get([]byte(id))
		if data == nil {
			return nil
		}

		edge = &CallGraphEdge{}
		return json.Unmarshal(data, edge)
	})

	return edge, err
}

// DeleteEdge deletes a call graph edge.
func (s *BoltDBStorage) DeleteEdge(ctx context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.deleteEdgeInternal(tx, id)
	})
}

func (s *BoltDBStorage) deleteEdgeInternal(tx *bolt.Tx, id string) error {
	// Get edge to remove from indexes
	if data := tx.Bucket(bucketEdges).Get([]byte(id)); data != nil {
		edge := &CallGraphEdge{}
		if err := json.Unmarshal(data, edge); err == nil {
			s.removeFromIndex(tx, bucketCallerIndex, edge.ToNodeID, id)
			s.removeFromIndex(tx, bucketCalleeIndex, edge.FromNodeID, id)
		}
	}

	return tx.Bucket(bucketEdges).Delete([]byte(id))
}

// GetCallers returns all callers of a function.
func (s *BoltDBStorage) GetCallers(ctx context.Context, funcID string) ([]*CallGraphEdge, error) {
	var edges []*CallGraphEdge

	err := s.db.View(func(tx *bolt.Tx) error {
		edgeIDs, err := s.getIndexValues(tx, bucketCallerIndex, funcID)
		if err != nil {
			return err
		}

		for _, edgeID := range edgeIDs {
			if data := tx.Bucket(bucketEdges).Get([]byte(edgeID)); data != nil {
				edge := &CallGraphEdge{}
				if err := json.Unmarshal(data, edge); err == nil {
					edges = append(edges, edge)
				}
			}
		}

		return nil
	})

	return edges, err
}

// GetCallees returns all callees of a function.
func (s *BoltDBStorage) GetCallees(ctx context.Context, funcID string) ([]*CallGraphEdge, error) {
	var edges []*CallGraphEdge

	err := s.db.View(func(tx *bolt.Tx) error {
		edgeIDs, err := s.getIndexValues(tx, bucketCalleeIndex, funcID)
		if err != nil {
			return err
		}

		for _, edgeID := range edgeIDs {
			if data := tx.Bucket(bucketEdges).Get([]byte(edgeID)); data != nil {
				edge := &CallGraphEdge{}
				if err := json.Unmarshal(data, edge); err == nil {
					edges = append(edges, edge)
				}
			}
		}

		return nil
	})

	return edges, err
}

// GetCallChain retrieves the call chain for a function.
func (s *BoltDBStorage) GetCallChain(ctx context.Context, funcID string, depth int, direction Direction) ([]*CallGraphNode, []*CallGraphEdge, error) {
	var nodes []*CallGraphNode
	var edges []*CallGraphEdge
	visited := make(map[string]bool)

	err := s.db.View(func(tx *bolt.Tx) error {
		return s.traverseCallChain(tx, funcID, depth, direction, visited, &nodes, &edges)
	})

	return nodes, edges, err
}

func (s *BoltDBStorage) traverseCallChain(tx *bolt.Tx, funcID string, depth int, direction Direction, visited map[string]bool, nodes *[]*CallGraphNode, edges *[]*CallGraphEdge) error {
	if depth <= 0 || visited[funcID] {
		return nil
	}
	visited[funcID] = true

	// Get node
	if data := tx.Bucket(bucketNodes).Get([]byte(funcID)); data != nil {
		node := &CallGraphNode{}
		if err := json.Unmarshal(data, node); err == nil {
			*nodes = append(*nodes, node)
		}
	}

	// Traverse based on direction
	if direction == DirectionUp || direction == DirectionBoth {
		edgeIDs, _ := s.getIndexValues(tx, bucketCallerIndex, funcID)
		for _, edgeID := range edgeIDs {
			if data := tx.Bucket(bucketEdges).Get([]byte(edgeID)); data != nil {
				edge := &CallGraphEdge{}
				if err := json.Unmarshal(data, edge); err == nil {
					*edges = append(*edges, edge)
					s.traverseCallChain(tx, edge.FromNodeID, depth-1, DirectionUp, visited, nodes, edges)
				}
			}
		}
	}

	if direction == DirectionDown || direction == DirectionBoth {
		edgeIDs, _ := s.getIndexValues(tx, bucketCalleeIndex, funcID)
		for _, edgeID := range edgeIDs {
			if data := tx.Bucket(bucketEdges).Get([]byte(edgeID)); data != nil {
				edge := &CallGraphEdge{}
				if err := json.Unmarshal(data, edge); err == nil {
					*edges = append(*edges, edge)
					s.traverseCallChain(tx, edge.ToNodeID, depth-1, DirectionDown, visited, nodes, edges)
				}
			}
		}
	}

	return nil
}

// BatchCreateNodes creates multiple nodes in a single transaction.
func (s *BoltDBStorage) BatchCreateNodes(ctx context.Context, nodesToCreate []*CallGraphNode) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, node := range nodesToCreate {
			data, err := json.Marshal(node)
			if err != nil {
				return err
			}

			if err := tx.Bucket(bucketNodes).Put([]byte(node.ID), data); err != nil {
				return err
			}

			s.addToIndex(tx, bucketModuleIndex, node.Module, node.ID)
			s.addToIndex(tx, bucketFileIndex, node.FilePath, node.ID)
			s.addToIndex(tx, bucketNameIndex, node.Name, node.ID)
		}
		return nil
	})
}

// BatchCreateEdges creates multiple edges in a single transaction.
func (s *BoltDBStorage) BatchCreateEdges(ctx context.Context, edgesToCreate []*CallGraphEdge) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, edge := range edgesToCreate {
			data, err := json.Marshal(edge)
			if err != nil {
				return err
			}

			if err := tx.Bucket(bucketEdges).Put([]byte(edge.ID), data); err != nil {
				return err
			}

			s.addToIndex(tx, bucketCallerIndex, edge.ToNodeID, edge.ID)
			s.addToIndex(tx, bucketCalleeIndex, edge.FromNodeID, edge.ID)
		}
		return nil
	})
}

// GetNodesByModule returns all nodes in a module.
func (s *BoltDBStorage) GetNodesByModule(ctx context.Context, module string) ([]*CallGraphNode, error) {
	var nodes []*CallGraphNode

	err := s.db.View(func(tx *bolt.Tx) error {
		nodeIDs, err := s.getIndexValues(tx, bucketModuleIndex, module)
		if err != nil {
			return err
		}

		for _, nodeID := range nodeIDs {
			if data := tx.Bucket(bucketNodes).Get([]byte(nodeID)); data != nil {
				node := &CallGraphNode{}
				if err := json.Unmarshal(data, node); err == nil {
					nodes = append(nodes, node)
				}
			}
		}

		return nil
	})

	return nodes, err
}

// GetNodesByFile returns all nodes in a file.
func (s *BoltDBStorage) GetNodesByFile(ctx context.Context, filePath string) ([]*CallGraphNode, error) {
	var nodes []*CallGraphNode

	err := s.db.View(func(tx *bolt.Tx) error {
		nodeIDs, err := s.getIndexValues(tx, bucketFileIndex, filePath)
		if err != nil {
			return err
		}

		for _, nodeID := range nodeIDs {
			if data := tx.Bucket(bucketNodes).Get([]byte(nodeID)); data != nil {
				node := &CallGraphNode{}
				if err := json.Unmarshal(data, node); err == nil {
					nodes = append(nodes, node)
				}
			}
		}

		return nil
	})

	return nodes, err
}

// Index helper functions

func (s *BoltDBStorage) addToIndex(tx *bolt.Tx, bucket []byte, key, value string) error {
	if key == "" {
		return nil
	}

	b := tx.Bucket(bucket)
	existing := b.Get([]byte(key))

	var values []string
	if existing != nil {
		json.Unmarshal(existing, &values)
	}

	// Check if value already exists
	for _, v := range values {
		if v == value {
			return nil
		}
	}

	values = append(values, value)
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}

	return b.Put([]byte(key), data)
}

func (s *BoltDBStorage) removeFromIndex(tx *bolt.Tx, bucket []byte, key, value string) error {
	if key == "" {
		return nil
	}

	b := tx.Bucket(bucket)
	existing := b.Get([]byte(key))
	if existing == nil {
		return nil
	}

	var values []string
	json.Unmarshal(existing, &values)

	// Remove value
	newValues := make([]string, 0, len(values))
	for _, v := range values {
		if v != value {
			newValues = append(newValues, v)
		}
	}

	if len(newValues) == 0 {
		return b.Delete([]byte(key))
	}

	data, err := json.Marshal(newValues)
	if err != nil {
		return err
	}

	return b.Put([]byte(key), data)
}

func (s *BoltDBStorage) getIndexValues(tx *bolt.Tx, bucket []byte, key string) ([]string, error) {
	data := tx.Bucket(bucket).Get([]byte(key))
	if data == nil {
		return nil, nil
	}

	var values []string
	err := json.Unmarshal(data, &values)
	return values, err
}

// Stats returns storage statistics.
func (s *BoltDBStorage) Stats(ctx context.Context) (*StorageStats, error) {
	stats := &StorageStats{}

	s.db.View(func(tx *bolt.Tx) error {
		stats.NodeCount = tx.Bucket(bucketNodes).Stats().KeyN
		stats.EdgeCount = tx.Bucket(bucketEdges).Stats().KeyN
		return nil
	})

	return stats, nil
}

// Vacuum performs database maintenance.
func (s *BoltDBStorage) Vacuum(ctx context.Context) error {
	// BoltDB doesn't have a vacuum operation, but we can compact by copying
	return nil
}

// GetNodeByName retrieves nodes by function name.
func (s *BoltDBStorage) GetNodeByName(ctx context.Context, name string) ([]*CallGraphNode, error) {
	var nodes []*CallGraphNode

	err := s.db.View(func(tx *bolt.Tx) error {
		nodeIDs, err := s.getIndexValues(tx, bucketNameIndex, name)
		if err != nil {
			return err
		}

		for _, nodeID := range nodeIDs {
			if data := tx.Bucket(bucketNodes).Get([]byte(nodeID)); data != nil {
				node := &CallGraphNode{}
				if err := json.Unmarshal(data, node); err == nil {
					nodes = append(nodes, node)
				}
			}
		}

		return nil
	})

	return nodes, err
}

// ClearAll removes all data from the database.
func (s *BoltDBStorage) ClearAll(ctx context.Context) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			bucketNodes,
			bucketEdges,
			bucketCallerIndex,
			bucketCalleeIndex,
			bucketModuleIndex,
			bucketFileIndex,
			bucketNameIndex,
		}

		for _, name := range buckets {
			if err := tx.DeleteBucket(name); err != nil && err != bolt.ErrBucketNotFound {
				return err
			}
			if _, err := tx.CreateBucket(name); err != nil {
				return err
			}
		}

		return nil
	})
}
