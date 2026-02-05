// Package index Neo4j 图索引实现
package index

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jIndex Neo4j 图索引
type Neo4jIndex struct {
	config   *types.Neo4jConfig
	driver   neo4j.DriverWithContext
	database string
}

// NewNeo4jIndex 创建 Neo4j 索引
func NewNeo4jIndex(cfg *types.Neo4jConfig) (*Neo4jIndex, error) {
	driver, err := neo4j.NewDriverWithContext(
		cfg.URI,
		neo4j.BasicAuth(cfg.Username, cfg.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("create driver: %w", err)
	}

	// 验证连接
	ctx := context.Background()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		driver.Close(ctx)
		return nil, fmt.Errorf("verify connectivity: %w", err)
	}

	database := cfg.Database
	if database == "" {
		database = "neo4j"
	}

	return &Neo4jIndex{
		config:   cfg,
		driver:   driver,
		database: database,
	}, nil
}

// IndexEntity 索引实体
func (n *Neo4jIndex) IndexEntity(ctx context.Context, entity *Entity) error {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		cypher := `
			MERGE (e:Entity {id: $id})
			SET e.name = $name, e.type = $type, e.attributes = $attributes
			RETURN e
		`
		_, err := tx.Run(ctx, cypher, map[string]interface{}{
			"id":         entity.ID,
			"name":       entity.Name,
			"type":       entity.Type,
			"attributes": entity.Attributes,
		})
		return nil, err
	})

	return err
}

// IndexRelation 索引关系
func (n *Neo4jIndex) IndexRelation(ctx context.Context, relation *Relation) error {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		cypher := `
			MATCH (from:Entity {name: $from})
			MATCH (to:Entity {name: $to})
			MERGE (from)-[r:RELATES {type: $relationType}]->(to)
			SET r.weight = $weight
			RETURN r
		`
		_, err := tx.Run(ctx, cypher, map[string]interface{}{
			"from":         relation.FromEntity,
			"to":           relation.ToEntity,
			"relationType": relation.RelationType,
			"weight":       relation.Weight,
		})
		return nil, err
	})

	return err
}

// QueryEntities 查询实体
func (n *Neo4jIndex) QueryEntities(ctx context.Context, query string, limit int) ([]*Entity, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		cypher := `
			MATCH (e:Entity)
			WHERE e.name CONTAINS $query OR e.type CONTAINS $query
			RETURN e.id AS id, e.name AS name, e.type AS type, e.attributes AS attributes
			LIMIT $limit
		`
		records, err := tx.Run(ctx, cypher, map[string]interface{}{
			"query": query,
			"limit": limit,
		})
		if err != nil {
			return nil, err
		}

		var entities []*Entity
		for records.Next(ctx) {
			record := records.Record()
			entity := &Entity{
				ID:   getStringValue(record, "id"),
				Name: getStringValue(record, "name"),
				Type: getStringValue(record, "type"),
			}
			if attrs, ok := record.Get("attributes"); ok && attrs != nil {
				if attrsMap, ok := attrs.(map[string]interface{}); ok {
					entity.Attributes = attrsMap
				}
			}
			entities = append(entities, entity)
		}
		return entities, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]*Entity), nil
}

// QueryRelations 查询关系
func (n *Neo4jIndex) QueryRelations(ctx context.Context, entityName string, depth int) ([]*Relation, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		cypher := fmt.Sprintf(`
			MATCH (e:Entity {name: $name})-[r*1..%d]-(related:Entity)
			UNWIND r AS rel
			WITH DISTINCT startNode(rel) AS from, endNode(rel) AS to, rel
			RETURN from.name AS fromEntity, to.name AS toEntity, rel.type AS relationType, rel.weight AS weight
		`, depth)

		records, err := tx.Run(ctx, cypher, map[string]interface{}{
			"name": entityName,
		})
		if err != nil {
			return nil, err
		}

		var relations []*Relation
		for records.Next(ctx) {
			record := records.Record()
			relation := &Relation{
				FromEntity:   getStringValue(record, "fromEntity"),
				ToEntity:     getStringValue(record, "toEntity"),
				RelationType: getStringValue(record, "relationType"),
			}
			if weight, ok := record.Get("weight"); ok && weight != nil {
				if w, ok := weight.(float64); ok {
					relation.Weight = w
				}
			}
			relations = append(relations, relation)
		}
		return relations, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]*Relation), nil
}

// HealthCheck 健康检查
func (n *Neo4jIndex) HealthCheck(ctx context.Context) error {
	return n.driver.VerifyConnectivity(ctx)
}

// Close 关闭
func (n *Neo4jIndex) Close() error {
	return n.driver.Close(context.Background())
}

// GetCounts 获取节点和边的数量
func (n *Neo4jIndex) GetCounts(ctx context.Context) (nodeCount int64, edgeCount int64, err error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database})
	defer session.Close(ctx)

	// 获取节点数量
	nodeResult, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, "MATCH (n:Entity) RETURN count(n) AS count", nil)
		if err != nil {
			return int64(0), err
		}
		if result.Next(ctx) {
			if count, ok := result.Record().Get("count"); ok {
				if c, ok := count.(int64); ok {
					return c, nil
				}
			}
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, 0, err
	}
	nodeCount = nodeResult.(int64)

	// 获取边数量
	edgeResult, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, "MATCH ()-[r]->() RETURN count(r) AS count", nil)
		if err != nil {
			return int64(0), err
		}
		if result.Next(ctx) {
			if count, ok := result.Record().Get("count"); ok {
				if c, ok := count.(int64); ok {
					return c, nil
				}
			}
		}
		return int64(0), nil
	})
	if err != nil {
		return nodeCount, 0, err
	}
	edgeCount = edgeResult.(int64)

	return nodeCount, edgeCount, nil
}

// GetGraphStats 获取图统计信息
func (n *Neo4jIndex) GetGraphStats(ctx context.Context) (*Neo4jStats, error) {
	nodeCount, edgeCount, err := n.GetCounts(ctx)
	if err != nil {
		return nil, err
	}

	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database})
	defer session.Close(ctx)

	// 获取实体类型统计
	entityTypes := make(map[string]int64)
	_, err = session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, `
			MATCH (n:Entity)
			RETURN n.type AS type, count(n) AS count
		`, nil)
		if err != nil {
			return nil, err
		}
		for result.Next(ctx) {
			record := result.Record()
			if typeVal, ok := record.Get("type"); ok && typeVal != nil {
				if t, ok := typeVal.(string); ok {
					if countVal, ok := record.Get("count"); ok {
						if c, ok := countVal.(int64); ok {
							entityTypes[t] = c
						}
					}
				}
			}
		}
		return nil, nil
	})

	// 获取关系类型统计
	relationTypes := make(map[string]int64)
	_, err = session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, `
			MATCH ()-[r]->()
			RETURN type(r) AS type, count(r) AS count
		`, nil)
		if err != nil {
			return nil, err
		}
		for result.Next(ctx) {
			record := result.Record()
			if typeVal, ok := record.Get("type"); ok && typeVal != nil {
				if t, ok := typeVal.(string); ok {
					if countVal, ok := record.Get("count"); ok {
						if c, ok := countVal.(int64); ok {
							relationTypes[t] = c
						}
					}
				}
			}
		}
		return nil, nil
	})

	return &Neo4jStats{
		NodeCount:     nodeCount,
		EdgeCount:     edgeCount,
		EntityTypes:   entityTypes,
		RelationTypes: relationTypes,
	}, nil
}

// Neo4jStats Neo4j统计信息
type Neo4jStats struct {
	NodeCount     int64            `json:"node_count"`
	EdgeCount     int64            `json:"edge_count"`
	EntityTypes   map[string]int64 `json:"entity_types"`
	RelationTypes map[string]int64 `json:"relation_types"`
}

// getStringValue 从记录中获取字符串值
func getStringValue(record *neo4j.Record, key string) string {
	if val, ok := record.Get(key); ok && val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// 确保 Neo4jIndex 实现了 GraphIndex 接口
var _ GraphIndex = (*Neo4jIndex)(nil)
