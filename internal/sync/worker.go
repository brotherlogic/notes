package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

type Worker struct {
	store     *storage.Storage
	gdrive    GDriveClient
	binaryDir string
}

func NewWorker(store *storage.Storage, gdrive GDriveClient, binaryDir string) *Worker {
	return &Worker{
		store:     store,
		gdrive:    gdrive,
		binaryDir: binaryDir,
	}
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
func (s *Worker) SyncUserNotes(ctx context.Context, username string) error {
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

	var pages []*pb.Page

	// Ensure binary storage directory exists
	err = os.MkdirAll(s.binaryDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create binary storage directory: %w", err)
	}

	// 3. Process each file
	for _, file := range files {
		pageID := fmt.Sprintf("%s-page-%s", folderID, file.ID)
		localPath := filepath.Join(s.binaryDir, pageID+".bin")

		// Download binary file
		data, err := s.gdrive.DownloadFile(ctx, file.ID)
		if err != nil {
			return fmt.Errorf("failed to download file %s: %w", file.ID, err)
		}

		// Write to flat filesystem path
		err = os.WriteFile(localPath, data, 0644)
		if err != nil {
			return fmt.Errorf("failed to write local file: %w", err)
		}

		// Construct Page metadata
		page := &pb.Page{
			Id:            pageID,
			PageNumber:    parsePageNumber(file.Name),
			DriveFileId:   file.ID,
			Processed:     false,
			CreatedTime:   time.Now().Unix(),
			UpdatedTime:   file.UpdatedTime,
			LocalFilePath: localPath,
		}

		pages = append(pages, page)
	}

	// 4. Save Notebook metadata to pstore
	if len(pages) > 0 {
		notebook := &pb.Notebook{
			Id:            folderID,
			Title:         folderID,
			DriveFolderId: folderID,
			Pages:         pages,
			LastUpdated:   time.Now().Unix(),
		}

		err = s.store.SaveNotebook(ctx, notebook)
		if err != nil {
			return fmt.Errorf("failed to save synced notebook: %w", err)
		}
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
