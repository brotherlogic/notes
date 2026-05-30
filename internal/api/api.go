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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brotherlogic/notes/internal/storage"
	"github.com/brotherlogic/notes/internal/sync"
	pb "github.com/brotherlogic/notes/proto"
	"google.golang.org/protobuf/encoding/protojson"
)

type GitHubClient interface {
	CreateIssue(ctx context.Context, token, repo, title, body string, cropData []byte) error
}

type SyncProgressProvider interface {
	GetSyncProgress(username string) *sync.SyncProgress
	SyncUserNotes(ctx context.Context, username string) error
}

type Server struct {
	store              *storage.Storage
	binaryDir          string
	ghClient           GitHubClient
	gitHubClientID     string
	gitHubClientSecret string
	gDriveClientID     string
	gDriveClientSecret string
	redirectHost       string
	syncProvider       SyncProgressProvider
}

func (s *Server) SetSyncProvider(provider SyncProgressProvider) {
	s.syncProvider = provider
}

func NewServer(store *storage.Storage) *Server {
	return &Server{
		store:    store,
		ghClient: &RealGitHubClient{},
	}
}

func (s *Server) SetOAuthCredentials(githubID, githubSecret, gdriveID, gdriveSecret string) {
	s.gitHubClientID = githubID
	s.gitHubClientSecret = githubSecret
	s.gDriveClientID = gdriveID
	s.gDriveClientSecret = gdriveSecret
}

func (s *Server) SetRedirectHost(host string) {
	s.redirectHost = host
}

func (s *Server) getRedirectURI(r *http.Request, path string) string {
	// If redirectHost is configured, use it as the base URL
	if s.redirectHost != "" {
		host := strings.TrimSuffix(s.redirectHost, "/")
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		return host + path
	}

	scheme := "http"
	if r.TLS != nil || strings.ToLower(r.Header.Get("X-Forwarded-Proto")) == "https" {
		scheme = "https"
	}
	// Force https for production custom domains to avoid Ingress/reverse-proxy SSL termination mismatch
	if !strings.Contains(r.Host, "localhost") && !strings.Contains(r.Host, "127.0.0.1") {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, path)
}

// HandleGitHubLogin redirects the user to the GitHub OAuth login page.
func (s *Server) HandleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	if s.gitHubClientID == "" || r.URL.Query().Get("mock") == "true" {
		http.Redirect(w, r, "/login/github/callback?code=mock_github_code", http.StatusFound)
		return
	}

	redirectURI := s.getRedirectURI(r, "/login/github/callback")

	authURL := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=repo,user",
		s.gitHubClientID, redirectURI)
	http.Redirect(w, r, authURL, http.StatusFound)
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
	if code == "mock_github_code" || s.gitHubClientID == "" {
		username = "test-github-user"
		token = "gho_mock_token"
	} else {
		// Real GitHub OAuth code exchange path (production)
		redirectURI := s.getRedirectURI(r, "/login/github/callback")

		// 1. Exchange code for access token
		tokenReqPayload := map[string]string{
			"client_id":     s.gitHubClientID,
			"client_secret": s.gitHubClientSecret,
			"code":          code,
			"redirect_uri":  redirectURI,
		}
		jsonBytes, err := json.Marshal(tokenReqPayload)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal token request payload: %v", err), http.StatusInternalServerError)
			return
		}

		req, err := http.NewRequestWithContext(r.Context(), "POST", "https://github.com/login/oauth/access_token", bytes.NewReader(jsonBytes))
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create token request: %v", err), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to execute token request: %v", err), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			http.Error(w, fmt.Sprintf("github token exchange returned status %s: %s", resp.Status, string(body)), http.StatusBadRequest)
			return
		}

		var tokenResp struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
			ErrorDesc   string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			http.Error(w, fmt.Sprintf("failed to decode token response: %v", err), http.StatusInternalServerError)
			return
		}

		if tokenResp.Error != "" {
			http.Error(w, fmt.Sprintf("github token exchange error: %s (%s)", tokenResp.Error, tokenResp.ErrorDesc), http.StatusBadRequest)
			return
		}

		if tokenResp.AccessToken == "" {
			http.Error(w, "github token exchange returned empty access token", http.StatusBadRequest)
			return
		}

		token = tokenResp.AccessToken

		// 2. Fetch user profile to get login name
		userReq, err := http.NewRequestWithContext(r.Context(), "GET", "https://api.github.com/user", nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create user info request: %v", err), http.StatusInternalServerError)
			return
		}
		userReq.Header.Set("Authorization", "token "+token)
		userReq.Header.Set("Accept", "application/vnd.github.v3+json")
		userReq.Header.Set("User-Agent", "brotherlogic-notes")

		userResp, err := http.DefaultClient.Do(userReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to execute user info request: %v", err), http.StatusInternalServerError)
			return
		}
		defer userResp.Body.Close()

		if userResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(userResp.Body)
			http.Error(w, fmt.Sprintf("github user info request returned status %s: %s", userResp.Status, string(body)), http.StatusBadRequest)
			return
		}

		var userProfile struct {
			Login string `json:"login"`
		}
		if err := json.NewDecoder(userResp.Body).Decode(&userProfile); err != nil {
			http.Error(w, fmt.Sprintf("failed to decode user profile response: %v", err), http.StatusInternalServerError)
			return
		}

		if userProfile.Login == "" {
			http.Error(w, "github user profile returned empty username", http.StatusBadRequest)
			return
		}

		username = userProfile.Login
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

// HandleGDriveLogin redirects the user to the Google Drive OAuth authorization page.
func (s *Server) HandleGDriveLogin(w http.ResponseWriter, r *http.Request) {
	// Validate user is logged in via the session cookie
	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if s.gDriveClientID == "" || r.URL.Query().Get("mock") == "true" {
		http.Redirect(w, r, "/link/gdrive/callback?code=mock_google_code", http.StatusFound)
		return
	}

	redirectURI := s.getRedirectURI(r, "/link/gdrive/callback")

	authURL := fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=https://www.googleapis.com/auth/drive.readonly&access_type=offline&prompt=consent",
		s.gDriveClientID, redirectURI)
	http.Redirect(w, r, authURL, http.StatusFound)
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
	if code == "mock_google_code" || s.gDriveClientID == "" {
		accessToken = "ya29.mock_token"
		refreshToken = "1//mock_refresh_token"
		expiry = time.Now().Add(1 * time.Hour).Unix()
	} else {
		// Real Google Drive OAuth code exchange path (production)
		redirectURI := s.getRedirectURI(r, "/link/gdrive/callback")

		formValues := url.Values{}
		formValues.Set("client_id", s.gDriveClientID)
		formValues.Set("client_secret", s.gDriveClientSecret)
		formValues.Set("code", code)
		formValues.Set("redirect_uri", redirectURI)
		formValues.Set("grant_type", "authorization_code")

		req, err := http.NewRequestWithContext(r.Context(), "POST", "https://oauth2.googleapis.com/token", strings.NewReader(formValues.Encode()))
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create token request: %v", err), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to execute token request: %v", err), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			http.Error(w, fmt.Sprintf("google token exchange returned status %s: %s", resp.Status, string(body)), http.StatusBadRequest)
			return
		}

		var tokenResp struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			http.Error(w, fmt.Sprintf("failed to decode token response: %v", err), http.StatusInternalServerError)
			return
		}

		if tokenResp.AccessToken == "" {
			http.Error(w, "google token exchange returned empty access token", http.StatusBadRequest)
			return
		}

		accessToken = tokenResp.AccessToken
		refreshToken = tokenResp.RefreshToken
		if refreshToken == "" {
			// Google only returns refresh_token on the first authorization (prompt=consent).
			// If it's empty, we should keep the existing refresh token in the configuration.
			ctx := r.Context()
			existingConfig, err := s.store.GetUserConfig(ctx, username)
			if err == nil {
				refreshToken = existingConfig.GdriveRefreshToken
			}
		}
		expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix()
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

func (s *Server) refreshGDriveTokenIfNeeded(ctx context.Context, username string, config *pb.UserConfig) (string, error) {
	if config.GdriveRefreshToken == "" {
		return config.GdriveOauthToken, nil
	}

	// Buffer of 5 minutes before actual expiry to prevent race conditions
	if time.Now().Unix() < config.GdriveTokenExpiry-300 {
		return config.GdriveOauthToken, nil
	}

	// Token has expired or is about to expire, perform OAuth 2.0 refresh
	formValues := url.Values{}
	formValues.Set("client_id", s.gDriveClientID)
	formValues.Set("client_secret", s.gDriveClientSecret)
	formValues.Set("refresh_token", config.GdriveRefreshToken)
	formValues.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(formValues.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("google token refresh returned status %s: %s", resp.Status, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("google token refresh returned empty access token")
	}

	config.GdriveOauthToken = tokenResp.AccessToken
	config.GdriveTokenExpiry = time.Now().Unix() + tokenResp.ExpiresIn

	err = s.store.SaveUserConfig(ctx, config)
	if err != nil {
		return "", fmt.Errorf("failed to save refreshed user config: %w", err)
	}

	return config.GdriveOauthToken, nil
}

// HandleListGDriveFolders queries the Google Drive API for all folders in the user's GDrive.
func (s *Server) HandleListGDriveFolders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	username := cookie.Value
	ctx := r.Context()

	config, err := s.store.GetUserConfig(ctx, username)
	if err != nil {
		http.Error(w, "user config not found", http.StatusNotFound)
		return
	}

	if config.GdriveOauthToken == "" {
		http.Error(w, "gdrive oauth token not configured", http.StatusBadRequest)
		return
	}

	// 1. Automatically refresh the access token if expired
	token, err := s.refreshGDriveTokenIfNeeded(ctx, username, config)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to validate google tokens: %v", err), http.StatusUnauthorized)
		return
	}

	// 2. Query Google Drive API for folders
	url := "https://www.googleapis.com/drive/v3/files?q=mimeType='application/vnd.google-apps.folder'+and+trashed=false&fields=files(id,name)&pageSize=100"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create folders request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to execute folders request: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("google drive API returned status %s: %s", resp.Status, string(body)), http.StatusBadRequest)
		return
	}

	var data struct {
		Files []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data.Files)
}

// HandleLogout clears the user session cookie and logs the user out.
func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Overwrite the cookie to expire it immediately
	http.SetCookie(w, &http.Cookie{
		Name:     "notes_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	w.WriteHeader(http.StatusOK)
}

type ConfigureFolderRequest struct {
	FolderID string `json:"folder_id"`
}

// fetchFolderDetails queries Google Drive for specific folder details (name and file count).
func (s *Server) fetchFolderDetails(ctx context.Context, username string, config *pb.UserConfig, folderID string) (string, int, error) {
	if config.GdriveOauthToken == "" {
		return "", 0, fmt.Errorf("gdrive oauth token not configured")
	}

	token, err := s.refreshGDriveTokenIfNeeded(ctx, username, config)
	if err != nil {
		return "", 0, fmt.Errorf("failed to validate google tokens: %w", err)
	}

	// 1. Fetch folder metadata (name)
	folderURL := fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s?fields=name", folderID)
	fReq, err := http.NewRequestWithContext(ctx, "GET", folderURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create folder request: %w", err)
	}
	fReq.Header.Set("Authorization", "Bearer "+token)

	fResp, err := http.DefaultClient.Do(fReq)
	if err != nil {
		return "", 0, fmt.Errorf("failed to execute folder request: %w", err)
	}
	defer fResp.Body.Close()

	if fResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(fResp.Body)
		return "", 0, fmt.Errorf("google drive API returned status %s for folder: %s", fResp.Status, string(body))
	}

	var folderData struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(fResp.Body).Decode(&folderData); err != nil {
		return "", 0, fmt.Errorf("failed to decode folder details: %w", err)
	}

	// 2. Query folder for count of files inside it
	filesURL := fmt.Sprintf("https://www.googleapis.com/drive/v3/files?q='%s'+in+parents+and+trashed=false&fields=files(id)&pageSize=1000", folderID)
	filesReq, err := http.NewRequestWithContext(ctx, "GET", filesURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create files request: %w", err)
	}
	filesReq.Header.Set("Authorization", "Bearer "+token)

	filesResp, err := http.DefaultClient.Do(filesReq)
	if err != nil {
		return "", 0, fmt.Errorf("failed to execute files request: %w", err)
	}
	defer filesResp.Body.Close()

	if filesResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(filesResp.Body)
		return "", 0, fmt.Errorf("google drive API returned status %s for files count: %s", filesResp.Status, string(body))
	}

	var filesData struct {
		Files []interface{} `json:"files"`
	}
	if err := json.NewDecoder(filesResp.Body).Decode(&filesData); err != nil {
		return "", 0, fmt.Errorf("failed to decode files count: %w", err)
	}

	return folderData.Name, len(filesData.Files), nil
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

	folderName := req.FolderID
	fileCount := 0
	if config.GdriveOauthToken != "" {
		name, count, err := s.fetchFolderDetails(ctx, username, config, req.FolderID)
		if err == nil {
			folderName = name
			fileCount = count
		}
	}

	if s.syncProvider != nil {
		go func() {
			_ = s.syncProvider.SyncUserNotes(context.Background(), username)
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"folder_id":   req.FolderID,
		"folder_name": folderName,
		"file_count":  fileCount,
	})
}

// HandleGetFolderDetails handles retrieving details of a Google Drive folder.
func (s *Server) HandleGetFolderDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	username := cookie.Value
	ctx := r.Context()

	folderID := r.URL.Query().Get("folder_id")
	if folderID == "" {
		http.Error(w, "missing folder_id parameter", http.StatusBadRequest)
		return
	}

	config, err := s.store.GetUserConfig(ctx, username)
	if err != nil {
		http.Error(w, "user config not found", http.StatusNotFound)
		return
	}

	folderName := folderID
	fileCount := 0

	if config.GdriveOauthToken == "" {
		http.Error(w, "gdrive oauth token not configured", http.StatusBadRequest)
		return
	}

	name, count, err := s.fetchFolderDetails(ctx, username, config, folderID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get folder details: %v", err), http.StatusBadRequest)
		return
	}
	folderName = name
	fileCount = count

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"folder_id":   folderID,
		"folder_name": folderName,
		"file_count":  fileCount,
	})
}

// HandleGetSyncStatus handles checking the real-time sync progress.
func (s *Server) HandleGetSyncStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	username := cookie.Value

	if s.syncProvider == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"active": false})
		return
	}

	progress := s.syncProvider.GetSyncProgress(username)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(progress)
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

type TogglePageProcessedRequest struct {
	Processed bool `json:"processed"`
}

// HandleTogglePageProcessed toggles the processed state of a single note page.
func (s *Server) HandleTogglePageProcessed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := r.URL.Path
	prefix := "/api/pages/"
	suffix := "/processed"

	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	pageID := path[len(prefix) : len(path)-len(suffix)]
	if pageID == "" {
		http.Error(w, "bad request: missing page ID", http.StatusBadRequest)
		return
	}

	var req TogglePageProcessedRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	parts := strings.Split(pageID, "-page-")
	if len(parts) < 2 {
		http.Error(w, "invalid page ID format", http.StatusBadRequest)
		return
	}
	notebookID := parts[0]

	ctx := r.Context()
	notebook, err := s.store.GetNotebook(ctx, notebookID)
	if err != nil {
		http.Error(w, "notebook not found", http.StatusNotFound)
		return
	}

	found := false
	for _, page := range notebook.Pages {
		if page.Id == pageID {
			page.Processed = req.Processed
			page.UpdatedTime = time.Now().Unix()
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "page not found in notebook", http.StatusNotFound)
		return
	}

	err = s.store.SaveNotebook(ctx, notebook)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to save notebook: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
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

// HandleGetUserConfig retrieves the user configuration.
func (s *Server) HandleGetUserConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	username := cookie.Value
	ctx := r.Context()
	config, err := s.store.GetUserConfig(ctx, username)
	if err != nil {
		http.Error(w, "user config not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	bytes, err := protojson.Marshal(config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

// HandleGetNotebooks retrieves all notebooks for the user.
func (s *Server) HandleGetNotebooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("notes_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	notebooks, err := s.store.GetNotebooks(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get notebooks: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	var serialized []string
	for _, nb := range notebooks {
		bytes, err := protojson.Marshal(nb)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		serialized = append(serialized, string(bytes))
	}

	w.Write([]byte("[" + strings.Join(serialized, ",") + "]"))
}
