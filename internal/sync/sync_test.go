package sync_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brotherlogic/notes/internal/storage"
	"github.com/brotherlogic/notes/internal/sync"
	pb "github.com/brotherlogic/notes/proto"
	pstore_client "github.com/brotherlogic/pstore/client"
)

type MockGDriveClient struct {
	Files          []*sync.GDriveFile
	Data           map[string][]byte
	DownloadErrors map[string]error
	ListError      error
	BlockChan      map[string]chan struct{}
}

func (m *MockGDriveClient) ListFiles(ctx context.Context, folderID string) ([]*sync.GDriveFile, error) {
	if m.ListError != nil {
		return nil, m.ListError
	}
	return m.Files, nil
}

func (m *MockGDriveClient) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	if m.BlockChan != nil {
		if ch, exists := m.BlockChan[fileID]; exists {
			<-ch
		}
	}
	if m.DownloadErrors != nil {
		if err := m.DownloadErrors[fileID]; err != nil {
			return nil, err
		}
	}
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

func TestSyncUserNotes_UserLock(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	tempDir, err := os.MkdirTemp("", "notes_sync_lock_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	username := "lock-user"

	err = store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:      username,
		GdriveNotesFolderId: "folder_lock",
		GdriveOauthToken:    "mock_token",
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	blockCh := make(chan struct{})
	mockGDrive := &MockGDriveClient{
		Files: []*sync.GDriveFile{
			{
				ID:          "file_lock",
				Name:        "Lock.note",
				UpdatedTime: 1600000000,
			},
		},
		Data: map[string][]byte{
			"file_lock": []byte("pages=1"),
		},
		BlockChan: map[string]chan struct{}{
			"file_lock": blockCh,
		},
	}

	worker := sync.NewWorker(store, mockGDrive, tempDir)

	errChan := make(chan error, 1)
	go func() {
		errChan <- worker.SyncUserNotes(ctx, username)
	}()

	// Wait a tiny bit for the goroutine to enter DownloadFile and block
	time.Sleep(50 * time.Millisecond)

	// Trigger second sync run for the same user, which should fail due to lock
	err2 := worker.SyncUserNotes(ctx, username)
	if err2 == nil {
		t.Error("Expected second concurrent sync to fail, but got nil")
	} else if !strings.Contains(err2.Error(), "sync already in progress") {
		t.Errorf("Expected 'sync already in progress' error, got %v", err2)
	}

	// Unblock the first sync and wait for it to finish
	close(blockCh)
	err1 := <-errChan
	if err1 != nil {
		t.Errorf("First sync failed unexpectedly: %v", err1)
	}
}

func TestSyncUserNotes_ErrorIsolation(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	tempDir, err := os.MkdirTemp("", "notes_error_isolation_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	username := "error-user"

	err = store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:      username,
		GdriveNotesFolderId: "folder_err",
		GdriveOauthToken:    "mock_token",
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	mockGDrive := &MockGDriveClient{
		Files: []*sync.GDriveFile{
			{
				ID:          "file_valid",
				Name:        "Valid.note",
				UpdatedTime: 1600000000,
			},
			{
				ID:          "file_corrupt",
				Name:        "Corrupt.note",
				UpdatedTime: 1600000000,
			},
		},
		Data: map[string][]byte{
			"file_valid":   []byte("pages=1"),
			"file_corrupt": []byte("corrupt data that is not JSON or PNG"),
		},
	}

	worker := sync.NewWorker(store, mockGDrive, tempDir)

	err = worker.SyncUserNotes(ctx, username)
	if err != nil {
		t.Fatalf("SyncUserNotes failed completely: %v", err)
	}

	// Verify Valid.note is ACTIVE
	nbValid, err := store.GetNotebook(ctx, "file_valid")
	if err != nil {
		t.Fatalf("Failed to get Valid notebook: %v", err)
	}
	if nbValid.Status != pb.NotebookStatus_NOTEBOOK_ACTIVE {
		t.Errorf("Expected Valid notebook to be ACTIVE, got %v", nbValid.Status)
	}

	// Verify Corrupt.note is UNPROCESSABLE
	nbCorrupt, err := store.GetNotebook(ctx, "file_corrupt")
	if err != nil {
		t.Fatalf("Failed to get Corrupt notebook: %v", err)
	}
	if nbCorrupt.Status != pb.NotebookStatus_NOTEBOOK_UNPROCESSABLE {
		t.Errorf("Expected Corrupt notebook to be UNPROCESSABLE, got %v", nbCorrupt.Status)
	}
}

func TestSyncUserNotes_ChangeDetection(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	tempDir, err := os.MkdirTemp("", "notes_change_detection_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	username := "change-user"

	err = store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:      username,
		GdriveNotesFolderId: "folder_change",
		GdriveOauthToken:    "mock_token",
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	mockGDrive := &MockGDriveClient{
		Files: []*sync.GDriveFile{
			{
				ID:          "file_change",
				Name:        "Change.note",
				UpdatedTime: 1600000000,
			},
		},
		Data: map[string][]byte{
			"file_change": []byte("pages=1"),
		},
	}

	worker := sync.NewWorker(store, mockGDrive, tempDir)

	// First sync
	err = worker.SyncUserNotes(ctx, username)
	if err != nil {
		t.Fatalf("First sync failed: %v", err)
	}

	nb, err := store.GetNotebook(ctx, "file_change")
	if err != nil {
		t.Fatalf("Failed to get notebook: %v", err)
	}
	pagePath := nb.Pages[0].LocalFilePath
	initialInfo, err := os.Stat(pagePath)
	if err != nil {
		t.Fatalf("Failed to stat page file: %v", err)
	}
	initialModTime := initialInfo.ModTime()

	// Wait a moment so mod time would change if written
	time.Sleep(10 * time.Millisecond)

	// Second sync - should skip file write because hash is unchanged
	err = worker.SyncUserNotes(ctx, username)
	if err != nil {
		t.Fatalf("Second sync failed: %v", err)
	}

	secondInfo, err := os.Stat(pagePath)
	if err != nil {
		t.Fatalf("Failed to stat page file on second sync: %v", err)
	}

	if !secondInfo.ModTime().Equal(initialModTime) {
		t.Error("Expected file modification time to remain unchanged (file write skipped)")
	}

	// Wait a moment so mod time would change when written
	time.Sleep(10 * time.Millisecond)

	// Third sync - update the file's name to trigger content changes (different title inside mock PNG)
	mockGDrive.Files[0].Name = "ChangeModified.note"
	mockGDrive.Files[0].UpdatedTime = 1600000001
	err = worker.SyncUserNotes(ctx, username)
	if err != nil {
		t.Fatalf("Third sync failed: %v", err)
	}

	thirdInfo, err := os.Stat(pagePath)
	if err != nil {
		t.Fatalf("Failed to stat page file on third sync: %v", err)
	}

	if thirdInfo.ModTime().Equal(secondInfo.ModTime()) {
		t.Error("Expected file modification time to change on third sync (file write occurred)")
	}

	// Verify that the updated notebook status and new page hash is successfully updated in storage
	updatedNB, err := store.GetNotebook(ctx, "file_change")
	if err != nil {
		t.Fatalf("Failed to get updated notebook: %v", err)
	}
	if updatedNB.Title != "ChangeModified" {
		t.Errorf("Expected notebook title 'ChangeModified', got %q", updatedNB.Title)
	}
	if updatedNB.Pages[0].ImageHash == nb.Pages[0].ImageHash {
		t.Error("Expected page ImageHash to be updated/different, but it remained the same")
	}
}

func TestSyncUserNotes_SoftArchiving(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)

	tempDir, err := os.MkdirTemp("", "notes_soft_archive_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	username := "archive-user"
	folderID := "folder_archive"

	err = store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername:      username,
		GdriveNotesFolderId: folderID,
		GdriveOauthToken:    "mock_token",
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	// Preset a notebook that will become archived because it is not on GDrive
	err = store.SaveNotebook(ctx, &pb.Notebook{
		Id:            "file_archived",
		Title:         "ArchivedNotebook",
		DriveFolderId: folderID,
		Status:        pb.NotebookStatus_NOTEBOOK_ACTIVE,
	})
	if err != nil {
		t.Fatalf("Failed to save preset notebook: %v", err)
	}

	// GDrive lists files, but "file_archived" is missing!
	mockGDrive := &MockGDriveClient{
		Files: []*sync.GDriveFile{
			{
				ID:          "file_present",
				Name:        "Present.note",
				UpdatedTime: 1600000000,
			},
		},
		Data: map[string][]byte{
			"file_present": []byte("pages=1"),
		},
	}

	worker := sync.NewWorker(store, mockGDrive, tempDir)

	err = worker.SyncUserNotes(ctx, username)
	if err != nil {
		t.Fatalf("SyncUserNotes failed: %v", err)
	}

	// Verify file_archived is marked DELETED_ON_REMOTE
	nbArchived, err := store.GetNotebook(ctx, "file_archived")
	if err != nil {
		t.Fatalf("Failed to get archived notebook: %v", err)
	}
	if nbArchived.Status != pb.NotebookStatus_NOTEBOOK_DELETED_ON_REMOTE {
		t.Errorf("Expected notebook status to be DELETED_ON_REMOTE, got %v", nbArchived.Status)
	}

	// Verify file_present is marked ACTIVE
	nbPresent, err := store.GetNotebook(ctx, "file_present")
	if err != nil {
		t.Fatalf("Failed to get present notebook: %v", err)
	}
	if nbPresent.Status != pb.NotebookStatus_NOTEBOOK_ACTIVE {
		t.Errorf("Expected present notebook to be ACTIVE, got %v", nbPresent.Status)
	}
}
