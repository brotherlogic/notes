package api_test

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brotherlogic/notes/internal/api"
	"github.com/brotherlogic/notes/internal/storage"
	pb "github.com/brotherlogic/notes/proto"
	pstore_client "github.com/brotherlogic/pstore/client"
)

type MockGitHubClient struct {
	Repo     string
	Title    string
	Body     string
	CropData []byte
}

func (m *MockGitHubClient) CreateIssue(ctx context.Context, token, repo, title, body string, cropData []byte) error {
	m.Repo = repo
	m.Title = title
	m.Body = body
	m.CropData = cropData
	return nil
}

func writeValidPNG(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for x := 0; x < 200; x++ {
		for y := 0; y < 200; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func TestHandleCreateGitHubIssue(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	// Set up temporary local binary storage folder
	tempDir, err := os.MkdirTemp("", "issues_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	username := "test-github-user"
	pageID := "folder_123-page-file_abc"
	localPath := filepath.Join(tempDir, pageID+".bin")

	// 1. Write a valid PNG image file to local storage path
	err = writeValidPNG(localPath)
	if err != nil {
		t.Fatalf("Failed to write mock PNG page file: %v", err)
	}

	// 2. Preset Notebook & Page metadata in pstore
	err = store.SaveNotebook(ctx, &pb.Notebook{
		Id:            "folder_123",
		GithubProject: "brotherlogic/notes",
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
		GithubUsername:   username,
		GithubOauthToken: "gho_mock_token",
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	mockGH := &MockGitHubClient{}
	server := api.NewServer(store)
	server.SetBinaryDir(tempDir)
	server.SetGitHubClient(mockGH)

	// 3. Post payload with coordinates and details
	payload := `{"page_id": "folder_123-page-file_abc", "x": 10, "y": 20, "width": 50, "height": 40, "title": " Handwritten Bug", "body": "Found a hand-written bug here!"}`
	req := httptest.NewRequest("POST", "/api/issues/create", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	w := httptest.NewRecorder()

	// 4. Call handler
	server.HandleCreateIssue(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %v", resp.StatusCode)
	}

	// 5. Assert MockGitHubClient captured correct inputs
	if mockGH.Repo != "brotherlogic/notes" {
		t.Errorf("Expected repo 'brotherlogic/notes', got %q", mockGH.Repo)
	}
	if mockGH.Title != " Handwritten Bug" {
		t.Errorf("Expected title ' Handwritten Bug', got %q", mockGH.Title)
	}
	if mockGH.Body != "Found a hand-written bug here!" {
		t.Errorf("Expected body 'Found a hand-written bug here!', got %q", mockGH.Body)
	}

	// Verify we got cropped image bytes (non-empty)
	if len(mockGH.CropData) == 0 {
		t.Errorf("Expected non-empty cropped image data")
	}
}
