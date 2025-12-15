# 前端魔法：打造沉浸式 Deep Research 体验

> [!TIP]
> 一个优秀的 Agent 不仅要有强大的大脑，还要有优雅的交互。`agent-web` 的前端展示了如何将复杂的后台思考过程可视化。

## 1. 视觉冲击力

打开 `agent-web`，你首先感受到的是一种**现代科技感**。
- **深色模式**: 默认采用深邃的黑色背景，配以霓虹蓝/紫的强调色，营造出专注、极客的氛围。
- **流式响应**: 所有的文字输出都是打字机效果，仿佛 AI 正在你面前实时思考和撰写。这不是简单的 CSS 动画，而是基于 SSE (Server-Sent Events) 的真实数据流。

## 2. 实时通信的核心：SSE

不同于传统的 WebSocket，`agent-web` 选择了更轻量级的 SSE (Server-Sent Events) 来实现服务器向客户端的单向推送。

### 为什么选择 SSE？
- **单向数据流**：Agent 的工作流主要是“服务器思考 -> 推送日志/结果 -> 客户端展示”，非常契合 SSE 的单向特性。
- **HTTP 兼容性**：SSE 本质上是一个长连接的 HTTP 请求，无需像 WebSocket 那样进行协议升级，对防火墙和代理更友好。
- **自动重连**：浏览器的 `EventSource` API 内置了断线自动重连机制。当连接意外中断时，浏览器会自动尝试恢复连接，无需开发者手动编写复杂的重连逻辑。

### 后端实现 (`cmd/agent-web/main.go`)

在 Go 中实现 SSE 非常简单，核心是设置正确的 Content-Type 并禁用缓存：

```go
// 1. 设置 SSE 标头
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")
w.Header().Set("X-Accel-Buffering", "no") // 禁用 Nginx 缓冲

flusher, ok := w.(http.Flusher)
if !ok {
    http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
    return
}

// 2. 监听事件通道并推送
handler := session.Handler
for {
    select {
    case event := <-handler.eventChan:
        // 序列化事件数据
        data, err := json.Marshal(event)
        if err != nil {
            continue
        }
        // 写入 SSE 格式数据: "data: <json>\n\n"
        fmt.Fprintf(w, "data: %s\n\n", data)
        flusher.Flush() // 立即发送到客户端
    case <-r.Context().Done():
        return
    }
}
```

### 前端处理 (`cmd/agent-web/ui/app.js`)

前端使用 `EventSource` 监听事件流，并通过 `handleEvent` 函数根据 `event.type` 进行分发处理：

```javascript
function connectSSE() {
    // ... 关闭旧连接 ...
    eventSource = new EventSource(`/events?session_id=${sessionId}`);

    eventSource.onmessage = (event) => {
        const data = JSON.parse(event.data);
        handleEvent(data);
    };
}

function handleEvent(data) {
    switch (data.type) {
        case 'log':
            // 解析日志内容，更新左侧的任务状态图标（进行中/完成/失败）
            // 并将日志追加到右侧的终端窗口
            handleLog(data.content);
            break;
            
        case 'plan_review':
            // 收到规划事件，弹出模态框展示 JSON 计划
            // 用户可以在此修改计划或直接批准
            showPlanReview(data.plan);
            break;
            
        case 'response':
            // 收到最终报告，创建一个新的标签页展示 Markdown 内容
            const tabId = createReportTab(data.content);
            activateTab(tabId);
            
            // 如果包含多模态内容，自动渲染对应的按钮
            if (data.ppt) renderPPTButton(data.ppt);
            if (data.podcast) renderPodcastButton(data.podcast);
            break;
            
        case 'done':
            addLog('success', '任务已完成！');
            setLoading(false);
            break;
    }
}
```

这种事件驱动的架构使得前端非常轻量，只需专注于数据的展示，而复杂的逻辑状态都由后端维护。

## 3. 杀手级功能：Session Replay (会话回放)

`agent-web` 最令人震撼的功能之一是**会话回放**。它不仅仅是保存了聊天记录，而是完整记录了 Agent 的每一次思考、每一个步骤、每一条日志。

> 像目前真正的产品级的AI 应用，比如 manus、gospark 等，都提供了回放的功能。字节的deerflow 也提供了回放的功能。
> 回放功能的意义在于，用户可以回看 Agent 的思考过程，理解 Agent 的决策逻辑，查看deep research 的报告效果，这对于调试和优化 Agent 非常有帮助。

### 原理ß
1.  **录制**: 在会话过程中，`WebInteractionHandler` 会将所有发生的 `Event` 保存在内存中。
2.  **存储**: 会话结束时，这些事件序列被序列化为 JSON 文件存储在 `sessions/` 目录下。
3.  **回放**: 当用户点击历史会话时，前端加载对应的 JSON 文件。
4.  **模拟**: 前端并不一次性渲染所有内容，而是通过 `setTimeout` 模拟事件的时间间隔，**重演**当时的思考过程。用户可以看到日志一行行跳出，就像 Agent 正在重新执行任务一样。

```javascript
// 前端回放逻辑伪代码
events.forEach((event, index) => {
    setTimeout(() => {
        handleEvent(event); // 复用实时的事件处理逻辑
    }, index * 100); // 简单的延时模拟，也可以根据 event.timestamp 计算真实延时
});
```

这种"时间胶囊"式的回放体验，对于复盘 Agent 的决策路径、调试错误非常有价值。

## 4. 动态交互组件

为了提升用户体验，`agent-web` 实现了两个关键的动态交互组件：

### 4.1 计划审查 (Plan Review) —— Human-in-the-loop
在 Agent 生成初步计划后，系统不会立即执行，而是通过 `plan_review` 事件暂停，并弹出一个模态框。
- **可视化展示**: 将 JSON 格式的计划渲染为清晰的任务列表。
- **用户干预**: 用户可以点击“批准”直接执行，或者点击“修改”输入新的指令（例如“去掉第三步，增加对竞品的分析”）。
- **动态调整**: 如果用户选择修改，后端 `PlanningAgent` 会接收反馈并重新生成计划，直到用户满意为止。

### 4.2 多模态展示 (Multimodal Display)
当 `response` 事件包含多模态内容时，前端会动态创建专门的 UI 元素，而不是简单地显示链接：

- **多标签页系统**: 每一个生成的报告都会在一个新的 Tab 中打开，用户可以在“终端”、“报告 1”、“报告 2”之间自由切换，互不干扰。
- **PPT 预览**: 如果生成了 PPT，界面会出现一个紫色的 `查看 PPT` 按钮，点击后会在新窗口全屏预览生成的 HTML 幻灯片。
- **播客脚本**: 如果生成了播客，会出现蓝色的 `查看播客` 按钮。点击后会打开一个包含双人对话脚本的 Tab，并提供 `.txt` 格式的下载功能。

这种设计将复杂的 Agent 输出组织得井井有条，让用户能够在一个界面内完成所有的阅读和交互。
