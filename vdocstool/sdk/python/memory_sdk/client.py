"""Memory SDK 客户端实现"""

import json
from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Dict, List, Optional, Union

import requests

try:
    import grpc
    from . import memory_pb2
    from . import memory_pb2_grpc

    GRPC_AVAILABLE = True
except ImportError:
    GRPC_AVAILABLE = False

from .types import (
    ArchiveRequest,
    ArchiveResponse,
    BatchRetrieveRequest,
    BatchRetrieveResponse,
    BatchStoreRequest,
    BatchStoreResponse,
    ContextItem,
    EntityInfo,
    EntityRelationsResponse,
    HealthResponse,
    MessageInput,
    RecallTopicResponse,
    Relation,
    RetrieveOptions,
    RetrieveRequest,
    RetrieveResponse,
    SearchStats,
    SessionInfo,
    StatsResponse,
    StoreOptions,
    StoreRequest,
    StoreResponse,
    SummaryResponse,
    SwitchTopicResponse,
    TopicInfo,
)


@dataclass
class ClientConfig:
    """客户端配置"""

    protocol: str = "rest"  # "rest" or "grpc"
    address: str = "http://localhost:8080"
    api_key: str = ""
    timeout: float = 30.0
    max_retries: int = 3


class MemoryClientBase(ABC):
    """Memory 客户端基类"""

    @abstractmethod
    def store(
        self,
        session_id: str,
        message: Union[MessageInput, Dict],
        source: str = "",
        metadata: Optional[Dict[str, str]] = None,
        options: Optional[StoreOptions] = None,
    ) -> StoreResponse:
        """存储消息"""
        pass

    @abstractmethod
    def retrieve(
        self,
        session_id: str,
        query: str,
        options: Optional[RetrieveOptions] = None,
    ) -> RetrieveResponse:
        """检索上下文"""
        pass

    @abstractmethod
    def batch_store(
        self,
        session_id: str,
        messages: List[Union[MessageInput, Dict]],
        preserve_order: bool = True,
        parallel_processing: bool = False,
    ) -> BatchStoreResponse:
        """批量存储"""
        pass

    @abstractmethod
    def batch_retrieve(
        self, requests: List[Dict], parallel: bool = True, timeout_ms: int = 30000
    ) -> BatchRetrieveResponse:
        """批量检索"""
        pass

    @abstractmethod
    def get_session(self, session_id: str) -> SessionInfo:
        """获取会话信息"""
        pass

    @abstractmethod
    def delete_session(self, session_id: str) -> bool:
        """删除会话"""
        pass

    @abstractmethod
    def switch_topic(self, session_id: str, new_topic: str) -> SwitchTopicResponse:
        """切换主题"""
        pass

    @abstractmethod
    def recall_topic(self, session_id: str, topic_query: str) -> RecallTopicResponse:
        """召回主题"""
        pass

    @abstractmethod
    def list_topics(self, session_id: str) -> List[TopicInfo]:
        """列出主题"""
        pass

    @abstractmethod
    def get_entity_relations(
        self, entity_name: str, depth: int = 2
    ) -> EntityRelationsResponse:
        """获取实体关系"""
        pass

    @abstractmethod
    def summarize(self, session_id: str) -> SummaryResponse:
        """生成摘要"""
        pass

    @abstractmethod
    def archive(
        self, session_id: str, topic_title: str = ""
    ) -> ArchiveResponse:
        """归档会话"""
        pass

    @abstractmethod
    def get_stats(self) -> StatsResponse:
        """获取统计"""
        pass

    @abstractmethod
    def health(self) -> HealthResponse:
        """健康检查"""
        pass

    @abstractmethod
    def close(self):
        """关闭客户端"""
        pass


class RESTClient(MemoryClientBase):
    """REST 客户端实现"""

    def __init__(self, config: ClientConfig):
        self.config = config
        self.base_url = config.address.rstrip("/")
        self.session = requests.Session()
        self.session.timeout = config.timeout
        if config.api_key:
            self.session.headers["X-API-Key"] = config.api_key
        self.session.headers["Content-Type"] = "application/json"

    def _request(
        self,
        method: str,
        path: str,
        data: Optional[Dict] = None,
    ) -> Dict:
        """发送请求"""
        url = f"{self.base_url}{path}"

        for attempt in range(self.config.max_retries):
            try:
                if method == "GET":
                    response = self.session.get(url)
                elif method == "POST":
                    response = self.session.post(url, json=data)
                elif method == "DELETE":
                    response = self.session.delete(url)
                else:
                    raise ValueError(f"Unsupported method: {method}")

                response.raise_for_status()
                return response.json() if response.text else {}
            except requests.RequestException as e:
                if attempt == self.config.max_retries - 1:
                    raise
                continue

    def _to_message_input(self, msg: Union[MessageInput, Dict]) -> MessageInput:
        """转换为 MessageInput"""
        if isinstance(msg, MessageInput):
            return msg
        return MessageInput(
            role=msg.get("role", ""),
            content=msg.get("content", ""),
            topic_id=msg.get("topic_id"),
            timestamp=msg.get("timestamp"),
        )

    def store(
        self,
        session_id: str,
        message: Union[MessageInput, Dict],
        source: str = "",
        metadata: Optional[Dict[str, str]] = None,
        options: Optional[StoreOptions] = None,
    ) -> StoreResponse:
        msg = self._to_message_input(message)
        req = StoreRequest(
            session_id=session_id,
            message=msg,
            source=source,
            metadata=metadata,
            options=options,
        )
        data = self._request("POST", "/api/v1/memory/store", req.to_dict())
        return StoreResponse.from_dict(data)

    def retrieve(
        self,
        session_id: str,
        query: str,
        options: Optional[RetrieveOptions] = None,
    ) -> RetrieveResponse:
        req = RetrieveRequest(session_id=session_id, query=query, options=options)
        data = self._request("POST", "/api/v1/memory/retrieve", req.to_dict())
        return RetrieveResponse.from_dict(data)

    def batch_store(
        self,
        session_id: str,
        messages: List[Union[MessageInput, Dict]],
        preserve_order: bool = True,
        parallel_processing: bool = False,
    ) -> BatchStoreResponse:
        msg_inputs = [self._to_message_input(m) for m in messages]
        req = BatchStoreRequest(
            session_id=session_id,
            messages=msg_inputs,
            preserve_order=preserve_order,
            parallel_processing=parallel_processing,
        )
        data = self._request("POST", "/api/v1/memory/batch/store", req.to_dict())
        return BatchStoreResponse.from_dict(data)

    def batch_retrieve(
        self, requests: List[Dict], parallel: bool = True, timeout_ms: int = 30000
    ) -> BatchRetrieveResponse:
        from .types import SingleRetrieveRequest

        reqs = [
            SingleRetrieveRequest(
                session_id=r.get("session_id", ""),
                query=r.get("query", ""),
            )
            for r in requests
        ]
        req = BatchRetrieveRequest(
            requests=reqs, parallel=parallel, timeout_ms=timeout_ms
        )
        data = self._request("POST", "/api/v1/memory/batch/retrieve", req.to_dict())
        return BatchRetrieveResponse.from_dict(data)

    def get_session(self, session_id: str) -> SessionInfo:
        data = self._request("GET", f"/api/v1/memory/sessions/{session_id}")
        return SessionInfo.from_dict(data)

    def delete_session(self, session_id: str) -> bool:
        self._request("DELETE", f"/api/v1/memory/sessions/{session_id}")
        return True

    def switch_topic(self, session_id: str, new_topic: str) -> SwitchTopicResponse:
        data = self._request(
            "POST",
            f"/api/v1/memory/sessions/{session_id}/topics/switch",
            {"new_topic": new_topic},
        )
        return SwitchTopicResponse.from_dict(data)

    def recall_topic(self, session_id: str, topic_query: str) -> RecallTopicResponse:
        data = self._request(
            "POST",
            f"/api/v1/memory/sessions/{session_id}/topics/recall",
            {"topic_query": topic_query},
        )
        return RecallTopicResponse.from_dict(data)

    def list_topics(self, session_id: str) -> List[TopicInfo]:
        data = self._request("GET", f"/api/v1/memory/sessions/{session_id}/topics")
        return [TopicInfo.from_dict(t) for t in data.get("topics", [])]

    def get_entity_relations(
        self, entity_name: str, depth: int = 2
    ) -> EntityRelationsResponse:
        data = self._request(
            "GET", f"/api/v1/memory/entities/{entity_name}/relations?depth={depth}"
        )
        return EntityRelationsResponse.from_dict(data)

    def summarize(self, session_id: str) -> SummaryResponse:
        data = self._request(
            "POST", f"/api/v1/memory/sessions/{session_id}/summarize"
        )
        return SummaryResponse.from_dict(data)

    def archive(
        self, session_id: str, topic_title: str = ""
    ) -> ArchiveResponse:
        req = ArchiveRequest(session_id=session_id, topic_title=topic_title)
        data = self._request("POST", "/api/v1/memory/archive", req.to_dict())
        return ArchiveResponse.from_dict(data)

    def get_stats(self) -> StatsResponse:
        data = self._request("GET", "/api/v1/memory/stats")
        return StatsResponse.from_dict(data)

    def health(self) -> HealthResponse:
        data = self._request("GET", "/api/v1/memory/health")
        return HealthResponse.from_dict(data)

    def close(self):
        self.session.close()


class GRPCClient(MemoryClientBase):
    """gRPC 客户端实现"""

    def __init__(self, config: ClientConfig):
        if not GRPC_AVAILABLE:
            raise ImportError(
                "gRPC is not available. Install grpcio and grpcio-tools."
            )

        self.config = config
        self.channel = grpc.insecure_channel(config.address)
        self.stub = memory_pb2_grpc.MemoryServiceStub(self.channel)

        # 设置元数据
        self.metadata = []
        if config.api_key:
            self.metadata.append(("x-api-key", config.api_key))

    def _to_message_input(self, msg: Union[MessageInput, Dict]) -> MessageInput:
        """转换为 MessageInput"""
        if isinstance(msg, MessageInput):
            return msg
        return MessageInput(
            role=msg.get("role", ""),
            content=msg.get("content", ""),
            topic_id=msg.get("topic_id"),
            timestamp=msg.get("timestamp"),
        )

    def store(
        self,
        session_id: str,
        message: Union[MessageInput, Dict],
        source: str = "",
        metadata: Optional[Dict[str, str]] = None,
        options: Optional[StoreOptions] = None,
    ) -> StoreResponse:
        msg = self._to_message_input(message)

        pb_msg = memory_pb2.MessageInput(
            role=msg.role,
            content=msg.content,
            topic_id=msg.topic_id or "",
        )
        if msg.timestamp:
            pb_msg.timestamp = int(msg.timestamp.timestamp())

        pb_options = None
        if options:
            pb_options = memory_pb2.StoreOptions(
                extract_entities=options.extract_entities,
                generate_embedding=options.generate_embedding,
                importance=options.importance,
            )

        request = memory_pb2.StoreRequest(
            session_id=session_id,
            source=source,
            message=pb_msg,
            metadata=metadata or {},
            options=pb_options,
        )

        response = self.stub.Store(request, metadata=self.metadata)

        return StoreResponse(
            message_id=response.message_id,
            tier=response.tier,
            token_count=response.token_count,
            entities_extracted=list(response.entities_extracted),
            archive_triggered=response.archive_triggered,
        )

    def retrieve(
        self,
        session_id: str,
        query: str,
        options: Optional[RetrieveOptions] = None,
    ) -> RetrieveResponse:
        pb_options = None
        if options:
            pb_time_range = None
            if options.time_range:
                pb_time_range = memory_pb2.TimeRange(
                    start=int(options.time_range.start.timestamp()),
                    end=int(options.time_range.end.timestamp()),
                )

            pb_options = memory_pb2.RetrieveOptions(
                token_budget=options.token_budget,
                topic_id=options.topic_id or "",
                time_range=pb_time_range,
                filters=options.filters or {},
                include_summary=options.include_summary,
                include_entities=options.include_entities,
            )

        request = memory_pb2.RetrieveRequest(
            session_id=session_id,
            query=query,
            options=pb_options,
        )

        response = self.stub.Retrieve(request, metadata=self.metadata)

        from datetime import datetime

        context = [
            ContextItem(
                message_id=c.message_id,
                role=c.role,
                content=c.content,
                timestamp=datetime.fromtimestamp(c.timestamp),
                relevance_score=c.relevance_score,
                tier=c.tier,
            )
            for c in response.context
        ]

        entities = [
            EntityInfo(name=e.name, type=e.type, mentions=e.mentions)
            for e in response.entities
        ]

        search_stats = None
        if response.HasField("search_stats"):
            search_stats = SearchStats(
                l1_hits=response.search_stats.l1_hits,
                l2_hits=response.search_stats.l2_hits,
                l3_hits=response.search_stats.l3_hits,
                search_latency_ms=response.search_stats.search_latency_ms,
            )

        return RetrieveResponse(
            context=context,
            total_tokens=response.total_tokens,
            summary=response.summary,
            entities=entities,
            search_stats=search_stats,
        )

    def batch_store(
        self,
        session_id: str,
        messages: List[Union[MessageInput, Dict]],
        preserve_order: bool = True,
        parallel_processing: bool = False,
    ) -> BatchStoreResponse:
        pb_messages = []
        for msg in messages:
            m = self._to_message_input(msg)
            pb_msg = memory_pb2.MessageInput(
                role=m.role,
                content=m.content,
                topic_id=m.topic_id or "",
            )
            if m.timestamp:
                pb_msg.timestamp = int(m.timestamp.timestamp())
            pb_messages.append(pb_msg)

        request = memory_pb2.BatchStoreRequest(
            session_id=session_id,
            messages=pb_messages,
            options=memory_pb2.BatchStoreOptions(
                preserve_order=preserve_order,
                parallel_processing=parallel_processing,
            ),
        )

        response = self.stub.BatchStore(request, metadata=self.metadata)

        results = [
            StoreResponse(
                message_id=r.message_id,
                tier=r.tier,
                token_count=r.token_count,
                entities_extracted=list(r.entities_extracted),
                archive_triggered=r.archive_triggered,
            )
            for r in response.results
        ]

        return BatchStoreResponse(
            results=results,
            success_count=response.success_count,
            failed_count=response.failed_count,
            total_tokens=response.total_tokens,
        )

    def batch_retrieve(
        self, requests: List[Dict], parallel: bool = True, timeout_ms: int = 30000
    ) -> BatchRetrieveResponse:
        pb_requests = [
            memory_pb2.SingleRetrieveRequest(
                session_id=r.get("session_id", ""),
                query=r.get("query", ""),
            )
            for r in requests
        ]

        request = memory_pb2.BatchRetrieveRequest(
            requests=pb_requests,
            options=memory_pb2.BatchRetrieveOptions(
                parallel=parallel,
                timeout_ms=timeout_ms,
            ),
        )

        response = self.stub.BatchRetrieve(request, metadata=self.metadata)

        from datetime import datetime

        results = []
        for r in response.results:
            context = [
                ContextItem(
                    message_id=c.message_id,
                    role=c.role,
                    content=c.content,
                    timestamp=datetime.fromtimestamp(c.timestamp),
                    relevance_score=c.relevance_score,
                    tier=c.tier,
                )
                for c in r.context
            ]
            results.append(
                RetrieveResponse(
                    context=context,
                    total_tokens=r.total_tokens,
                    summary=r.summary,
                    entities=[],
                    search_stats=None,
                )
            )

        return BatchRetrieveResponse(results=results)

    def get_session(self, session_id: str) -> SessionInfo:
        from datetime import datetime

        request = memory_pb2.GetSessionRequest(session_id=session_id)
        response = self.stub.GetSession(request, metadata=self.metadata)

        return SessionInfo(
            session_id=response.session_id,
            message_count=response.message_count,
            token_count=response.token_count,
            topic_count=response.topic_count,
            current_topic_id=response.current_topic_id,
            created_at=datetime.fromtimestamp(response.created_at),
            last_active_at=datetime.fromtimestamp(response.last_active_at),
        )

    def delete_session(self, session_id: str) -> bool:
        request = memory_pb2.DeleteSessionRequest(session_id=session_id)
        response = self.stub.DeleteSession(request, metadata=self.metadata)
        return response.deleted

    def switch_topic(self, session_id: str, new_topic: str) -> SwitchTopicResponse:
        request = memory_pb2.SwitchTopicRequest(
            session_id=session_id, new_topic=new_topic
        )
        response = self.stub.SwitchTopic(request, metadata=self.metadata)

        return SwitchTopicResponse(
            capsule_id=response.capsule_id,
            topic_title=response.topic_title,
            message_count=response.message_count,
            token_count=response.token_count,
        )

    def recall_topic(self, session_id: str, topic_query: str) -> RecallTopicResponse:
        request = memory_pb2.RecallTopicRequest(
            session_id=session_id, topic_query=topic_query
        )
        response = self.stub.RecallTopic(request, metadata=self.metadata)

        return RecallTopicResponse(
            capsule_id=response.capsule_id,
            topic_title=response.topic_title,
            loaded_from_l3=response.loaded_from_l3,
            summary=response.summary,
        )

    def list_topics(self, session_id: str) -> List[TopicInfo]:
        from datetime import datetime

        request = memory_pb2.ListTopicsRequest(session_id=session_id)
        response = self.stub.ListTopics(request, metadata=self.metadata)

        return [
            TopicInfo(
                topic_id=t.topic_id,
                title=t.title,
                message_count=t.message_count,
                token_count=t.token_count,
                created_at=datetime.fromtimestamp(t.created_at),
                last_active_at=datetime.fromtimestamp(t.last_active_at),
                tier=t.tier,
            )
            for t in response.topics
        ]

    def get_entity_relations(
        self, entity_name: str, depth: int = 2
    ) -> EntityRelationsResponse:
        request = memory_pb2.GetEntityRelationsRequest(
            entity_name=entity_name, depth=depth
        )
        response = self.stub.GetEntityRelations(request, metadata=self.metadata)

        relations = [
            Relation(
                source=r.source,
                target=r.target,
                type=r.type,
                weight=r.weight,
            )
            for r in response.relations
        ]

        return EntityRelationsResponse(
            entity_name=response.entity_name,
            relations=relations,
            depth=response.depth,
        )

    def summarize(self, session_id: str) -> SummaryResponse:
        request = memory_pb2.SummarizeRequest(session_id=session_id)
        response = self.stub.Summarize(request, metadata=self.metadata)

        return SummaryResponse(
            summary=response.summary,
            message_count=response.message_count,
            token_count=response.token_count,
            key_entities=list(response.key_entities),
        )

    def archive(
        self, session_id: str, topic_title: str = ""
    ) -> ArchiveResponse:
        request = memory_pb2.ArchiveRequest(
            session_id=session_id, topic_title=topic_title
        )
        response = self.stub.Archive(request, metadata=self.metadata)

        return ArchiveResponse(
            capsule_id=response.capsule_id,
            archive_id=response.archive_id,
            archived=response.archived,
        )

    def get_stats(self) -> StatsResponse:
        from .types import TierStats, TotalStats

        request = memory_pb2.GetStatsRequest()
        response = self.stub.GetStats(request, metadata=self.metadata)

        return StatsResponse(
            l1=TierStats(
                session_count=response.l1.session_count,
                message_count=response.l1.message_count,
                token_count=response.l1.token_count,
                size_bytes=response.l1.size_bytes,
            ),
            l2=TierStats(
                session_count=response.l2.session_count,
                message_count=response.l2.message_count,
                token_count=response.l2.token_count,
                size_bytes=response.l2.size_bytes,
            ),
            l3=TierStats(
                session_count=response.l3.session_count,
                message_count=response.l3.message_count,
                token_count=response.l3.token_count,
                size_bytes=response.l3.size_bytes,
            ),
            total=TotalStats(
                total_sessions=response.total.total_sessions,
                total_messages=response.total.total_messages,
                total_tokens=response.total.total_tokens,
                total_size_bytes=response.total.total_size_bytes,
            ),
        )

    def health(self) -> HealthResponse:
        from datetime import datetime

        request = memory_pb2.HealthRequest()
        response = self.stub.Health(request, metadata=self.metadata)

        return HealthResponse(
            status=response.status,
            version=response.version,
            timestamp=datetime.fromtimestamp(response.timestamp),
        )

    def close(self):
        if self.channel:
            self.channel.close()


def MemoryClient(config: Optional[ClientConfig] = None) -> MemoryClientBase:
    """创建 Memory 客户端

    Args:
        config: 客户端配置。如果为 None，使用默认配置。

    Returns:
        Memory 客户端实例
    """
    if config is None:
        config = ClientConfig()

    if config.protocol == "grpc":
        return GRPCClient(config)
    elif config.protocol == "rest":
        return RESTClient(config)
    else:
        raise ValueError(f"Unsupported protocol: {config.protocol}")
