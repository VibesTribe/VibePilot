# FREE API Keys - What You Actually Have (April 2026)
# Signups done from regular Chrome (not CDP window)

=================================================================
YOUR WORKING FREE KEYS (stored in ~/.hermes/.env and ~/.governor_env)
=================================================================

1. GOOGLE AI STUDIO (Gemini) -- PRIMARY
   - Key: stored in ~/.hermes/.env as GEMINI_API_KEY/GOOGLE_API_KEY
   - Free models: gemini-2.5-flash, gemini-2.0-flash, gemini-3-flash-preview
   - Limits: ~15 RPM, 1,500 RPD, 1M TPM
   - Context: 1M tokens
   - Status: WORKING, this is your current model

2. GROQ -- SPEED TIER
   - Key: stored in ~/.hermes/.env as GROQ_API_KEY
   - Free models:
     * llama-3.1-8b-instant: 30 RPM, 14.4K RPD, 6K TPM, 500K TPD
     * compound / compound-mini: 30 RPM, 250 RPD, 70K TPM
     * gpt-oss-120b / gpt-oss-20b (free tier)
     * llama-4-scout (free tier)
     * qwen-3-32b (free tier)
     * Orpheus TTS, Whisper STT
   - Status: WORKING, configured as fallback #1

3. NVIDIA NIM -- VERIFIED WORKING
   - Key: stored in ~/.hermes/.env as NVIDIA_API_KEY
   - 131 models available including:
     * deepseek-ai/deepseek-v3.2 (latest DeepSeek)
     * meta/llama-4-maverick-17b-128e-instruct
     * meta/llama-3.1-405b-instruct
     * qwen/qwen3-coder-480b-a35b-instruct
     * mistralai/mistral-large-3-675b-instruct
     * google/gemma-4-31b-it
     * google/gemma-3n-e4b-it (multimodal)
   - Tested: gemma-3-4b-it responded correctly
   - Status: WORKING, needs wiring into Hermes config

4. OPENROUTER -- CAREFUL
   - Key: stored in ~/.hermes/.env as OPENROUTER_API_KEY
   - Balance: -17 cents (negative!)
   - ONLY use models with :free suffix ($0 cost)
   - Free models: gemma-4-31b, nemotron-3-super, qwen3-coder-480b,
     glm-4.5-air, gpt-oss-120b/20b, minimax-m2.5, llama-3.3-70b
   - Status: CONFIGURED for free models only as fallback

=================================================================
NOT FREE (do not use):
=================================================================

- SiliconFlow: ALL models paid (key returned "Api key is invalid" -- needs credit deposit)
- SambaNova: ALL models are paid ($0.10-$7.00 per million tokens)
- Together AI: $5 signup credits only, then paid
- Alibaba/Qwen: requires full address + payment setup
- ZAI/GLM: subscription ending May 1

=================================================================
HERMES FALLBACK ORDER:
=================================================================

Primary:  Gemini 2.5 Flash (Google AI Studio, free)
  |
  v rate limited
Fallback: Groq llama-3.1-8b-instant (free, 30 RPM)
  |
  v rate limited
Fallback: Groq compound (free, agentic)
  |
  v rate limited
Fallback: NVIDIA NIM gemma-3-4b-it (free, 131 models)
  |
  v rate limited
Fallback: NVIDIA NIM llama-3.3-70b-instruct
  |
  v rate limited
Fallback: OpenRouter gemma-4-31b:free
  |
  v rate limited
Fallback: OpenRouter nemotron-3-super:free
  |
  v rate limited
Fallback: OpenRouter qwen3-coder-480b:free
  |
  v all cloud failed
STOP: No local fallback. x220 too weak for useful local inference.
Recovery: Wait for rate limit reset, or use GitHub+Supabase DR to rebuild elsewhere.
