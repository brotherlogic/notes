package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brotherlogic/notes/internal/api"
	"github.com/brotherlogic/notes/internal/storage"
	pb "github.com/brotherlogic/notes/proto"
	pstore_client "github.com/brotherlogic/pstore/client"
)

func TestGitHubLoginCallback(t *testing.T) {
	// Initialize mocked client and storage
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	// Set up the API server
	server := api.NewServer(store)

	// 1. Create a dummy request to the GitHub callback handler
	req := httptest.NewRequest("GET", "/login/github/callback?code=mock_github_code", nil)
	w := httptest.NewRecorder()

	// 2. Call handler
	server.HandleGitHubCallback(w, req)

	resp := w.Result()

	// 3. Assert Response status and session cookies
	if resp.StatusCode != http.StatusFound { // Redirects on success
		t.Errorf("Expected status 302 Found, got %v", resp.StatusCode)
	}

	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "notes_session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("Session cookie 'notes_session' was not set or empty")
	}

	if !sessionCookie.HttpOnly {
		t.Errorf("Expected session cookie to be HttpOnly")
	}
}

func TestGoogleDriveLinkCallback(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	server := api.NewServer(store)

	ctx := context.Background()
	username := "test-github-user"

	// Preset a user in pstore to link gdrive to
	err := store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername: username,
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	// 1. Create request with active session cookie
	req := httptest.NewRequest("GET", "/link/gdrive/callback?code=mock_google_code", nil)
	// Simulate login state via custom session token (dummy token representing user)
	req.AddCookie(&http.Cookie{Name: "notes_session", Value: username})

	w := httptest.NewRecorder()

	// 2. Call Google link handler
	server.HandleGDriveCallback(w, req)

	resp := w.Result()

	// 3. Assert success redirect
	if resp.StatusCode != http.StatusFound {
		t.Errorf("Expected status 302 Found, got %v", resp.StatusCode)
	}

	// 4. Verify that UserConfig in storage now contains Google OAuth refresh and access tokens
	config, err := store.GetUserConfig(ctx, username)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}

	if config.GdriveOauthToken == "" {
		t.Errorf("Expected GdriveOauthToken to be populated, got empty string")
	}
	if config.GdriveRefreshToken == "" {
		t.Errorf("Expected GdriveRefreshToken to be populated, got empty string")
	}
}

func TestConfigureFolderEndpoint(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	server := api.NewServer(store)

	ctx := context.Background()
	username := "test-github-user"

	// Preset a user in pstore
	err := store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername: username,
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	// 1. Create a request with session cookie
	payload := `{"folder_id": "drive_folder_xyz789"}`
	req := httptest.NewRequest("POST", "/api/config/folder", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "notes_session", Value: username})

	w := httptest.NewRecorder()

	// 2. Call configure handler
	server.HandleConfigureFolder(w, req)

	resp := w.Result()

	// 3. Assert success status
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %v", resp.StatusCode)
	}

	// 4. Verify that UserConfig has been updated
	config, err := store.GetUserConfig(ctx, username)
	if err != nil {
		t.Fatalf("Failed to retrieve user config: %v", err)
	}

	if config.GdriveNotesFolderId != "drive_folder_xyz789" {
		t.Errorf("Expected GdriveNotesFolderId 'drive_folder_xyz789', got %q", config.GdriveNotesFolderId)
	}
}

func TestGetUserConfigAndNotebooks(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	server := api.NewServer(store)

	ctx := context.Background()
	username := "test-github-user"

	// 1. Verify unauthorized responses when notes_session cookie is missing
	req := httptest.NewRequest("GET", "/api/user/config", nil)
	w := httptest.NewRecorder()
	server.HandleGetUserConfig(w, req)
	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized for missing session cookie, got %v", w.Result().StatusCode)
	}

	req2 := httptest.NewRequest("GET", "/api/notebooks", nil)
	w2 := httptest.NewRecorder()
	server.HandleGetNotebooks(w2, req2)
	if w2.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized for missing session cookie, got %v", w2.Result().StatusCode)
	}

	// 2. Preset a user config
	err := store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername: username,
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	// 3. Verify authorized response for user config
	req3 := httptest.NewRequest("GET", "/api/user/config", nil)
	req3.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	w3 := httptest.NewRecorder()
	server.HandleGetUserConfig(w3, req3)
	if w3.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %v", w3.Result().StatusCode)
	}

	// 4. Verify authorized response for notebooks
	req4 := httptest.NewRequest("GET", "/api/notebooks", nil)
	req4.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	w4 := httptest.NewRecorder()
	server.HandleGetNotebooks(w4, req4)
	if w4.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %v", w4.Result().StatusCode)
	}
}
