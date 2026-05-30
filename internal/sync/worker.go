package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brotherlogic/notes/internal/storage"
	pb "github.com/brotherlogic/notes/proto"
)

type GDriveFile struct {
	ID          string
	Name        string
	MimeType    string
	UpdatedTime int64
}

// GDriveClient abstracts the Google Drive API.
type GDriveClient interface {
	ListFiles(ctx context.Context, folderID string) ([]*GDriveFile, error)
	DownloadFile(ctx context.Context, fileID string) ([]byte, error)
}

type SyncProgress struct {
	Active  bool   `json:"active"`
	Current int32  `json:"current"`
	Total   int32  `json:"total"`
	Error   string `json:"error,omitempty"`
}

type Worker struct {
	store      *storage.Storage
	gdrive     GDriveClient
	binaryDir  string
	progress   map[string]*SyncProgress
	progressMu sync.RWMutex
}

func NewWorker(store *storage.Storage, gdrive GDriveClient, binaryDir string) *Worker {
	return &Worker{
		store:     store,
		gdrive:    gdrive,
		binaryDir: binaryDir,
		progress:  make(map[string]*SyncProgress),
	}
}

func (w *Worker) GetSyncProgress(username string) *SyncProgress {
	w.progressMu.RLock()
	defer w.progressMu.RUnlock()
	if p, exists := w.progress[username]; exists {
		return &SyncProgress{
			Active:  p.Active,
			Current: p.Current,
			Total:   p.Total,
			Error:   p.Error,
		}
	}
	return &SyncProgress{Active: false}
}

// parsePageNumber parses the page number from filenames like "Notebook 1 - Page 5.png" -> 5
func parsePageNumber(name string) int32 {
	re := regexp.MustCompile(`(?i)Page[ _-]?(\d+)`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 1 {
		val, _ := strconv.Atoi(matches[1])
		return int32(val)
	}
	return 1
}

// SyncUserNotes polls the configured Google Drive folder for a user and syncs new notes.
func (s *Worker) SyncUserNotes(ctx context.Context, username string) (err error) {
	s.progressMu.Lock()
	s.progress[username] = &SyncProgress{Active: true, Total: 0, Current: 0}
	s.progressMu.Unlock()

	defer func() {
		s.progressMu.Lock()
		s.progress[username].Active = false
		if err != nil {
			s.progress[username].Error = err.Error()
		}
		s.progressMu.Unlock()
	}()

	// 1. Get user configuration
	config, err := s.store.GetUserConfig(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to get user config: %w", err)
	}

	folderID := config.GdriveNotesFolderId
	if folderID == "" {
		return fmt.Errorf("no Google Drive notes folder configured for user %s", username)
	}

	// 2. List files in the configured folder
	files, err := s.gdrive.ListFiles(ctx, folderID)
	if err != nil {
		return fmt.Errorf("failed to list drive files: %w", err)
	}

	// Filter for .note files
	var noteFiles []*GDriveFile
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file.Name), ".note") {
			noteFiles = append(noteFiles, file)
		}
	}

	s.progressMu.Lock()
	s.progress[username].Total = int32(len(noteFiles))
	s.progressMu.Unlock()

	// Ensure binary storage directory exists
	err = os.MkdirAll(s.binaryDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create binary storage directory: %w", err)
	}

	// 3. Process each .note file as a multi-page Notebook
	for i, file := range noteFiles {
		notebookTitle := strings.TrimSuffix(file.Name, ".note")
		notebookID := file.ID

		// Download binary .note file bytes
		data, err := s.gdrive.DownloadFile(ctx, file.ID)
		if err != nil {
			return fmt.Errorf("failed to download file %s: %w", file.ID, err)
		}

		// Convert .note bytes to PNG pages
		pngPages, err := ConvertNoteToPNGs(ctx, notebookTitle, data)
		if err != nil {
			return fmt.Errorf("failed to convert .note file %s: %w", file.Name, err)
		}

		// Retrieve existing notebook to preserve metadata (processed states, linked github projects)
		existingNotebook, err := s.store.GetNotebook(ctx, notebookID)
		processedMap := make(map[int32]bool)
		githubProjectMap := make(map[int32]string)
		if err == nil && existingNotebook != nil {
			for _, p := range existingNotebook.Pages {
				processedMap[p.PageNumber] = p.Processed
				githubProjectMap[p.PageNumber] = p.GithubProject
			}
		}

		var pages []*pb.Page
		for pageIdx, pngBytes := range pngPages {
			pageNum := int32(pageIdx + 1)
			pageID := fmt.Sprintf("%s-page-%d", notebookID, pageNum)
			localPath := filepath.Join(s.binaryDir, pageID+".bin")

			// Write PNG page bytes to flat filesystem path
			err = os.WriteFile(localPath, pngBytes, 0644)
			if err != nil {
				return fmt.Errorf("failed to write local page file: %w", err)
			}

			page := &pb.Page{
				Id:            pageID,
				PageNumber:    pageNum,
				DriveFileId:   file.ID,
				Processed:     false,
				CreatedTime:   time.Now().Unix(),
				UpdatedTime:   file.UpdatedTime,
				LocalFilePath: localPath,
			}

			// Preserve existing metadata if available
			if proc, exists := processedMap[pageNum]; exists {
				page.Processed = proc
			}
			if ghProj, exists := githubProjectMap[pageNum]; exists {
				page.GithubProject = ghProj
			}

			pages = append(pages, page)
		}

		// 4. Save distinct Notebook metadata to pstore
		notebook := &pb.Notebook{
			Id:            notebookID,
			Title:         notebookTitle,
			DriveFolderId: folderID,
			Pages:         pages,
			LastUpdated:   time.Now().Unix(),
		}

		err = s.store.SaveNotebook(ctx, notebook)
		if err != nil {
			return fmt.Errorf("failed to save synced notebook %s: %w", notebookID, err)
		}

		s.progressMu.Lock()
		s.progress[username].Current = int32(i + 1)
		s.progressMu.Unlock()
	}

	return nil
}

// SyncAllUsers queries all registered users and triggers SyncUserNotes for each.
func (s *Worker) SyncAllUsers(ctx context.Context) error {
	users, err := s.store.GetUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active users for sync: %w", err)
	}

	for _, username := range users {
		// Sync each user independently; if one fails, log and continue syncing other users
		err = s.SyncUserNotes(ctx, username)
		if err != nil {
			fmt.Printf("Sync error for user %s: %v\n", username, err)
		}
	}

	return nil
}

// Start spawns a background goroutine that polls and syncs notes periodically.
func (s *Worker) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		// Perform initial sync run on startup
		_ = s.SyncAllUsers(ctx)

		for {
			select {
			case <-ticker.C:
				_ = s.SyncAllUsers(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
