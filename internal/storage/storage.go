package storage

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/brotherlogic/notes/proto"
	pstore_client "github.com/brotherlogic/pstore/client"
	pstore_pb "github.com/brotherlogic/pstore/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type Storage struct {
	client pstore_client.PStoreClient
}

func NewStorage(client pstore_client.PStoreClient) *Storage {
	return &Storage{client: client}
}

// SaveUserConfig saves the user's OAuth tokens and configurations to pstore.
func (s *Storage) SaveUserConfig(ctx context.Context, config *pb.UserConfig) error {
	if config.GithubUsername == "" {
		return fmt.Errorf("github username cannot be empty")
	}

	anyVal, err := anypb.New(config)
	if err != nil {
		return fmt.Errorf("failed to marshal UserConfig: %w", err)
	}

	key := fmt.Sprintf("user_config/%s", config.GithubUsername)
	req := &pstore_pb.WriteRequest{
		Key:   key,
		Value: anyVal,
	}

	_, err = s.client.Write(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to write UserConfig: %w", err)
	}
	return nil
}

// GetUserConfig retrieves the user's OAuth tokens and configurations from pstore.
func (s *Storage) GetUserConfig(ctx context.Context, username string) (*pb.UserConfig, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	key := fmt.Sprintf("user_config/%s", username)
	req := &pstore_pb.ReadRequest{
		Key: key,
	}

	resp, err := s.client.Read(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to read UserConfig: %w", err)
	}

	if resp.GetValue() == nil {
		return nil, fmt.Errorf("user config not found for key: %s", key)
	}

	config := &pb.UserConfig{}
	err = proto.Unmarshal(resp.GetValue().Value, config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal UserConfig from bytes: %w", err)
	}

	return config, nil
}

// SaveNotebook saves the notebook structure (including page metadata) to pstore.
func (s *Storage) SaveNotebook(ctx context.Context, notebook *pb.Notebook) error {
	if notebook.Id == "" {
		return fmt.Errorf("notebook id cannot be empty")
	}

	anyVal, err := anypb.New(notebook)
	if err != nil {
		return fmt.Errorf("failed to marshal Notebook: %w", err)
	}

	key := fmt.Sprintf("notebook/%s", notebook.Id)
	req := &pstore_pb.WriteRequest{
		Key:   key,
		Value: anyVal,
	}

	_, err = s.client.Write(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to write Notebook: %w", err)
	}
	return nil
}

// GetNotebook retrieves a specific notebook from pstore.
func (s *Storage) GetNotebook(ctx context.Context, id string) (*pb.Notebook, error) {
	if id == "" {
		return nil, fmt.Errorf("notebook id cannot be empty")
	}

	key := fmt.Sprintf("notebook/%s", id)
	req := &pstore_pb.ReadRequest{
		Key: key,
	}

	resp, err := s.client.Read(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to read Notebook: %w", err)
	}

	if resp.GetValue() == nil {
		return nil, fmt.Errorf("notebook not found for key: %s", key)
	}

	notebook := &pb.Notebook{}
	err = proto.Unmarshal(resp.GetValue().Value, notebook)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal Notebook from bytes: %w", err)
	}

	return notebook, nil
}

// GetUsers returns a list of all user names registered in the pstore.
func (s *Storage) GetUsers(ctx context.Context) ([]string, error) {
	req := &pstore_pb.GetKeysRequest{
		Prefix: "user_config/",
	}

	resp, err := s.client.GetKeys(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user keys: %w", err)
	}

	var users []string
	for _, key := range resp.GetKeys() {
		parts := strings.Split(key, "/")
		if len(parts) > 1 {
			users = append(users, parts[1])
		}
	}
	return users, nil
}
