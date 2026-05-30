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

func TestGitHubLoginRedirect(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	server := api.NewServer(store)

	// Case 1: No credentials set, should fallback to mock callback redirect
	req := httptest.NewRequest("GET", "/login/github", nil)
	w := httptest.NewRecorder()
	server.HandleGitHubLogin(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("Expected status 302 Found, got %v", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	if !strings.Contains(location, "mock_github_code") {
		t.Errorf("Expected fallback mock redirect, got %q", location)
	}

	// Case 2: Credentials set, should build real authorization URL
	server.SetOAuthCredentials("my_client_id", "my_secret", "", "")
	w2 := httptest.NewRecorder()
	server.HandleGitHubLogin(w2, req)
	resp2 := w2.Result()
	if resp2.StatusCode != http.StatusFound {
		t.Errorf("Expected status 302 Found, got %v", resp2.StatusCode)
	}
	location2 := resp2.Header.Get("Location")
	if !strings.Contains(location2, "client_id=my_client_id") || !strings.Contains(location2, "github.com/login/oauth/authorize") {
		t.Errorf("Expected real github authorize URL, got %q", location2)
	}
}

func TestGDriveLoginRedirect(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	server := api.NewServer(store)

	// Case 1: Unauthorized without session cookie
	req := httptest.NewRequest("GET", "/link/gdrive", nil)
	w := httptest.NewRecorder()
	server.HandleGDriveLogin(w, req)
	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized for missing session cookie, got %v", w.Result().StatusCode)
	}

	// Add valid session cookie
	req.AddCookie(&http.Cookie{Name: "notes_session", Value: "test-user"})

	// Case 2: No credentials set, should fallback to mock callback redirect
	w2 := httptest.NewRecorder()
	server.HandleGDriveLogin(w2, req)
	resp2 := w2.Result()
	if resp2.StatusCode != http.StatusFound {
		t.Errorf("Expected status 302 Found, got %v", resp2.StatusCode)
	}
	location2 := resp2.Header.Get("Location")
	if !strings.Contains(location2, "mock_google_code") {
		t.Errorf("Expected fallback mock redirect, got %q", location2)
	}

	// Case 3: Credentials set, should build real authorization URL
	server.SetOAuthCredentials("", "", "my_gdrive_id", "my_gdrive_secret")
	w3 := httptest.NewRecorder()
	server.HandleGDriveLogin(w3, req)
	resp3 := w3.Result()
	if resp3.StatusCode != http.StatusFound {
		t.Errorf("Expected status 302 Found, got %v", resp3.StatusCode)
	}
	location3 := resp3.Header.Get("Location")
	if !strings.Contains(location3, "client_id=my_gdrive_id") || !strings.Contains(location3, "accounts.google.com/o/oauth2/v2/auth") {
		t.Errorf("Expected real google authorize URL, got %q", location3)
	}
}

func TestHandleLogout(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	server := api.NewServer(store)

	req := httptest.NewRequest("POST", "/api/logout", nil)
	w := httptest.NewRecorder()

	server.HandleLogout(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %v", resp.StatusCode)
	}

	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "notes_session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatalf("Expected notes_session cookie to be cleared, but cookie was not set in response")
	}

	if sessionCookie.MaxAge != -1 || sessionCookie.Value != "" {
		t.Errorf("Expected session cookie MaxAge to be -1 and Value empty, got MaxAge %d and Value %q", sessionCookie.MaxAge, sessionCookie.Value)
	}
}

func TestGetRedirectURIVariations(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	server := api.NewServer(store)

	// Case 1: Dynamic fallback (default)
	server.SetOAuthCredentials("client_id_123", "secret_123", "", "")
	req := httptest.NewRequest("GET", "/login/github", nil)
	req.Host = "custom-domain.org"
	w := httptest.NewRecorder()
	server.HandleGitHubLogin(w, req)
	resp := w.Result()
	location := resp.Header.Get("Location")
	// Since custom-domain.org doesn't contain localhost/127.0.0.1, it should force https
	expectedPrefix := "https://custom-domain.org/login/github/callback"
	if !strings.Contains(location, "redirect_uri="+expectedPrefix) {
		t.Errorf("Expected redirect_uri to contain %q, got %q", expectedPrefix, location)
	}

	// Case 2: Configured REDIRECT_HOST override
	server.SetRedirectHost("https://notes.brotherlogic.org")
	w2 := httptest.NewRecorder()
	server.HandleGitHubLogin(w2, req)
	resp2 := w2.Result()
	location2 := resp2.Header.Get("Location")
	expectedOverride := "https://notes.brotherlogic.org/login/github/callback"
	if !strings.Contains(location2, "redirect_uri="+expectedOverride) {
		t.Errorf("Expected redirect_uri to contain overridden %q, got %q", expectedOverride, location2)
	}

	// Case 3: Configured REDIRECT_HOST override without protocol prefix (should default to https)
	server.SetRedirectHost("notes.brotherlogic.org")
	w3 := httptest.NewRecorder()
	server.HandleGitHubLogin(w3, req)
	resp3 := w3.Result()
	location3 := resp3.Header.Get("Location")
	if !strings.Contains(location3, "redirect_uri="+expectedOverride) {
		t.Errorf("Expected redirect_uri to contain overridden %q, got %q", expectedOverride, location3)
	}
}
