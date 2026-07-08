package conflict

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func validState() ConflictState {
	return ConflictState{Version: 2, OriginalSourceBranch: "feature/a", SourceBranch: "feature/a", TargetBranch: "develop", SourceCommit: "abc", MergeCommit: "def", ConflictFiles: []string{"b.txt", "a.txt", "a.txt"}, Phase: PhaseResolving}
}

func TestGeneratedBranchStateRoundTrip(t *testing.T) {
	state := validState()
	state.SourceBranch = "feature/conflictResolution/develop/08072026"
	state.GeneratedBranch = true
	store := StateStore{Path: filepath.Join(t.TempDir(), "state.json")}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil || !got.GeneratedBranch || got.OriginalSourceBranch != "feature/a" {
		t.Fatalf("state = %#v, err = %v", got, err)
	}
}

func TestStateStoreRoundTripCanonicalizesFiles(t *testing.T) {
	store := StateStore{Path: filepath.Join(t.TempDir(), "git", "gutil", "conflict-state.json")}
	if err := store.Save(validState()); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.ConflictFiles, []string{"a.txt", "b.txt"}) {
		t.Fatalf("files = %#v", got.ConflictFiles)
	}
	if err := store.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("load error = %v", err)
	}
	if err := store.Remove(); err != nil {
		t.Fatalf("idempotent remove: %v", err)
	}
}

func TestStateStoreRejectsInvalidState(t *testing.T) {
	tests := []ConflictState{
		{},
		{Version: 3, SourceBranch: "a", TargetBranch: "b", SourceCommit: "c", MergeCommit: "d", ConflictFiles: []string{"x"}, Phase: PhaseResolving},
		{Version: 2, SourceBranch: "a", TargetBranch: "b", SourceCommit: "c", MergeCommit: "d", ConflictFiles: []string{"x"}, Phase: "wrong"},
		{Version: 2, SourceBranch: "a", TargetBranch: "b", SourceCommit: "c", MergeCommit: "d", ConflictFiles: []string{"x"}, Phase: PhaseCommitted},
	}
	for _, state := range tests {
		store := StateStore{Path: filepath.Join(t.TempDir(), "state.json")}
		if err := store.Save(state); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("state %#v: err = %v", state, err)
		}
	}
}

func TestCommittedStateRequiresCommit(t *testing.T) {
	state := validState()
	state.Phase = PhaseCommitted
	state.Commit = "fed"
	store := StateStore{Path: filepath.Join(t.TempDir(), "state.json")}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil || got.Commit != "fed" {
		t.Fatalf("state = %#v, err = %v", got, err)
	}
}
