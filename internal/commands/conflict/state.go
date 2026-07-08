package conflict

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	stateVersion   = 1
	PhaseResolving = "resolving"
	PhaseCommitted = "committed"
)

var (
	ErrStateNotFound = errors.New("gUtil conflict state not found")
	ErrInvalidState  = errors.New("invalid gUtil conflict state")
)

type ConflictState struct {
	Version       int      `json:"version"`
	SourceBranch  string   `json:"sourceBranch"`
	TargetBranch  string   `json:"targetBranch"`
	SourceCommit  string   `json:"sourceCommit"`
	MergeCommit   string   `json:"mergeCommit"`
	ConflictFiles []string `json:"conflictFiles"`
	Phase         string   `json:"phase"`
	Commit        string   `json:"commit,omitempty"`
}

type StateStore struct{ Path string }

func (s StateStore) Load() (ConflictState, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return ConflictState{}, ErrStateNotFound
	}
	if err != nil {
		return ConflictState{}, fmt.Errorf("read gUtil conflict state: %w", err)
	}
	var state ConflictState
	if err := json.Unmarshal(data, &state); err != nil {
		return ConflictState{}, fmt.Errorf("%w: malformed JSON: %v", ErrInvalidState, err)
	}
	if err := normalizeAndValidate(&state); err != nil {
		return ConflictState{}, err
	}
	return state, nil
}

func (s StateStore) Exists() (bool, error) {
	_, err := os.Stat(s.Path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("inspect gUtil conflict state: %w", err)
}

func (s StateStore) Save(state ConflictState) error {
	if err := normalizeAndValidate(&state); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode gUtil conflict state: %w", err)
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create gUtil state directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".conflict-state-*")
	if err != nil {
		return fmt.Errorf("create temporary gUtil state: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return fmt.Errorf("write gUtil conflict state: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync gUtil conflict state: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close gUtil conflict state: %w", err)
	}
	if err := os.Rename(tempPath, s.Path); err != nil {
		return fmt.Errorf("replace gUtil conflict state: %w", err)
	}
	return nil
}

func (s StateStore) Remove() error {
	err := os.Remove(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove gUtil conflict state: %w", err)
	}
	return nil
}

func normalizeAndValidate(state *ConflictState) error {
	if state.Version != stateVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrInvalidState, state.Version)
	}
	if strings.TrimSpace(state.SourceBranch) == "" || strings.TrimSpace(state.TargetBranch) == "" || strings.TrimSpace(state.SourceCommit) == "" || strings.TrimSpace(state.MergeCommit) == "" {
		return fmt.Errorf("%w: required identity is missing", ErrInvalidState)
	}
	set := make(map[string]struct{}, len(state.ConflictFiles))
	for _, file := range state.ConflictFiles {
		if file = strings.TrimSpace(file); file != "" {
			set[file] = struct{}{}
		}
	}
	state.ConflictFiles = state.ConflictFiles[:0]
	for file := range set {
		state.ConflictFiles = append(state.ConflictFiles, file)
	}
	sort.Strings(state.ConflictFiles)
	if len(state.ConflictFiles) == 0 {
		return fmt.Errorf("%w: no conflict files", ErrInvalidState)
	}
	if state.Phase != PhaseResolving && state.Phase != PhaseCommitted {
		return fmt.Errorf("%w: unsupported phase %q", ErrInvalidState, state.Phase)
	}
	if state.Phase == PhaseCommitted && strings.TrimSpace(state.Commit) == "" {
		return fmt.Errorf("%w: committed phase has no commit", ErrInvalidState)
	}
	if state.Phase == PhaseResolving {
		state.Commit = ""
	}
	return nil
}
