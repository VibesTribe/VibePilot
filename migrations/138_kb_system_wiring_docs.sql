-- Migration 138: KB knowledge items for system wiring documentation
-- These entries ensure agents (via kb_context_pack) know how VibePilot's
-- subsystems actually work, what code they touch, and what data flows through them.

-- ============================================================
-- 1. EMBEDDING PIPELINE
-- ============================================================
INSERT INTO kb_knowledge_items (repo_id, item_type, name, title, summary, metadata, priority, status)
VALUES (
    'vibepilot',
    'pattern',
    'embedding_pipeline',
    'KB Embedding Pipeline — NVIDIA via OpenRouter, pgvector, Seed Embedding Context Packs',
    E'The KB uses nvidia/llama-nemotron-embed-vl-1b-v2:free via OpenRouter for all embeddings. '
    'Outputs 2048 dims, truncated to 2000 for pgvector HNSW index. Three tables store vectors: '
    'kb_code_symbols, kb_doc_sections, kb_knowledge_items (all vector(2000)). '
    'Embedding scripts: ~/knowledgebase/scripts/embed.py (backfill), embed_fast.py (fast incremental). '
    'Sync runs on every git commit via post-commit hook (sync_direct.py -> post_sync.py). '
    '3am cron (sync_all.py) handles embedding backfill only. '
    'Seed embedding approach in kb_context_pack RPC: finds ONE symbol via ILIKE keyword match, '
    'stores its vector in PL/pgSQL variable, then uses cosine similarity (embedding <=> seed) '
    'to find all semantically similar symbols. No external API call inside SQL. '
    'Multi-word queries split into individual keywords for broader OR matching. '
    'Go code: governor/internal/kb/kb.go SearchAllSemantic() (note: comment says 768-dim nomic, '
    'but DB is actually 2000-dim NVIDIA -- comment is stale). '
    'API key: OPENROUTER_API_KEY from encrypted vault (NEVER .env). '
    'Cost: FREE tier model, no per-token charge. '
    'MCP server exposes semantic search tools on port 8901.',
    '{"model": "nvidia/llama-nemotron-embed-vl-1b-v2:free", "dimensions": 2000, "provider": "openrouter", "cost": "free", "tables": ["kb_code_symbols", "kb_doc_sections", "kb_knowledge_items"], "scripts": ["embed.py", "embed_fast.py", "sync_direct.py"], "api_key": "OPENROUTER_API_KEY"}',
    'high',
    'active'
) ON CONFLICT (item_type, name) DO UPDATE SET
    title = EXCLUDED.title,
    summary = EXCLUDED.summary,
    metadata = EXCLUDED.metadata,
    priority = EXCLUDED.priority,
    status = EXCLUDED.status;

-- ============================================================
-- 2. DASHBOARD CHAT WIRING
-- ============================================================
INSERT INTO kb_knowledge_items (repo_id, item_type, name, title, summary, metadata, priority, status)
VALUES (
    'vibepilot',
    'pattern',
    'dashboard_chat_wiring',
    'Dashboard Chat Wiring — Hermes SSE Gateway, Token Tracking, ROI Flow',
    E'The VibeFlow dashboard connects to Hermes gateway via OpenAI-compatible SSE API at http://localhost:8642. '
    'Frontend calls POST /v1/runs to start a run, then GET /v1/runs/{id}/events for SSE stream. '
    'Events: run.created, content.delta, content.done, tool.call, tool.output, run.completed, done. '
    'run.completed now includes usage object with input_tokens, output_tokens, total_tokens. '
    '\n\nAgent config: ~/.hermes/config.yaml. Dashboard chat uses api_server platform with model gemini-2.5-flash. '
    'Toolset: memory, web, terminal, file, browser, delegation, tts, vision, session_search, todo, skills. '
    'Also has KB MCP tools (mcp_kb_kb_search_knowledge, etc.) for querying the knowledge base. '
    'System prompt defines it as VibePilot consultant agent (research, analyze, plan features). '
    '\n\nToken cost tracking: After SSE stream completes, Hermes POSTs usage to governor webhook '
    'POST http://localhost:8080/api/chat/usage with {session_id, model_id, tokens_in, tokens_out}. '
    'Governor calls record_chat_usage RPC -> chat_usage table -> theoretical_cost_usd calculated via calc_run_costs. '
    'ROI report (get_full_roi_report) includes chat_usage_summary section. '
    'Dashboard shows chat costs alongside task pipeline costs. '
    '\n\nKey files: '
    'Hermes api_server.py (SSE handler, usage extraction, governor webhook call), '
    'governor/webhooks/chat_usage.go (endpoint handler), '
    'governor/webhooks/server.go (route registration, dashboard data query), '
    'governor/db/rpc.go (allowlist: record_chat_usage), '
    'migrations/136_chat_usage_tracking.sql (chat_usage table + RPCs), '
    'migrations/137_roi_include_chat_costs.sql (ROI report update). '
    'Frontend: vibeflow/apps/dashboard/ connects to http://localhost:8642 (Hermes) and http://localhost:8080 (governor).',
    '{"hermes_port": 8642, "governor_port": 8080, "model": "gemini-2.5-flash", "transport": "SSE", "token_tracking": "POST /api/chat/usage", "toolset": ["memory", "web", "terminal", "file", "browser", "delegation", "tts", "vision", "session_search", "todo", "skills", "kb_mcp"]}',
    'high',
    'active'
) ON CONFLICT (item_type, name) DO UPDATE SET
    title = EXCLUDED.title,
    summary = EXCLUDED.summary,
    metadata = EXCLUDED.metadata,
    priority = EXCLUDED.priority,
    status = EXCLUDED.status;

-- ============================================================
-- 3. TELEGRAM WIRING
-- ============================================================
INSERT INTO kb_knowledge_items (repo_id, item_type, name, title, summary, metadata, priority, status)
VALUES (
    'vibepilot',
    'pattern',
    'telegram_wiring',
    'Telegram Bot Wiring — Message Flow, Tools, Agent Capabilities',
    E'Hermes gateway runs a Telegram bot via python-telegram-bot library (telegram.py adapter). '
    'Enabled in ~/.hermes/config.yaml under platforms.telegram. Bot token from encrypted vault (TELEGRAM_BOT_TOKEN). '
    'Messages flow: User sends message in Telegram -> telegram-telegram-bot polling/webhook -> '
    'Hermes gateway telegram.py adapter -> agent.run_conversation() -> response streamed back. '
    '\n\nAgent config: Same consultant agent as dashboard chat. Uses Z.AI GLM-5 as primary model '
    'with fallback chain: gemini-2.5-flash -> gemini-2.0-flash -> groq llama-3.3-70b -> openrouter free models. '
    'Toolset (same as dashboard): memory, web, terminal, file, browser, delegation, tts, vision, session_search, todo, skills. '
    'Plus KB MCP tools for knowledge base queries. '
    '\n\nTelegram-specific features: '
    '- TTS audio responses (text_to_speech tool sends voice messages) '
    '- Image support (user sends photos -> vision analysis) '
    '- Voice message transcription (via whisper) '
    '- Inline keyboards for interactive decisions '
    '- Group chat support with @bot mention detection '
    '\n\nKey files: '
    '~/.hermes/hermes-agent/gateway/platforms/telegram.py (adapter), '
    '~/.hermes/config.yaml (platform + toolset config), '
    '~/.hermes/hermes-agent/gateway/config.py (PlatformConfig parsing). '
    '\n\nToken tracking: NOT currently wired to governor chat_usage (only dashboard chat reports tokens). '
    'Telegram sessions use session_id based on chat_id.',
    '{"adapter": "telegram.py", "library": "python-telegram-bot", "model": "glm-5 (primary)", "fallbacks": ["gemini-2.5-flash", "gemini-2.0-flash", "groq", "openrouter-free"], "toolset": ["memory", "web", "terminal", "file", "browser", "delegation", "tts", "vision", "session_search", "todo", "skills", "kb_mcp"], "token_tracking": "NOT wired to governor (only dashboard chat)"}',
    'high',
    'active'
) ON CONFLICT (item_type, name) DO UPDATE SET
    title = EXCLUDED.title,
    summary = EXCLUDED.summary,
    metadata = EXCLUDED.metadata,
    priority = EXCLUDED.priority,
    status = EXCLUDED.status;

-- ============================================================
-- 4. CHAT USAGE ROI FLOW
-- ============================================================
INSERT INTO kb_knowledge_items (repo_id, item_type, name, title, summary, metadata, priority, status)
VALUES (
    'vibepilot',
    'pattern',
    'chat_usage_roi_flow',
    'Chat Usage to ROI Calculator Flow — Dashboard Chat Token Cost Tracking',
    E'End-to-end data flow for tracking dashboard chat token costs in the ROI calculator. '
    '\n\n1. User chats in dashboard -> VibeFlow frontend POSTs to Hermes http://localhost:8642/v1/runs '
    '2. Hermes processes via gemini-2.5-flash agent, tracks session_prompt_tokens and session_completion_tokens '
    '3. After SSE stream completes, Hermes extracts usage from agent object '
    '4. Hermes POSTs to governor webhook: http://localhost:8080/api/chat/usage '
    '   Payload: {session_id, model_id, tokens_in, tokens_out, token_source} '
    '5. Governor handleChatUsage (chat_usage.go) calls record_chat_usage RPC '
    '6. RPC calculates cost via calc_run_costs(model_id, tokens_in, tokens_out) '
    '7. Stores in chat_usage table (id, session_id, model_id, tokens_in, tokens_out, theoretical_cost_usd, created_at) '
    '8. Also calls increment_lifetime_counters to keep global counters accurate '
    '9. Dashboard /api/dashboard queries chat_usage table (limit 500, order by created_at desc) '
    '10. get_full_roi_report includes chat_usage_summary (aggregated by model_id) '
    '11. Frontend ROI panel displays chat costs alongside task pipeline costs '
    '\n\nTelegram chat usage is NOT yet wired into this flow (only dashboard chat). '
    '\n\nKey files: '
    'api_server.py (Hermes: usage extraction + webhook call), '
    'chat_usage.go (governor: endpoint), '
    'rpc.go (allowlist entry), '
    'server.go (route + dashboard query), '
    'migrations/136 (table + record_chat_usage RPC), '
    'migrations/137 (get_full_roi_report update). '
    '\n\nGovernor webhook URL: configured via GOVERNOR_WEBHOOK_URL env var (default http://localhost:8080). '
    'RPC allowlist must include record_chat_usage (governor/internal/db/rpc.go).',
    '{"data_flow": ["Hermes SSE", "POST /api/chat/usage", "record_chat_usage RPC", "chat_usage table", "get_full_roi_report", "dashboard frontend"], "tables": ["chat_usage"], "rpcs": ["record_chat_usage", "get_chat_cost_summary", "get_full_roi_report", "calc_run_costs"]}',
    'medium',
    'active'
) ON CONFLICT (item_type, name) DO UPDATE SET
    title = EXCLUDED.title,
    summary = EXCLUDED.summary,
    metadata = EXCLUDED.metadata,
    priority = EXCLUDED.priority,
    status = EXCLUDED.status;
