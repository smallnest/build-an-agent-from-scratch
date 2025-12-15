document.addEventListener('DOMContentLoaded', () => {
    const planContainer = document.getElementById('plan-container');
    const terminalContainer = document.getElementById('terminal-container');
    const chatForm = document.getElementById('chat-form');
    const userInput = document.getElementById('user-input');

    let eventSource = null;
    let currentTaskIndex = -1;
    let tasks = [];
    let isReplaying = false;

    // Current Session ID
    let sessionId = '';

    function generateSessionId() {
        sessionId = 'session-' + Math.random().toString(36).substr(2, 9) + '-' + Date.now();
        console.log('New Session ID:', sessionId);
    }

    // Generate initial session ID
    generateSessionId();

    // Auto-resize textarea
    userInput.addEventListener('input', function () {
        this.style.height = 'auto';
        this.style.height = (this.scrollHeight) + 'px';
        if (this.value === '') {
            this.style.height = '';
        }
    });

    // Submit on Enter (Shift+Enter for new line)
    userInput.addEventListener('keydown', function (e) {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            chatForm.dispatchEvent(new Event('submit'));
        }
    });

    const pptCheckbox = document.getElementById('ppt-checkbox');
    const podcastCheckbox = document.getElementById('podcast-checkbox');

    // Fetch config
    fetch('/api/config')
        .then(response => response.json())
        .then(config => {
            if (config.ppt) {
                pptCheckbox.disabled = false;
            }
            if (config.podcast) {
                podcastCheckbox.disabled = false;
            }
        })
        .catch(err => console.error('Failed to load config:', err));

    // Handle form submission
    chatForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        let text = userInput.value.trim();
        if (!text) return;

        setLoading(true);

        // Generate a new session ID for every request to ensure independence
        generateSessionId();

        // Close existing SSE connection if any, to ensure we connect to the new session
        if (eventSource) {
            eventSource.close();
            eventSource = null;
        }

        // Append instructions based on checkboxes
        if (pptCheckbox.checked) {
            text += " 同时生成 PPT 演示文稿。";
        }
        if (podcastCheckbox.checked) {
            text += " 同时生成播客。";
        }

        // Clear previous state
        userInput.value = '';
        userInput.style.height = '';
        pptCheckbox.checked = false;
        podcastCheckbox.checked = false;

        // Clear previous state (except terminal history)
        planContainer.innerHTML = '<div class="empty-state">Planning...</div>';
        currentTaskIndex = -1;
        tasks = [];

        addLog('info', `> User Request: ${text}`);

        try {
            const response = await fetch('/api/chat', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    message: text,
                    session_id: sessionId
                }),
            });

            if (!response.ok) {
                throw new Error('网络响应不正常');
            }

            // Always connect SSE for the new session
            connectSSE();

        } catch (error) {
            console.error('Error:', error);
            addLog('error', '发送消息错误: ' + error.message);
            setLoading(false);
        }
    });

    function connectSSE() {
        if (eventSource) {
            eventSource.close();
        }
        eventSource = new EventSource(`/events?session_id=${sessionId}`);

        eventSource.onmessage = (event) => {
            const data = JSON.parse(event.data);
            handleEvent(data);
        };

        eventSource.onerror = (error) => {
            // console.error('SSE Error:', error);
            eventSource.close();
            eventSource = null;
        };
    }

    const tabsContainer = document.querySelector('.window-tabs');
    const rightPanel = document.querySelector('.panel.right-panel');
    let reportCount = 0;

    // Initial tab handling for Terminal
    const terminalTab = document.querySelector('.tab[data-tab="terminal"]');
    terminalTab.addEventListener('click', () => activateTab('terminal'));

    function activateTab(tabId) {
        // Deactivate all
        document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

        // Activate target
        const tab = document.querySelector(`.tab[data-tab="${tabId}"]`);
        const content = document.getElementById(`${tabId}-container`);

        if (tab && content) {
            tab.classList.add('active');
            content.classList.add('active');
        }
    }

    function createReportTab(content) {
        reportCount++;
        const tabId = `report-${reportCount}`;

        // Create Tab
        const tab = document.createElement('div');
        tab.className = 'tab';
        tab.dataset.tab = tabId;
        tab.innerHTML = `
            报告 ${reportCount}
            <span class="close-tab" title="关闭报告"><i class="fas fa-times"></i></span>
        `;

        // Create Content Container
        const container = document.createElement('div');
        container.id = `${tabId}-container`;
        container.className = 'tab-content';
        container.innerHTML = `<div class="report-content">${content}</div>`;

        // Add to DOM
        tabsContainer.appendChild(tab);
        rightPanel.appendChild(container);

        // Event Listeners
        tab.addEventListener('click', (e) => {
            if (!e.target.closest('.close-tab')) {
                activateTab(tabId);
            }
        });

        tab.querySelector('.close-tab').addEventListener('click', (e) => {
            e.stopPropagation();
            closeReportTab(tabId);
        });

        return tabId;
    }

    function closeReportTab(tabId) {
        const tab = document.querySelector(`.tab[data-tab="${tabId}"]`);
        const content = document.getElementById(`${tabId}-container`);

        if (tab.classList.contains('active')) {
            activateTab('terminal');
        }

        tab.remove();
        content.remove();
    }

    function handleEvent(data) {
        switch (data.type) {
            case 'log':
                handleLog(data.content);
                break;
            case 'response':
                addLog('success', '收到响应。');

                // Create new report tab
                const tabId = createReportTab(data.content);
                activateTab(tabId);

                // Add button to view report
                const viewBtn = document.createElement('button');
                viewBtn.textContent = '查看报告';
                viewBtn.className = 'view-report-btn';
                viewBtn.style.cssText = 'background: #2da44e; border: none; color: white; padding: 5px 10px; border-radius: 4px; cursor: pointer; margin-top: 5px; font-size: 0.85rem; margin-right: 10px;';

                // Capture current content and reportCount for this button
                const currentContent = data.content;
                const currentReportCount = reportCount;
                const currentTabId = tabId;

                viewBtn.onclick = () => {
                    // Check if tab still exists
                    const existingTab = document.querySelector(`.tab[data-tab="${currentTabId}"]`);
                    if (existingTab) {
                        activateTab(currentTabId);
                    } else {
                        const newTabId = createReportTab(currentContent);
                        activateTab(newTabId);
                    }
                };

                const div = document.createElement('div');
                div.className = 'log-line info';
                div.appendChild(viewBtn);

                // Handle Podcast
                if (data.podcast) {
                    const podcastBtn = document.createElement('button');
                    podcastBtn.textContent = '查看播客';
                    podcastBtn.className = 'view-podcast-btn';
                    podcastBtn.style.cssText = 'background: #00add8; border: none; color: white; padding: 5px 10px; border-radius: 4px; cursor: pointer; margin-top: 5px; font-size: 0.85rem; margin-right: 10px;';

                    const podcastScript = data.podcast;
                    let podcastTabId = null;

                    podcastBtn.onclick = () => {
                        if (podcastTabId) {
                            const existingTab = document.querySelector(`.tab[data-tab="${podcastTabId}"]`);
                            if (existingTab) {
                                activateTab(podcastTabId);
                                return;
                            }
                        }
                        podcastTabId = createPodcastTab(podcastScript);
                        activateTab(podcastTabId);
                    };
                    div.appendChild(podcastBtn);
                }

                // Handle PPT
                if (data.ppt) {
                    const pptBtn = document.createElement('button');
                    pptBtn.textContent = '查看 PPT';
                    pptBtn.className = 'view-ppt-btn';
                    pptBtn.style.cssText = 'background: #8e44ad; border: none; color: white; padding: 5px 10px; border-radius: 4px; cursor: pointer; margin-top: 5px; font-size: 0.85rem;';

                    pptBtn.onclick = () => {
                        window.open(data.ppt, '_blank');
                    };
                    div.appendChild(pptBtn);
                }

                terminalContainer.appendChild(div);
                terminalContainer.scrollTop = terminalContainer.scrollHeight;
                break;
            case 'plan_review':
                if (isReplaying) {
                    renderPlan(data.plan);
                    addLog('system', '计划已在回放中自动确认。');
                } else {
                    showPlanReview(data.plan);
                }
                break;
            case 'error':
                addLog('error', data.content);
                setLoading(false);
                break;
            case 'done':
                addLog('success', '任务已完成！');
                // Refresh history sessions list to include the new session
                setTimeout(loadHistorySessions, 1000); // Small delay to ensure backend has saved the file
                setLoading(false);
                break;
        }
    }

    function createPodcastTab(script) {
        reportCount++; // Reuse report counter for unique IDs
        const tabId = `podcast-${reportCount}`;

        // Create Tab
        const tab = document.createElement('div');
        tab.className = 'tab';
        tab.dataset.tab = tabId;
        tab.innerHTML = `
            播客 ${reportCount}
            <span class="close-tab" title="关闭播客"><i class="fas fa-times"></i></span>
        `;

        // Create Content Container
        const container = document.createElement('div');
        container.id = `${tabId}-container`;
        container.className = 'tab-content';

        // Render script
        let scriptHtml = `
            <div class="podcast-controls" style="margin-bottom: 20px; padding-bottom: 10px; border-bottom: 1px solid #eaecef; display: flex; justify-content: flex-end;">
                <button class="export-script-btn" style="background: #2da44e; border: none; color: white; padding: 8px 16px; border-radius: 6px; cursor: pointer; font-size: 0.9rem; display: flex; align-items: center; gap: 8px;">
                    <i class="fas fa-download"></i> 导出脚本
                </button>
            </div>
            <div class="podcast-script">
        `;

        if (Array.isArray(script)) {
            script.forEach(line => {
                const speakerClass = line.speaker.toLowerCase().replace(/\s+/g, '-');
                scriptHtml += `
                    <div class="dialogue-line ${speakerClass}">
                        <div class="speaker">${line.speaker}</div>
                        <div class="text">${line.text}</div>
                    </div>
                `;
            });
        } else {
            scriptHtml += `<pre>${JSON.stringify(script, null, 2)}</pre>`;
        }
        scriptHtml += '</div>';

        container.innerHTML = `<div class="report-content">${scriptHtml}</div>`;

        // Add event listener for export button
        const exportBtn = container.querySelector('.export-script-btn');
        if (exportBtn) {
            exportBtn.addEventListener('click', () => {
                let textContent = '';
                if (Array.isArray(script)) {
                    textContent = script.map(line => `${line.speaker}: ${line.text}`).join('\n\n');
                } else {
                    textContent = JSON.stringify(script, null, 2);
                }

                const blob = new Blob([textContent], { type: 'text/plain' });
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = `podcast_script_${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.txt`;
                document.body.appendChild(a);
                a.click();
                window.URL.revokeObjectURL(url);
                document.body.removeChild(a);
            });
        }

        // Add to DOM
        tabsContainer.appendChild(tab);
        rightPanel.appendChild(container);

        // Event Listeners
        tab.addEventListener('click', (e) => {
            if (!e.target.closest('.close-tab')) {
                activateTab(tabId);
            }
        });

        tab.querySelector('.close-tab').addEventListener('click', (e) => {
            e.stopPropagation();
            closeReportTab(tabId);
        });

        return tabId;
    }

    function handleLog(content) {
        // Parse specific log formats to update UI
        if (content.includes('📍 步骤')) {
            // Format: 📍 步骤 1/4: [SEARCH] Description
            const match = content.match(/步骤 (\d+)\/(\d+): \[(.*?)\] (.*)/);
            if (match) {
                const index = parseInt(match[1]) - 1;
                const type = match[3];
                const desc = match[4];

                updateTaskStatus(index, 'active');
                addLog('highlight', content);
                return;
            }
        } else if (content.includes('✓ 完成')) {
            updateTaskStatus(currentTaskIndex, 'completed');
            addLog('success', content);
            return;
        } else if (content.includes('✗ 失败')) {
            updateTaskStatus(currentTaskIndex, 'failed');
            addLog('error', content);
            return;
        }

        // Regular log
        addLog('info', content);
    }

    function addLog(type, content) {
        const div = document.createElement('div');
        div.className = `log-line ${type}`;

        const time = new Date().toLocaleTimeString('en-US', { hour12: false });
        const timestamp = document.createElement('span');
        timestamp.className = 'timestamp';
        timestamp.textContent = `[${time}]`;

        div.appendChild(timestamp);
        div.appendChild(document.createTextNode(content));

        terminalContainer.appendChild(div);
        terminalContainer.scrollTop = terminalContainer.scrollHeight;
    }

    function renderPlan(plan) {
        addLog('info', '正在渲染计划...');
        if (!plan || !plan.tasks || !Array.isArray(plan.tasks)) {
            addLog('error', '收到的计划数据无效');
            console.error('Invalid plan:', plan);
            return;
        }

        planContainer.innerHTML = '';
        tasks = plan.tasks;
        currentTaskIndex = -1;

        if (tasks.length === 0) {
            planContainer.innerHTML = '<div class="empty-state">计划中没有任务</div>';
            return;
        }

        tasks.forEach((task, index) => {
            const template = document.getElementById('plan-item-template');
            const clone = template.content.cloneNode(true);
            const item = clone.querySelector('.plan-item');

            item.id = `task-${index}`;
            item.querySelector('.task-desc').textContent = task.description;
            item.querySelector('.task-meta').textContent = task.type;

            // Set icon based on state (initial state is pending)
            const icon = item.querySelector('.status-icon i');
            icon.className = 'far fa-circle';

            planContainer.appendChild(item);
        });
        addLog('success', `计划渲染完成，共 ${tasks.length} 个任务。`);
    }

    function updateTaskStatus(index, status) {
        if (index < 0 || index >= tasks.length) return;

        // Update previous task if moving to next
        if (status === 'active') {
            if (currentTaskIndex !== -1 && currentTaskIndex !== index) {
                updateTaskStatus(currentTaskIndex, 'completed');
            }
            currentTaskIndex = index;
        }

        const item = document.getElementById(`task-${index}`);
        if (!item) return;

        item.className = `plan-item ${status}`;
        const icon = item.querySelector('.status-icon i');

        switch (status) {
            case 'active':
                icon.className = 'fas fa-spinner fa-spin';
                break;
            case 'completed':
                icon.className = 'fas fa-check-circle';
                break;
            case 'failed':
                icon.className = 'fas fa-times-circle';
                break;
            default:
                icon.className = 'far fa-circle';
        }
    }

    function showPlanReview(plan) {
        // First render the plan in the left panel
        renderPlan(plan);

        // Then show modal
        const template = document.getElementById('plan-review-modal-template');
        const clone = template.content.cloneNode(true);
        const modalOverlay = clone.querySelector('.modal-overlay');

        const planPreview = clone.querySelector('.plan-preview');
        // Format plan for preview
        let previewText = `目标: ${plan.description}\n\n任务:\n`;
        plan.tasks.forEach((t, i) => {
            previewText += `${i + 1}. [${t.type}] ${t.description}\n`;
        });
        planPreview.textContent = previewText;

        const approveBtn = clone.querySelector('.approve-btn');
        const modifyBtn = clone.querySelector('.modify-btn');
        const modInputDiv = clone.querySelector('.modification-input');
        const submitModBtn = clone.querySelector('.submit-mod-btn');
        const modTextarea = modInputDiv.querySelector('textarea');

        approveBtn.addEventListener('click', async () => {
            await sendResponse('');
            modalOverlay.remove();
            addLog('system', '计划已批准。');
        });

        modifyBtn.addEventListener('click', () => {
            modInputDiv.style.display = 'flex';
            approveBtn.style.display = 'none';
            modifyBtn.style.display = 'none';
        });

        submitModBtn.addEventListener('click', async () => {
            const modification = modTextarea.value.trim();
            if (!modification) return;

            await sendResponse(modification);
            modalOverlay.remove();
            addLog('system', '计划修改已提交: ' + modification);
        });

        document.body.appendChild(modalOverlay);
    }

    async function sendResponse(content) {
        try {
            await fetch('/api/respond', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    response: content,
                    session_id: sessionId
                }),
            });
        } catch (error) {
            console.error('Error sending response:', error);
            addLog('error', '发送响应错误: ' + error.message);
        }
    }

    function setLoading(isLoading) {
        userInput.disabled = isLoading;
        const sendBtn = document.getElementById('send-btn');
        sendBtn.disabled = isLoading;

        if (isLoading) {
            sendBtn.style.opacity = '0.5';
            sendBtn.style.cursor = 'not-allowed';
        } else {
            sendBtn.style.opacity = '1';
            sendBtn.style.cursor = 'pointer';
            userInput.focus();
        }
    }

    // Load history sessions on startup
    loadHistorySessions();

    function loadHistorySessions() {
        const container = document.getElementById('history-sessions-list');
        if (!container) return;

        fetch('/api/sessions')
            .then(res => res.json())
            .then(sessions => {
                container.innerHTML = '';
                if (sessions.length === 0) {
                    container.innerHTML = '<div style="color: #666; font-size: 0.9rem;">暂无历史会话</div>';
                    return;
                }

                // Sort by timestamp desc
                sessions.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));

                sessions.forEach(session => {
                    const card = document.createElement('div');
                    card.className = 'history-session-card';

                    // Extract user request from ID
                    // Format: sanitized_request-session_id
                    // We want everything before the last "-session-"
                    let displayTitle = session.id;
                    const sessionIndex = session.id.lastIndexOf('-session-');
                    if (sessionIndex !== -1) {
                        const requestPart = session.id.substring(0, sessionIndex);
                        displayTitle = requestPart.replace(/_/g, ' ');
                    }

                    if (!displayTitle || displayTitle.length === 0) {
                        displayTitle = session.id;
                    }

                    const date = new Date(session.timestamp).toLocaleDateString();

                    card.innerHTML = `
                        <div class="history-session-title" title="${displayTitle}">${displayTitle}</div>
                    `;

                    card.addEventListener('click', () => {
                        replaySession(session.id);
                        // Scroll to top to see replay
                        window.scrollTo({ top: 0, behavior: 'smooth' });
                    });

                    container.appendChild(card);
                });
            })
            .catch(err => {
                console.error('Failed to load sessions:', err);
                container.innerHTML = '<div style="color: red;">加载失败</div>';
            });
    }

    function replaySession(sessionId) {
        isReplaying = true;
        // Clear current state
        planContainer.innerHTML = '<div class="empty-state">Replaying session...</div>';
        terminalContainer.innerHTML = '';
        currentTaskIndex = -1;
        tasks = [];

        // Remove all report/podcast tabs except terminal
        document.querySelectorAll('.tab').forEach(tab => {
            if (tab.dataset.tab !== 'terminal') {
                closeReportTab(tab.dataset.tab);
            }
        });
        activateTab('terminal');

        // Extract display title for log
        let displayTitle = sessionId;
        const sessionIndex = sessionId.lastIndexOf('-session-');
        if (sessionIndex !== -1) {
            const requestPart = sessionId.substring(0, sessionIndex);
            displayTitle = requestPart.replace(/_/g, ' ');
        }
        addLog('system', `> 开始回放会话: ${displayTitle}`);

        fetch(`/api/replay?session_id=${sessionId}`)
            .then(res => res.json())
            .then(events => {
                if (!Array.isArray(events)) {
                    throw new Error('Invalid session data');
                }

                if (events.length === 0) {
                    addLog('system', '> 会话数据为空');
                    isReplaying = false;
                    return;
                }

                // Sort events by timestamp to ensure correct order
                events.sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));

                // Calculate relative delays
                const startTime = new Date(events[0].timestamp).getTime();

                let i = 0;
                function playNext() {
                    if (i >= events.length) {
                        // addLog('system', '> 回放结束');
                        isReplaying = false;
                        return;
                    }

                    const event = events[i];
                    handleEvent(event);

                    i++;
                    if (i < events.length) {
                        const nextEvent = events[i];
                        const currentEventTime = new Date(event.timestamp).getTime();
                        const nextEventTime = new Date(nextEvent.timestamp).getTime();

                        // Calculate delay, cap at 2 seconds to avoid waiting too long for long pauses
                        let delay = nextEventTime - currentEventTime;
                        if (delay < 0) delay = 0;
                        if (delay > 2000) delay = 2000; // Max 2s delay
                        if (delay < 100) delay = 100;   // Min 100ms delay for visual flow

                        setTimeout(playNext, delay);
                    } else {
                        // addLog('system', '> 回放结束');
                        isReplaying = false;
                    }
                }

                // Start playback
                playNext();
            })
            .catch(err => {
                addLog('error', `回放失败: ${err.message}`);
                isReplaying = false;
            });
    }

});
