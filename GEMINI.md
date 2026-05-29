# AI Coding Assistant Guidelines (GEMINI.md)

Welcome! This file acts as the repository-specific "instruction manual" and coding guardrail system for AI assistants (like Gemini) working in this codebase. Always adhere to these standards when creating, modifying, or refactoring code.

---

## 1. Project Context & Tech Stack

This project is a **Notes** management system designed to sync, visualize, and process personal notes.

### Core System Capabilities
1. **Supernotes Synchronization**: Asynchronously syncs Supernotes uploaded to Google Drive with local storage.
2. **Web Visualization**: Automatically converts synchronized notes into web-visible note pages.
3. **GitHub Integration**: Links notebook pages to GitHub projects and supports selecting note text to generate GitHub issues.
4. **Processing Workflow**: Enables marking note pages as processed via the UI.

### Technical Stack
- **Backend**: Go (Golang) using modern Go principles.
- **Frontend**: React application built and bundled with Vite.
- **Serialization & Communication**: Protocol Buffers (`proto3`) for both internal communication (gRPC) and data serialization.
- **Persistence**: `github.com/brotherlogic/pstore` for persisting Protobuf data in Kubernetes.
- **Authentication**: GitHub OAuth.
- **Infrastructure**: Fully hosted and deployed within Kubernetes.

---

## 2. Directory Layout & Conventions

The codebase follows standard Go and React design conventions. Maintain this structure when adding new code:

```
/workspaces/tasks/
├── cmd/                # Entrypoints for backend service binaries
├── internal/           # Private Go application and business logic
│   ├── sync/           # Google Drive syncing logic
│   ├── api/            # gRPC service handlers & server implementation
│   └── storage/        # pstore integration wrappers
├── proto/              # Protobuf schema (.proto) definitions
├── frontend/           # React + Vite frontend application
│   ├── src/
│   │   ├── components/ # Reusable UI components
│   │   ├── styles/     # Premium styling and design system tokens
│   │   └── App.jsx     # Frontend entrypoint
│   └── package.json
└── GEMINI.md           # This file (AI developer guidelines)
```

---

## 3. Tool & Command Cheat Sheet

Always run the appropriate tools to compile, format, and test code during development.

### Protocol Buffers (gRPC) Compilation
When modifying `.proto` files, compile the Go code using `protoc` from the repository root:
```bash
protoc --go_out=. --go-grpc_out=. proto/*.proto
```

### Backend (Go) Commands
- **Formatting**: Format Go files using standard guidelines before committing:
  ```bash
  go fmt ./...
  ```
- **Dependencies**: Add missing and remove unused modules:
  ```bash
  go mod tidy
  ```
- **Testing**: Run backend unit tests:
  ```bash
  go test -v ./...
  ```

### Frontend (React/Vite) Commands
Run commands inside the `/frontend` directory:
- **Installation**: `npm install`
- **Development Server**: `npm run dev`
- **Lints & Style Check**: `npm run lint`
- **Production Build**: `npm run build`

---

## 4. Coding Guardrails & Best Practices

To maintain code quality and prevent common bugs, enforce the following guidelines at all times:

### Backend (Go) Standards
- **Explicit Error Handling**: Always check returned errors. Never discard them using `_` unless explicitly justified in a comment. Return detailed, wrapped context errors using `%w`.
- **No Global State**: Do not store state or database connections in global package variables. Use struct-based dependency injection.
- **Context Propagation**: Always pass and respect `context.Context` in function signatures, especially in HTTP/gRPC handlers, sync loops, and storage calls.
- **Concurrency**: Use channels and sync primitives carefully. Avoid goroutine leaks by managing lifetimes through contexts or waitgroups.

### Serialization & Persistence Standards
- **Schema-First Design**: Always define data structures in a `.proto` file within `/proto` before writing storage/comms code.
- **Backwards Compatibility**: When modifying schemas, do not alter existing tag numbers. Follow protobuf field addition and deprecation rules.
- **pstore Persistence**: Use `github.com/brotherlogic/pstore` client wrappers inside `/internal/storage` for persisting and retrieving serialized protobuf messages.

### Frontend (React/Vite) & Design Standards
- **Functional Components**: Use modern React functional components and hooks (`useState`, `useEffect`, etc.).
- **Premium Styling (Vanilla CSS)**: Avoid generic, flat, or default designs. Follow these aesthetic rules:
  - Use curated, high-end color palettes (sleek dark mode, dynamic glassmorphism).
  - Use modern typography (e.g., *Inter* or *Outfit* via Google Fonts).
  - Add smooth transitions, hover states, and micro-animations for active feedback.
  - Implement full responsiveness using clean flexbox, grid, and CSS media queries.
- **No Placeholders**: Never include empty placeholders or dummy components. All assets or mock components must look professional and fully-formed.

---

## 5. AI Assistant Operational Rules

- **No Placeholder Code**: Never emit code comments like `// TODO: implement later` or dummy implementations. All code must be complete, compilable, and correct.
- **Preserve Comments**: Maintain all unrelated comments and docstrings in existing code blocks when performing edits.
- **Self-Documenting Plan**: Before executing complex architectural changes, write/update an implementation plan artifact to verify with the user.
- **Automatic Issue Closing**: Ensure that code changes pushed to feature branches always close their referenced issue. Referencing the issue with closing keywords (e.g., `Resolves #45` or `Closes #45`) in commit messages or pull request descriptions ensures that it is automatically closed upon merge.
- **Living Document**: If you or the user discover a recurring codebase pattern, mistake, or standard, update `GEMINI.md` to capture the new rule.
