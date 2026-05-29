package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/brotherlogic/notes/internal/api"
	"github.com/brotherlogic/notes/internal/storage"
	notesSync "github.com/brotherlogic/notes/internal/sync"
	pstore_client "github.com/brotherlogic/pstore/client"
)

// DynamicGDriveClient maps dynamic token configurations to Google Drive client instances.
type DynamicGDriveClient struct {
	store      *storage.Storage
	tokenCache map[string]string
	mu         sync.RWMutex
}

func NewDynamicGDriveClient(store *storage.Storage) *DynamicGDriveClient {
	return &DynamicGDriveClient{
		store:      store,
		tokenCache: make(map[string]string),
	}
}

func (d *DynamicGDriveClient) ListFiles(ctx context.Context, folderID string) ([]*notesSync.GDriveFile, error) {
	// Find user config associated with folderID
	users, err := d.store.GetUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve active users: %w", err)
	}

	var token string
	for _, username := range users {
		config, err := d.store.GetUserConfig(ctx, username)
		if err == nil && config.GdriveNotesFolderId == folderID {
			token = config.GdriveOauthToken
			break
		}
	}

	if token == "" {
		return nil, fmt.Errorf("gdrive oauth token not found for folder: %s", folderID)
	}

	realClient := notesSync.NewRealGDriveClient(token)
	files, err := realClient.ListFiles(ctx, folderID)
	if err != nil {
		return nil, err
	}

	// Cache the token for subsequent DownloadFile calls
	d.mu.Lock()
	for _, f := range files {
		d.tokenCache[f.ID] = token
	}
	d.mu.Unlock()

	return files, nil
}

func (d *DynamicGDriveClient) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	d.mu.RLock()
	token, exists := d.tokenCache[fileID]
	d.mu.RUnlock()

	if !exists {
		// Fallback: search all configs if cache miss (e.g. server restarted during loop)
		users, err := d.store.GetUsers(ctx)
		if err == nil {
			for _, username := range users {
				config, err := d.store.GetUserConfig(ctx, username)
				if err == nil && config.GdriveOauthToken != "" {
					token = config.GdriveOauthToken
					break
				}
			}
		}
	}

	if token == "" {
		return nil, fmt.Errorf("access token not found for file ID: %s", fileID)
	}

	realClient := notesSync.NewRealGDriveClient(token)
	return realClient.DownloadFile(ctx, fileID)
}

// spaHandler implements http.Handler to serve Single Page Application static files.
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Prepend staticPath to the request URL path to resolve files
	path := filepath.Join(h.staticPath, r.URL.Path)

	// Check if file exists
	fi, err := os.Stat(path)
	if os.IsNotExist(err) || fi.IsDir() {
		// If file doesn't exist or is a directory, serve index.html (SPA fallback)
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Serve the static file
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

func main() {
	log.Println("Starting Notes Management System Production Server...")

	// 1. Parse configuration from environment variables
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("System configuration error: %v", err)
	}
	log.Printf("Configuration loaded successfully. Port: %d, DataDir: %s", cfg.Port, cfg.DataDir)

	// Ensure flat binary directory path exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create binary storage directory: %v", err)
	}

	// 2. Initialize Database Client (pstore)
	pstoreClient, err := pstore_client.GetClient()
	if err != nil {
		log.Fatalf("Failed to initialize database client: %v", err)
	}
	log.Println("Database client connection pool established.")

	// Instantiate Core Storage
	store := storage.NewStorage(pstoreClient)

	// Instantiate Dynamic Google Drive client wrapper
	dynamicGDrive := NewDynamicGDriveClient(store)

	// Instantiate Sync Worker
	worker := notesSync.NewWorker(store, dynamicGDrive, cfg.DataDir)

	// Instantiate API Server
	server := api.NewServer(store)
	server.SetBinaryDir(cfg.DataDir)
	server.SetOAuthCredentials(cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GDriveClientID, cfg.GDriveClientSecret)

	// 3. Orchestrate Background Sync Loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("Starting background Google Drive synchronization worker loop (5m ticker)...")
	worker.Start(ctx, 5*time.Minute)

	// 4. Wire Up HTTP Server Routing
	mux := http.NewServeMux()

	// OAuth login routes
	mux.HandleFunc("/login/github", server.HandleGitHubLogin)
	mux.HandleFunc("/link/gdrive", server.HandleGDriveLogin)

	// OAuth callback routes
	mux.HandleFunc("/login/github/callback", server.HandleGitHubCallback)
	mux.HandleFunc("/link/gdrive/callback", server.HandleGDriveCallback)

	// Config and Toggle API endpoints
	mux.HandleFunc("/api/user/config", server.HandleGetUserConfig)
	mux.HandleFunc("/api/notebooks", server.HandleGetNotebooks)
	mux.HandleFunc("/api/configure-folder", server.HandleConfigureFolder)
	mux.HandleFunc("/api/logout", server.HandleLogout)
	mux.HandleFunc("/api/pages/", func(w http.ResponseWriter, r *http.Request) {
		// Handle either serving raw asset image or toggling processed status
		if filepath.Base(r.URL.Path) == "processed" {
			server.HandleTogglePageProcessed(w, r)
		} else {
			server.HandleServeAsset(w, r)
		}
	})

	// GitHub visual crop issue creations
	mux.HandleFunc("/api/issues/create", server.HandleCreateIssue)

	// Serve React Frontend SPA built static assets
	spa := spaHandler{staticPath: cfg.FrontendDir, indexPath: "index.html"}
	mux.Handle("/", spa)

	// Start Server
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 5. Orchestrate Clean Graceful Shutdown
	go func() {
		log.Printf("HTTP Server listening and serving on %s", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP Server ListenAndServe error: %v", err)
		}
	}()

	// Signal channel to intercept termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received
	sig := <-sigChan
	log.Printf("Termination signal captured: %v. Initiating graceful shutdown...", sig)

	// Cancel context to stop sync background workers
	cancel()

	// Establish a timeout context to flush HTTP requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP Server Shutdown failed: %v", err)
	}

	log.Println("Notes Management System Server gracefully terminated.")
}
