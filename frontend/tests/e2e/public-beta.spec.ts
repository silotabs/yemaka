import { expect, test, type Page, type Route } from '@playwright/test';

type MockMessage = {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  model?: string;
  sourceKind?: string;
  sources?: string[];
};

type MockConversation = {
  id: string;
  title: string;
  createdAt: string;
  updatedAt: string;
  messages: MockMessage[];
};

const now = '2026-06-11T00:00:00Z';

test.beforeEach(async ({ page }) => {
  await installMockAPI(page);
});

test('app loads, starts a new chat, streams a simple answer, and records no fatal console errors', async ({ page }) => {
  const errors = captureFatalConsoleErrors(page);
  await gotoApp(page);

  await expect(composer(page)).toBeVisible();
  await ask(page, 'Say hello in one sentence.');

  await expect(page.getByText('Hello from Yemaka public-beta QA.')).toBeVisible();
  expect(errors()).toEqual([]);
});

test('ambiguous prompts and follow-ups ask for clarification instead of guessing', async ({ page }) => {
  await gotoApp(page);

  await ask(page, 'Do it for that one.');
  await expect(page.getByText(/Which task should I continue, switch to, or clarify/i)).toBeVisible();

  await ask(page, 'Use the new target instead.');
  await expect(page.getByText(/This looks like a new target/i)).toBeVisible();
});

test('provider and file-copy failures show product-grade recovery guidance', async ({ page }) => {
  await gotoApp(page);

  await ask(page, 'Find latest news but the provider is unavailable.');
  await expect(page.getByText(/Search provider is not configured/i)).toBeVisible();

  await ask(page, 'Explain upload copy versus grant path.');
  await expect(page.getByText(/Uploaded files are conversation-scoped copies/i)).toBeVisible();

  await ask(page, 'Show route recovery and context compaction markers.');
  await expect(page.getByText(/Route Recovery: provider_unavailable/i)).toBeVisible();
  await expect(page.getByText(/Context Compaction: active task preserved/i)).toBeVisible();
});

test('Response Mode settings are visible, persist, and keep local-first defaults disabled', async ({ page }) => {
  await gotoApp(page, '/#/settings');

  await expect(page.getByRole('group', { name: /response mode/i })).toBeVisible();
  await expect(page.getByText(/Chat still hides raw model thinking/i)).toBeVisible();
  await page.getByRole('button', { name: 'Fast' }).click();
  await page.getByRole('button', { name: /save settings/i }).click();
  await expect(page.getByText(/Settings saved/i)).toBeVisible();

  await page.reload();
  await expect(page.getByRole('button', { name: 'Fast' })).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByText(/Safety checks, approvals, tool lanes, and local-first defaults stay unchanged/i)).toBeVisible();

  await expect(page.getByLabel('Show thinking trace')).not.toBeChecked();
  await expect(page.getByLabel('Internet search')).not.toBeChecked();
  await expect(page.getByLabel('Cloud fallback')).not.toBeChecked();
  await expect(page.getByText(/Cloud fallback and connectors stay disabled/i)).toBeVisible();
  await expect(page.getByLabel('Embeddings')).not.toBeChecked();
});

test('background settings refresh preserves an unsaved settings draft', async ({ page }) => {
  await gotoApp(page, '/#/settings');

  const fast = page.getByRole('button', { name: 'Fast' });
  await expect(fast).toBeVisible();
  await fast.click();
  await expect(fast).toHaveAttribute('aria-pressed', 'true');

  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await expect(fast).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByRole('button', { name: 'Balanced' })).toHaveAttribute('aria-pressed', 'false');
});

test('runtime restart warns before interrupting work and uses the guarded local endpoint', async ({ page }) => {
  await gotoApp(page, '/#/settings');

  await page.getByRole('button', { name: 'Restart Yemaka' }).click();
  await expect(page.getByText(/Restart Yemaka now\? Any in-progress response or local action will stop/i)).toBeVisible();
  await page.getByRole('button', { name: 'Continue' }).click();
  await expect(page.getByText(/Yemaka is restarting/i)).toBeVisible();
});

test('composer expands additional enabled skills on demand', async ({ page }) => {
  await gotoApp(page);

  await page.getByTitle('Yemaka controls').click();
  const moreSkills = page.getByRole('button', { name: /More skills/i });
  await expect(moreSkills).toHaveAttribute('aria-expanded', 'false');
  await expect(page.getByText('Code Note Review')).not.toBeVisible();

  await moreSkills.click();
  await expect(moreSkills).toHaveAttribute('aria-expanded', 'true');
  await expect(page.getByText('Code Note Review')).toBeVisible();
});

test('sidebar local-profile settings shortcut opens Settings', async ({ page }) => {
  await gotoApp(page);

  await page.getByLabel('Open settings').click();
  await expect(page.getByText('Runtime', { exact: true })).toBeVisible();
});

test('core beta pages load and expose trace/recovery/compaction diagnostics where available', async ({ page }) => {
  await gotoApp(page, '/#/memory');
  await expect(page.getByText(/Memory/i).first()).toBeVisible();
  await expect(page.getByText(/Local Knowledge Graph/i)).toBeVisible();

  await gotoApp(page, '/#/settings');
  await expect(page.getByText('Runtime', { exact: true })).toBeVisible();

  await gotoApp(page, '/#/tools');
  await expect(page.getByText(/Replay Diagnostics/i)).toBeVisible();
  await expect(page.getByText(/Route Recovery/i)).toBeVisible();
  await expect(page.getByText(/web-search-lane/i)).toBeVisible();
});

async function gotoApp(page: Page, path = '/') {
  await page.goto(path, { waitUntil: 'domcontentloaded' });
}

function composer(page: Page) {
  return page.getByRole('textbox', { name: /ask (about your files|yemaka)/i });
}

async function ask(page: Page, prompt: string) {
  await composer(page).fill(prompt);
  await page.getByRole('button', { name: /send prompt/i }).click();
  await expect(page.getByText(answerForPrompt(prompt))).toBeVisible();
  await expect(page.getByRole('button', { name: /send prompt/i })).toBeVisible();
}

function captureFatalConsoleErrors(page: Page) {
  const fatal: string[] = [];
  page.on('console', (message) => {
    if (message.type() !== 'error') return;
    const text = message.text();
    if (/TypeError|ReferenceError|Unhandled|Failed to load resource| 40[04] | 50\d /.test(text)) {
      fatal.push(text);
    }
  });
  page.on('pageerror', (error) => fatal.push(error.message));
  return () => fatal;
}

async function installMockAPI(page: Page) {
  const conversations = new Map<string, MockConversation>();
  const status = {
    configPath: '/tmp/yemaka-e2e/config.yaml',
    profilePath: '/tmp/yemaka-e2e/profiles/default',
    sqlitePath: '/tmp/yemaka-e2e/yemaka.db',
    sqliteOk: true,
    sqliteError: '',
    ollamaOk: false,
    ollamaError: 'Ollama is not required in mocked browser E2E.',
    defaultModel: 'mock-local',
    lowMemoryModel: 'mock-local',
    selectedModel: 'mock-local',
    lowMemoryMode: true,
    setupComplete: true,
    modelReady: false,
    ragEnabled: true,
    shellEnabled: false,
    telemetry: false,
    modelStatuses: [],
    skillCount: 0
  };
  let settings = {
    settingsSchemaVersion: 2,
    featureSupport: { responseMode: true, showThinkingTrace: true },
    theme: 'system',
    lowMemoryMode: true,
    maxContextTokens: 4096,
    responseMode: 'balanced',
    showThinkingTrace: false,
    ui: { theme: 'system' },
    rag: { enabled: true, embeddings: { enabled: false, model: '' } },
    internet: { enabled: false, search: { enabled: false, provider: 'none', endpoint: '', apiKeyEnv: '' } },
    knowledge: { enabled: true, influenceEnabled: false },
    cloudFallback: { enabled: false, baseURL: '', model: '', apiKeyEnv: '' },
    connectors: {
      enabled: false,
      local_api: { enabled: false, tokenEnv: '' },
      mcp_server: { enabled: false, tokenEnv: '' },
      slack: { enabled: false, tokenEnv: '' },
      discord: { enabled: false, tokenEnv: '' },
      telegram: { enabled: false, tokenEnv: '' },
      email: { enabled: false, tokenEnv: '' }
    },
    shellEnabled: false,
    telemetry: false,
    modelRoles: {}
  };

  await page.route('**/api/ask/stream', async (route) => {
    const request = route.request();
    const body = request.postDataJSON() as { content?: string; conversationId?: string; skill?: string } | null;
    const prompt = String(body?.content || '');
    const conversationId = body?.conversationId || 'conv_e2e';
    const answer = answerForPrompt(prompt);
    const conversation = conversations.get(conversationId) || {
      id: conversationId,
      title: prompt.slice(0, 48) || 'Public beta QA',
      createdAt: now,
      updatedAt: now,
      messages: []
    };
    const userId = `msg_user_${conversation.messages.length + 1}`;
    const assistantId = `msg_assistant_${conversation.messages.length + 2}`;
    conversation.messages.push({ id: userId, role: 'user', content: prompt, model: 'yemaka' });
    conversation.messages.push({
      id: assistantId,
      role: 'assistant',
      content: answer,
      model: 'mock-local',
      sourceKind: sourceKindForPrompt(prompt),
      sources: prompt.toLowerCase().includes('provider') ? ['route_recovery:provider_unavailable'] : []
    });
    conversations.set(conversationId, conversation);

    const frames = [
      event({ type: 'message.saved', data: { conversation_id: conversationId, message_id: userId, role: 'user', content: prompt } }),
      event({ type: 'task.classified', data: { task_type: 'chat', risk_level: 'low' } }),
      event({ type: 'route.selected', data: { route: sourceKindForPrompt(prompt), tool_lane: toolLaneForPrompt(prompt), response_mode: settings.responseMode } }),
      event({ type: 'model.selected', data: { model: 'mock-local' } }),
      event({ type: 'model.token', token: answer }),
      event({
        type: 'message.saved',
        data: {
          conversation_id: conversationId,
          message_id: assistantId,
          parent_message_id: userId,
          role: 'assistant',
          content: answer,
          variant_index: '0'
        }
      }),
      event({ type: 'agent.completed', data: { conversation_id: conversationId } }),
      event({
        type: 'result',
        result: {
          text: answer,
          model: 'mock-local',
          conversationId,
          skill: body?.skill || '',
          sourceKind: sourceKindForPrompt(prompt),
          sources: prompt.toLowerCase().includes('provider') ? ['route_recovery:provider_unavailable'] : [],
          toolCalls: [],
          userMessageId: userId,
          assistantMessageId: assistantId,
          parentMessageId: userId,
          variantIndex: 0
        }
      })
    ].join('');
    await route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      body: frames
    });
  });

  await page.route('**/api/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    const path = url.pathname;
    const requestBody = method === 'POST' ? postBody(route) : {};

    if (path === '/api/ask/stream') return route.fallback();

    if (path === '/api/status') return json(route, status);
    if (path === '/api/setup') return json(route, setupState());
    if (path === '/api/settings' && method === 'GET') return json(route, settings);
    if (path === '/api/settings' && method === 'POST') {
      settings = mergeSettings(settings, requestBody);
      return json(route, settings);
    }
    if (path === '/api/runtime/restart' && method === 'POST') {
      if (request.headers()['x-yemaka-runtime-control'] !== '1') return error(route, 403, 'runtime control request is required');
      return json(route, { action: 'restart', accepted: true, message: 'Yemaka is restarting.' });
    }
    if (path === '/api/runtime/shutdown' && method === 'POST') {
      if (request.headers()['x-yemaka-runtime-control'] !== '1') return error(route, 403, 'runtime control request is required');
      return json(route, { action: 'shutdown', accepted: true, message: 'Yemaka is shutting down.' });
    }
    if (path === '/api/conversations') return json(route, Array.from(conversations.values()).map(conversationSummary));
    if (path.startsWith('/api/conversations/')) {
      const id = decodeURIComponent(path.split('/').pop() || '');
      return json(route, conversations.get(id) || emptyConversation(id));
    }
    if (path === '/api/tool-runs') return json(route, toolRuns());
    if (path === '/api/tools/catalog') return json(route, toolCatalog());
    if (path === '/api/replay/traces' || path === '/api/replay') return json(route, replayTraces());
    if (path === '/api/replay/explain') return json(route, replayExplanation());
    if (path === '/api/feedback/review') return json(route, qaReview());
    if (path === '/api/ops/status') return json(route, opsStatus());
    if (path === '/api/memory') return json(route, memoryResults());
    if (path === '/api/knowledge/status') return json(route, { enabled: true, influenceEnabled: false, pending: 0, approved: 1, rejected: 0 });
    if (path === '/api/knowledge/entities') return json(route, knowledgeEntities());
    if (path === '/api/knowledge/proposals') return json(route, []);
    if (path === '/api/documents/search') return json(route, []);
    if (path === '/api/documents/inventory') return json(route, []);
    if (path === '/api/documents/workspace-grants') return json(route, []);
    if (path === '/api/internet/status') return json(route, internetStatus());
    if (path === '/api/internet/requests') return json(route, []);
    if (path === '/api/internet/cache') return json(route, { entries: [], fresh: 0, stale: 0 });
    if (path === '/api/jobs') return json(route, []);
    if (path === '/api/jobs/runs') return json(route, []);
    if (path === '/api/jobs/status') return json(route, jobStatus());
    if (path === '/api/heartbeat/status') return json(route, { overall: 'disabled', generatedAt: now, checks: [] });
    if (path === '/api/extensions') return json(route, []);
    if (path === '/api/extensions/failure-trends') return json(route, []);
    if (path === '/api/connectors') return json(route, []);
    if (path === '/api/connectors/generated') return json(route, []);
    if (path === '/api/skills') return json(route, mockSkills());
    if (path === '/api/domain-packs') return json(route, { installed: [], templates: [] });
    if (path === '/api/domain-packs/skills') return json(route, []);
    if (path === '/api/models') return json(route, []);
    if (path === '/api/model-profiles') return json(route, []);
    if (path === '/api/notifications') return json(route, []);
    if (method === 'POST') return json(route, { status: 'ok' });
    return json(route, {});
  });

  function setupState() {
    return {
      setupComplete: true,
      ollamaOk: false,
      ollamaError: 'Ollama is mocked in browser E2E.',
      configPath: status.configPath,
      lowMemoryMode: true,
      mode: 'student_laptop',
      selectedModel: 'mock-local',
      modelReady: false,
      installedModels: [],
      configuredDefaults: [],
      recommendedMode: 'student_laptop',
      recommendedModel: '',
      suggestions: []
    };
  }
}

function event(payload: Record<string, unknown>) {
  return `data: ${JSON.stringify(payload)}\n\n`;
}

function answerForPrompt(prompt: string) {
  const lower = prompt.toLowerCase();
  if (lower.includes('do it') || lower.includes('ambiguous')) {
    return 'Which task should I continue, switch to, or clarify before I use tools?';
  }
  if (lower.includes('new target')) {
    return 'This looks like a new target. I can switch safely after you confirm the target and scope.';
  }
  if (lower.includes('provider') || lower.includes('latest news')) {
    return 'Search provider is not configured. Open Settings and configure a search provider before current-news requests.';
  }
  if (lower.includes('upload') || lower.includes('grant')) {
    return 'Uploaded files are conversation-scoped copies. Use a workspace grant when Yemaka needs the real filesystem path.';
  }
  if (lower.includes('recovery') || lower.includes('compaction')) {
    return 'Route Recovery: provider_unavailable -> request_configuration. Context Compaction: active task preserved.';
  }
  return 'Hello from Yemaka public-beta QA.';
}

function sourceKindForPrompt(prompt: string) {
  return prompt.toLowerCase().includes('provider') || prompt.toLowerCase().includes('latest news') ? 'internet' : 'chat';
}

function toolLaneForPrompt(prompt: string) {
  return prompt.toLowerCase().includes('provider') || prompt.toLowerCase().includes('latest news') ? 'web_search_lane' : 'chat_lane';
}

function mergeSettings(current: Record<string, unknown>, incoming: unknown) {
  const patch = typeof incoming === 'object' && incoming !== null ? (incoming as Record<string, unknown>) : {};
  return {
    ...current,
    ...patch,
    ui: {
      ...((current.ui as Record<string, unknown> | undefined) || {}),
      theme: patch.uiTheme || (patch.ui as Record<string, unknown> | undefined)?.theme || (current.ui as Record<string, unknown> | undefined)?.theme || 'system'
    },
    featureSupport: { responseMode: true, showThinkingTrace: true },
    internet: mergeNested(current.internet, patch.internet),
    rag: mergeNested(current.rag, patch.rag),
    cloudFallback: mergeNested(current.cloudFallback, patch.cloudFallback),
    connectors: mergeNested(current.connectors, patch.connectors),
    knowledge: mergeNested(current.knowledge, patch.knowledge)
  };
}

function mergeNested(current: unknown, incoming: unknown) {
  if (typeof current !== 'object' || current === null) return incoming;
  if (typeof incoming !== 'object' || incoming === null) return current;
  return { ...(current as Record<string, unknown>), ...(incoming as Record<string, unknown>) };
}

function conversationSummary(conversation: MockConversation) {
  return {
    id: conversation.id,
    title: conversation.title,
    createdAt: conversation.createdAt,
    updatedAt: conversation.updatedAt,
    messageCount: conversation.messages.length,
    pinned: false,
    starred: false
  };
}

function emptyConversation(id: string) {
  return { id, title: 'Public beta QA', createdAt: now, updatedAt: now, messages: [] };
}

function toolRuns() {
  return [
    {
      id: 'run_route_recovery',
      conversationId: 'conv_e2e',
      toolName: 'route_recovery',
      status: 'completed',
      riskLevel: 'low',
      startedAt: now,
      completedAt: now,
      input: { trigger: 'provider_unavailable', selected_route: 'current_news' },
      output: {
        final_action: 'request_configuration',
        route: 'current_news',
        tool_lane: 'web_search_lane',
        context_compaction: 'active task preserved'
      }
    }
  ];
}

function toolCatalog() {
  return [
    {
      name: 'internet_search',
      status: 'disabled',
      surfaces: ['chat-callable'],
      chatCallable: false,
      modelCallable: false,
      requiresApproval: true,
      enabledByDefault: false,
      mutating: false,
      notes: 'Disabled by default.'
    }
  ];
}

function replayTraces() {
  return [
    {
      id: 'trace_route_recovery',
      path: '/tmp/route-recovery-web-search-lane-context-compaction.json',
      filename: 'route-recovery-web-search-lane-context-compaction.json',
      size: 1024,
      updatedAt: now
    }
  ];
}

function replayExplanation() {
  return {
    id: 'trace_route_recovery',
    summary: 'Selected route current_news, lane web_search_lane, Route Recovery provider_unavailable, Context Compaction preserved active task.',
    route: 'current_news',
    toolLane: 'web_search_lane',
    recovery: 'provider_unavailable',
    compaction: 'active task preserved',
    stages: []
  };
}

function qaReview() {
  return {
    generatedAt: now,
    summary: { replayFailures: 0, routeCorrections: 0, regressionSuggestions: 0 },
    routeFailureCategories: [{ category: 'provider_config_issue', count: 1 }],
    replayFailures: [],
    routeCorrections: [],
    regressionSuggestions: []
  };
}

function opsStatus() {
  return {
    generatedAt: now,
    timeline: [
      {
        id: 'ops_route_recovery',
        kind: 'replay',
        title: 'Route Recovery provider_unavailable',
        status: 'warning',
        at: now,
        summary: 'web_search_lane requested provider configuration.'
      }
    ]
  };
}

function memoryResults() {
  return [
    {
      id: 'mem_compaction',
      kind: 'task_state',
      content: 'Context Compaction preserved active task, pending approval, and selected sources.',
      createdAt: now,
      score: 1
    }
  ];
}

function mockSkills() {
  return [
    'brief_writer',
    'contract_review',
    'meeting_notes',
    'project_explainer',
    'research_assistant',
    'code_note_review',
    'claim_check'
  ].map((name) => ({
    name,
    version: '1.0.0',
    description: 'Mock enabled skill',
    triggers: [],
    requiredTools: [],
    enabled: true,
    valid: true,
    validationError: '',
    source: 'test',
    dir: `/skills/${name}`
  }));
}

function knowledgeEntities() {
  return [
    {
      id: 'kg_advisory',
      label: 'Advisory knowledge graph',
      kind: 'system',
      status: 'approved',
      provenance: 'manual_review',
      confidence: 0.9,
      createdAt: now,
      updatedAt: now
    }
  ];
}

function internetStatus() {
  return {
    enabled: false,
    search: {
      enabled: false,
      provider: 'none',
      status: 'disabled',
      healthScore: 0,
      fallbackReady: false,
      nextAction: 'Configure a provider before current-news requests.'
    },
    cache: { fresh: 0, stale: 0 }
  };
}

function jobStatus() {
  return {
    enabled: false,
    totalJobs: 0,
    enabledJobs: 0,
    approvedJobs: 0,
    runningJobs: 0,
    maxParallelJobs: 1,
    archivedJobs: 0,
    invalidInputJobs: 0,
    lastRunAt: '',
    lastRunStatus: 'disabled'
  };
}

function postBody(route: Route) {
  try {
    return route.request().postDataJSON();
  } catch {
    return {};
  }
}

async function json(route: Route, payload: unknown) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(payload)
  });
}

async function error(route: Route, status: number, message: string) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify({ error: message })
  });
}
