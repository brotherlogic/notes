package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brotherlogic/notes/internal/api"
	"github.com/brotherlogic/notes/internal/storage"
	pb "github.com/brotherlogic/notes/proto"
	pstore_client "github.com/brotherlogic/pstore/client"
)

func TestHandleTogglePageProcessed(t *testing.T) {
	testClient := pstore_client.GetTestClient()
	store := storage.NewStorage(testClient)
	server := api.NewServer(store)

	ctx := context.Background()
	username := "test-github-user"
	notebookID := "folder_abc"
	pageID := "folder_abc-page-file_xyz"

	// 1. Preset UserConfig
	err := store.SaveUserConfig(ctx, &pb.UserConfig{
		GithubUsername: username,
	})
	if err != nil {
		t.Fatalf("Failed to preset user: %v", err)
	}

	// 2. Preset Notebook with a page that has processed = false
	err = store.SaveNotebook(ctx, &pb.Notebook{
		Id: notebookID,
		Pages: []*pb.Page{
			{
				Id:        pageID,
				Processed: false,
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to preset notebook: %v", err)
	}

	// 3. Make POST request to mark the page as processed
	payload := `{"processed": true}`
	req := httptest.NewRequest("POST", "/api/pages/folder_abc-page-file_xyz/processed", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "notes_session", Value: username})
	w := httptest.NewRecorder()

	// 4. Call handler
	server.HandleTogglePageProcessed(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %v", resp.StatusCode)
	}

	// 5. Retrieve notebook and assert page processed flag is true
	notebook, err := store.GetNotebook(ctx, notebookID)
	if err != nil {
		t.Fatalf("Failed to retrieve notebook: %v", err)
	}

	if len(notebook.Pages) != 1 {
		t.Fatalf("Expected 1 page, got %d", len(notebook.Pages))
	}

	if !notebook.Pages[0].Processed {
		t.Errorf("Expected page processed flag to be true, got false")
	}
}
