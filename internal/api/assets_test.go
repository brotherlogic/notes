package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/brotherlogic/notes/internal/api"
	"github.com/brotherlogic/notes/internal/storage"
	pb "github.com/brotherlogic/notes/proto"
	pstore_client "github.com/brotherlogic/pstore/client"
)

func TestHandleServeAsset(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	// Set up temporary local binary storage folder
	tempDir, err := os.MkdirTemp("", "assets_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	server := api.NewServer(store)
	// Configure server binary directory path
	server.SetBinaryDir(tempDir)

	ctx := context.Background()
	username := "test-github-user"
	pageID := "folder_123-page-file_abc"
	localPath := filepath.Join(tempDir, pageID+".bin")

	// 1. Write dummy image file to local storage path
	expectedBytes := []byte("fake_image_png_bytes")
	err = os.WriteFile(localPath, expectedBytes, 0644)
	if err != nil {
		t.Fatalf("Failed to write mock page file: %v", err)
	}

	// 2. Preset Notebook & Page metadata in pstore
	err = store.SaveNotebook(ctx, &pb.Notebook{
		Id: "folder_123",
		Pages: []*pb.Page{
			{
				Id:            pageID,
				LocalFilePath: localPath,
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to preset notebook: %v", err)
	}

	// Preset user session config
	err = store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername: username,
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	// --- TEST CASE 1: Valid Session & Valid Page ID ---
	req := httptest.NewRequest("GET", "/api/pages/folder_123-page-file_abc/image", nil)
	req.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	w := httptest.NewRecorder()

	server.HandleServeAsset(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %v", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "image/png" {
		t.Errorf("Expected Content-Type 'image/png', got %q", contentType)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(bodyBytes) != string(expectedBytes) {
		t.Errorf("Expected body %q, got %q", string(expectedBytes), string(bodyBytes))
	}

	// --- TEST CASE 2: Missing Session Cookie ---
	reqUnauth := httptest.NewRequest("GET", "/api/pages/folder_123-page-file_abc/image", nil)
	wUnauth := httptest.NewRecorder()

	server.HandleServeAsset(wUnauth, reqUnauth)
	respUnauth := wUnauth.Result()

	if respUnauth.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized, got %v", respUnauth.StatusCode)
	}

	// --- TEST CASE 3: Non-existent Page ID ---
	reqNotFound := httptest.NewRequest("GET", "/api/pages/folder_123-page-file_fake/image", nil)
	reqNotFound.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	wNotFound := httptest.NewRecorder()

	server.HandleServeAsset(wNotFound, reqNotFound)
	respNotFound := wNotFound.Result()

	if respNotFound.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 Not Found, got %v", respNotFound.StatusCode)
	}
}
