# PLAN: Personal Fitness Tracking App (FitCompanion)

## Overview
This plan outlines the development of "FitCompanion", a personal fitness tracking application. The application will feature daily activity tracking, customizable workout plans, nutrition advice, and progress tracking with analytics. The tech stack will be React Native for the frontend, Node.js with Express for the backend, and MongoDB for the database. The architecture will be modular to facilitate maintenance and updates. Security requirements include user data encryption and secure authentication.

## Tasks

### T001: Extract Constraints and Operating Envelope
**Confidence:** 0.98
**Category:** planning
**Dependencies:** none
**Target Files:** N/A

#### Prompt Packet
```markdown
# TASK: T001 - Extract Constraints and Operating Envelope

## Context
This task is the first step in the 6-phase pipeline for generating a PRD. It involves identifying and documenting all hard constraints, soft preferences, forbidden patterns, and reversibility requirements for the "FitCompanion" personal fitness tracking app.

## What to Build
Extract the following information from the provided PRD content and any relevant context:

- **Hard Constraints:** Any non-negotiable limitations (e.g., API budget, hardware limitations).
- **Soft Preferences:** Desired but not mandatory characteristics (e.g., preferred tech stack elements if not strictly defined).
- **Forbidden Patterns:** Any approaches or technologies to avoid.
- **Reversibility:** How easily can changes be undone or rolled back?
- **Project Scope:** Overall goals and boundaries of the application.
- **User Needs:** Key user requirements that must be met.
- **Functionalities:** High-level features to be implemented.

## Files
- N/A - This task involves analysis and documentation, not file creation or modification.
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [],
  "tests_written": [],
  "constraints": {
    "hard": [
      "API budget: $2.81",
      "Hardware: X220 (for potential development/testing constraints, though not directly in app usage)"
    ],
    "soft": [
      "Prioritize free tiers first for API usage."
    ],
    "forbidden_patterns": [],
    "reversibility": "Changes should be designed with rollback capabilities where feasible."
  },
  "project_scope": "Development of a personal fitness tracking application.",
  "user_needs": [
    "Track daily activity",
    "Customize workout plans",
    "Receive nutrition advice",
    "Track progress and analytics"
  ],
  "functionalities": [
    "Daily activity tracking",
    "Customizable workout plans",
    "Nutrition advice",
    "Progress tracking and analytics"
  ]
}
```

### T002: Atomize Requirements and Assign IDs
**Confidence:** 0.98
**Category:** planning
**Dependencies:** T001
**Target Files:** N/A

#### Prompt Packet
```markdown
# TASK: T002 - Atomize Requirements and Assign IDs

## Context
Following the extraction of constraints, this task involves breaking down the "FitCompanion" app's high-level features into atomic, testable requirements. Each requirement needs to be assigned a unique ID, priority, acceptance criteria, and failure criteria.

## What to Build
For each core feature identified in the PRD content and T001's output, define atomic requirements. Assign the following to each requirement:

- **FR-XXX ID:** A unique identifier (e.g., FR-001, FR-002).
- **Priority:** P1 (must-have), P2 (should-have), P3 (nice-to-have).
- **Acceptance Criteria:** Specific conditions that must be met for the requirement to be considered complete.
- **Failure Criteria:** Conditions under which the requirement is considered to have failed.

**Core Features to Atomize:**
- Daily activity tracking
- Customizable workout plans
- Nutrition advice
- Progress tracking and analytics
- User data encryption
- Secure authentication

## Files
- N/A - This task involves analysis and documentation, not file creation or modification.
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": [],
  "tests_written": [],
  "requirements": [
    {
      "id": "FR-001",
      "feature": "Daily Activity Tracking",
      "description": "Allow users to log daily steps and active minutes.",
      "priority": "P1",
      "acceptance_criteria": [
        "User can manually input daily step count.",
        "User can manually input daily active minutes.",
        "Logged data is stored and retrievable."
      ],
      "failure_criteria": [
        "Inability to log or retrieve daily activity data."
      ]
    },
    {
      "id": "FR-002",
      "feature": "Daily Activity Tracking",
      "description": "Automatically track steps via device integration (e.g., HealthKit/Google Fit).",
      "priority": "P2",
      "acceptance_criteria": [
        "App integrates with HealthKit/Google Fit.",
        "Step data is automatically synced and displayed."
      ],
      "failure_criteria": [
        "Automatic step tracking fails or data is inaccurate."
      ]
    },
    {
      "id": "FR-003",
      "feature": "Customizable Workout Plans",
      "description": "Allow users to create and save custom workout routines.",
      "priority": "P1",
      "acceptance_criteria": [
        "User can define exercises, sets, reps, and rest times.",
        "User can save and name custom workout plans.",
        "Saved plans can be loaded and executed."
      ],
      "failure_criteria": [
        "Inability to save, load, or define custom workouts."
      ]
    },
    {
      "id": "FR-004",
      "feature": "Customizable Workout Plans",
      "description": "Provide pre-defined workout templates (e.g., beginner, intermediate, advanced).",
      "priority": "P2",
      "acceptance_criteria": [
        "App includes a library of pre-defined workout templates.",
        "Users can select and start pre-defined workouts."
      ],
      "failure_criteria": [
        "No pre-defined workout templates are available."
      ]
    },
    {
      "id": "FR-005",
      "feature": "Nutrition Advice",
      "description": "Provide basic nutritional guidance and calorie tracking.",
      "priority": "P2",
      "acceptance_criteria": [
        "Users can log meals and view estimated calorie intake.",
        "App offers general healthy eating tips."
      ],
      "failure_criteria": [
        "Inability to log meals or inaccurate calorie estimates."
      ]
    },
    {
      "id": "FR-006",
      "feature": "Progress Tracking and Analytics",
      "description": "Display user progress over time through charts and summaries.",
      "priority": "P1",
      "acceptance_criteria": [
        "Visualizations for steps, active minutes, and workout frequency are displayed.",
        "Historical data is accessible."
      ],
      "failure_criteria": [
        "Progress charts are missing or inaccurate."
      ]
    },
    {
      "id": "FR-007",
      "feature": "User Data Encryption",
      "description": "Encrypt sensitive user data at rest.",
      "priority": "P1",
      "acceptance_criteria": [
        "User data in the database is encrypted."
      ],
      "failure_criteria": [
        "User data is stored in plain text."
      ]
    },
    {
      "id": "FR-008",
      "feature": "Secure Authentication",
      "description": "Implement secure user registration and login process.",
      "priority": "P1",
      "acceptance_criteria": [
        "Users can register with email and password.",
        "Users can log in securely.",
        "Password reset functionality is available."
      ],
      "failure_criteria": [
        "Insecure authentication methods are used.",
        "User registration or login fails."
      ]
    }
  ]
}
```

### T003: Conduct Market Research and Gap Analysis
**Confidence:** 0.96
**Category:** research
**Dependencies:** T001, T002
**Target Files:** N/A

#### Prompt Packet
```markdown
# TASK: T003 - Conduct Market Research and Gap Analysis

## Context
With the constraints and atomic requirements defined, this phase focuses on understanding the competitive landscape for the "FitCompanion" fitness tracking app. This involves identifying similar products, their strengths, weaknesses, and market gaps.

## What to Build
Conduct market research on existing personal fitness tracking applications. Identify:

- **Similar Products:** List 3-5 popular apps in the market.
- **Strengths:** What do these apps do well?
- **Weaknesses:** What are their shortcomings or areas for improvement?
- **Market Gaps:** What features or user needs are currently underserved by existing solutions?
- **Minimum Viable Feature Set (MVFS):** Based on research, what is the absolute minimum set of features required for a successful launch?
- **Positioning:** How should FitCompanion be positioned in the market to stand out?

## Files
- N/A - This task involves research and analysis, output will be documented in the plan.
```

#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": [],
  "tests_written": [],
  "market_research": {
    "similar_products": [
      "MyFitnessPal",
      "Strava",
      "Fitbit App"
    ],
    "strengths": {
      "MyFitnessPal": "Extensive food database, calorie tracking, community support.",
      "Strava": "Excellent activity tracking for runners and cyclists, social features, performance analytics.",
      "Fitbit App": "Seamless hardware integration, holistic health view (sleep, activity)."
    },
    "weaknesses": {
      "MyFitnessPal": "Can feel overwhelming, UI can be dated, workout features are basic.",
      "Strava": "Less focus on nutrition, can be intimidating for beginners, premium features can be costly.",
      "Fitbit App": "Primarily tied to Fitbit hardware, workout customization can be limited."
    },
    "market_gaps": [
      "Integrated, user-friendly workout planning with real-time form guidance.",
      "Personalized nutrition plans that adapt to activity levels dynamically.",
      "A simpler, more intuitive interface for beginners that doesn't sacrifice advanced features.",
      "Focus on holistic well-being beyond just fitness (e.g., stress management, mindful activity)."
    ],
    "minimum_viable_feature_set": [
      "Basic daily activity logging (steps, active minutes)",
      "Manual workout logging",
      "User registration and secure login",
      "Basic progress display (e.g., weekly step summary)"
    ],
    "positioning": "FitCompanion will position itself as an all-in-one, beginner-friendly fitness companion that integrates activity, nutrition, and personalized workout planning with a clean, intuitive interface, focusing on achievable progress and holistic well-being."
  }
}
```

### T004: Design Logical Architecture and Stress-Test Constraints
**Confidence:** 0.97
**Category:** architecture
**Dependencies:** T001, T003
**Target Files:** N/A

#### Prompt Packet
```markdown
# TASK: T004 - Design Logical Architecture and Stress-Test Constraints

## Context
This task focuses on designing the logical architecture for "FitCompanion" and ensuring it aligns with the previously identified constraints, particularly the hardware (X220) and API budget ($2.81), and prioritizing free tiers.

## What to Build

1.  **Design Logical Architecture:** Propose a high-level logical architecture for FitCompanion. Specify the main components and their interactions.
    *   Frontend: React Native
    *   Backend: Node.js with Express
    *   Database: MongoDB
    *   Key modules/services (e.g., Authentication, Activity Tracking, Workout Planning, Nutrition, Analytics).

2.  **Stress-Test Against Constraints:** Analyze the proposed architecture against the identified resource constraints:
    *   **API Budget ($2.81):** Estimate potential API calls and their costs. Identify areas where free tiers can be leveraged or optimized to stay within budget.
    *   **Hardware (X220):** Consider how the architecture would perform on a typical user device (e.g., mobile phone) and if the backend/database choices are resource-efficient. (Note: X220 is typically a development machine constraint, implying efficient backend processing is desired).
    *   **Free Tiers:** Identify specific services or API endpoints where free tiers are applicable and should be prioritized.

## Files
- N/A - This task involves architectural design and analysis.
```

#### Expected Output
```json
{
  "task_id": "T004",
  "files_created": [],
  "tests_written": [],
  "architecture": {
    "description": "A modular architecture comprising a React Native frontend, Node.js/Express backend, and MongoDB database.",
    "components": [
      "Frontend (React Native)",
      "Backend API (Node.js/Express)",
      "Authentication Service",
      "Activity Tracking Module",
      "Workout Planning Module",
      "Nutrition Module",
      "Analytics Module",
      "Database (MongoDB)"
    ],
    "interactions": "Frontend communicates with the Backend API via RESTful endpoints. Backend services interact with MongoDB."
  },
  "constraint_analysis": {
    "api_budget_analysis": {
      "estimated_calls": "(Detailed estimation to be done in later phases, but initial focus on minimizing redundant calls)",
      "budget_strategy": "Prioritize efficient data fetching, batch requests where possible, leverage free tier APIs for non-critical data (e.g., general nutrition facts lookup if a free API exists).",
      "free_tier_opportunities": [
        "Public nutrition databases (if available as free API)",
        "Potentially some analytics reporting services (if external usage is minimal)"
      ]
    },
    "hardware_performance": {
      "assessment": "React Native is generally performant on mobile devices. Node.js backend is efficient for I/O bound tasks. MongoDB scales well. Architecture is designed for modularity, which aids in performance optimization of individual modules.",
      "considerations": "Ensure efficient data serialization and minimize payload sizes for frontend communication."
    }
  }
}
```

### T005: Define Dependency and Interface Contracts
**Confidence:** 0.97
**Category:** planning
**Dependencies:** T004
**Target Files:** N/A

#### Prompt Packet
```markdown
# TASK: T005 - Define Dependency and Interface Contracts

## Context
Following the architectural design, this task focuses on defining the precise interfaces and contracts between the "FitCompanion" components, including data shapes, error contracts, side effects, and recovery strategies.

## What to Build
Define the key interfaces and contracts for the "FitCompanion" application:

1.  **Component Interfaces:** Specify how major components interact (e.g., Frontend <-> Backend API).
    *   **Data Shapes:** Define the expected JSON request and response formats for key API endpoints.
    *   **Error Contracts:** Define a consistent error response format (e.g., `{ "error": { "code": "AUTH_FAILED", "message": "Invalid credentials" } }`).
    *   **Side Effects:** Identify critical side effects of API calls (e.g., data modification, external notifications).
    *   **Recovery Strategies:** Outline basic strategies for handling common errors (e.g., retries for transient network issues, user feedback for invalid input).

2.  **Internal Module Contracts (High-Level):** For critical internal modules (e.g., Authentication, Activity Tracking), briefly describe their input/output contracts.

## Files
- N/A - This task involves defining contracts and documentation, not file creation.
```

#### Expected Output
```json
{
  "task_id": "T005",
  "files_created": [],
  "tests_written": [],
  "interface_contracts": {
    "frontend_backend_api": {
      "data_shapes": {
        "user_registration_request": {
          "email": "string",
          "password": "string"
        },
        "user_registration_response": {
          "user_id": "string"
        },
        "login_request": {
          "email": "string",
          "password": "string"
        },
        "login_response": {
          "token": "string"
        },
        "log_activity_request": {
          "activity_type": "string",
          "value": "number",
          "unit": "string"
        },
        "log_activity_response": {
          "success": "boolean"
        },
        "get_progress_response": {
          "steps_history": "array<object>",
          "workouts_completed": "integer"
        }
      },
      "error_contract": {
        "code": "string (e.g., AUTH_FAILED, INVALID_INPUT, SERVER_ERROR)",
        "message": "string (human-readable error description)"
      },
      "side_effects": [
        "User registration: Creates user record.",
        "Log activity: Updates user's activity log.",
        "Login: Issues authentication token."
      ],
      "recovery_strategies": [
        "For 4xx errors (client-side): Display user-friendly error messages.",
        "For 5xx errors (server-side): Implement retry logic with exponential backoff for transient issues. Notify user of persistent errors.",
        "Network errors: Display connection status and attempt to retry."
      ]
    },
    "internal_modules_high_level": {
      "authentication_service": {
        "description": "Handles user registration, login, token generation/validation, and password reset.",
        "input": "Credentials, tokens.",
        "output": "User ID, authentication tokens, password reset tokens."
      },
      "activity_tracking_module": {
        "description": "Logs and retrieves user activity data (steps, active minutes, workouts).
        ",
        "input": "Activity data points.",
        "output": "Aggregated activity data, historical logs."
      }
    }
  }
}
```

### T006: Generate PRD Document and Critique-Revise
**Confidence:** 0.95
**Category:** documentation
**Dependencies:** T001, T002, T003, T004, T005
**Target Files:** docs/prd/personal-fitness-tracking-app.md

#### Prompt Packet
```markdown
# TASK: T006 - Generate PRD Document and Critique-Revise

## Context
This task consolidates all the information gathered and analyzed in previous phases (constraints, requirements, market research, architecture, contracts) into a comprehensive PRD document for the "FitCompanion" app. The PRD will then undergo a critique and revision cycle.

## What to Build

1.  **Generate PRD Document:** Create a PRD document in markdown format at `docs/prd/personal-fitness-tracking-app.md`. The PRD should include:
    *   **Summary:** Brief overview of the app, its name (FitCompanion), type (Personal), core features, tech stack, architecture, and security requirements.
    *   **App Details:** Elaborate on the app's purpose and target audience.
    *   **Core Features:** Detailed descriptions of each feature identified in T002, including FR-XXX IDs, priorities, and acceptance criteria.
    *   **Tech Stack:** Justification for chosen technologies (React Native, Node.js/Express, MongoDB).
    *   **Architecture:** High-level architectural diagram or description (from T004).
    *   **Interface Contracts:** Summary of key API interfaces and error handling (from T005).
    *   **Market Research Summary:** Key findings on competitors and market gaps (from T003).
    *   **Constraints:** All identified constraints (from T001).
    *   **Unknowns Register:** A placeholder section for any unresolved questions.

2.  **Critique and Revise (Initial Pass):** Review the generated PRD for completeness, clarity, and minimalism. Identify any ambiguities or missing information. For this initial pass, assume the critique is internal and focus on self-correction.

## Files
- `docs/prd/personal-fitness-tracking-app.md` - The generated PRD document.
```

#### Expected Output
```json
{
  "task_id": "T006",
  "files_created": ["docs/prd/personal-fitness-tracking-app.md"],
  "tests_written": [],
  "prd_content": "# FitCompanion: Personal Fitness Tracker PRD\n\n## 1. Summary\n\nFitCompanion is a personal fitness tracking application designed to empower users to manage their health and wellness through comprehensive activity tracking, personalized workout plans, nutrition advice, and insightful progress analytics. It aims to provide a user-friendly, all-in-one solution for individuals seeking to improve their fitness journey.\n\n- **App Name:** FitCompanion\n- **Type:** Personal Fitness Tracker\n- **Core Features:** Daily activity tracking, customizable workout plans, nutrition advice, progress tracking and analytics, secure authentication, user data encryption.\n- **Tech Stack:** React Native (Frontend), Node.js/Express (Backend), MongoDB (Database).\n- **Architecture:** Modular design for maintainability and scalability.\n\n## 2. App Details\n\nFitCompanion aims to be the go-to companion for individuals looking to adopt and maintain a healthier lifestyle. It caters to a broad audience, from beginners seeking guidance to fitness enthusiasts wanting to optimize their routines. The app focuses on providing actionable insights and flexible tools to support users' fitness goals.

## 3. Core Features\n\n### 3.1. Daily Activity Tracking (FR-001, FR-002)\n- **Description:** Log and automatically track user's daily physical activity, including steps and active minutes.\n- **Priority:** P1 (Manual logging), P2 (Device integration).\n- **Acceptance Criteria:** \n  - User can manually input daily step count.\n  - User can manually input daily active minutes.\n  - App integrates with HealthKit/Google Fit for automatic step tracking (P2).\n  - Logged data is stored, retrievable, and displayed in progress reports.\n- **Failure Criteria:** Inability to log, retrieve, or display activity data accurately; failure of device integration (P2).\n
### 3.2. Customizable Workout Plans (FR-003, FR-004)\n- **Description:** Allow users to create, save, and execute custom workout routines, and select from pre-defined templates.\n- **Priority:** P1 (Customizable), P2 (Pre-defined templates).\n- **Acceptance Criteria:** \n  - User can define exercises, sets, reps, and rest times.\n  - User can save and name custom workout plans.\n  - App includes a library of pre-defined workout templates.\n  - Saved and pre-defined plans can be loaded and followed.\n- **Failure Criteria:** Inability to save, load, define workouts; absence of pre-defined templates.\n
### 3.3. Nutrition Advice (FR-005)\n- **Description:** Provide basic nutritional guidance, meal logging, and estimated calorie intake tracking.\n- **Priority:** P2.\n- **Acceptance Criteria:** \n  - Users can log meals and view estimated calorie intake.\n  - App offers general healthy eating tips.\n- **Failure Criteria:** Inability to log meals or inaccurate calorie estimates.\n
### 3.4. Progress Tracking and Analytics (FR-006)\n- **Description:** Visualize user progress over time through charts and summaries.\n- **Priority:** P1.\n- **Acceptance Criteria:** \n  - Visualizations for steps, active minutes, and workout frequency are displayed.\n  - Historical data is accessible.\n- **Failure Criteria:** Progress charts are missing or inaccurate.\n
### 3.5. Security Requirements (FR-007, FR-008)\n- **Description:** Implement robust security measures including user data encryption and secure authentication.\n- **Priority:** P1.\n- **Acceptance Criteria:** \n  - User data in the database is encrypted at rest.\n  - Users can register with email/password securely.\n  - Users can log in securely with password reset functionality.\n- **Failure Criteria:** User data stored in plain text; insecure authentication methods; registration/login failures.\n
## 4. Tech Stack\n\n- **Frontend:** React Native - Chosen for cross-platform development efficiency, enabling deployment on both iOS and Android from a single codebase. Its component-based architecture aligns with our modular design goals.\n- **Backend:** Node.js with Express - Selected for its performance in I/O-bound operations, large ecosystem of libraries, and suitability for real-time applications. Its JavaScript base aligns with the frontend, potentially reducing context switching for developers.\n- **Database:** MongoDB - A NoSQL document database chosen for its flexibility in handling evolving data schemas, scalability, and ease of integration with Node.js. It's well-suited for storing user profiles, activity logs, and workout data.\n
## 5. Architecture\n\nFitCompanion will employ a modular architecture. The frontend (React Native) will interact with the backend API (Node.js/Express) via RESTful endpoints. The backend will house distinct modules for Authentication, Activity Tracking, Workout Planning, Nutrition, and Analytics, each interacting with a MongoDB database. This modularity allows for independent development, testing, and scaling of features.\n\n*(High-level diagram to be generated visually in a later step)*\n
## 6. Interface Contracts\n\nKey interfaces include the communication between the React Native frontend and the Node.js/Express backend API. Contracts define data shapes for requests/responses (e.g., user registration, activity logging), a standardized error format (`{ "error": { "code": "...", "message": "..." } }`), and expected side effects. Recovery strategies will involve retries for transient errors and user feedback for invalid inputs.

## 7. Market Research Summary\n\nCompetitor analysis revealed that while apps like MyFitnessPal excel at calorie tracking and Strava at performance analytics, there's a market gap for an integrated, beginner-friendly solution offering comprehensive activity tracking, customizable workout plans, and adaptive nutrition advice within a clean, intuitive interface. FitCompanion aims to fill this gap by providing a holistic approach to fitness.

## 8. Constraints\n\n- **API Budget:** Strict adherence to a $2.81 API budget, prioritizing free tiers where applicable.\n- **Hardware:** Consideration for efficient processing on typical user devices.\n- **Development:** Focus on modularity for maintainability and updates.\n
## 9. Unknowns Register\n\n- Detailed API cost estimations for each function call.\n- Specific choice of free nutrition API, if available and suitable.\n- Fallback API providers for critical functionalities if primary ones exceed budget or hit rate limits.\n
## 10. Revision History\n\n- Initial Draft (Version 1.0)\n"
}
```
",
  "tasks": [
    {
      "task_number": "T001",
      "title": "Extract Constraints and Operating Envelope",
      "category": "planning",
      "confidence": 0.98,
      "dependencies": [],
      "prompt_packet": "# TASK: T001 - Extract Constraints and Operating Envelope

## Context
This task is the first step in the 6-phase pipeline for generating a PRD. It involves identifying and documenting all hard constraints, soft preferences, forbidden patterns, and reversibility requirements for the "FitCompanion" personal fitness tracking app.

## What to Build
Extract the following information from the provided PRD content and any relevant context:

- **Hard Constraints:** Any non-negotiable limitations (e.g., API budget, hardware limitations).
- **Soft Preferences:** Desired but not mandatory characteristics (e.g., preferred tech stack elements if not strictly defined).
- **Forbidden Patterns:** Any approaches or technologies to avoid.
- **Reversibility:** How easily can changes be undone or rolled back?
- **Project Scope:** Overall goals and boundaries of the application.
- **User Needs:** Key user requirements that must be met.
- **Functionalities:** High-level features to be implemented.

## Files
- N/A - This task involves analysis and documentation, not file creation or modification.
",
      "expected_output": {
        "task_id": "T001",
        "files_created": [],
        "tests_written": [],
        "constraints": {
          "hard": [
            "API budget: $2.81",
            "Hardware: X220 (for potential development/testing constraints, though not directly in app usage)"
          ],
          "soft": [
            "Prioritize free tiers first for API usage."
          ],
          "forbidden_patterns": [],
          "reversibility": "Changes should be designed with rollback capabilities where feasible."
        },
        "project_scope": "Development of a personal fitness tracking application.",
        "user_needs": [
          "Track daily activity",
          "Customize workout plans",
          "Receive nutrition advice",
          "Track progress and analytics"
        ],
        "functionalities": [
          "Daily activity tracking",
          "Customizable workout plans",
          "Nutrition advice",
          "Progress tracking and analytics"
        ]
      }
    },
    {
      "task_number": "T002",
      "title": "Atomize Requirements and Assign IDs",
      "category": "planning",
      "confidence": 0.98,
      "dependencies": [
        "T001"
      ],
      "prompt_packet": "# TASK: T002 - Atomize Requirements and Assign IDs

## Context
Following the extraction of constraints, this task involves breaking down the "FitCompanion" app's high-level features into atomic, testable requirements. Each requirement needs to be assigned a unique ID, priority, acceptance criteria, and failure criteria.

## What to Build
For each core feature identified in the PRD content and T001's output, define atomic requirements. Assign the following to each requirement:

- **FR-XXX ID:** A unique identifier (e.g., FR-001, FR-002).
- **Priority:** P1 (must-have), P2 (should-have), P3 (nice-to-have).
- **Acceptance Criteria:** Specific conditions that must be met for the requirement to be considered complete.
- **Failure Criteria:** Conditions under which the requirement is considered to have failed.

**Core Features to Atomize:**
- Daily activity tracking
- Customizable workout plans
- Nutrition advice
- Progress tracking and analytics
- User data encryption
- Secure authentication

## Files
- N/A - This task involves analysis and documentation, not file creation or modification.
",
      "expected_output": {
        "task_id": "T002",
        "files_created": [],
        "tests_written": [],
        "requirements": [
          {
            "id": "FR-001",
            "feature": "Daily Activity Tracking",
            "description": "Allow users to log daily steps and active minutes.",
            "priority": "P1",
            "acceptance_criteria": [
              "User can manually input daily step count.",
              "User can manually input daily active minutes.",
              "Logged data is stored and retrievable."
            ],
            "failure_criteria": [
              "Inability to log or retrieve daily activity data."
            ]
          },
          {
            "id": "FR-002",
            "feature": "Daily Activity Tracking",
            "description": "Automatically track steps via device integration (e.g., HealthKit/Google Fit).",
            "priority": "P2",
            "acceptance_criteria": [
              "App integrates with HealthKit/Google Fit.",
              "Step data is automatically synced and displayed."
            ],
            "failure_criteria": [
              "Automatic step tracking fails or data is inaccurate."
            ]
          },
          {
            "id": "FR-003",
            "feature": "Customizable Workout Plans",
            "description": "Allow users to create and save custom workout routines.",
            "priority": "P1",
            "acceptance_criteria": [
              "User can define exercises, sets, reps, and rest times.",
              "User can save and name custom workout plans.",
              "Saved plans can be loaded and executed."
            ],
            "failure_criteria": [
              "Inability to save, load, or define custom workouts."
            ]
          },
          {
            "id": "FR-004",
            "feature": "Customizable Workout Plans",
            "description": "Provide pre-defined workout templates (e.g., beginner, intermediate, advanced).",
            "priority": "P2",
            "acceptance_criteria": [
              "App includes a library of pre-defined workout templates.",
              "Users can select and start pre-defined workouts."
            ],
            "failure_criteria": [
              "No pre-defined workout templates are available."
            ]
          },
          {
            "id": "FR-005",
            "feature": "Nutrition Advice",
            "description": "Provide basic nutritional guidance and calorie tracking.",
            "priority": "P2",
            "acceptance_criteria": [
              "Users can log meals and view estimated calorie intake.",
              "App offers general healthy eating tips."
            ],
            "failure_criteria": [
              "Inability to log meals or inaccurate calorie estimates."
            ]
          },
          {
            "id": "FR-006",
            "feature": "Progress Tracking and Analytics",
            "description": "Display user progress over time through charts and summaries.",
            "priority": "P1",
            "acceptance_criteria": [
              "Visualizations for steps, active minutes, and workout frequency are displayed.",
              "Historical data is accessible."
            ],
            "failure_criteria": [
              "Progress charts are missing or inaccurate."
            ]
          },
          {
            "id": "FR-007",
            "feature": "User Data Encryption",
            "description": "Encrypt sensitive user data at rest.",
            "priority": "P1",
            "acceptance_criteria": [
              "User data in the database is encrypted."
            ],
            "failure_criteria": [
              "User data is stored in plain text."
            ]
          },
          {
            "id": "FR-008",
            "feature": "Secure Authentication",
            "description": "Implement secure user registration and login process.",
            "priority": "P1",
            "acceptance_criteria": [
              "Users can register with email and password.",
              "Users can log in securely.",
              "Password reset functionality is available."
            ],
            "failure_criteria": [
              "Insecure authentication methods are used.",
              "User registration or login fails."
            ]
          }
        ]
      }
    },
    {
      "task_number": "T003",
      "title": "Conduct Market Research and Gap Analysis",
      "category": "research",
      "confidence": 0.96,
      "dependencies": [
        "T001",
        "T002"
      ],
      "prompt_packet": "# TASK: T003 - Conduct Market Research and Gap Analysis

## Context
With the constraints and atomic requirements defined, this phase focuses on understanding the competitive landscape for the "FitCompanion" fitness tracking app. This involves identifying similar products, their strengths, weaknesses, and market gaps.

## What to Build
Conduct market research on existing personal fitness tracking applications. Identify:

- **Similar Products:** List 3-5 popular apps in the market.
- **Strengths:** What do these apps do well?
- **Weaknesses:** What are their shortcomings or areas for improvement?
- **Market Gaps:** What features or user needs are currently underserved by existing solutions?
- **Minimum Viable Feature Set (MVFS):** Based on research, what is the absolute minimum set of features required for a successful launch?
- **Positioning:** How should FitCompanion be positioned in the market to stand out?

## Files
- N/A - This task involves research and analysis, output will be documented in the plan.
",
      "expected_output": {
        "task_id": "T003",
        "files_created": [],
        "tests_written": [],
        "market_research": {
          "similar_products": [
            "MyFitnessPal",
            "Strava",
            "Fitbit App"
          ],
          "strengths": {
            "MyFitnessPal": "Extensive food database, calorie tracking, community support.",
            "Strava": "Excellent activity tracking for runners and cyclists, social features, performance analytics.",
            "Fitbit App": "Seamless hardware integration, holistic health view (sleep, activity)."
          },
          "weaknesses": {
            "MyFitnessPal": "Can feel overwhelming, UI can be dated, workout features are basic.",
            "Strava": "Less focus on nutrition, can be intimidating for beginners, premium features can be costly.",
            "Fitbit App": "Primarily tied to Fitbit hardware, workout customization can be limited."
          },
          "market_gaps": [
            "Integrated, user-friendly workout planning with real-time form guidance.",
            "Personalized nutrition plans that adapt to activity levels dynamically.",
            "A simpler, more intuitive interface for beginners that doesn't sacrifice advanced features.",
            "Focus on holistic well-being beyond just fitness (e.g., stress management, mindful activity)."
          ],
          "minimum_viable_feature_set": [
            "Basic daily activity logging (steps, active minutes)",
            "Manual workout logging",
            "User registration and secure login",
            "Basic progress display (e.g., weekly step summary)"
          ],
          "positioning": "FitCompanion will position itself as an all-in-one, beginner-friendly fitness companion that integrates activity, nutrition, and personalized workout planning with a clean, intuitive interface, focusing on achievable progress and holistic well-being."
        }
      }
    },
    {
      "task_number": "T004",
      "title": "Design Logical Architecture and Stress-Test Constraints",
      "category": "architecture",
      "confidence": 0.97,
      "dependencies": [
        "T001",
        "T003"
      ],
      "prompt_packet": "# TASK: T004 - Design Logical Architecture and Stress-Test Constraints

## Context
This task focuses on designing the logical architecture for "FitCompanion" and ensuring it aligns with the previously identified constraints, particularly the hardware (X220) and API budget ($2.81), and prioritizing free tiers.

## What to Build

1.  **Design Logical Architecture:** Propose a high-level logical architecture for FitCompanion. Specify the main components and their interactions.
    *   Frontend: React Native
    *   Backend: Node.js with Express
    *   Database: MongoDB
    *   Key modules/services (e.g., Authentication, Activity Tracking, Workout Planning, Nutrition, Analytics).

2.  **Stress-Test Against Constraints:** Analyze the proposed architecture against the identified resource constraints:
    *   **API Budget ($2.81):** Estimate potential API calls and their costs. Identify areas where free tiers can be leveraged or optimized to stay within budget.
    *   **Hardware (X220):** Consider how the architecture would perform on a typical user device (e.g., mobile phone) and if the backend/database choices are resource-efficient. (Note: X220 is typically a development machine constraint, implying efficient backend processing is desired).
    *   **Free Tiers:** Identify specific services or API endpoints where free tiers are applicable and should be prioritized.

## Files
- N/A - This task involves architectural design and analysis.
",
      "expected_output": {
        "task_id": "T004",
        "files_created": [],
        "tests_written": [],
        "architecture": {
          "description": "A modular architecture comprising a React Native frontend, Node.js/Express backend, and MongoDB database.",
          "components": [
            "Frontend (React Native)",
            "Backend API (Node.js/Express)",
            "Authentication Service",
            "Activity Tracking Module",
            "Workout Planning Module",
            "Nutrition Module",
            "Analytics Module",
            "Database (MongoDB)"
          ],
          "interactions": "Frontend communicates with the Backend API via RESTful endpoints. Backend services interact with MongoDB."
        },
        "constraint_analysis": {
          "api_budget_analysis": {
            "estimated_calls": "(Detailed estimation to be done in later phases, but initial focus on minimizing redundant calls)",
            "budget_strategy": "Prioritize efficient data fetching, batch requests where possible, leverage free tier APIs for non-critical data (e.g., general nutrition facts lookup if a free API exists).",
            "free_tier_opportunities": [
              "Public nutrition databases (if available as free API)",
              "Potentially some analytics reporting services (if external usage is minimal)"
            ]
          },
          "hardware_performance": {
            "assessment": "React Native is generally performant on mobile devices. Node.js backend is efficient for I/O bound tasks. MongoDB scales well. Architecture is designed for modularity, which aids in performance optimization of individual modules.",
            "considerations": "Ensure efficient data serialization and minimize payload sizes for frontend communication."
          }
        }
      }
    },
    {
      "task_number": "T005",
      "title": "Define Dependency and Interface Contracts",
      "category": "planning",
      "confidence": 0.97,
      "dependencies": [
        "T004"
      ],
      "prompt_packet": "# TASK: T005 - Define Dependency and Interface Contracts

## Context
Following the architectural design, this task focuses on defining the precise interfaces and contracts between the "FitCompanion" components, including data shapes, error contracts, side effects, and recovery strategies.

## What to Build
Define the key interfaces and contracts for the "FitCompanion" application:

1.  **Component Interfaces:** Specify how major components interact (e.g., Frontend <-> Backend API).
    *   **Data Shapes:** Define the expected JSON request and response formats for key API endpoints.
    *   **Error Contracts:** Define a consistent error response format (e.g., `{ "error": { "code": "AUTH_FAILED", "message": "Invalid credentials" } }`).
    *   **Side Effects:** Identify critical side effects of API calls (e.g., data modification, external notifications).
    *   **Recovery Strategies:** Outline basic strategies for handling common errors (e.g., retries for transient network issues, user feedback for invalid input).

2.  **Internal Module Contracts (High-Level):** For critical internal modules (e.g., Authentication, Activity Tracking), briefly describe their input/output contracts.

## Files
- N/A - This task involves defining contracts and documentation, not file creation.
",
      "expected_output": {
        "task_id": "T005",
        "files_created": [],
        "tests_written": [],
        "interface_contracts": {
          "frontend_backend_api": {
            "data_shapes": {
              "user_registration_request": {
                "email": "string",
                "password": "string"
              },
              "user_registration_response": {
                "user_id": "string"
              },
              "login_request": {
                "email": "string",
                "password": "string"
              },
              "login_response": {
                "token": "string"
              },
              "log_activity_request": {
                "activity_type": "string",
                "value": "number",
                "unit": "string"
              },
              "log_activity_response": {
                "success": "boolean"
              },
              "get_progress_response": {
                "steps_history": "array<object>",
                "workouts_completed": "integer"
              }
            },
            "error_contract": {
              "code": "string (e.g., AUTH_FAILED, INVALID_INPUT, SERVER_ERROR)",
              "message": "string (human-readable error description)"
            },
            "side_effects": [
              "User registration: Creates user record.",
              "Log activity: Updates user's activity log.",
              "Login: Issues authentication token."
            ],
            "recovery_strategies": [
              "For 4xx errors (client-side): Display user-friendly error messages.",
              "For 5xx errors (server-side): Implement retry logic with exponential backoff for transient issues. Notify user of persistent errors.",
              "Network errors: Display connection status and attempt to retry."
            ]
          },
          "internal_modules_high_level": {
            "authentication_service": {
              "description": "Handles user registration, login, token generation/validation, and password reset.",
              "input": "Credentials, tokens.",
              "output": "User ID, authentication tokens, password reset tokens."
            },
            "activity_tracking_module": {
              "description": "Logs and retrieves user activity data (steps, active minutes, workouts).
        ",
              "input": "Activity data points.",
              "output": "Aggregated activity data, historical logs."
            }
          }
        }
      }
    },
    {
      "task_number": "T006",
      "title": "Generate PRD Document and Critique-Revise",
      "category": "documentation",
      "confidence": 0.95,
      "dependencies": [
        "T001",
        "T002",
        "T003",
        "T004",
        "T005"
      ],
      "prompt_packet": "# TASK: T006 - Generate PRD Document and Critique-Revise

## Context
This task consolidates all the information gathered and analyzed in previous phases (constraints, requirements, market research, architecture, contracts) into a comprehensive PRD document for the "FitCompanion" app. The PRD will then undergo a critique and revision cycle.

## What to Build

1.  **Generate PRD Document:** Create a PRD document in markdown format at `docs/prd/personal-fitness-tracking-app.md`. The PRD should include:
    *   **Summary:** Brief overview of the app, its name (FitCompanion), type (Personal), core features, tech stack, architecture, and security requirements.
    *   **App Details:** Elaborate on the app's purpose and target audience.
    *   **Core Features:** Detailed descriptions of each feature identified in T002, including FR-XXX IDs, priorities, and acceptance criteria.
    *   **Tech Stack:** Justification for chosen technologies (React Native, Node.js/Express, MongoDB).
    *   **Architecture:** High-level architectural diagram or description (from T004).
    *   **Interface Contracts:** Summary of key API interfaces and error handling (from T005).
    *   **Market Research Summary:** Key findings on competitors and market gaps (from T003).
    *   **Constraints:** All identified constraints (from T001).
    *   **Unknowns Register:** A placeholder section for any unresolved questions.

2.  **Critique and Revise (Initial Pass):** Review the generated PRD for completeness, clarity, and minimalism. Identify any ambiguities or missing information. For this initial pass, assume the critique is internal and focus on self-correction.

## Files
- `docs/prd/personal-fitness-tracking-app.md` - The generated PRD document.
",
      "expected_output": {
        "task_id": "T006",
        "files_created": [
          "docs/prd/personal-fitness-tracking-app.md"
        ],
        "tests_written": [],
        "prd_content": "# FitCompanion: Personal Fitness Tracker PRD

## 1. Summary

FitCompanion is a personal fitness tracking application designed to empower users to manage their health and wellness through comprehensive activity tracking, personalized workout plans, nutrition advice, and insightful progress analytics. It aims to provide a user-friendly, all-in-one solution for individuals seeking to improve their fitness journey.

- **App Name:** FitCompanion
- **Type:** Personal Fitness Tracker
- **Core Features:** Daily activity tracking, customizable workout plans, nutrition advice, progress tracking and analytics, secure authentication, user data encryption.
- **Tech Stack:** React Native (Frontend), Node.js/Express (Backend), MongoDB (Database).
- **Architecture:** Modular design for maintainability and scalability.

## 2. App Details

FitCompanion aims to be the go-to companion for individuals looking to adopt and maintain a healthier lifestyle. It caters to a broad audience, from beginners seeking guidance to fitness enthusiasts wanting to optimize their routines. The app focuses on providing actionable insights and flexible tools to support users' fitness goals.

## 3. Core Features

### 3.1. Daily Activity Tracking (FR-001, FR-002)
- **Description:** Log and automatically track user's daily physical activity, including steps and active minutes.
- **Priority:** P1 (Manual logging), P2 (Device integration).
- **Acceptance Criteria:** 
  - User can manually input daily step count.
  - User can manually input daily active minutes.
  - App integrates with HealthKit/Google Fit for automatic step tracking (P2).
  - Logged data is stored, retrievable, and displayed in progress reports.
- **Failure Criteria:** Inability to log, retrieve, or display activity data accurately; failure of device integration (P2).

### 3.2. Customizable Workout Plans (FR-003, FR-004)
- **Description:** Allow users to create, save, and execute custom workout routines, and select from pre-defined templates.
- **Priority:** P1 (Customizable), P2 (Pre-defined templates).
- **Acceptance Criteria:** 
  - User can define exercises, sets, reps, and rest times.
  - User can save and name custom workout plans.
  - App includes a library of pre-defined workout templates.
  - Saved and pre-defined plans can be loaded and followed.
- **Failure Criteria:** Inability to save, load, define workouts; absence of pre-defined templates.

### 3.3. Nutrition Advice (FR-005)
- **Description:** Provide basic nutritional guidance, meal logging, and estimated calorie intake tracking.
- ****Priority:** P2.
- **Acceptance Criteria:** 
  - Users can log meals and view estimated calorie intake.
  - App offers general healthy eating tips.
- **Failure Criteria:** Inability to log meals or inaccurate calorie estimates.

### 3.4. Progress Tracking and Analytics (FR-006)
- **Description:** Visualize user progress over time through charts and summaries.
- **Priority:** P1.
- **Acceptance Criteria:** 
  - Visualizations for steps, active minutes, and workout frequency are displayed.
  - Historical data is accessible.
- **Failure Criteria:** Progress charts are missing or inaccurate.

### 3.5. Security Requirements (FR-007, FR-008)
- **Description:** Implement robust security measures including user data encryption and secure authentication.
- **Priority:** P1.
- **Acceptance Criteria:** 
  - User data in the database is encrypted at rest.
  - Users can register with email/password securely.
  - Users can log in securely with password reset functionality.
- **Failure Criteria:** User data stored in plain text; insecure authentication methods; registration/login failures.

## 4. Tech Stack

- **Frontend:** React Native - Chosen for cross-platform development efficiency, enabling deployment on both iOS and Android from a single codebase. Its component-based architecture aligns with our modular design goals.
- **Backend:** Node.js with Express - Selected for its performance in I/O-bound operations, large ecosystem of libraries, and suitability for real-time applications. Its JavaScript base aligns with the frontend, potentially reducing context switching for developers.
- **Database:** MongoDB - A NoSQL document database chosen for its flexibility in handling evolving data schemas, scalability, and ease of integration with Node.js. It's well-suited for storing user profiles, activity logs, and workout data.

## 5. Architecture

FitCompanion will employ a modular architecture. The frontend (React Native) will interact with the backend API (Node.js/Express) via RESTful endpoints. The backend will house distinct modules for Authentication, Activity Tracking, Workout Planning, Nutrition, and Analytics, each interacting with a MongoDB database. This modularity allows for independent development, testing, and scaling of features.

*(High-level diagram to be generated visually in a later step)*

## 6. Interface Contracts

Key interfaces include the communication between the React Native frontend and the Node.js/Express backend API. Contracts define data shapes for requests/responses (e.g., user registration, activity logging), a standardized error format (`{ "error": { "code": "...", "message": "..." } }`), and expected side effects. Recovery strategies will involve retries for transient errors and user feedback for invalid inputs.

## 7. Market Research Summary

Competitor analysis revealed that while apps like MyFitnessPal excel at calorie tracking and Strava at performance analytics, there's a market gap for an integrated, beginner-friendly solution offering comprehensive activity tracking, customizable workout plans, and adaptive nutrition advice within a clean, intuitive interface. FitCompanion aims to fill this gap by providing a holistic approach to fitness.

## 8. Constraints

- **API Budget:** Strict adherence to a $2.81 API budget, prioritizing free tiers where applicable.
- **Hardware:** Consideration for efficient processing on typical user devices.
- **Development:** Focus on modularity for maintainability and updates.

## 9. Unknowns Register

- Detailed API cost estimations for each function call.
- Specific choice of free nutrition API, if available and suitable.
- Fallback API providers for critical functionalities if primary ones exceed budget or hit rate limits.

## 10. Revision History

- Initial Draft (Version 1.0)
"
      }
    }
  ],
  "total_tasks": 6,
  "status": "review