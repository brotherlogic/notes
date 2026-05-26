# Notes Management System - TDD Issue Board

This document contains the official, detailed issue specifications for building the **Notes Management System** from scratch. Each issue represents a thin vertical slice designed to be implemented using a strict **Test-Driven Development (TDD)** Red-Green-Refactor loop.

---

## 📋 Component Breakdown

The system is decomposed into five core components:
1. **Core Serialization (`proto/`)**: Protobuf schemas defining the data boundary.
2. **Storage Layer (`internal/storage/`)**: Integrates with `brotherlogic/pstore` for metadata and assumes local disk folders for binary assets.
3. **Identity & Auth (`internal/api/auth/`)**: Secure session cookies linking GitHub and Google Drive accounts.
4. **Asynchronous Sync (`internal/sync/`)**: Ticker-based background processor polling and syncing files from Google Drive folder.
5. **Interactive UI (`frontend/` & `internal/api/issues/`)**: Responsive glassmorphism dashboard supporting bounding-box visual crop issue creation.

---

## 🎯 Vertical TDD Issue Specifications

### [ISSUE-01] Protobuf Schemas & `pstore` Database Wrapper
*   **Component**: Serialization & Storage Layer
*   **Goal**: Define the protobuf structures for `UserConfig`, `Notebook`, and `Page`, and build a persistence manager wrapping `brotherlogic/pstore`.
*   **User Story**: As a developer, I want to store and retrieve user configs and notebook metadata in a structured, serialized database so that states are persisted across restarts.

#### **TDD Loop Directions**
1.  🔴 **RED**: Write a unit test `internal/storage/storage_test.go` that:
    - Initialises a mocked `pstore_client.GetTestClient()`.
    - Creates a new `storage.NewStorage(testClient)`.
    - Defines a sample `UserConfig` and calling `store.SaveUserConfig(ctx, config)`.
    - Asserts that retrieving it back via `store.GetUserConfig(ctx, username)` returns the exact same fields.
    - *Compilation/Execution should fail because the types and methods do not exist.*
2.  🟢 **GREEN**:
    - Write the proto schema in `proto/notes.proto`.
    - Compile using: `protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/*.proto`.
    - Implement `internal/storage/storage.go` with `SaveUserConfig`, `GetUserConfig`, `SaveNotebook`, and `GetNotebook` wrapping the `pstore` client. Convert protobuf objects using `anypb.New(...)` and `anypb.UnmarshalTo(...)`.
    - Run `go test ./...` and ensure it passes.
3.  🔵 **REFACTOR**:
    - Refactor any repetitive conversion boilerplate. Ensure all errors are explicitly wrapped and context is propagated correctly.

---

### [ISSUE-02] Double OAuth Link & Secure Session Management
*   **Component**: Identity & Auth Layer
*   **Goal**: Implement GitHub login (authentication) and Google Drive OAuth linking (precursor config), and maintain safe user sessions using secure, HTTP-only cookies.
*   **User Story**: As a user, I want to log in securely with my GitHub account, then link my Google Drive and configure my notes folder, so the system can access my files on my behalf.

#### **TDD Loop Directions**
1.  🔴 **RED**: Write an integration/unit test in `internal/api/auth_test.go` asserting that:
    - Requesting `/login/github/callback` with a mock auth code sets an `HTTP-only` secure session cookie containing a cryptographically signed token.
    - Requesting `/link/gdrive/callback` exchanges the Google auth code for refresh/access tokens and updates the user's stored `UserConfig` in the `pstore`.
2.  🟢 **GREEN**:
    - Build auth endpoints inside `internal/api/auth.go` using standard OAuth clients (`golang.org/x/oauth2` for GitHub and Google).
    - Implement cookie session signing/validation.
    - Save GDrive refresh tokens securely to the user's configuration in `pstore`.
3.  🔵 **REFACTOR**:
    - Refactor callback handlers to extract code exchanges into reusable utilities. Ensure all API error cases return standard, beautiful JSON errors rather than raw Go panics.

---

### [ISSUE-03] Google Drive Folder Scanner & Binary File Sync Loop
*   **Component**: Asynchronous Sync
*   **Goal**: Build a background worker ticker that scans each user's configured Google Drive folder, downloads notebooks/pages, and saves them.
*   **User Story**: As a user, I want the system to automatically and asynchronously sync note images/PDFs from my configured Google Drive folder so that my notes are visible in the web app.

#### **TDD Loop Directions**
1.  🔴 **RED**: Write a unit test `internal/sync/sync_test.go` that:
    - Mock the Google Drive client using a fake folder structure listing and fake file download bytes.
    - Start the `sync.NewWorker(storage, gdriveMock)`.
    - Assert that after a trigger run, the worker creates new `Notebook` and `Page` metadata and writes the downloaded file bytes to our binary asset path.
2.  🟢 **GREEN**:
    - Implement `internal/sync/worker.go` with a ticker channel (running every 5 minutes).
    - Use the Google Drive API (`google.golang.org/api/drive/v3`) authenticated via the user's Google OAuth refresh token to poll the configured folder.
    - Sync metadata to `pstore` and write downloaded page images/PDFs to the designated storage path.
3.  🔵 **REFACTOR**:
    - Refactor to implement an incremental sync (conditional GET or `updatedTime` comparison) to prevent duplicate downloads and save network bandwidth.

---

### [ISSUE-04] Responsive Page Viewer UI & Asset Stream API
*   **Component**: Interactive UI (Frontend + Go API)
*   **Goal**: Expose an API endpoint to serve synced binary files and build a stunning React dashboard featuringOutfit typography, smooth page transition animations, and dark glassmorphic cards.
*   **User Story**: As a user, I want to load my notes visually in a premium web dashboard, filtering by page numbers and switching pages with elegant micro-animations.

#### **TDD Loop Directions**
1.  🔴 **RED**:
    - Go Test: Assert that requesting `/api/pages/{page_id}/image` returns the exact binary bytes from storage with the proper `Content-Type` header (e.g. `image/png` or `application/pdf`).
    - UI Test: Assert that `NotebookDashboard` component mounts, displays a list of synced notebooks, and renders `<img>` tags pointing to the asset API.
2.  🟢 **GREEN**:
    - Add the image asset static server handler in `internal/api/assets.go`.
    - Bootstrap the React application inside `/frontend` using Vite.
    - Write premium styling tokens in `frontend/src/styles/index.css`.
    - Build `App.jsx`, `NotebookDashboard.jsx`, and `PageViewer.jsx` components.
3.  🔵 **REFACTOR**:
    - Add transition animations (e.g., sliding or fading effects when flipping pages) and enforce strict responsive styling for desktop and tablet screen bounds.

---

### [ISSUE-05] Bounding Box Visual Crop & GitHub Issue Generation
*   **Component**: Interactive UI
*   **Goal**: Enable users to click-and-drag bounding boxes on the notebook image to crop a segment in-memory and automatically file a GitHub issue.
*   **User Story**: As a user, I want to draw a rectangle over a section of my handwritten note page and submit it to create a GitHub issue containing the cropped image in the linked repository.

#### **TDD Loop Directions**
1.  🔴 **RED**: Write an integration test in `internal/api/issues_test.go` asserting that:
    - Sending a POST request to `/api/issues/create` with a `page_id` and coordinates `{x, y, w, h}` crops the target image on disk in-memory and creates a GitHub issue in the target repository using the user's logged-in GitHub OAuth token.
2.  🟢 **GREEN**:
    - Implement image cropping in Go using `image.Decode` and the `SubImage` interface.
    - Wire up `google/go-github` client to upload the cropped image (e.g. as a repository content file or issue attachment) and submit the issue.
    - Build the React crop-drawing component in the frontend using standard canvas or SVG overlays.
3.  🔵 **REFACTOR**:
    - Add safety checks to handle edge cases where users draw bounding boxes extending outside of the note image bounds.

---

### [ISSUE-06] Processed Soft Archive Toggle & UI State
*   **Component**: Interactive UI & Storage
*   **Goal**: Allow users to mark note pages as "processed", hiding them by default with a dashboard toggle to display them grayed-out at the end.
*   **User Story**: As a user, I want to mark a page as processed once its action items are handled, so that my dashboard stays clean while keeping access to historical notes.

#### **TDD Loop Directions**
1.  🔴 **RED**: Write a backend test verifying that fetching `/api/notebooks` filters out processed pages by default unless a `show_processed=true` query parameter is supplied. Write a frontend test asserting that processed pages render with lowered opacity and a read-only state.
2.  🟢 **GREEN**:
    - Add the `processed` toggle endpoint in Go and adjust list query filters.
    - Wire up the "Mark Processed" button in the React UI.
    - Implement the "Show Processed" global toggle switch in the React frontend header.
3.  🔵 **REFACTOR**:
    - Refactor UI layout to sort active pages first and group all processed pages beautifully in an archived section.
