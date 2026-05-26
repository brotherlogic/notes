package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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
