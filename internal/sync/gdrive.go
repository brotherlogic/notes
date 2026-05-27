package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RealGDriveClient struct {
	token string
}

func NewRealGDriveClient(token string) *RealGDriveClient {
	return &RealGDriveClient{token: token}
}

func (r *RealGDriveClient) ListFiles(ctx context.Context, folderID string) ([]*GDriveFile, error) {
	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files?q='%s'+in+parents+and+trashed=false&fields=files(id,name,mimeType,modifiedTime)", folderID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gdrive api returned status %s: %s", resp.Status, string(body))
	}

	var data struct {
		Files []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			MimeType     string `json:"mimeType"`
			ModifiedTime string `json:"modifiedTime"`
		} `json:"files"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []*GDriveFile
	for _, f := range data.Files {
		t, _ := time.Parse(time.RFC3339, f.ModifiedTime)
		results = append(results, &GDriveFile{
			ID:          f.ID,
			Name:        f.Name,
			MimeType:    f.MimeType,
			UpdatedTime: t.Unix(),
		})
	}

	return results, nil
}

func (r *RealGDriveClient) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s?alt=media", fileID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gdrive download returned status %s: %s", resp.Status, string(body))
	}

	return io.ReadAll(resp.Body)
}
