package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
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
	Repos    []api.GitHubRepo
}

func (m *MockGitHubClient) CreateIssue(ctx context.Context, token, repo, title, body string, cropData []byte) error {
	m.Repo = repo
	m.Title = title
	m.Body = body
	m.CropData = cropData
	return nil
}

func (m *MockGitHubClient) ListRepos(ctx context.Context, token string) ([]api.GitHubRepo, error) {
	return m.Repos, nil
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

func TestHandleCreateGitHubIssue_Archived(t *testing.T) {
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

	// 2. Preset Notebook & Page metadata in pstore with Status = pb.NotebookStatus_NOTEBOOK_DELETED_ON_REMOTE
	err = store.SaveNotebook(ctx, &pb.Notebook{
		Id:            "folder_123",
		Status:        pb.NotebookStatus_NOTEBOOK_DELETED_ON_REMOTE,
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

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403 Forbidden, got %v", resp.StatusCode)
	}

	// 5. Assert MockGitHubClient was not invoked
	if mockGH.Repo != "" {
		t.Errorf("Expected mock GitHub client to not be invoked, but Repo was set to %q", mockGH.Repo)
	}
}

func TestHandleUpdateNotebookGitHubProject(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	server := api.NewServer(store)

	ctx := context.Background()
	username := "test-github-user"
	notebookID := "folder_123"

	if err := store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:   username,
		GithubOauthToken: "gho_mock_token",
	}); err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	if err := store.SaveNotebook(ctx, &pb.Notebook{
		Id:     notebookID,
		Title:  "My Notes",
		Status: pb.NotebookStatus_NOTEBOOK_ACTIVE,
	}); err != nil {
		t.Fatalf("Failed to preset notebook: %v", err)
	}

	// 1. Valid update
	payload := `{"notebook_id": "folder_123", "github_project": "brotherlogic/notes"}`
	req := httptest.NewRequest("POST", "/api/notebooks/github-project", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	w := httptest.NewRecorder()

	server.HandleUpdateNotebookGitHubProject(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %v", w.Result().StatusCode)
	}

	nb, err := store.GetNotebook(ctx, notebookID)
	if err != nil {
		t.Fatalf("Failed to get notebook: %v", err)
	}
	if nb.GithubProject != "brotherlogic/notes" {
		t.Errorf("Expected GithubProject 'brotherlogic/notes', got %q", nb.GithubProject)
	}

	// 2. Invalid format (missing slash)
	badPayload := `{"notebook_id": "folder_123", "github_project": "invalid-repo-name"}`
	badReq := httptest.NewRequest("POST", "/api/notebooks/github-project", strings.NewReader(badPayload))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	badW := httptest.NewRecorder()

	server.HandleUpdateNotebookGitHubProject(badW, badReq)
	if badW.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request for invalid repo format, got %v", badW.Result().StatusCode)
	}

	// 3. Archived notebook should return 403 Forbidden
	if err := store.SaveNotebook(ctx, &pb.Notebook{
		Id:     "archived_nb",
		Title:  "Archived",
		Status: pb.NotebookStatus_NOTEBOOK_DELETED_ON_REMOTE,
	}); err != nil {
		t.Fatalf("Failed to save archived notebook: %v", err)
	}

	archPayload := `{"notebook_id": "archived_nb", "github_project": "brotherlogic/notes"}`
	archReq := httptest.NewRequest("POST", "/api/notebooks/github-project", strings.NewReader(archPayload))
	archReq.Header.Set("Content-Type", "application/json")
	archReq.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	archW := httptest.NewRecorder()

	server.HandleUpdateNotebookGitHubProject(archW, archReq)
	if archW.Result().StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403 Forbidden for archived notebook, got %v", archW.Result().StatusCode)
	}
}

func TestHandleListGitHubRepos(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	server := api.NewServer(store)

	ctx := context.Background()
	username := "test-github-user"

	if err := store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:   username,
		GithubOauthToken: "gho_mock_token",
	}); err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	mockGH := &MockGitHubClient{
		Repos: []api.GitHubRepo{
			{ID: 1, Name: "notes", FullName: "brotherlogic/notes"},
			{ID: 2, Name: "pstore", FullName: "brotherlogic/pstore"},
		},
	}
	server.SetGitHubClient(mockGH)

	req := httptest.NewRequest("GET", "/api/github/repos", nil)
	req.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	w := httptest.NewRecorder()

	server.HandleListGitHubRepos(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %v", w.Result().StatusCode)
	}
	if !strings.Contains(w.Body.String(), "brotherlogic/notes") {
		t.Errorf("Expected response body to contain 'brotherlogic/notes', got %s", w.Body.String())
	}
}

func TestRealGitHubClient_CreateIssue_UsesCropURLInsteadOfBase64(t *testing.T) {
	originalTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = originalTransport }()

	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	tempDir, err := os.MkdirTemp("", "real_gh_issues_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	username := "test-github-user"
	pageID := "folder_123-page-1"
	localPath := filepath.Join(tempDir, pageID+".bin")

	if err := writeValidPNG(localPath); err != nil {
		t.Fatalf("Failed to write mock PNG page file: %v", err)
	}

	if err := store.SaveNotebook(ctx, &pb.Notebook{
		Id:            "folder_123",
		GithubProject: "brotherlogic/notes",
		Pages: []*pb.Page{
			{
				Id:            pageID,
				LocalFilePath: localPath,
			},
		},
	}); err != nil {
		t.Fatalf("Failed to preset notebook: %v", err)
	}

	if err := store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:   username,
		GithubOauthToken: "gho_mock_token",
	}); err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	var capturedTitle, capturedBody string
	http.DefaultClient.Transport = &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && req.URL.String() == "https://api.github.com/repos/brotherlogic/notes/issues" {
				var payload struct {
					Title string `json:"title"`
					Body  string `json:"body"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					return nil, err
				}
				capturedTitle = payload.Title
				capturedBody = payload.Body
				return &http.Response{
					StatusCode: http.StatusCreated,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"id": 101}`)),
				}, nil
			}
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		},
	}

	// Use default RealGitHubClient created by NewServer
	server := api.NewServer(store)
	server.SetBinaryDir(tempDir)
	server.SetRedirectHost("https://notes.brotherlogic.org")

	payload := `{"page_id": "folder_123-page-1", "x": 10, "y": 20, "width": 50, "height": 40, "title": "Fix crop URL", "body": "Issue description text"}`
	req := httptest.NewRequest("POST", "/api/issues/create", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	w := httptest.NewRecorder()

	server.HandleCreateIssue(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %v", resp.StatusCode)
	}

	if capturedTitle != "Fix crop URL" {
		t.Errorf("Expected title 'Fix crop URL', got %q", capturedTitle)
	}

	if strings.Contains(capturedBody, "data:image/png;base64,") {
		t.Errorf("Expected issue body NOT to contain base64 data URI, got: %s", capturedBody)
	}

	expectedPrefix := "https://notes.brotherlogic.org/api/crops/"
	if !strings.Contains(capturedBody, expectedPrefix) {
		t.Errorf("Expected issue body to contain crop URL prefix %q, got: %s", expectedPrefix, capturedBody)
	}

	// Extract the /api/crops/<hash>.png path from the Markdown image tag and fetch it via HandleServeCrop
	idx := strings.Index(capturedBody, "/api/crops/")
	if idx == -1 {
		t.Fatalf("Failed to find /api/crops/ in issue body: %s", capturedBody)
	}
	cropPath := strings.TrimSuffix(capturedBody[idx:], ")")

	// Request without session cookie (simulating GitHub Camo proxy)
	cropReq := httptest.NewRequest("GET", cropPath, nil)
	cropW := httptest.NewRecorder()
	server.HandleServeCrop(cropW, cropReq)
	cropResp := cropW.Result()

	if cropResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK from HandleServeCrop, got %v", cropResp.StatusCode)
	}
	if cropResp.Header.Get("Content-Type") != "image/png" {
		t.Errorf("Expected Content-Type 'image/png', got %q", cropResp.Header.Get("Content-Type"))
	}

	decodedImg, err := png.Decode(cropResp.Body)
	if err != nil {
		t.Fatalf("Failed to decode served crop PNG: %v", err)
	}
	if decodedImg.Bounds().Dx() != 50 || decodedImg.Bounds().Dy() != 40 {
		t.Errorf("Expected cropped image dimensions 50x40, got %dx%d", decodedImg.Bounds().Dx(), decodedImg.Bounds().Dy())
	}

	// Verify security & error states on HandleServeCrop
	// 1. Path traversal / malformed ID
	badReq := httptest.NewRequest("GET", "/api/crops/..%2Fsecret.png", nil)
	badW := httptest.NewRecorder()
	server.HandleServeCrop(badW, badReq)
	if badW.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for malformed crop ID, got %v", badW.Result().StatusCode)
	}

	// 2. Non-existent crop ID
	notFoundReq := httptest.NewRequest("GET", "/api/crops/0123456789abcdef.png", nil)
	notFoundW := httptest.NewRecorder()
	server.HandleServeCrop(notFoundW, notFoundReq)
	if notFoundW.Result().StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found for missing crop ID, got %v", notFoundW.Result().StatusCode)
	}

	// 3. Disallowed HTTP method
	postReq := httptest.NewRequest("POST", cropPath, nil)
	postW := httptest.NewRecorder()
	server.HandleServeCrop(postW, postReq)
	if postW.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed for POST, got %v", postW.Result().StatusCode)
	}
}
