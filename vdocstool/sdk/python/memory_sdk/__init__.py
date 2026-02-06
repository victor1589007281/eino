"""Memory SDK - Python client for Memory Service

支持 gRPC 和 REST 两种协议访问 Memory 服务。

Usage:
    from memory_sdk import MemoryClient, ClientConfig

    # 使用 REST 协议
    config = ClientConfig(protocol="rest", address="http://localhost:8080")
    client = MemoryClient(config)

    # 存储消息
    response = client.store(
        session_id="session-123",
        message={"role": "user", "content": "Hello, world!"}
    )

    # 检索上下文
    context = client.retrieve(session_id="session-123", query="What did I say?")
"""

from .client import MemoryClient, ClientConfig
from .types import (
    StoreRequest,
    StoreResponse,
    RetrieveRequest,
    RetrieveResponse,
    MessageInput,
    ContextItem,
    SessionInfo,
    TopicInfo,
    EntityInfo,
    Relation,
    StatsResponse,
)

__version__ = "1.0.0"
__all__ = [
    "MemoryClient",
    "ClientConfig",
    "StoreRequest",
    "StoreResponse",
    "RetrieveRequest",
    "RetrieveResponse",
    "MessageInput",
    "ContextItem",
    "SessionInfo",
    "TopicInfo",
    "EntityInfo",
    "Relation",
    "StatsResponse",
]
