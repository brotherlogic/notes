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

	// Set up mock Google Drive files
	mockGDrive := &MockGDriveClient{
		Files: []*sync.GDriveFile{
			{
				ID:          "file_page_1",
				Name:        "Notebook 1 - Page 1.png",
				MimeType:    "image/png",
				UpdatedTime: 1600000000,
			},
		},
		Data: map[string][]byte{
			"file_page_1": []byte("mock_png_image_bytes"),
		},
	}

	// Initialize sync worker
	worker := sync.NewWorker(store, mockGDrive, tempDir)

	// 1. Trigger the sync run for the user
	err = worker.SyncUserNotes(ctx, username)
	if err != nil {
		t.Fatalf("SyncUserNotes failed: %v", err)
	}

	// 2. Verify that a Notebook was created in storage
	notebookID := "folder_123"
	notebook, err := store.GetNotebook(ctx, notebookID)
	if err != nil {
		t.Fatalf("Failed to retrieve synced notebook: %v", err)
	}

	if notebook.Title != "folder_123" {
		t.Errorf("Expected notebook title 'folder_123', got %q", notebook.Title)
	}

	if len(notebook.Pages) != 1 {
		t.Fatalf("Expected 1 synced page, got %d", len(notebook.Pages))
	}

	page := notebook.Pages[0]
	if page.DriveFileId != "file_page_1" {
		t.Errorf("Expected page DriveFileId 'file_page_1', got %q", page.DriveFileId)
	}

	// 3. Verify that the raw binary file was saved to the temp disk directory
	expectedFilePath := filepath.Join(tempDir, page.Id+".bin")
	data, err := os.ReadFile(expectedFilePath)
	if err != nil {
		t.Fatalf("Failed to read downloaded binary page file: %v", err)
	}

	if string(data) != "mock_png_image_bytes" {
		t.Errorf("Expected file contents 'mock_png_image_bytes', got %q", string(data))
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
				Name:        "Notebook - Page 1.png",
				UpdatedTime: 1600000000,
			},
		},
		Data: map[string][]byte{
			"file_abc": []byte("image_data"),
		},
	}

	worker := sync.NewWorker(store, mockGDrive, tempDir)

	// 1. Run sync for all users
	err = worker.SyncAllUsers(ctx)
	if err != nil {
		t.Fatalf("SyncAllUsers failed: %v", err)
	}

	// 2. Verify that both notebooks were created
	_, err = store.GetNotebook(ctx, "folder_one")
	if err != nil {
		t.Errorf("Failed to retrieve notebook for user-one: %v", err)
	}

	_, err = store.GetNotebook(ctx, "folder_two")
	if err != nil {
		t.Errorf("Failed to retrieve notebook for user-two: %v", err)
	}
}
