# Courier Agent Pipeline Test

## Overview
Verify that the courier agent pipeline works end-to-end for a small research task that requires no codebase access or code changes. This test confirms the full lifecycle: webhook → plan → supervisor → task → dispatch → courier → result delivery.

## Goal
Execute a single small research question via the courier agent system to verify that browser-based AI platforms can be used for zero-code tasks. The task must be completable by visiting a web page and reading published information — no account login, no API keys, no code changes.

## Tasks
1. **Research task**: Visit the Python Software Foundation website (python.org) and determine what the latest stable Python 3 release version is as of July 2026. Output the version number and release date as a brief markdown note.

## Success Criteria
- A plan is created from the PRD commit via GitHub webhook
- The planner agent extracts the task and creates a task record
- The task is dispatched to a courier agent
- The courier agent returns a result with the Python version information
- No code changes are made to the VibePilot or governor codebase
- The result is saved to the project knowledgebase
