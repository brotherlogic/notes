package storage_test

import (
	"context"
	"testing"

	"github.com/brotherlogic/notes/internal/storage"
	pb "github.com/brotherlogic/notes/proto"
	pstore_client "github.com/brotherlogic/pstore/client"
)

func TestSaveAndGetUserConfig(t *testing.T) {
	// Initialize a mocked PStore client
	testClient := pstore_client.GetTestClient()

	// Initialize our storage manager with the mocked client
	store := storage.NewStorage(testClient)

	ctx := context.Background()
	username := "test-github-user"

	// Define the expected user config
	config := &pb.UserConfig{
		GithubUsername:      username,
		GithubOauthToken:    "gho_test_token",
		GdriveOauthToken:    "ya29.test_token",
		GdriveRefreshToken:  "1//test_refresh_token",
		GdriveTokenExpiry:   1700000000,
		GdriveNotesFolderId: "gdrive_folder_abc123",
		LastSyncTime:        1600000000,
	}

	// 1. Save user configuration
	err := store.SaveUserConfig(ctx, config)
	if err != nil {
		t.Fatalf("Failed to save user config: %v", err)
	}

	// 2. Retrieve user configuration
	retrieved, err := store.GetUserConfig(ctx, username)
	if err != nil {
		t.Fatalf("Failed to retrieve user config: %v", err)
	}

	// 3. Verify fields match exactly
	if retrieved.GithubUsername != config.GithubUsername {
		t.Errorf("Expected GithubUsername %q, got %q", config.GithubUsername, retrieved.GithubUsername)
	}
	if retrieved.GithubOauthToken != config.GithubOauthToken {
		t.Errorf("Expected GithubOauthToken %q, got %q", config.GithubOauthToken, retrieved.GithubOauthToken)
	}
	if retrieved.GdriveOauthToken != config.GdriveOauthToken {
		t.Errorf("Expected GdriveOauthToken %q, got %q", config.GdriveOauthToken, retrieved.GdriveOauthToken)
	}
	if retrieved.GdriveRefreshToken != config.GdriveRefreshToken {
		t.Errorf("Expected GdriveRefreshToken %q, got %q", config.GdriveRefreshToken, retrieved.GdriveRefreshToken)
	}
	if retrieved.GdriveNotesFolderId != config.GdriveNotesFolderId {
		t.Errorf("Expected GdriveNotesFolderId %q, got %q", config.GdriveNotesFolderId, retrieved.GdriveNotesFolderId)
	}
}

func TestSaveAndGetNotebook(t *testing.T) {
	// Initialize mocked client
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	ctx := context.Background()
	notebookID := "notebook-123"

	notebook := &pb.Notebook{
		Id:            notebookID,
		Title:         "My Handwritten Notes",
		DriveFolderId: "drive_notebook_xyz",
		GithubProject: "brotherlogic/notes",
		LastUpdated:   1700000000,
		Pages: []*pb.Page{
			{
				Id:            "page-1",
				PageNumber:    1,
				DriveFileId:   "file_abc",
				GithubProject: "brotherlogic/notes",
				Processed:     false,
				CreatedTime:   1600000000,
				UpdatedTime:   1600000000,
				LocalFilePath: "/data/pages/page-1.bin",
			},
		},
	}

	// 1. Save notebook
	err := store.SaveNotebook(ctx, notebook)
	if err != nil {
		t.Fatalf("Failed to save notebook: %v", err)
	}

	// 2. Retrieve notebook
	retrieved, err := store.GetNotebook(ctx, notebookID)
	if err != nil {
		t.Fatalf("Failed to retrieve notebook: %v", err)
	}

	// 3. Verify fields match exactly
	if retrieved.Id != notebook.Id {
		t.Errorf("Expected Id %q, got %q", notebook.Id, retrieved.Id)
	}
	if retrieved.Title != notebook.Title {
		t.Errorf("Expected Title %q, got %q", notebook.Title, retrieved.Title)
	}
	if len(retrieved.Pages) != len(notebook.Pages) {
		t.Fatalf("Expected %d pages, got %d", len(notebook.Pages), len(retrieved.Pages))
	}
	if retrieved.Pages[0].Id != notebook.Pages[0].Id {
		t.Errorf("Expected page Id %q, got %q", notebook.Pages[0].Id, retrieved.Pages[0].Id)
	}
}
