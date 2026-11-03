package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/brotherlogic/notes/internal/storage"
	pb "github.com/brotherlogic/notes/proto"
	pstore_client "github.com/brotherlogic/pstore/client"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDynamicGDriveClient_ListFiles_RefreshesExpiredToken(t *testing.T) {
	originalTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = originalTransport }()

	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	ctx := context.Background()

	username := "sync-user"
	folderID := "folder-abc"
	expiredExpiry := time.Now().Add(-10 * time.Minute).Unix()

	err := store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:      username,
		GdriveNotesFolderId: folderID,
		GdriveOauthToken:    "expired-access-token",
		GdriveRefreshToken:  "valid-refresh-token",
		GdriveTokenExpiry:   expiredExpiry,
	})
	if err != nil {
		t.Fatalf("Failed to save initial user config: %v", err)
	}

	refreshCalled := false
	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "https://oauth2.googleapis.com/token" {
			refreshCalled = true
			bodyBytes, _ := io.ReadAll(req.Body)
			bodyStr := string(bodyBytes)
			if !strings.Contains(bodyStr, "grant_type=refresh_token") || !strings.Contains(bodyStr, "refresh_token=valid-refresh-token") {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error": "invalid_request"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"access_token": "refreshed-access-token", "expires_in": 3600}`)),
			}, nil
		}

		if strings.Contains(req.URL.Path, "/drive/v3/files") {
			authHeader := req.Header.Get("Authorization")
			if authHeader != "Bearer refreshed-access-token" {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Status:     "401 Unauthorized",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error": {"code": 401, "message": "Invalid Credentials"}}`)),
				}, nil
			}

			if req.URL.Query().Get("alt") == "media" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("note-binary-payload")),
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"files": [{"id": "file-1", "name": "Test.note", "mimeType": "application/octet-stream", "modifiedTime": "2026-01-01T00:00:00Z"}]}`)),
			}, nil
		}

		return nil, fmt.Errorf("unexpected request URL: %s", req.URL.String())
	})

	client := NewDynamicGDriveClient(store)
	files, err := client.ListFiles(ctx, folderID)
	if err != nil {
		t.Fatalf("Expected ListFiles to succeed after token refresh, got error: %v", err)
	}
	if !refreshCalled {
		t.Errorf("Expected OAuth token refresh endpoint to be called")
	}
	if len(files) != 1 || files[0].ID != "file-1" {
		t.Fatalf("Unexpected files returned: %+v", files)
	}

	// Verify updated token and expiry were persisted in pstore
	updatedConfig, err := store.GetUserConfig(ctx, username)
	if err != nil {
		t.Fatalf("Failed to read updated user config: %v", err)
	}
	if updatedConfig.GdriveOauthToken != "refreshed-access-token" {
		t.Errorf("Expected persisted GdriveOauthToken 'refreshed-access-token', got %q", updatedConfig.GdriveOauthToken)
	}
	if updatedConfig.GdriveTokenExpiry <= time.Now().Unix() {
		t.Errorf("Expected persisted GdriveTokenExpiry to be in the future, got %d", updatedConfig.GdriveTokenExpiry)
	}

	// Verify subsequent DownloadFile uses the refreshed token
	data, err := client.DownloadFile(ctx, "file-1")
	if err != nil {
		t.Fatalf("Expected DownloadFile to succeed with refreshed token, got error: %v", err)
	}
	if string(data) != "note-binary-payload" {
		t.Errorf("Expected downloaded content 'note-binary-payload', got %q", string(data))
	}
}

func TestDynamicGDriveClient_DownloadFile_FallbackRefreshesExpiredToken(t *testing.T) {
	originalTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = originalTransport }()

	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	ctx := context.Background()

	username := "sync-user"
	err := store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:      username,
		GdriveNotesFolderId: "folder-abc",
		GdriveOauthToken:    "expired-access-token",
		GdriveRefreshToken:  "valid-refresh-token",
		GdriveTokenExpiry:   time.Now().Add(-5 * time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("Failed to save initial user config: %v", err)
	}

	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "https://oauth2.googleapis.com/token" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"access_token": "fallback-refreshed-token", "expires_in": 3600}`)),
			}, nil
		}

		if strings.Contains(req.URL.Path, "/drive/v3/files/uncached-file") {
			if req.Header.Get("Authorization") != "Bearer fallback-refreshed-token" {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Status:     "401 Unauthorized",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error": "unauthorized"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("fallback-file-bytes")),
			}, nil
		}

		return nil, fmt.Errorf("unexpected request URL: %s", req.URL.String())
	})

	client := NewDynamicGDriveClient(store)
	data, err := client.DownloadFile(ctx, "uncached-file")
	if err != nil {
		t.Fatalf("Expected DownloadFile fallback to refresh token and succeed, got error: %v", err)
	}
	if string(data) != "fallback-file-bytes" {
		t.Errorf("Expected 'fallback-file-bytes', got %q", string(data))
	}

	updatedConfig, err := store.GetUserConfig(ctx, username)
	if err != nil {
		t.Fatalf("Failed to read updated user config: %v", err)
	}
	if updatedConfig.GdriveOauthToken != "fallback-refreshed-token" {
		t.Errorf("Expected persisted GdriveOauthToken 'fallback-refreshed-token', got %q", updatedConfig.GdriveOauthToken)
	}
}

func TestDynamicGDriveClient_ListFiles_ValidTokenNotRefreshed(t *testing.T) {
	originalTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = originalTransport }()

	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	ctx := context.Background()

	username := "sync-user"
	folderID := "folder-valid"
	err := store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:      username,
		GdriveNotesFolderId: folderID,
		GdriveOauthToken:    "valid-access-token",
		GdriveRefreshToken:  "valid-refresh-token",
		GdriveTokenExpiry:   time.Now().Add(1 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("Failed to save initial user config: %v", err)
	}

	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "https://oauth2.googleapis.com/token" {
			t.Fatalf("Unexpected token refresh call for non-expired token")
		}
		if strings.Contains(req.URL.Path, "/drive/v3/files") {
			if req.Header.Get("Authorization") != "Bearer valid-access-token" {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Status:     "401 Unauthorized",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error": "unauthorized"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"files": [{"id": "file-valid", "name": "Valid.note", "mimeType": "application/octet-stream", "modifiedTime": "2026-01-01T00:00:00Z"}]}`)),
			}, nil
		}
		return nil, fmt.Errorf("unexpected request URL: %s", req.URL.String())
	})

	client := NewDynamicGDriveClient(store)
	files, err := client.ListFiles(ctx, folderID)
	if err != nil {
		t.Fatalf("Expected ListFiles to succeed with valid token, got error: %v", err)
	}
	if len(files) != 1 || files[0].ID != "file-valid" {
		t.Fatalf("Unexpected files returned: %+v", files)
	}
}
