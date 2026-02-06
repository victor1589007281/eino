"""Memory SDK 数据类型定义"""

from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional


@dataclass
class MessageInput:
    """消息输入"""
    role: str
    content: str
    topic_id: Optional[str] = None
    timestamp: Optional[datetime] = None

    def to_dict(self) -> dict:
        data = {"role": self.role, "content": self.content}
        if self.topic_id:
            data["topic_id"] = self.topic_id
        if self.timestamp:
            data["timestamp"] = int(self.timestamp.timestamp())
        return data

    @classmethod
    def from_dict(cls, data: dict) -> "MessageInput":
        timestamp = None
        if data.get("timestamp"):
            timestamp = datetime.fromtimestamp(data["timestamp"])
        return cls(
            role=data.get("role", ""),
            content=data.get("content", ""),
            topic_id=data.get("topic_id"),
            timestamp=timestamp,
        )


@dataclass
class StoreOptions:
    """存储选项"""
    extract_entities: bool = False
    generate_embedding: bool = True
    importance: float = 0.5

    def to_dict(self) -> dict:
        return {
            "extract_entities": self.extract_entities,
            "generate_embedding": self.generate_embedding,
            "importance": self.importance,
        }


@dataclass
class StoreRequest:
    """存储请求"""
    session_id: str
    message: MessageInput
    source: str = ""
    metadata: Optional[Dict[str, str]] = None
    options: Optional[StoreOptions] = None

    def to_dict(self) -> dict:
        data = {
            "session_id": self.session_id,
            "message": self.message.to_dict(),
        }
        if self.source:
            data["source"] = self.source
        if self.metadata:
            data["metadata"] = self.metadata
        if self.options:
            data["options"] = self.options.to_dict()
        return data


@dataclass
class StoreResponse:
    """存储响应"""
    message_id: str
    tier: str
    token_count: int
    entities_extracted: List[str] = field(default_factory=list)
    archive_triggered: bool = False

    @classmethod
    def from_dict(cls, data: dict) -> "StoreResponse":
        return cls(
            message_id=data.get("message_id", ""),
            tier=data.get("tier", ""),
            token_count=data.get("token_count", 0),
            entities_extracted=data.get("entities_extracted", []),
            archive_triggered=data.get("archive_triggered", False),
        )


@dataclass
class TimeRange:
    """时间范围"""
    start: datetime
    end: datetime

    def to_dict(self) -> dict:
        return {
            "start": int(self.start.timestamp()),
            "end": int(self.end.timestamp()),
        }


@dataclass
class RetrieveOptions:
    """检索选项"""
    token_budget: int = 4000
    topic_id: Optional[str] = None
    time_range: Optional[TimeRange] = None
    filters: Optional[Dict[str, str]] = None
    include_summary: bool = False
    include_entities: bool = False

    def to_dict(self) -> dict:
        data = {
            "token_budget": self.token_budget,
            "include_summary": self.include_summary,
            "include_entities": self.include_entities,
        }
        if self.topic_id:
            data["topic_id"] = self.topic_id
        if self.time_range:
            data["time_range"] = self.time_range.to_dict()
        if self.filters:
            data["filters"] = self.filters
        return data


@dataclass
class RetrieveRequest:
    """检索请求"""
    session_id: str
    query: str
    options: Optional[RetrieveOptions] = None

    def to_dict(self) -> dict:
        data = {
            "session_id": self.session_id,
            "query": self.query,
        }
        if self.options:
            data["options"] = self.options.to_dict()
        return data


@dataclass
class ContextItem:
    """上下文项"""
    message_id: str
    role: str
    content: str
    timestamp: datetime
    relevance_score: float
    tier: str

    @classmethod
    def from_dict(cls, data: dict) -> "ContextItem":
        return cls(
            message_id=data.get("message_id", ""),
            role=data.get("role", ""),
            content=data.get("content", ""),
            timestamp=datetime.fromtimestamp(data.get("timestamp", 0)),
            relevance_score=data.get("relevance_score", 0.0),
            tier=data.get("tier", ""),
        )


@dataclass
class EntityInfo:
    """实体信息"""
    name: str
    type: str
    mentions: int

    @classmethod
    def from_dict(cls, data: dict) -> "EntityInfo":
        return cls(
            name=data.get("name", ""),
            type=data.get("type", ""),
            mentions=data.get("mentions", 0),
        )


@dataclass
class SearchStats:
    """搜索统计"""
    l1_hits: int
    l2_hits: int
    l3_hits: int
    search_latency_ms: int

    @classmethod
    def from_dict(cls, data: dict) -> "SearchStats":
        return cls(
            l1_hits=data.get("l1_hits", 0),
            l2_hits=data.get("l2_hits", 0),
            l3_hits=data.get("l3_hits", 0),
            search_latency_ms=data.get("search_latency_ms", 0),
        )


@dataclass
class RetrieveResponse:
    """检索响应"""
    context: List[ContextItem]
    total_tokens: int
    summary: str = ""
    entities: List[EntityInfo] = field(default_factory=list)
    search_stats: Optional[SearchStats] = None

    @classmethod
    def from_dict(cls, data: dict) -> "RetrieveResponse":
        context = [ContextItem.from_dict(c) for c in data.get("context", [])]
        entities = [EntityInfo.from_dict(e) for e in data.get("entities", [])]
        search_stats = None
        if data.get("search_stats"):
            search_stats = SearchStats.from_dict(data["search_stats"])
        return cls(
            context=context,
            total_tokens=data.get("total_tokens", 0),
            summary=data.get("summary", ""),
            entities=entities,
            search_stats=search_stats,
        )


@dataclass
class SessionInfo:
    """会话信息"""
    session_id: str
    message_count: int
    token_count: int
    topic_count: int
    current_topic_id: str
    created_at: datetime
    last_active_at: datetime

    @classmethod
    def from_dict(cls, data: dict) -> "SessionInfo":
        return cls(
            session_id=data.get("session_id", ""),
            message_count=data.get("message_count", 0),
            token_count=data.get("token_count", 0),
            topic_count=data.get("topic_count", 0),
            current_topic_id=data.get("current_topic_id", ""),
            created_at=datetime.fromtimestamp(data.get("created_at", 0)),
            last_active_at=datetime.fromtimestamp(data.get("last_active_at", 0)),
        )


@dataclass
class TopicInfo:
    """主题信息"""
    topic_id: str
    title: str
    message_count: int
    token_count: int
    created_at: datetime
    last_active_at: datetime
    tier: str

    @classmethod
    def from_dict(cls, data: dict) -> "TopicInfo":
        return cls(
            topic_id=data.get("topic_id", ""),
            title=data.get("title", ""),
            message_count=data.get("message_count", 0),
            token_count=data.get("token_count", 0),
            created_at=datetime.fromtimestamp(data.get("created_at", 0)),
            last_active_at=datetime.fromtimestamp(data.get("last_active_at", 0)),
            tier=data.get("tier", ""),
        )


@dataclass
class Relation:
    """关系"""
    source: str
    target: str
    type: str
    weight: float

    @classmethod
    def from_dict(cls, data: dict) -> "Relation":
        return cls(
            source=data.get("source", ""),
            target=data.get("target", ""),
            type=data.get("type", ""),
            weight=data.get("weight", 0.0),
        )


@dataclass
class EntityRelationsResponse:
    """实体关系响应"""
    entity_name: str
    relations: List[Relation]
    depth: int

    @classmethod
    def from_dict(cls, data: dict) -> "EntityRelationsResponse":
        relations = [Relation.from_dict(r) for r in data.get("relations", [])]
        return cls(
            entity_name=data.get("entity_name", ""),
            relations=relations,
            depth=data.get("depth", 0),
        )


@dataclass
class SummaryResponse:
    """摘要响应"""
    summary: str
    message_count: int
    token_count: int
    key_entities: List[str]

    @classmethod
    def from_dict(cls, data: dict) -> "SummaryResponse":
        return cls(
            summary=data.get("summary", ""),
            message_count=data.get("message_count", 0),
            token_count=data.get("token_count", 0),
            key_entities=data.get("key_entities", []),
        )


@dataclass
class ArchiveRequest:
    """归档请求"""
    session_id: str
    topic_title: str = ""

    def to_dict(self) -> dict:
        data = {"session_id": self.session_id}
        if self.topic_title:
            data["topic_title"] = self.topic_title
        return data


@dataclass
class ArchiveResponse:
    """归档响应"""
    capsule_id: str
    archive_id: str
    archived: bool

    @classmethod
    def from_dict(cls, data: dict) -> "ArchiveResponse":
        return cls(
            capsule_id=data.get("capsule_id", ""),
            archive_id=data.get("archive_id", ""),
            archived=data.get("archived", False),
        )


@dataclass
class TierStats:
    """层级统计"""
    session_count: int
    message_count: int
    token_count: int
    size_bytes: int

    @classmethod
    def from_dict(cls, data: dict) -> "TierStats":
        return cls(
            session_count=data.get("session_count", 0),
            message_count=data.get("message_count", 0),
            token_count=data.get("token_count", 0),
            size_bytes=data.get("size_bytes", 0),
        )


@dataclass
class TotalStats:
    """总计统计"""
    total_sessions: int
    total_messages: int
    total_tokens: int
    total_size_bytes: int

    @classmethod
    def from_dict(cls, data: dict) -> "TotalStats":
        return cls(
            total_sessions=data.get("total_sessions", 0),
            total_messages=data.get("total_messages", 0),
            total_tokens=data.get("total_tokens", 0),
            total_size_bytes=data.get("total_size_bytes", 0),
        )


@dataclass
class StatsResponse:
    """统计响应"""
    l1: TierStats
    l2: TierStats
    l3: TierStats
    total: TotalStats

    @classmethod
    def from_dict(cls, data: dict) -> "StatsResponse":
        return cls(
            l1=TierStats.from_dict(data.get("l1", {})),
            l2=TierStats.from_dict(data.get("l2", {})),
            l3=TierStats.from_dict(data.get("l3", {})),
            total=TotalStats.from_dict(data.get("total", {})),
        )


@dataclass
class HealthResponse:
    """健康检查响应"""
    status: str
    version: str
    timestamp: datetime

    @classmethod
    def from_dict(cls, data: dict) -> "HealthResponse":
        return cls(
            status=data.get("status", ""),
            version=data.get("version", ""),
            timestamp=datetime.fromtimestamp(data.get("timestamp", 0)),
        )


@dataclass
class BatchStoreRequest:
    """批量存储请求"""
    session_id: str
    messages: List[MessageInput]
    preserve_order: bool = True
    parallel_processing: bool = False

    def to_dict(self) -> dict:
        return {
            "session_id": self.session_id,
            "messages": [m.to_dict() for m in self.messages],
            "options": {
                "preserve_order": self.preserve_order,
                "parallel_processing": self.parallel_processing,
            },
        }


@dataclass
class BatchStoreResponse:
    """批量存储响应"""
    results: List[StoreResponse]
    success_count: int
    failed_count: int
    total_tokens: int

    @classmethod
    def from_dict(cls, data: dict) -> "BatchStoreResponse":
        results = [StoreResponse.from_dict(r) for r in data.get("results", [])]
        return cls(
            results=results,
            success_count=data.get("success_count", 0),
            failed_count=data.get("failed_count", 0),
            total_tokens=data.get("total_tokens", 0),
        )


@dataclass
class SingleRetrieveRequest:
    """单个检索请求"""
    session_id: str
    query: str

    def to_dict(self) -> dict:
        return {
            "session_id": self.session_id,
            "query": self.query,
        }


@dataclass
class BatchRetrieveRequest:
    """批量检索请求"""
    requests: List[SingleRetrieveRequest]
    parallel: bool = True
    timeout_ms: int = 30000

    def to_dict(self) -> dict:
        return {
            "requests": [r.to_dict() for r in self.requests],
            "options": {
                "parallel": self.parallel,
                "timeout_ms": self.timeout_ms,
            },
        }


@dataclass
class BatchRetrieveResponse:
    """批量检索响应"""
    results: List[RetrieveResponse]

    @classmethod
    def from_dict(cls, data: dict) -> "BatchRetrieveResponse":
        results = [RetrieveResponse.from_dict(r) for r in data.get("results", [])]
        return cls(results=results)


@dataclass
class SwitchTopicResponse:
    """切换主题响应"""
    capsule_id: str
    topic_title: str
    message_count: int
    token_count: int

    @classmethod
    def from_dict(cls, data: dict) -> "SwitchTopicResponse":
        return cls(
            capsule_id=data.get("capsule_id", ""),
            topic_title=data.get("topic_title", ""),
            message_count=data.get("message_count", 0),
            token_count=data.get("token_count", 0),
        )


@dataclass
class RecallTopicResponse:
    """召回主题响应"""
    capsule_id: str
    topic_title: str
    loaded_from_l3: bool
    summary: str

    @classmethod
    def from_dict(cls, data: dict) -> "RecallTopicResponse":
        return cls(
            capsule_id=data.get("capsule_id", ""),
            topic_title=data.get("topic_title", ""),
            loaded_from_l3=data.get("loaded_from_l3", False),
            summary=data.get("summary", ""),
        )
