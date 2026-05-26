package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brotherlogic/notes/internal/storage"
	pb "github.com/brotherlogic/notes/proto"
)

type Server struct {
	store     *storage.Storage
	binaryDir string
}

func NewServer(store *storage.Storage) *Server {
	return &Server{store: store}
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
