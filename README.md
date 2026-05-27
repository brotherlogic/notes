# Notes Management System

A premium, secure Notes Management System featuring a Go backend server and a React (Vite) frontend. The application automatically synchronizes handwritten Supernotes from a Google Drive folder to a local cache, visualizes pages in a sleek dark glassmorphism dashboard, and enables users to visually crop note page segments to file native GitHub issues in linked repositories.

---

## 🚀 Key Features

*   **Asynchronous GDrive Polling**: Background workers poll users' configured Google Drive folders periodically, parse page numbers dynamically, and sync pages to flat file storage.
*   **Database Persistence**: Automatically marshals and saves configuration schemas (`UserConfig`, `Notebook`, `Page`) to a Kubernetes `brotherlogic/pstore` gRPC database.
*   **Sleek Glassmorphic Dashboard**: A premium dark-theme UI built with Outfit/Inter typography, fluid page-turn transitions, pagination, and soft archive toggling.
*   **Drag-to-Crop GitHub Dispatches**: Click-and-drag visual SVG canvas selector to crop sub-images in-memory and submit them as Base64 image tags in native GitHub issues.
*   **Production Delivery**: Fully containerized multi-stage Docker build integrating automatic Git SemVer tagging and immediate pushes to GitHub Container Registry (GHCR).

---

## 🛠️ Developer Setup & Installation

### 1. Prerequisites
Ensure you have the following installed on your developer machine:
*   **Go** (`1.26.3` or higher)

*Note: The React client inside `/frontend` is automatically bundled and served statically by the Go server container in production. For local development, static files are routed to the frontend dist folder.*

### 2. Local Environment Configuration
Create a `.env` file in the root directory (or export variables directly) containing the following details:
```bash
PORT=8080
DATA_DIR=/tmp/notes/binaries
FRONTEND_DIR=./frontend/dist
PSTORE_ADDRESS=localhost:50051 # Set to your pstore service target
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret
GDRIVE_CLIENT_ID=your_gdrive_client_id
GDRIVE_CLIENT_SECRET=your_gdrive_client_secret
```

### 3. Compile & Run the Server
Initialize the database client connection and start the backend HTTP server:
```bash
# Run backend tests
go test -v ./...

# Start the server binary
go run cmd/notes-server/*.go
```

### 4. Build Docker Container
To build the lightweight, multi-stage production Alpine runner:
```bash
docker build -t ghcr.io/brotherlogic/notes:latest .
```

---

## 🔑 OAuth & Credentials Setup Guide

To run the double OAuth link layer, you must configure applications on both developer portals to retrieve the required Client IDs and Secrets.

### 1. GitHub OAuth Credentials
1.  Log in to [GitHub](https://github.com/) and navigate to **Settings -> Developer Settings -> OAuth Apps**.
2.  Click **Register a new application** (or **New OAuth App**).
3.  Configure the application details:
    *   **Application Name**: `Notes Manager`
    *   **Homepage URL**: `http://localhost:8080` (or your production domain e.g., `https://notes.brotherlogic.org`)
    *   **Authorization Callback URL**: `http://localhost:8080/login/github/callback` (or your production host callback)
4.  Click **Register Application**.
5.  Copy the generated **Client ID** (assign to `GITHUB_CLIENT_ID`).
6.  Click **Generate a new client secret** and copy the resulting string (assign to `GITHUB_CLIENT_SECRET`).

### 2. Google Drive API Credentials
1.  Log in to the [Google Cloud Console](https://console.cloud.google.com/).
2.  Select or create a Google Cloud Project.
3.  Enable the **Google Drive API**:
    *   Search for "Google Drive API" in the top search bar.
    *   Click the API card and click **Enable**.
4.  Configure the **OAuth Consent Screen**:
    *   Navigate to **APIs & Services -> OAuth consent screen**.
    *   Choose User Type (**External** or **Internal**) and click Create.
    *   Provide app info and add the scope: `.../auth/drive.readonly` (to download notebook pages).
5.  Create **Credentials**:
    *   Navigate to **APIs & Services -> Credentials**.
    *   Click **Create Credentials** at the top and select **OAuth client ID**.
    *   Select **Web application** as the Application type.
    *   Under **Authorized redirect URIs**, click Add URI and paste:
        `http://localhost:8080/link/gdrive/callback` (or your production redirect host).
6.  Click **Create**.
7.  Copy the **Your Client ID** (assign to `GDRIVE_CLIENT_ID`) and **Your Client Secret** (assign to `GDRIVE_CLIENT_SECRET`) from the popup modal.

---

## 📂 Repository Reference Docs
*   **[REQUIREMENTS.md](file:///workspaces/tasks/REQUIREMENTS.md)**: Full Kubernetes cluster parameters, `pvc.yaml` mount points, service ports, and deployment files.
*   **[implementation_plan.md](file:///workspaces/tasks/implementation_plan.md)**: Slices design, storage mock bug workarounds, and database persistence architectures.
*   **[walkthrough.md](file:///workspaces/tasks/walkthrough.md)**: Step-by-step developer walkthrough of soft archives, entrypoints, and CI/CD tagging.
