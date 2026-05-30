package sync_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/brotherlogic/notes/internal/storage"
	"github.com/brotherlogic/notes/internal/sync"
	pb "github.com/brotherlogic/notes/proto"
	pstore_client "github.com/brotherlogic/pstore/client"
)

type MockGDriveClient struct {
	Files []*sync.GDriveFile
	Data  map[string][]byte
}

func (m *MockGDriveClient) ListFiles(ctx context.Context, folderID string) ([]*sync.GDriveFile, error) {
	return m.Files, nil
}

func (m *MockGDriveClient) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	return m.Data[fileID], nil
}

func TestSyncUserNotes(t *testing.T) {
	// Initialize mocked client and storage
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	// Set up temporary local binary storage folder
	tempDir, err := os.MkdirTemp("", "notes_sync_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	username := "test-user"

	// Preset UserConfig in pstore
	err = store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:      username,
		GdriveNotesFolderId: "folder_123",
		GdriveOauthToken:    "mock_oauth_token",
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	// Set up mock Google Drive files with a multi-page .note file
	mockGDrive := &MockGDriveClient{
		Files: []*sync.GDriveFile{
			{
				ID:          "file_note_1",
				Name:        "Lectures.note",
				MimeType:    "application/octet-stream",
				UpdatedTime: 1600000000,
			},
		},
		Data: map[string][]byte{
			"file_note_1": []byte("pages=2"), // ConvertNoteToPNGs will parse this to generate 2 mock pages
		},
	}

	// Initialize sync worker
	worker := sync.NewWorker(store, mockGDrive, tempDir)

	// 1. Trigger the sync run for the user
	err = worker.SyncUserNotes(ctx, username)
	if err != nil {
		t.Fatalf("SyncUserNotes failed: %v", err)
	}

	// 2. Verify that a Notebook was created using the .note file GDrive ID in storage
	notebookID := "file_note_1"
	notebook, err := store.GetNotebook(ctx, notebookID)
	if err != nil {
		t.Fatalf("Failed to retrieve synced notebook: %v", err)
	}

	if notebook.Title != "Lectures" {
		t.Errorf("Expected notebook title 'Lectures', got %q", notebook.Title)
	}

	if len(notebook.Pages) != 2 {
		t.Fatalf("Expected 2 synced pages, got %d", len(notebook.Pages))
	}

	// Verify page 1
	page1 := notebook.Pages[0]
	if page1.DriveFileId != "file_note_1" {
		t.Errorf("Expected page 1 DriveFileId 'file_note_1', got %q", page1.DriveFileId)
	}
	if page1.PageNumber != 1 {
		t.Errorf("Expected page 1 PageNumber 1, got %d", page1.PageNumber)
	}

	// Verify page 2
	page2 := notebook.Pages[1]
	if page2.DriveFileId != "file_note_1" {
		t.Errorf("Expected page 2 DriveFileId 'file_note_1', got %q", page2.DriveFileId)
	}
	if page2.PageNumber != 2 {
		t.Errorf("Expected page 2 PageNumber 2, got %d", page2.PageNumber)
	}

	// 3. Verify that the raw binary page files were saved to the temp disk directory
	expectedFilePath1 := filepath.Join(tempDir, page1.Id+".bin")
	if _, err := os.Stat(expectedFilePath1); os.IsNotExist(err) {
		t.Fatalf("Page 1 binary file not found at: %s", expectedFilePath1)
	}

	expectedFilePath2 := filepath.Join(tempDir, page2.Id+".bin")
	if _, err := os.Stat(expectedFilePath2); os.IsNotExist(err) {
		t.Fatalf("Page 2 binary file not found at: %s", expectedFilePath2)
	}
}

func TestSyncAllUsers(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	tempDir, err := os.MkdirTemp("", "notes_sync_all_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Preset two users
	err = store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:      "user-one",
		GdriveNotesFolderId: "folder_one",
		GdriveOauthToken:    "mock_token",
	})
	if err != nil {
		t.Fatalf("Failed to preset user-one: %v", err)
	}

	err = store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:      "user-two",
		GdriveNotesFolderId: "folder_two",
		GdriveOauthToken:    "mock_token",
	})
	if err != nil {
		t.Fatalf("Failed to preset user-two: %v", err)
	}

	mockGDrive := &MockGDriveClient{
		Files: []*sync.GDriveFile{
			{
				ID:          "file_abc",
				Name:        "Notebook.note",
				UpdatedTime: 1600000000,
			},
		},
		Data: map[string][]byte{
			"file_abc": []byte("pages=1"),
		},
	}

	worker := sync.NewWorker(store, mockGDrive, tempDir)

	// 1. Run sync for all users
	err = worker.SyncAllUsers(ctx)
	if err != nil {
		t.Fatalf("SyncAllUsers failed: %v", err)
	}

	// 2. Verify that both notebooks were created with GDrive IDs as notebook keys
	_, err = store.GetNotebook(ctx, "file_abc")
	if err != nil {
		t.Errorf("Failed to retrieve notebook: %v", err)
	}
}
