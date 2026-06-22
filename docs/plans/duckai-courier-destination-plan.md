# PLAN: Duck.ai Courier Destination

## Overview
Integrate Duck.ai as a new web courier destination into VibePilot, providing access to advanced models like GPT-5 mini and Claude Haiku 4.5 without requiring user login. This will enhance model diversity and automation reliability within the courier system, adhering to strict hardware, API budget, and privacy constraints.

## Tasks

### T001: Add Duck.ai connector configuration
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none
**Target Files:** governor/config/connectors.json

#### Prompt Packet
```markdown
# TASK: T001 - Add Duck.ai connector configuration

## Context
This task involves adding a new entry for Duck.ai to the `connectors.json` file to integrate it as a web courier destination in VibePilot.

## What to Build
Modify the `connectors.json` file to include a new connector configuration for Duck.ai.

- **ID:** `duckai-web`
- **Name:** `Duck.ai Web`
- **URL:** `https://duck.ai`
- **Type:** `web`
- **Auth:** `none`
- **Models:** `["gpt-5-mini", "gpt-4o-mini", "gpt-oss-120b", "llama-4-scout", "claude-haiku-4-5", "mistral-small-4"]`
- **Notes:** `Zero-login platform with access to frontier models and file upload capabilities.`

Ensure the new entry adheres to the existing JSON structure and formatting.

## Files
- `governor/config/connectors.json` - The file to be modified.
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [],
  "files_modified": ["governor/config/connectors.json"],
  "tests_written": []
}
```

### T002: Create Playwright automation script for Duck.ai
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T001
**Target Files:** src/courier/duckai-web.ts

#### Prompt Packet
```markdown
# TASK: T002 - Create Playwright automation script for Duck.ai

## Context
This task requires the creation of a Playwright automation script (`duckai-web.ts`) that implements the `BaseCourier` interface for the Duck.ai web platform. This script will handle navigation, interaction, and response parsing for Duck.ai.

## What to Build
Create the file `src/courier/duckai-web.ts`. This script should:

1.  **Extend `BaseCourier`:** Implement the methods defined in `BaseCourier` (navigate, login, chat, parseResponse, supportsFileUpload, uploadFile).
2.  **`navigate()`:** Implement logic to go to `https://duck.ai`. Use `await page.goto('https://duck.ai');`.
3.  **`login()`:** Implement a no-operation login. This should simply return `Promise.resolve()` as Duck.ai does not require login.
4.  **`chat(message: string, options?: ChatOptions)`:**
    *   If `options.files` is provided, implement file upload logic using Duck.ai's UI.
    *   Fill the main chat input field with the `message`.
    *   Click the send button. Use the selector `[data-testid="send-button"]` or a similar robust selector if this one fails. Wait for the response element to appear.
    *   Return the text content of the AI's response.
5.  **`parseResponse()`:** Extract the AI's response text from the relevant DOM elements (e.g., response divs). Ensure this is robust against minor UI changes.
6.  **`supportsFileUpload()`:** Return `true` as Duck.ai supports file uploads (images, PDFs).
7.  **`uploadFile(file: File)`:** Implement logic to trigger the file input element and handle file uploads, waiting for completion.
8.  **Error Handling:** Implement specific courier error types (`CourierNavigationError`, `CourierElementError`, `CourierParseError`, `CourierUploadError`) that extend `CourierError` for navigation, element interaction, response parsing, and file upload failures respectively.
9.  **Follow Existing Patterns:** Adhere strictly to the structure, coding style, and error handling patterns found in other web courier scripts (e.g., `huggingchat-web.ts`).

## Files
- `src/courier/duckai-web.ts` - The new Playwright automation script.
- `src/courier/base.ts` - Reference for the `BaseCourier` interface and `ChatOptions`.
- `src/courier/courier_errors.ts` - Reference for custom courier error types.
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": ["src/courier/duckai-web.ts"],
  "files_modified": [],
  "tests_written": []
}
```

### T003: Ensure zero-login workflow functions correctly
**Confidence:** 0.99
**Category:** testing
**Dependencies:** T002
**Target Files:** src/courier/duckai-web.ts

#### Prompt Packet
```markdown
# TASK: T003 - Ensure zero-login workflow functions correctly

## Context
This task verifies that the Duck.ai courier script operates without requiring any authentication steps, confirming the 'zero-login' requirement.

## What to Build
Review and test the `src/courier/duckai-web.ts` script to confirm:

1.  **`navigate()` Method:** Ensure it directly navigates to `https://duck.ai` without redirects or intermediate authentication pages.
2.  **`login()` Method:** Verify that the `login()` method in `duckai-web.ts` is implemented as a no-operation (e.g., returns `Promise.resolve()` immediately) and does not attempt any authentication actions.
3.  **No Authentication UI Handling:** Confirm that the `chat()` and related methods do not contain logic for handling login forms, modals, pop-ups, or redirects related to authentication.
4.  **Acceptance Criteria:** The script successfully performs a chat operation after navigation without any prompts or failures related to authentication.

## Files
- `src/courier/duckai-web.ts` - The script to be verified.
```

#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": [],
  "files_modified": [],
  "tests_written": []
}
```

### T004: Support all 6 available Duck.ai models via configuration
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T001, T002
**Target Files:** governor/config/connectors.json, src/courier/duckai-web.ts

#### Prompt Packet
```markdown
# TASK: T004 - Support all 6 available Duck.ai models via configuration

## Context
This task ensures that the Duck.ai connector configuration and its Playwright script correctly expose and allow selection of all six free models available on Duck.ai. This is crucial for task routing flexibility and utilizing the platform's full capabilities.

## What to Build

1.  **Update `connectors.json` (T001 dependency):**
    *   Verify that the `models` array in the `duckai-web` connector entry in `governor/config/connectors.json` accurately lists all six models: `["gpt-5-mini", "gpt-4o-mini", "gpt-oss-120b", "llama-4-scout", "claude-haiku-4-5", "mistral-small-4"]`.
    *   Ensure a sensible default model is selected if none is explicitly specified in a task request (e.g., `gpt-4o-mini`). This default should be handled by the routing logic or the courier itself.

2.  **Implement Model Selection in `duckai-web.ts` (T002 dependency):**
    *   Modify the `chat()` method in `src/courier/duckai-web.ts` to accept an optional `options.model` parameter.
    *   If `options.model` is provided, implement logic to select and configure Duck.ai's UI to use that specific model. This might involve interacting with a model selection dropdown or similar UI element.
    *   If `options.model` is not provided, use the default model specified in `connectors.json`.
    *   Ensure that the script correctly handles requests to use each of the six listed models.

## Files
- `governor/config/connectors.json` - To verify and potentially update the model list and default.
- `src/courier/duckai-web.ts` - To implement model selection logic in the `chat` method.
```

#### Expected Output
```json
{
  "task_id": "T004",
  "files_created": [],
  "files_modified": [
    "governor/config/connectors.json",
    "src/courier/duckai-web.ts"
  ],
  "tests_written": []
}
```

### T005: Handle file upload capabilities (images, PDFs)
**Confidence:** 0.95
**Category:** coding
**Dependencies:** T002
**Target Files:** src/courier/duckai-web.ts

#### Prompt Packet
```markdown
# TASK: T005 - Handle file upload capabilities (images, PDFs)

## Context
This task adds support for Duck.ai's file upload feature, allowing users to send images and PDFs within their chat messages. This enhances the courier's utility for multimodal tasks.

## What to Build
Modify the `src/courier/duckai-web.ts` script to implement file upload functionality:

1.  **Update `chat()` method signature:** Ensure the `ChatOptions` interface includes an optional `files?: File[]` property.
2.  **Implement file upload logic:**
    *   Inside the `chat()` method, check if `options.files` is provided and non-empty.
    *   If files are present, locate and interact with Duck.ai's file input element (e.g., a hidden `<input type='file'>` element that's triggered by a button).
    *   Programmatically select the provided files.
    *   Wait for the file upload process to complete on Duck.ai's side (e.g., by monitoring for a specific UI indicator or network request completion).
3.  **Implement `supportsFileUpload()`:** Ensure this method in `duckai-web.ts` returns `true`.
4.  **Implement `uploadFile()` (if separate):** If Duck.ai requires a distinct step for file upload after selection, implement the `uploadFile(file: File)` method to handle this. Otherwise, integrate file selection and upload within the `chat()` method.
5.  **Error Handling:** Add specific error handling for file upload failures, potentially throwing a `CourierUploadError` with relevant details about the file and the error encountered.

## Files
- `src/courier/duckai-web.ts` - The script to be modified.
```

#### Expected Output
```json
{
  "task_id": "T005",
  "files_created": [],
  "files_modified": ["src/courier/duckai-web.ts"],
  "tests_written": []
}
```

### T006: Validate zero-cost operation
**Confidence:** 1.0
**Category:** testing
**Dependencies:** T001, T002
**Target Files:** N/A (Verification task)

#### Prompt Packet
```markdown
# TASK: T006 - Validate zero-cost operation

## Context
This task verifies that integrating Duck.ai as a courier destination adheres to the strict `0/month` API budget constraint. It ensures that no paid services, external API calls, or token consumption are involved beyond the free tier.

## What to Build
Perform the following verification steps:

1.  **Review `connectors.json`:** Confirm that the `duckai-web` connector configuration (added in T001) does not specify any paid API keys, external service integrations, or usage limits that would incur costs.
2.  **Analyze `duckai-web.ts` script:**
    *   Scrutinize the `src/courier/duckai-web.ts` script for any calls to external APIs, tokenizers, or services that are not part of Duck.ai's free web interface.
    *   Ensure all interactions are limited to navigating Duck.ai's web UI and interacting with its DOM elements.
3.  **Confirm Duck.ai's Free Tier:** Based on the research, Duck.ai uses its own free web interface and provides free models. Confirm that the implementation relies solely on this free web interface.
4.  **Acceptance Criteria:** No part of the implementation requires or consumes paid API resources, tokens, or services. The operation is fully contained within Duck.ai's free web tier.

## Files
- `governor/config/connectors.json` - For review of connector configuration.
- `src/courier/duckai-web.ts` - For review of script logic.
```

#### Expected Output
```json
{
  "task_id": "T006",
  "files_created": [],
  "files_modified": [],
  "tests_written": []
}
```
