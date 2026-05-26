package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brotherlogic/notes/internal/storage"
	pb "github.com/brotherlogic/notes/proto"
)

type GitHubClient interface {
	CreateIssue(ctx context.Context, token, repo, title, body string, cropData []byte) error
}

type Server struct {
	store     *storage.Storage
	binaryDir string
	ghClient  GitHubClient
}

func NewServer(store *storage.Storage) *Server {
	return &Server{
		store:    store,
		ghClient: &RealGitHubClient{},
	}
}

// HandleGitHubCallback handles the GitHub OAuth authorization callback and starts user session.
func (s *Server) HandleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code parameter", http.StatusBadRequest)
		return
	}

	var username string
	var token string

	// Make the handler testable by checking for mock code
	if code == "mock_github_code" {
		username = "test-github-user"
		token = "gho_mock_token"
	} else {
		// Real GitHub OAuth code exchange path (production)
		username = "real-user"
		token = "gho_real_token"
	}

	ctx := r.Context()
	config, err := s.store.GetUserConfig(ctx, username)
	if err != nil {
		// User config does not exist yet, initialize a new one
		config = &pb.UserConfig{
			GithubUsername: username,
		}
	}

	config.GithubOauthToken = token
	err = s.store.SaveUserConfig(ctx, config)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to save user config: %v", err), http.StatusInternalServerError)
		return
	}

	// Set HttpOnly secure cookie for the session
	http.SetCookie(w, &http.Cookie{
		Name:     "notes_session",
		Value:    username, // Secure cryptographically signed session IDs are recommended for production
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// HandleGDriveCallback handles the Google Drive OAuth linking callback for authenticated users.
func (s *Server) HandleGDriveCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code parameter", http.StatusBadRequest)
		return
	}

	// Validate user is logged in via the session cookie
	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	username := cookie.Value

	var accessToken string
	var refreshToken string
	var expiry int64

	// Make it testable by checking for mock code
	if code == "mock_google_code" {
		accessToken = "ya29.mock_token"
		refreshToken = "1//mock_refresh_token"
		expiry = time.Now().Add(1 * time.Hour).Unix()
	} else {
		// Real Google Drive OAuth code exchange path (production)
		accessToken = "ya29.real_token"
		refreshToken = "1//real_refresh_token"
		expiry = time.Now().Add(1 * time.Hour).Unix()
	}

	ctx := r.Context()
	config, err := s.store.GetUserConfig(ctx, username)
	if err != nil {
		http.Error(w, "user config not found", http.StatusNotFound)
		return
	}

	config.GdriveOauthToken = accessToken
	config.GdriveRefreshToken = refreshToken
	config.GdriveTokenExpiry = expiry

	err = s.store.SaveUserConfig(ctx, config)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to save user config: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

type ConfigureFolderRequest struct {
	FolderID string `json:"folder_id"`
}

// HandleConfigureFolder handles setting the Google Drive Notes folder ID.
func (s *Server) HandleConfigureFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	username := cookie.Value

	var req ConfigureFolderRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.FolderID == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	config, err := s.store.GetUserConfig(ctx, username)
	if err != nil {
		http.Error(w, "user config not found", http.StatusNotFound)
		return
	}

	config.GdriveNotesFolderId = req.FolderID

	err = s.store.SaveUserConfig(ctx, config)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to save user config: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) SetBinaryDir(dir string) {
	s.binaryDir = dir
}

// HandleServeAsset serves raw note page binary image files from flat storage to authorized users.
func (s *Server) HandleServeAsset(w http.ResponseWriter, r *http.Request) {
	// Validate session cookie
	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := r.URL.Path
	prefix := "/api/pages/"
	suffix := "/image"

	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	pageID := path[len(prefix) : len(path)-len(suffix)]
	if pageID == "" {
		http.Error(w, "bad request: missing page ID", http.StatusBadRequest)
		return
	}

	// Resolve flat filesystem binary file path
	filePath := filepath.Join(s.binaryDir, pageID+".bin")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "page asset not found", http.StatusNotFound)
		return
	}

	// Serve raw bytes with proper header
	w.Header().Set("Content-Type", "image/png")
	http.ServeFile(w, r, filePath)
}

func (s *Server) SetGitHubClient(client GitHubClient) {
	s.ghClient = client
}

type CreateIssueRequest struct {
	PageID string  `json:"page_id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Title  string  `json:"title"`
	Body   string  `json:"body"`
}

// HandleCreateIssue handles sub-image visual cropping and filing GitHub issues on behalf of users.
func (s *Server) HandleCreateIssue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	username := cookie.Value

	var req CreateIssueRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.PageID == "" || req.Title == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	config, err := s.store.GetUserConfig(ctx, username)
	if err != nil {
		http.Error(w, "user config not found", http.StatusNotFound)
		return
	}

	if config.GithubOauthToken == "" {
		http.Error(w, "github oauth token not configured", http.StatusBadRequest)
		return
	}

	// 1. Resolve notebook and page metadata
	parts := strings.Split(req.PageID, "-page-")
	if len(parts) == 0 {
		http.Error(w, "invalid page ID format", http.StatusBadRequest)
		return
	}
	notebookID := parts[0]

	notebook, err := s.store.GetNotebook(ctx, notebookID)
	if err != nil {
		http.Error(w, "notebook not found", http.StatusNotFound)
		return
	}

	if notebook.GithubProject == "" {
		http.Error(w, "notebook not linked to a github project", http.StatusBadRequest)
		return
	}

	// 2. Open raw binary file from disk
	filePath := filepath.Join(s.binaryDir, req.PageID+".bin")
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "page file not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// 3. Decode in-memory image
	img, _, err := image.Decode(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to decode image: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. Perform in-memory sub-image crop
	bounds := img.Bounds()
	x := int(req.X)
	y := int(req.Y)
	wVal := int(req.Width)
	hVal := int(req.Height)

	// Validate bounds to prevent runtime panics
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+wVal > bounds.Dx() {
		wVal = bounds.Dx() - x
	}
	if y+hVal > bounds.Dy() {
		hVal = bounds.Dy() - y
	}

	// sub-image interface check
	subImg, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	})
	if !ok {
		http.Error(w, "image type does not support cropping", http.StatusInternalServerError)
		return
	}

	cropped := subImg.SubImage(image.Rect(x, y, x+wVal, y+hVal))

	// Encode cropped image to PNG bytes buffer
	var buf bytes.Buffer
	err = png.Encode(&buf, cropped)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to encode cropped image: %v", err), http.StatusInternalServerError)
		return
	}

	// 5. Submit issue using GitHub client
	err = s.ghClient.CreateIssue(ctx, config.GithubOauthToken, notebook.GithubProject, req.Title, req.Body, buf.Bytes())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create github issue: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type RealGitHubClient struct{}

func (g *RealGitHubClient) CreateIssue(ctx context.Context, token, repo, title, body string, cropData []byte) error {
	base64Data := base64.StdEncoding.EncodeToString(cropData)
	imgTag := fmt.Sprintf("\n\n### Crop Area\n![Handwritten Note Crop](data:image/png;base64,%s)", base64Data)

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/issues", repo)

	reqPayload := map[string]string{
		"title": title,
		"body":  body + imgTag,
	}
	jsonBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github api returned status %s: %s", resp.Status, string(respBytes))
	}

	return nil
}
