package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type CheckpointManager struct {
	stateMachine *StateMachine
	storage      CheckpointStorage
}

type CheckpointStorage interface {
	Save(ctx context.Context, taskID string, checkpoint Checkpoint) error
	Load(ctx context.Context, taskID string) (*Checkpoint, error)
	Delete(ctx context.Context, taskID string) error
}

func NewCheckpointManager(sm *StateMachine, storage CheckpointStorage) *CheckpointManager {
	return &CheckpointManager{
		stateMachine: sm,
		storage:      storage,
	}
}

func (cm *CheckpointManager) SaveProgress(ctx context.Context, taskID string, step string, progress int, output string, files []string) error {
	checkpoint := Checkpoint{
		Step:      step,
		Progress:  progress,
		Output:    output,
		Files:     files,
		Timestamp: time.Now(),
	}

	cm.stateMachine.UpdateTask(taskID, func(t *TaskState) {
		t.Checkpoint = &checkpoint
	})

	if err := cm.storage.Save(ctx, taskID, checkpoint); err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}

	return nil
}

func (cm *CheckpointManager) Resume(ctx context.Context, taskID string) (*Checkpoint, error) {
	checkpoint, err := cm.storage.Load(ctx, taskID)
	if err != nil {
		return nil, err
	}

	cm.stateMachine.UpdateTask(taskID, func(t *TaskState) {
		t.Checkpoint = checkpoint
	})

	return checkpoint, nil
}

func (cm *CheckpointManager) Complete(ctx context.Context, taskID string) error {
	cm.stateMachine.UpdateTask(taskID, func(t *TaskState) {
		t.Checkpoint = nil
		t.Progress = 100
	})

	return cm.storage.Delete(ctx, taskID)
}

type MemoryCheckpointStorage struct {
	checkpoints map[string]Checkpoint
}

func NewMemoryCheckpointStorage() *MemoryCheckpointStorage {
	return &MemoryCheckpointStorage{
		checkpoints: make(map[string]Checkpoint),
	}
}

func (s *MemoryCheckpointStorage) Save(ctx context.Context, taskID string, checkpoint Checkpoint) error {
	s.checkpoints[taskID] = checkpoint
	return nil
}

func (s *MemoryCheckpointStorage) Load(ctx context.Context, taskID string) (*Checkpoint, error) {
	checkpoint, ok := s.checkpoints[taskID]
	if !ok {
		return nil, fmt.Errorf("checkpoint not found")
	}
	return &checkpoint, nil
}

func (s *MemoryCheckpointStorage) Delete(ctx context.Context, taskID string) error {
	delete(s.checkpoints, taskID)
	return nil
}

type DBCheckpointStorage struct {
	db interface {
		RPC(ctx context.Context, fn string, args map[string]any) (json.RawMessage, error)
	}
}

func NewDBCheckpointStorage(db interface {
	RPC(ctx context.Context, fn string, args map[string]any) (json.RawMessage, error)
}) *DBCheckpointStorage {
	return &DBCheckpointStorage{db: db}
}

func (s *DBCheckpointStorage) Save(ctx context.Context, taskID string, checkpoint Checkpoint) error {
	_, err := s.db.RPC(ctx, "save_checkpoint", map[string]any{
		"p_task_id":   taskID,
		"p_step":      checkpoint.Step,
		"p_progress":  checkpoint.Progress,
		"p_output":    checkpoint.Output,
		"p_files":     checkpoint.Files,
		"p_timestamp": checkpoint.Timestamp,
	})
	return err
}

func (s *DBCheckpointStorage) Load(ctx context.Context, taskID string) (*Checkpoint, error) {
	result, err := s.db.RPC(ctx, "load_checkpoint", map[string]any{
		"p_task_id": taskID,
	})
	if err != nil {
		return nil, err
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(result, &checkpoint); err != nil {
		return nil, fmt.Errorf("parse checkpoint: %w", err)
	}

	return &checkpoint, nil
}

func (s *DBCheckpointStorage) Delete(ctx context.Context, taskID string) error {
	_, err := s.db.RPC(ctx, "delete_checkpoint", map[string]any{
		"p_task_id": taskID,
	})
	return err
}
