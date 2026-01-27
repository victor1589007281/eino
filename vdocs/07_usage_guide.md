# Eino 使用指南

## 1. 快速开始

### 1.1 安装

```bash
# 安装核心库
go get github.com/cloudwego/eino

# 安装扩展库（包含各种组件实现）
go get github.com/cloudwego/eino-ext/...
```

### 1.2 最小示例

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/cloudwego/eino-ext/components/model/openai"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()
    
    // 创建 ChatModel
    model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        Model:  "gpt-4o",
        APIKey: "your-api-key",
    })
    if err != nil {
        panic(err)
    }
    
    // 调用模型
    response, err := model.Generate(ctx, []*schema.Message{
        schema.SystemMessage("You are a helpful assistant."),
        schema.UserMessage("Hello!"),
    })
    if err != nil {
        panic(err)
    }
    
    fmt.Println(response.Content)
}
```

## 2. 使用 Chain 编排

### 2.1 简单 Chain

```go
package main

import (
    "context"
    
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()
    
    // 创建组件
    chatTemplate, _ := prompt.FromMessages(schema.FString,
        schema.SystemMessage("You are a {role}."),
        schema.UserMessage("{query}"),
    )
    
    chatModel, _ := openai.NewChatModel(ctx, config)
    
    // 创建 Chain
    chain, err := compose.NewChain[map[string]any, *schema.Message]().
        AppendChatTemplate(chatTemplate).
        AppendChatModel(chatModel).
        Compile(ctx)
    
    if err != nil {
        panic(err)
    }
    
    // 执行
    result, err := chain.Invoke(ctx, map[string]any{
        "role":  "helpful assistant",
        "query": "What is Eino?",
    })
    
    fmt.Println(result.Content)
}
```

### 2.2 带 Lambda 的 Chain

```go
// 后处理 Lambda
postProcess := compose.InvokableLambda(func(ctx context.Context, msg *schema.Message) (string, error) {
    return fmt.Sprintf("AI: %s", msg.Content), nil
})

chain, _ := compose.NewChain[map[string]any, string]().
    AppendChatTemplate(chatTemplate).
    AppendChatModel(chatModel).
    AppendLambda(postProcess).
    Compile(ctx)
```

### 2.3 并行 Chain

```go
// 创建并行节点
parallel := compose.NewParallel()
parallel.AddChatModel("openai", openaiModel)
parallel.AddChatModel("claude", claudeModel)

chain, _ := compose.NewChain[map[string]any, map[string]any]().
    AppendChatTemplate(chatTemplate).
    AppendParallel(parallel).
    Compile(ctx)

// 结果是 map[string]any{"openai": msg1, "claude": msg2}
result, _ := chain.Invoke(ctx, input)
```

## 3. 使用 Graph 编排

### 3.1 工具调用 Graph

```go
package main

import (
    "context"
    
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()
    
    // 创建组件
    chatModel, _ := openai.NewChatModel(ctx, config)
    toolsNode, _ := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
        Tools: []tool.BaseTool{weatherTool, calculatorTool},
    })
    
    // 创建 Graph
    graph := compose.NewGraph[map[string]any, *schema.Message]()
    
    // 添加节点
    graph.AddChatTemplateNode("template", chatTemplate)
    graph.AddChatModelNode("model", chatModel)
    graph.AddToolsNode("tools", toolsNode)
    
    // 添加边
    graph.AddEdge(compose.START, "template")
    graph.AddEdge("template", "model")
    
    // 添加条件分支
    condition := func(ctx context.Context, msg *schema.Message) (string, error) {
        if len(msg.ToolCalls) > 0 {
            return "tools", nil
        }
        return compose.END, nil
    }
    branch := compose.NewGraphBranch(condition, map[string]bool{
        "tools":      true,
        compose.END: true,
    })
    graph.AddBranch("model", branch)
    
    // 工具结果返回模型
    graph.AddEdge("tools", "model")
    
    // 编译
    runnable, err := graph.Compile(ctx)
    if err != nil {
        panic(err)
    }
    
    // 执行
    result, _ := runnable.Invoke(ctx, map[string]any{
        "query": "What's the weather in Beijing?",
    })
}
```

### 3.2 带状态的 Graph

```go
// 定义状态
type ChatState struct {
    History []*schema.Message
    Context map[string]any
}

// 创建带状态的 Graph
graph := compose.NewGraph[*schema.Message, *schema.Message](
    compose.WithGenLocalState(func(ctx context.Context) *ChatState {
        return &ChatState{
            History: make([]*schema.Message, 0),
            Context: make(map[string]any),
        }
    }),
)

// 状态处理器
preHandler := func(ctx context.Context, input *schema.Message, state *ChatState) ([]*schema.Message, error) {
    // 将输入添加到历史
    state.History = append(state.History, input)
    return state.History, nil
}

postHandler := func(ctx context.Context, output *schema.Message, state *ChatState) (*schema.Message, error) {
    // 将输出添加到历史
    state.History = append(state.History, output)
    return output, nil
}

graph.AddChatModelNode("model", chatModel,
    compose.WithStatePreHandler(preHandler),
    compose.WithStatePostHandler(postHandler),
)
```

## 4. 使用 Workflow 编排

### 4.1 字段映射

```go
// 创建 Workflow
wf := compose.NewWorkflow[[]*schema.Message, *schema.Message]()

// 添加节点
wf.AddChatModelNode("model", chatModel).AddInput(compose.START)

// 字段映射：从 model 的 Content 映射到 lambda1 的 Input
wf.AddLambdaNode("lambda1", lambda1).
    AddInput("model", compose.MapFields("Content", "Input"))

// 从 model 的 Role 映射到 lambda2 的 Role
wf.AddLambdaNode("lambda2", lambda2).
    AddInput("model", compose.MapFields("Role", "Role"))

// 多输入映射
wf.AddLambdaNode("lambda3", lambda3).
    AddInput("lambda1", compose.MapFields("Output", "Query")).
    AddInput("lambda2", compose.MapFields("Output", "MetaData"))

// 输出
wf.End().AddInput("lambda3")

// 编译
runnable, _ := wf.Compile(ctx)
```

## 5. 使用 ReAct Agent

### 5.1 基本用法

```go
package main

import (
    "context"
    
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/flow/agent/react"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()
    
    // 创建工具
    tools := []tool.BaseTool{
        &SearchTool{},
        &CalculatorTool{},
    }
    
    // 创建 Agent
    agent, err := react.NewAgent(ctx, &react.AgentConfig{
        ToolCallingModel: chatModel,
        ToolsConfig: compose.ToolsNodeConfig{
            Tools: tools,
        },
        MessageModifier: react.NewPersonaModifier("You are a helpful assistant."),
        MaxStep: 20,
    })
    
    // 调用
    response, _ := agent.Generate(ctx, []*schema.Message{
        schema.UserMessage("Search for the latest news about AI"),
    })
    
    fmt.Println(response.Content)
}
```

### 5.2 流式输出

```go
// 流式调用
stream, err := agent.Stream(ctx, messages)
if err != nil {
    return err
}
defer stream.Close()

for {
    chunk, err := stream.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    fmt.Print(chunk.Content)
}
```

## 6. 使用回调

### 6.1 日志回调

```go
// 创建日志回调
loggingHandler := callbacks.NewHandlerBuilder().
    OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
        log.Printf("[%s] Start", info.Name)
        return ctx
    }).
    OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
        log.Printf("[%s] End", info.Name)
        return ctx
    }).
    Build()

// 使用回调
result, _ := runnable.Invoke(ctx, input, compose.WithCallbacks(loggingHandler))
```

### 6.2 全局回调

```go
// 初始化时设置全局回调
func init() {
    callbacks.AppendGlobalHandlers(loggingHandler, tracingHandler)
}
```

## 7. 流处理

### 7.1 流式调用

```go
// 流式调用
stream, err := runnable.Stream(ctx, input)
if err != nil {
    return err
}
defer stream.Close()

for {
    chunk, err := stream.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        return err
    }
    process(chunk)
}
```

### 7.2 流转换

```go
// 流类型转换
stringStream := schema.StreamReaderWithConvert(intStream, func(i int) (string, error) {
    return fmt.Sprintf("%d", i), nil
})

// 流合并
merged := schema.MergeStreamReaders(streams)

// 流复制
copies := stream.Copy(2)
```

## 8. 检索增强生成 (RAG)

### 8.1 基本 RAG

```go
// 创建组件
retriever, _ := vectorstore.NewRetriever(ctx, config)
chatModel, _ := openai.NewChatModel(ctx, modelConfig)

// 创建 RAG Chain
ragChain, _ := compose.NewChain[string, *schema.Message]().
    AppendLambda(compose.InvokableLambda(func(ctx context.Context, query string) ([]*schema.Document, error) {
        return retriever.Retrieve(ctx, query, retriever.WithTopK(5))
    })).
    AppendLambda(compose.InvokableLambda(func(ctx context.Context, docs []*schema.Document) (map[string]any, error) {
        context := formatDocs(docs)
        return map[string]any{"context": context, "query": query}, nil
    })).
    AppendChatTemplate(ragTemplate).
    AppendChatModel(chatModel).
    Compile(ctx)

// 使用
result, _ := ragChain.Invoke(ctx, "What is Eino?")
```

## 9. 文档处理

### 9.1 文档加载和索引

```go
// 加载文档
loader, _ := file.NewLoader(ctx, &file.LoaderConfig{
    Path: "documents/",
})
docs, _ := loader.Load(ctx)

// 分割文档
splitter, _ := text.NewSplitter(ctx, &text.SplitterConfig{
    ChunkSize:    1000,
    ChunkOverlap: 200,
})
chunks, _ := splitter.Transform(ctx, docs)

// 创建嵌入
embedder, _ := openai.NewEmbedder(ctx, &openai.EmbedderConfig{})

// 存储到向量数据库
indexer, _ := vectorstore.NewIndexer(ctx, config)
ids, _ := indexer.Store(ctx, chunks)
```

## 10. 错误处理

### 10.1 编译时错误

```go
runnable, err := graph.Compile(ctx)
if err != nil {
    // 类型不匹配、循环检测等
    switch {
    case errors.Is(err, compose.DAGInvalidLoopErr):
        log.Fatal("Graph contains cycle")
    case errors.Is(err, compose.ErrGraphCompiled):
        log.Fatal("Graph already compiled")
    default:
        log.Fatalf("Compile error: %v", err)
    }
}
```

### 10.2 运行时错误

```go
result, err := runnable.Invoke(ctx, input)
if err != nil {
    // 检查是否是 Graph 运行错误
    var graphErr *compose.GraphRunError
    if errors.As(err, &graphErr) {
        log.Printf("Graph run error: %v", graphErr)
    }
    
    // 检查是否是中断
    if info := compose.IsInterruptError(err); info != nil {
        log.Printf("Interrupted at: %v", info.BeforeNodes)
    }
}
```

## 11. 性能优化

### 11.1 并发执行

```go
// Graph 中的独立节点会自动并行执行
graph.AddEdge(compose.START, "node1")
graph.AddEdge(compose.START, "node2")  // node1 和 node2 并行
graph.AddEdge("node1", "merge")
graph.AddEdge("node2", "merge")        // merge 等待两者完成
```

### 11.2 流式处理

```go
// 使用流式可以降低延迟
stream, _ := runnable.Stream(ctx, input)
// 边生成边处理
```

### 11.3 选择正确的编排方式

| 场景 | 推荐 |
|------|------|
| 简单线性流程 | Chain |
| 复杂业务逻辑 | Graph |
| 需要字段映射 | Workflow |
| Agent 应用 | ReAct Agent |

## 12. 调试技巧

### 12.1 添加调试回调

```go
debugHandler := callbacks.NewHandlerBuilder().
    OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
        fmt.Printf(">>> [%s] Input: %+v\n", info.Name, input)
        return ctx
    }).
    OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
        fmt.Printf("<<< [%s] Output: %+v\n", info.Name, output)
        return ctx
    }).
    Build()
```

### 12.2 查看图结构

```go
// 编译时添加回调查看图信息
runnable, _ := graph.Compile(ctx,
    compose.WithGraphCompileCallbacks(&GraphInfoPrinter{}),
)
```

## 13. 常见问题

### Q: 如何处理超时？

```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

result, err := runnable.Invoke(ctx, input)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Request timed out")
}
```

### Q: 如何实现 Human-in-the-loop？

```go
// 使用中断功能
runnable, _ := graph.Compile(ctx,
    compose.WithInterruptBefore("confirm_node"),
)

// 执行到中断点会返回 InterruptError
result, err := runnable.Invoke(ctx, input, compose.WithCheckpointID("session-1"))
if info := compose.IsInterruptError(err); info != nil {
    // 等待人工确认后恢复
    result, _ = runnable.Invoke(ctx, input,
        compose.WithCheckpointID("session-1"),
        compose.WithResumeContext(humanInput),
    )
}
```

### Q: 如何自定义组件？

```go
// 实现接口即可
type MyRetriever struct{}

func (r *MyRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
    // 自定义检索逻辑
    return docs, nil
}
```

---

## 参考资源

- **官方文档**: [https://www.cloudwego.io/zh/docs/eino/](https://www.cloudwego.io/zh/docs/eino/)
- **示例代码**: [https://github.com/cloudwego/eino-examples](https://github.com/cloudwego/eino-examples)
- **扩展组件**: [https://github.com/cloudwego/eino-ext](https://github.com/cloudwego/eino-ext)
- **问题反馈**: [https://github.com/cloudwego/eino/issues](https://github.com/cloudwego/eino/issues)
