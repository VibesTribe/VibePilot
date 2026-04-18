# PRD: LLM Wiki — Knowledge Base for Large Language Models

## Overview
A Karpathy-style reference wiki for large language models. Single-page, fast, searchable. Think of it as "the missing manual" for every notable LLM — what it is, how it works, what it's good at, what it costs, where to try it.

## Problem
The LLM landscape changes weekly. There's no single, clean, up-to-date reference. People piece together info from Twitter threads, blog posts, and scattered docs. Model cards are marketing, not honest assessments.

## Target User
- Developers choosing models for projects
- Researchers tracking the landscape
- People new to AI who want to understand what's out there

## Core Features

### 1. Model Directory
- Grid/list of every notable LLM with key stats
- Filters: provider, size, open/closed, cost tier, capabilities
- Sort by: release date, parameter count, cost, benchmark scores
- Each card shows: name, provider, parameter count, context window, pricing, open/closed source

### 2. Model Detail Page
- Architecture summary (transformer variant, MoE, etc.)
- Training data highlights (what we know)
- Benchmark scores (MMLU, HumanEval, GPQA, etc.) in a simple table
- Pricing breakdown (input/output per 1M tokens)
- Known strengths and weaknesses (honest, not marketing)
- API access options (which providers offer it)
- Release date and version history
- Links to paper, model card, try-it-now URLs

### 3. Comparison Tool
- Side-by-side comparison of 2-4 models
- Highlight differences in: benchmarks, pricing, context, capabilities
- "Which should I use?" recommendation based on use case

### 4. Provider Directory  
- Each provider (OpenAI, Anthropic, Google, Meta, Mistral, Deepseek, etc.)
- What models they offer
- Pricing structure
- Free tier details
- API quality/reliability notes

### 5. Search
- Instant search across all models, providers, capabilities
- Natural language: "cheapest model for coding" or "open source with 128k context"

## Tech Stack
- Static site (Vite + React or plain HTML/JS)
- Data in JSON files (models.json, providers.json)
- Deployed on GitHub Pages or Cloudflare Pages
- Zero backend needed — all data is static JSON

## Data Structure

### models.json
```json
{
  "id": "gpt-4o",
  "name": "GPT-4o",
  "provider": "openai",
  "params": "unknown (estimated ~200B)",
  "context": 128000,
  "open_source": false,
  "released": "2024-05-13",
  "pricing": {
    "input_per_1m": 2.50,
    "output_per_1m": 10.00
  },
  "benchmarks": {
    "mmlu": 88.7,
    "humaneval": 90.2
  },
  "strengths": ["general purpose", "coding", "reasoning"],
  "weaknesses": ["cost", "closed source"],
  "access": ["openai-api", "azure", "openrouter"]
}
```

## Constraints
- MIT or Apache license only
- No server, no database, no auth
- Must load fast on mobile
- Accessible (WCAG AA)
- Works offline after first load (service worker)

## Success Criteria
- Covers 50+ models at launch
- Loads in <2 seconds
- Searchable and filterable
- Honest assessments (no marketing fluff)
- Updated at least weekly

## Scope for V1
- Model directory with cards
- Model detail pages
- Basic search
- Provider list
- Comparison tool (stretch goal)
- Mobile responsive

## NOT in V1
- User accounts
- Reviews/comments
- API integration testing
- Benchmark automation
- Multi-language
