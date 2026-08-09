package opmlbackup

import (
	"context"
	"errors"
	"testing"
)

type deleteRemote struct {
	deletedPath string
	deleteCalls int
	deleteErr   error
}

func (r *deleteRemote) Get(context.Context, string) ([]byte, string, error) {
	return nil, "", nil
}

func (r *deleteRemote) Put(context.Context, string, []byte, string) (string, error) {
	return "", nil
}

func (r *deleteRemote) List(context.Context, string) ([]ListEntry, error) {
	return nil, nil
}

func (r *deleteRemote) Delete(_ context.Context, path string) error {
	r.deleteCalls++
	r.deletedPath = path
	return r.deleteErr
}

func (r *deleteRemote) MkcolAll(context.Context, string) error { return nil }

func TestManagerDelete(t *testing.T) {
	remote := &deleteRemote{}
	manager := &Manager{}

	err := manager.Delete(
		context.Background(),
		remote,
		"clip-feeds-20260809T120000-workstation.opml",
	)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if remote.deletedPath != "clip/opml/clip-feeds-20260809T120000-workstation.opml" {
		t.Errorf("deleted path = %q", remote.deletedPath)
	}
}

func TestManagerDeleteRejectsInvalidID(t *testing.T) {
	tests := []string{
		"",
		"../backup.opml",
		"nested/backup.opml",
		`nested\backup.opml`,
		"backup.xml",
		"clip-feeds-%2e%2e%2fsecret.opml",
		"backup.opml?version=1",
	}

	for _, id := range tests {
		t.Run(id, func(t *testing.T) {
			remote := &deleteRemote{}
			err := (&Manager{}).Delete(context.Background(), remote, id)
			if err == nil {
				t.Fatal("Delete should reject invalid ID")
			}
			if remote.deleteCalls != 0 {
				t.Errorf("Delete called %d times", remote.deleteCalls)
			}
		})
	}
}

func TestManagerDeletePropagatesRemoteError(t *testing.T) {
	want := errors.New("remote failed")
	remote := &deleteRemote{deleteErr: want}

	err := (&Manager{}).Delete(context.Background(), remote, "backup.opml")
	if !errors.Is(err, want) {
		t.Fatalf("Delete error = %v, want wrapped remote error", err)
	}
}
