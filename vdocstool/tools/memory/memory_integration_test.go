//go:build integration

// Package memory 记忆工具集成测试
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
	"github.com/cloudwego/eino/vdocstool/tools/testutil"
)

// ====================== Mock 存储实现 ======================

// MockL1Storage Mock L1 存储
type MockL1Storage struct {
	redis    *testutil.MockRedis
	sessions map[string][]*storage.Message
}

func NewMockL1Storage() *MockL1Storage {
	return &MockL1Storage{
		redis:    testutil.NewMockRedis(),
		sessions: make(map[string][]*storage.Message),
	}
}

func (m *MockL1Storage) Tier() storage.StorageTier { return storage.TierL1 }

func (m *MockL1Storage) Store(ctx context.Context, data interface{}) error {
	return nil
}

func (m *MockL1Storage) Get(ctx context.Context, id string) (interface{}, error) {
	return nil, nil
}

func (m *MockL1Storage) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *MockL1Storage) Close() error {
	return m.redis.Close()
}

func (m *MockL1Storage) AppendMessage(ctx context.Context, sessionID string, msg *storage.Message) error {
	m.sessions[sessionID] = append(m.sessions[sessionID], msg)
	
	// 存储到 Redis
	key := fmt.Sprintf("l1:session:%s:messages", sessionID)
	data, _ := json.Marshal(msg)
	m.redis.RPush(ctx, key, string(data))
	
	return nil
}

func (m *MockL1Storage) GetRecentMessages(ctx context.Context, sessionID string, limit int) ([]*storage.Message, error) {
	messages := m.sessions[sessionID]
	if len(messages) <= limit {
		return messages, nil
	}
	return messages[len(messages)-limit:], nil
}

func (m *MockL1Storage) GetAllMessages(ctx context.Context, sessionID string) ([]*storage.Message, error) {
	return m.sessions[sessionID], nil
}

func (m *MockL1Storage) GetMessageCount(ctx context.Context, sessionID string) (int, error) {
	return len(m.sessions[sessionID]), nil
}

func (m *MockL1Storage) GetTokenCount(ctx context.Context, sessionID string) (int, error) {
	var total int
	for _, msg := range m.sessions[sessionID] {
		total += msg.TokenCount
	}
	return total, nil
}

func (m *MockL1Storage) ClearSession(ctx context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *MockL1Storage) GetActiveSessions(ctx context.Context) ([]string, error) {
	var sessions []string
	for k := range m.sessions {
		sessions = append(sessions, k)
	}
	return sessions, nil
}

func (m *MockL1Storage) ShouldArchive(ctx context.Context, sessionID string) (bool, string, error) {
	count := len(m.sessions[sessionID])
	if count >= 20 {
		return true, "message_count_exceeded", nil
	}
	
	tokenCount, _ := m.GetTokenCount(ctx, sessionID)
	if tokenCount >= 4000 {
		return true, "token_count_exceeded", nil
	}
	
	return false, "", nil
}

// MockL2Storage Mock L2 存储
type MockL2Storage struct {
	es       *testutil.MockElasticsearch
	milvus   *testutil.MockMilvus
	capsules map[string]*storage.TopicCapsule
}

func NewMockL2Storage() *MockL2Storage {
	es := testutil.NewMockElasticsearch()
	es.CreateIndex(context.Background(), "capsules", nil)
	
	milvus := testutil.NewMockMilvus()
	milvus.CreateCollection(context.Background(), "capsule_vectors", 1536)
	
	return &MockL2Storage{
		es:       es,
		milvus:   milvus,
		capsules: make(map[string]*storage.TopicCapsule),
	}
}

func (m *MockL2Storage) Tier() storage.StorageTier { return storage.TierL2 }

func (m *MockL2Storage) Store(ctx context.Context, data interface{}) error {
	return nil
}

func (m *MockL2Storage) Get(ctx context.Context, id string) (interface{}, error) {
	return m.capsules[id], nil
}

func (m *MockL2Storage) Delete(ctx context.Context, id string) error {
	delete(m.capsules, id)
	return nil
}

func (m *MockL2Storage) Close() error {
	m.es.Close()
	m.milvus.Close()
	return nil
}

func (m *MockL2Storage) StoreCapsule(ctx context.Context, capsule *storage.TopicCapsule) error {
	m.capsules[capsule.ID] = capsule
	
	// 索引到 ES
	doc := map[string]interface{}{
		"id":          capsule.ID,
		"session_id":  capsule.SessionID,
		"topic_title": capsule.TopicTitle,
		"summary":     capsule.Summary,
		"created_at":  capsule.CreatedAt,
	}
	m.es.Index(ctx, "capsules", capsule.ID, doc)
	
	// 存储向量到 Milvus
	if len(capsule.SummaryVector) > 0 {
		m.milvus.Insert(ctx, "capsule_vectors", capsule.ID, capsule.SummaryVector, map[string]interface{}{
			"session_id": capsule.SessionID,
		})
	}
	
	return nil
}

func (m *MockL2Storage) GetCapsule(ctx context.Context, capsuleID string) (*storage.TopicCapsule, error) {
	capsule, ok := m.capsules[capsuleID]
	if !ok {
		return nil, fmt.Errorf("capsule not found")
	}
	return capsule, nil
}

func (m *MockL2Storage) SearchCapsules(ctx context.Context, query *storage.CapsuleQuery) ([]*storage.TopicCapsule, error) {
	var results []*storage.TopicCapsule
	
	for _, capsule := range m.capsules {
		// 简单过滤
		if query.SessionID != "" && capsule.SessionID != query.SessionID {
			continue
		}
		
		results = append(results, capsule)
		
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}
	
	return results, nil
}

func (m *MockL2Storage) SearchByVector(ctx context.Context, vector []float64, topK int) ([]*storage.TopicCapsule, error) {
	results, err := m.milvus.Search(ctx, "capsule_vectors", vector, topK)
	if err != nil {
		return nil, err
	}
	
	var capsules []*storage.TopicCapsule
	for _, r := range results {
		if capsule, ok := m.capsules[r.ID]; ok {
			capsules = append(capsules, capsule)
		}
	}
	
	return capsules, nil
}

func (m *MockL2Storage) UpdateAccessTime(ctx context.Context, capsuleID string) error {
	if capsule, ok := m.capsules[capsuleID]; ok {
		capsule.LastAccessedAt = time.Now()
		capsule.AccessCount++
	}
	return nil
}

func (m *MockL2Storage) GetExpiredCapsules(ctx context.Context, threshold time.Duration) ([]*storage.TopicCapsule, error) {
	var expired []*storage.TopicCapsule
	cutoff := time.Now().Add(-threshold)
	
	for _, capsule := range m.capsules {
		if capsule.LastAccessedAt.Before(cutoff) {
			expired = append(expired, capsule)
		}
	}
	
	return expired, nil
}

func (m *MockL2Storage) DeleteCapsule(ctx context.Context, capsuleID string) error {
	delete(m.capsules, capsuleID)
	return nil
}

// MockL3Storage Mock L3 存储
type MockL3Storage struct {
	s3     *testutil.MockS3
	neo4j  *testutil.MockNeo4j
	archives map[string]*storage.TopicCapsule
}

func NewMockL3Storage() *MockL3Storage {
	s3 := testutil.NewMockS3()
	s3.CreateBucket(context.Background(), "memory-archives")
	
	return &MockL3Storage{
		s3:       s3,
		neo4j:    testutil.NewMockNeo4j(),
		archives: make(map[string]*storage.TopicCapsule),
	}
}

func (m *MockL3Storage) Tier() storage.StorageTier { return storage.TierL3 }

func (m *MockL3Storage) Store(ctx context.Context, data interface{}) error {
	return nil
}

func (m *MockL3Storage) Get(ctx context.Context, id string) (interface{}, error) {
	return m.archives[id], nil
}

func (m *MockL3Storage) Delete(ctx context.Context, id string) error {
	delete(m.archives, id)
	return nil
}

func (m *MockL3Storage) Close() error {
	m.s3.Close()
	m.neo4j.Close()
	return nil
}

func (m *MockL3Storage) ArchiveCapsule(ctx context.Context, capsule *storage.TopicCapsule) (string, error) {
	archiveID := fmt.Sprintf("archive_%s_%d", capsule.ID, time.Now().Unix())
	m.archives[archiveID] = capsule
	
	// 压缩存储到 S3
	data, _ := json.Marshal(capsule)
	key := fmt.Sprintf("archives/%s/%s.json.gz", capsule.SessionID, archiveID)
	m.s3.PutObjectCompressed(ctx, "memory-archives", key, data)
	
	return archiveID, nil
}

func (m *MockL3Storage) LoadCapsule(ctx context.Context, archiveID string) (*storage.TopicCapsule, error) {
	return m.archives[archiveID], nil
}

func (m *MockL3Storage) StoreEntity(ctx context.Context, entity *storage.Entity) error {
	m.neo4j.CreateNode(ctx, []string{"Entity", entity.Type}, map[string]interface{}{
		"name":      entity.Name,
		"sessionID": entity.SessionID,
	})
	return nil
}

func (m *MockL3Storage) StoreRelation(ctx context.Context, relation *storage.Relation) error {
	// 查找或创建节点
	fromNodes, _ := m.neo4j.FindNodesByProperty(ctx, "name", relation.FromEntity)
	toNodes, _ := m.neo4j.FindNodesByProperty(ctx, "name", relation.ToEntity)
	
	if len(fromNodes) > 0 && len(toNodes) > 0 {
		m.neo4j.CreateRelation(ctx, fromNodes[0].ID, toNodes[0].ID, relation.RelationType, map[string]interface{}{
			"weight": relation.Weight,
		})
	}
	
	return nil
}

func (m *MockL3Storage) QueryEntities(ctx context.Context, query string, limit int) ([]*storage.Entity, error) {
	nodes, err := m.neo4j.FindNodesByLabel(ctx, "Entity")
	if err != nil {
		return nil, err
	}
	
	var entities []*storage.Entity
	for _, node := range nodes {
		entities = append(entities, &storage.Entity{
			ID:   node.ID,
			Name: fmt.Sprintf("%v", node.Properties["name"]),
			Type: "Entity",
		})
		
		if limit > 0 && len(entities) >= limit {
			break
		}
	}
	
	return entities, nil
}

func (m *MockL3Storage) QueryRelations(ctx context.Context, entityName string, depth int) ([]*storage.Relation, error) {
	nodes, _ := m.neo4j.FindNodesByProperty(ctx, "name", entityName)
	if len(nodes) == 0 {
		return nil, nil
	}
	
	rels, err := m.neo4j.GetRelations(ctx, nodes[0].ID, depth)
	if err != nil {
		return nil, err
	}
	
	var relations []*storage.Relation
	for _, rel := range rels {
		relations = append(relations, &storage.Relation{
			ID:           rel.ID,
			FromEntity:   rel.FromNodeID,
			ToEntity:     rel.ToNodeID,
			RelationType: rel.Type,
		})
	}
	
	return relations, nil
}

func (m *MockL3Storage) GetArchiveList(ctx context.Context, sessionID string) ([]*storage.ArchiveInfo, error) {
	var archives []*storage.ArchiveInfo
	
	for id, capsule := range m.archives {
		if capsule.SessionID == sessionID {
			archives = append(archives, &storage.ArchiveInfo{
				ArchiveID:  id,
				CapsuleID:  capsule.ID,
				SessionID:  capsule.SessionID,
				TopicTitle: capsule.TopicTitle,
				ArchivedAt: time.Now(),
			})
		}
	}
	
	return archives, nil
}

// ====================== 集成测试用例 ======================

// TestMemoryTool_Integration_StoreAndRetrieve 测试存储和检索
func TestMemoryTool_Integration_StoreAndRetrieve(t *testing.T) {
	ctx := context.Background()
	
	// 创建 Mock 存储
	l1 := NewMockL1Storage()
	defer l1.Close()
	
	sessionID := "test-session-1"
	
	// 存储消息
	messages := []*storage.Message{
		{ID: "msg-1", SessionID: sessionID, Role: "user", Content: "你好，我想了解 Go 语言", TokenCount: 15, Timestamp: time.Now()},
		{ID: "msg-2", SessionID: sessionID, Role: "assistant", Content: "好的，Go 是一门高效的编程语言", TokenCount: 20, Timestamp: time.Now()},
		{ID: "msg-3", SessionID: sessionID, Role: "user", Content: "Go 有哪些特点？", TokenCount: 10, Timestamp: time.Now()},
	}
	
	for _, msg := range messages {
		if err := l1.AppendMessage(ctx, sessionID, msg); err != nil {
			t.Fatalf("Failed to store message: %v", err)
		}
	}
	
	// 检索消息
	retrieved, err := l1.GetAllMessages(ctx, sessionID)
	if err != nil {
		t.Fatalf("Failed to retrieve messages: %v", err)
	}
	
	if len(retrieved) != len(messages) {
		t.Errorf("Message count mismatch: got %d, want %d", len(retrieved), len(messages))
	}
	
	// 验证 Token 计数
	tokenCount, err := l1.GetTokenCount(ctx, sessionID)
	if err != nil {
		t.Fatalf("Failed to get token count: %v", err)
	}
	
	expectedTokens := 15 + 20 + 10
	if tokenCount != expectedTokens {
		t.Errorf("Token count mismatch: got %d, want %d", tokenCount, expectedTokens)
	}
	
	t.Logf("Successfully stored and retrieved %d messages with %d tokens", len(retrieved), tokenCount)
}

// TestMemoryTool_Integration_L1Overflow 测试 L1 溢出归档
func TestMemoryTool_Integration_L1Overflow(t *testing.T) {
	ctx := context.Background()
	
	l1 := NewMockL1Storage()
	l2 := NewMockL2Storage()
	defer l1.Close()
	defer l2.Close()
	
	sessionID := "test-session-overflow"
	
	// 添加消息直到触发归档
	for i := 0; i < 25; i++ {
		msg := &storage.Message{
			ID:         fmt.Sprintf("msg-%d", i),
			SessionID:  sessionID,
			Role:       "user",
			Content:    fmt.Sprintf("Message content %d with enough text to count", i),
			TokenCount: 200,
			Timestamp:  time.Now(),
		}
		l1.AppendMessage(ctx, sessionID, msg)
	}
	
	// 检查是否应该归档
	shouldArchive, reason, err := l1.ShouldArchive(ctx, sessionID)
	if err != nil {
		t.Fatalf("ShouldArchive failed: %v", err)
	}
	
	if !shouldArchive {
		t.Error("Expected shouldArchive to be true")
	}
	
	t.Logf("Archive triggered: %v, reason: %s", shouldArchive, reason)
	
	// 模拟归档到 L2
	if shouldArchive {
		capsule := &storage.TopicCapsule{
			ID:            fmt.Sprintf("capsule-%s", sessionID),
			SessionID:     sessionID,
			TopicTitle:    "Test Topic",
			Summary:       "This is a test conversation about messages",
			MessageCount:  25,
			TokenCount:    5000,
			CreatedAt:     time.Now(),
			LastAccessedAt: time.Now(),
		}
		
		if err := l2.StoreCapsule(ctx, capsule); err != nil {
			t.Fatalf("Failed to store capsule: %v", err)
		}
		
		// 清除 L1
		if err := l1.ClearSession(ctx, sessionID); err != nil {
			t.Fatalf("Failed to clear session: %v", err)
		}
		
		// 验证清除
		remaining, _ := l1.GetAllMessages(ctx, sessionID)
		if len(remaining) != 0 {
			t.Errorf("Expected 0 messages after clear, got %d", len(remaining))
		}
		
		t.Log("Successfully archived to L2 and cleared L1")
	}
}

// TestMemoryTool_Integration_TopicSwitch 测试主题切换
func TestMemoryTool_Integration_TopicSwitch(t *testing.T) {
	ctx := context.Background()
	
	l1 := NewMockL1Storage()
	l2 := NewMockL2Storage()
	defer l1.Close()
	defer l2.Close()
	
	sessionID := "test-session-topic"
	
	// 第一个主题的消息
	for i := 0; i < 5; i++ {
		l1.AppendMessage(ctx, sessionID, &storage.Message{
			ID:        fmt.Sprintf("topic1-msg-%d", i),
			SessionID: sessionID,
			TopicID:   "topic-1",
			Role:      "user",
			Content:   fmt.Sprintf("Topic 1 message %d", i),
			Timestamp: time.Now(),
		})
	}
	
	// 归档第一个主题
	capsule1 := &storage.TopicCapsule{
		ID:         "capsule-topic-1",
		SessionID:  sessionID,
		TopicTitle: "Topic 1: Introduction",
		Summary:    "Discussion about introduction",
		CreatedAt:  time.Now(),
	}
	l2.StoreCapsule(ctx, capsule1)
	l1.ClearSession(ctx, sessionID)
	
	// 第二个主题的消息
	for i := 0; i < 3; i++ {
		l1.AppendMessage(ctx, sessionID, &storage.Message{
			ID:        fmt.Sprintf("topic2-msg-%d", i),
			SessionID: sessionID,
			TopicID:   "topic-2",
			Role:      "user",
			Content:   fmt.Sprintf("Topic 2 message %d", i),
			Timestamp: time.Now(),
		})
	}
	
	// 验证 L1 中只有新主题的消息
	messages, _ := l1.GetAllMessages(ctx, sessionID)
	if len(messages) != 3 {
		t.Errorf("Expected 3 messages in L1, got %d", len(messages))
	}
	
	// 验证 L2 中有旧主题的胶囊
	capsules, _ := l2.SearchCapsules(ctx, &storage.CapsuleQuery{SessionID: sessionID})
	if len(capsules) != 1 {
		t.Errorf("Expected 1 capsule in L2, got %d", len(capsules))
	}
	
	t.Logf("Topic switch successful: L1 has %d messages, L2 has %d capsules", len(messages), len(capsules))
}

// TestMemoryTool_Integration_EntityRelations 测试实体关系
func TestMemoryTool_Integration_EntityRelations(t *testing.T) {
	ctx := context.Background()
	
	l3 := NewMockL3Storage()
	defer l3.Close()
	
	// 存储实体
	entities := []*storage.Entity{
		{ID: "e1", Name: "Go", Type: "technology", SessionID: "test"},
		{ID: "e2", Name: "Golang", Type: "technology", SessionID: "test"},
		{ID: "e3", Name: "Google", Type: "company", SessionID: "test"},
	}
	
	for _, e := range entities {
		if err := l3.StoreEntity(ctx, e); err != nil {
			t.Fatalf("Failed to store entity: %v", err)
		}
	}
	
	// 存储关系
	relations := []*storage.Relation{
		{FromEntity: "Go", ToEntity: "Golang", RelationType: "alias", Weight: 1.0},
		{FromEntity: "Go", ToEntity: "Google", RelationType: "created_by", Weight: 0.9},
	}
	
	for _, r := range relations {
		if err := l3.StoreRelation(ctx, r); err != nil {
			t.Fatalf("Failed to store relation: %v", err)
		}
	}
	
	// 查询实体
	foundEntities, err := l3.QueryEntities(ctx, "Go", 10)
	if err != nil {
		t.Fatalf("Failed to query entities: %v", err)
	}
	
	t.Logf("Found %d entities", len(foundEntities))
	
	// 导出图数据
	graphJSON, _ := l3.neo4j.ExportToJSON()
	t.Logf("Graph data: %s", graphJSON)
}

// TestMemoryTool_Integration_VectorSearch 测试向量搜索
func TestMemoryTool_Integration_VectorSearch(t *testing.T) {
	ctx := context.Background()
	
	l2 := NewMockL2Storage()
	defer l2.Close()
	
	// 创建带向量的胶囊
	capsules := []*storage.TopicCapsule{
		{
			ID:            "capsule-1",
			SessionID:     "test",
			TopicTitle:    "Go Programming",
			Summary:       "Discussion about Go programming language",
			SummaryVector: make([]float64, 1536), // 简化向量
			CreatedAt:     time.Now(),
		},
		{
			ID:            "capsule-2",
			SessionID:     "test",
			TopicTitle:    "Python Programming",
			Summary:       "Discussion about Python programming language",
			SummaryVector: make([]float64, 1536),
			CreatedAt:     time.Now(),
		},
	}
	
	// 设置不同的向量值来区分
	capsules[0].SummaryVector[0] = 0.9
	capsules[1].SummaryVector[0] = 0.1
	
	for _, c := range capsules {
		if err := l2.StoreCapsule(ctx, c); err != nil {
			t.Fatalf("Failed to store capsule: %v", err)
		}
	}
	
	// 向量搜索
	queryVector := make([]float64, 1536)
	queryVector[0] = 0.85 // 更接近 capsule-1
	
	results, err := l2.SearchByVector(ctx, queryVector, 2)
	if err != nil {
		t.Fatalf("Vector search failed: %v", err)
	}
	
	if len(results) == 0 {
		t.Error("Expected at least 1 result")
	}
	
	t.Logf("Vector search returned %d results", len(results))
	for i, r := range results {
		t.Logf("  %d. %s: %s", i+1, r.ID, r.TopicTitle)
	}
}

// TestMemoryTool_Integration_ArchiveToL3 测试归档到 L3
func TestMemoryTool_Integration_ArchiveToL3(t *testing.T) {
	ctx := context.Background()
	
	l2 := NewMockL2Storage()
	l3 := NewMockL3Storage()
	defer l2.Close()
	defer l3.Close()
	
	sessionID := "test-archive"
	
	// 在 L2 中创建胶囊
	capsule := &storage.TopicCapsule{
		ID:          "capsule-to-archive",
		SessionID:   sessionID,
		TopicTitle:  "Important Discussion",
		Summary:     "This is an important discussion that should be archived",
		CreatedAt:   time.Now().Add(-40 * 24 * time.Hour), // 40天前
		LastAccessedAt: time.Now().Add(-35 * 24 * time.Hour), // 35天未访问
	}
	l2.StoreCapsule(ctx, capsule)
	
	// 归档到 L3
	archiveID, err := l3.ArchiveCapsule(ctx, capsule)
	if err != nil {
		t.Fatalf("Archive failed: %v", err)
	}
	
	// 从 L2 删除
	l2.DeleteCapsule(ctx, capsule.ID)
	
	// 验证 L3 中存在
	loaded, err := l3.LoadCapsule(ctx, archiveID)
	if err != nil {
		t.Fatalf("Failed to load archived capsule: %v", err)
	}
	
	if loaded.TopicTitle != capsule.TopicTitle {
		t.Errorf("Title mismatch: got %s, want %s", loaded.TopicTitle, capsule.TopicTitle)
	}
	
	// 获取归档列表
	archives, err := l3.GetArchiveList(ctx, sessionID)
	if err != nil {
		t.Fatalf("Failed to get archive list: %v", err)
	}
	
	if len(archives) != 1 {
		t.Errorf("Expected 1 archive, got %d", len(archives))
	}
	
	t.Logf("Successfully archived to L3 with ID: %s", archiveID)
}

// ====================== 基准测试 ======================

func BenchmarkMockL1Storage_AppendMessage(b *testing.B) {
	ctx := context.Background()
	l1 := NewMockL1Storage()
	defer l1.Close()
	
	sessionID := "bench-session"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l1.AppendMessage(ctx, sessionID, &storage.Message{
			ID:        fmt.Sprintf("msg-%d", i),
			SessionID: sessionID,
			Role:      "user",
			Content:   "Benchmark message content",
			Timestamp: time.Now(),
		})
	}
}

func BenchmarkMockL2Storage_StoreCapsule(b *testing.B) {
	ctx := context.Background()
	l2 := NewMockL2Storage()
	defer l2.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l2.StoreCapsule(ctx, &storage.TopicCapsule{
			ID:            fmt.Sprintf("capsule-%d", i),
			SessionID:     "bench-session",
			TopicTitle:    "Benchmark Topic",
			Summary:       "Benchmark summary content",
			SummaryVector: make([]float64, 1536),
			CreatedAt:     time.Now(),
		})
	}
}

func BenchmarkMockMilvus_VectorSearch(b *testing.B) {
	ctx := context.Background()
	milvus := testutil.NewMockMilvus()
	milvus.CreateCollection(ctx, "bench", 1536)
	
	// 预填充数据
	for i := 0; i < 1000; i++ {
		vec := make([]float64, 1536)
		vec[i%1536] = float64(i) / 1000.0
		milvus.Insert(ctx, "bench", fmt.Sprintf("vec-%d", i), vec, nil)
	}
	
	queryVec := make([]float64, 1536)
	queryVec[0] = 0.5
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		milvus.Search(ctx, "bench", queryVec, 10)
	}
}
