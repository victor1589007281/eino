# Memory SDK for Python

Python SDK for accessing Memory Service via REST or gRPC.

## Installation

```bash
# Basic installation (REST only)
pip install memory-sdk

# With gRPC support
pip install memory-sdk[grpc]
```

## Quick Start

### REST Client

```python
from memory_sdk import MemoryClient, ClientConfig

# Create client
config = ClientConfig(
    protocol="rest",
    address="http://localhost:8080",
    api_key="your-api-key"
)
client = MemoryClient(config)

# Store a message
response = client.store(
    session_id="session-123",
    message={"role": "user", "content": "Hello, world!"}
)
print(f"Stored message: {response.message_id}")

# Retrieve context
context = client.retrieve(
    session_id="session-123",
    query="What did I say?"
)
for item in context.context:
    print(f"{item.role}: {item.content}")

# Close client
client.close()
```

### gRPC Client

```python
from memory_sdk import MemoryClient, ClientConfig

# Create gRPC client
config = ClientConfig(
    protocol="grpc",
    address="localhost:50051",
    api_key="your-api-key"
)
client = MemoryClient(config)

# Same API as REST client
response = client.store(
    session_id="session-123",
    message={"role": "user", "content": "Hello via gRPC!"}
)

client.close()
```

## API Reference

### Store

```python
response = client.store(
    session_id="session-id",
    message={"role": "user", "content": "message content"},
    source="my-service",
    metadata={"key": "value"},
    options=StoreOptions(
        extract_entities=True,
        generate_embedding=True,
        importance=0.8
    )
)
```

### Retrieve

```python
response = client.retrieve(
    session_id="session-id",
    query="search query",
    options=RetrieveOptions(
        token_budget=4000,
        include_summary=True,
        include_entities=True
    )
)
```

### Batch Operations

```python
# Batch store
response = client.batch_store(
    session_id="session-id",
    messages=[
        {"role": "user", "content": "Message 1"},
        {"role": "assistant", "content": "Response 1"},
    ],
    preserve_order=True
)

# Batch retrieve
response = client.batch_retrieve(
    requests=[
        {"session_id": "session-1", "query": "query 1"},
        {"session_id": "session-2", "query": "query 2"},
    ],
    parallel=True
)
```

### Session Management

```python
# Get session info
session = client.get_session("session-id")

# Delete session
client.delete_session("session-id")

# List topics
topics = client.list_topics("session-id")

# Switch topic
result = client.switch_topic("session-id", "New Topic")

# Recall topic
result = client.recall_topic("session-id", "topic query")
```

### Other Operations

```python
# Get entity relations
relations = client.get_entity_relations("entity-name", depth=2)

# Generate summary
summary = client.summarize("session-id")

# Archive session
result = client.archive("session-id", topic_title="Archive Title")

# Get stats
stats = client.get_stats()

# Health check
health = client.health()
```

## License

Apache License 2.0
