

### **Project Requirements Document: Advanced Tailscale Funnel Controller**

**Version:** 1.0
**Date:** May 30, 2025

**1. Introduction**
This document outlines the requirements for the "Advanced Tailscale Funnel Controller," a web application designed to provide secure, user-friendly, and robust management of Tailscale Funnel. The application will allow users to temporarily enable a self-hosted service for a specific duration, manage its state via a persistent SQLite database, and interact with it through a clear user interface. Key features include fault tolerance against duplicate requests, time-based secure tokens for API interactions, automatic timed disabling, and packaging as a Docker container.

**2. Goals**
* Develop a secure and reliable system for on-demand, temporary enabling of Tailscale Funnel.
* Provide a user-friendly interface for controlling and monitoring the Funnel status.
* Ensure fault tolerance and prevent inconsistent states through robust state management and checks.
* Package the application as a Docker container for easy deployment.
* Deliver a well-tested application with comprehensive unit and integration tests for the backend services.

**3. System Overview**
The system will consist of:
* A **Backend REST API** (e.g., built with Python/Flask or Python/FastAPI) responsible for business logic, Tailscale command execution, state management, and token authentication.
* A **SQLite Database** for persisting the Funnel's operational state (e.g., enabled status, disable timestamp).
* A **Frontend User Interface** (SPA - Single Page Application) providing user interaction and status display.
* A **Background Scheduler** (part of the backend application) for automatic disabling of the Funnel based on stored timestamps.
* The entire application will be packaged within a **Docker Container**.

**4. Functional Requirements**

**4.2. Core Funnel Management (Backend API)**
    * **FR2.1 Funnel State Persistence:**
        * **FR2.1.1 SQLite Database:** Utilize a SQLite database to store the Funnel's current state.
        * **FR2.1.2 Database Schema:** The database will store at least:
            * `funnel_status` (e.g., 'ENABLED', 'DISABLED')
            * `disable_at_timestamp` (UTC timestamp when the Funnel should be automatically disabled)
            * `last_updated_timestamp` (UTC timestamp of the last state change)
    * **FR2.2 Enable Funnel:**
        * **FR2.2.1 API Endpoint:** `POST /api/funnel/enable`
        * **FR2.2.2 Input:** `duration_minutes` (integer)
        * **FR2.2.3 Logic:**
            1.  Validate API token.
            2.  Query the SQLite database for the current `funnel_status` and `disable_at_timestamp`.
            3.  **Fault Tolerance Check 1 (DB State):** If DB indicates `funnel_status` is 'ENABLED' and `disable_at_timestamp` is in the future, return an appropriate response (e.g., already enabled, with current expiry) or update the `disable_at_timestamp` if the new duration is longer.
            4.  **Fault Tolerance Check 2 (Actual Funnel Status):** Execute a command to check the *actual* status of the Tailscale Funnel (e.g., via `tailscale funnel status` or equivalent, if available).
            5.  If not already enabled (or if safe to re-enable/update), execute the system command to enable Tailscale Funnel for the target service.
            6.  On successful command execution, update the database: `funnel_status` = 'ENABLED', `disable_at_timestamp` = current time + `duration_minutes`.
            7.  Return a success response including the new `disable_at_timestamp`.
    * **FR2.3 Disable Funnel (Manual):**
        * **FR2.3.1 API Endpoint:** `POST /api/funnel/disable`
        * **FR2.3.2 Logic:**
            1.  Validate API token.
            2.  Execute the system command to disable Tailscale Funnel.
            3.  On successful command execution, update the database: `funnel_status` = 'DISABLED', `disable_at_timestamp` = NULL.
            4.  Return a success response.
    * **FR2.4 Extend Funnel Duration:**
        * **FR2.4.1 API Endpoint:** `POST /api/funnel/extend`
        * **FR2.4.2 Input:** `additional_minutes` (integer)
        * **FR2.4.3 Logic:**
            1.  Validate API token.
            2.  Query the database. If `funnel_status` is not 'ENABLED' or `disable_at_timestamp` is in the past, return an error.
            3.  Update `disable_at_timestamp` in the database by adding `additional_minutes` to the existing `disable_at_timestamp`.
            4.  Return a success response including the new `disable_at_timestamp`.
    * **FR2.5 Get Funnel Status:**
        * **FR2.5.1 API Endpoint:** `GET /api/funnel/status`
        * **FR2.5.2 Logic:**
            1.  Validate API token.
            2.  Query and return the current `funnel_status` and `disable_at_timestamp` from the database.

**4.3. Automatic Funnel Disabling (Background Process)**
    * **FR3.1 Scheduled Task:** The backend application must include a background scheduler.
    * **FR3.2 Periodic Check:** The scheduler will periodically (e.g., every 30 seconds or 1 minute) query the SQLite database.
    * **FR3.3 Disabling Logic:** If a record shows `funnel_status` = 'ENABLED' AND `disable_at_timestamp` is now in the past:
        1.  Execute the system command to disable Tailscale Funnel.
        2.  If successful, update the database: `funnel_status` = 'DISABLED', `disable_at_timestamp` = NULL.
        3.  Log the automatic disable action.

**4.4. User Interface (Frontend)**
    * **FR4.1 Single Page Application (SPA):** Provide a clean, intuitive, and mobile-friendly web interface.
    * **FR4.2 Funnel Status Display:**
        * Clearly indicate if the Funnel is currently "Enabled" or "Disabled".
    * **FR4.3 Countdown Timer:**
        * When the Funnel is "Enabled," display a live countdown timer showing the remaining time until automatic disable (based on `disable_at_timestamp` fetched from the API).
    * **FR4.4 Controls:**
        * **Enable Section:**
            * Input field for "Duration in minutes."
            * "Enable Funnel" button.
        * **Active Funnel Section (visible only when Funnel is enabled):**
            * "Extend by +1 Minute" button.
            * "Extend by +5 Minutes" button.
            * "Extend by +15 Minutes" button.
            * "Disable Funnel Now" button.
    * **FR4.5 Dynamic Updates:** The UI must dynamically update based on API responses and the countdown timer without requiring full page reloads.
    * **FR4.6 Feedback:** Provide clear visual feedback to the user for actions (e.g., success messages, error notifications).

**5. Non-Functional Requirements**

* **NFR1.1 Containerization:** The application (backend, frontend, and all dependencies) must be packaged as a Docker image. A `Dockerfile` and, if useful, a `docker-compose.yml` for local development/testing must be provided.
* **NFR1.2 Database:** SQLite is the designated database for its simplicity and embeddability within the Docker container. The database file should be persisted if the container is expected to maintain state across restarts (e.g., via a Docker volume).
* **NFR1.3 Security:**
    * Ensure state-modifying API endpoints are protected by CSRF mechanisms.
    * Ensure all external commands (like `tailscale`) are called safely, sanitizing any user-provided input that might form part of such commands (though in this PRD, inputs like duration are integers, minimizing this risk for direct command injection).
    * Implement appropriate input validation on all API endpoints.
* **NFR1.4 Fault Tolerance & Robustness:**
    * The system must gracefully handle failures in executing Tailscale commands (e.g., Tailscale service not running) and log such errors.
    * API endpoints should be idempotent where sensible (e.g., multiple "disable" requests should not cause errors if already disabled).
    * Prevent duplicate "enable" actions as described in FR2.2.3 by checking both database state and actual Funnel status.
* **NFR1.5 Usability:** The UI should be simple, intuitive, and provide a good user experience, especially on mobile devices.
* **NFR1.6 Logging:** Implement structured logging for key events, errors, and state changes for easier debugging and monitoring.

**6. Testing Requirements**

* **TR1.1 Backend Unit Tests:**
    * Comprehensive unit tests for all backend modules, services, and helper functions.
    * Focus on business logic for token validation, state transitions, database interactions (mocked), and request validation.
    * Target high code coverage.
* **TR1.2 Backend Integration Tests:**
    * Test the full request/response cycle for each API endpoint.
    * These tests will interact with a real (test instance) SQLite database.
    * Mock the execution of `tailscale` shell commands to verify correct command formation and simulate success/failure scenarios.
    * Test the background scheduler's logic for automatic disabling against the test database.
    * Verify correct handling of duplicate requests and state conflicts.

**7. Deployment & Deliverables**
* Source code for the backend API, frontend UI, and any utility scripts.
* `Dockerfile` and associated Docker configuration files.
* Unit and Integration test suites.
* Documentation for:
    * Setup and deployment (how to build and run the Docker container).
    * API endpoint specifications.
    * Configuration of any necessary environment variables (e.g., token secrets, default duration).

---
