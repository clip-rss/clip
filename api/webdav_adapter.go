package api

import (
	"context"

	"github.com/clip-rss/clip/internal/opmlbackup"
	"github.com/clip-rss/clip/internal/webdav"
)

// webdavRemoteAdapter 将 webdav.Client 适配为 opmlbackup.Remote。
type webdavRemoteAdapter struct {
	client *webdav.Client
}

func (a *webdavRemoteAdapter) Get(ctx context.Context, path string) ([]byte, string, error) {
	return a.client.Get(ctx, path)
}

func (a *webdavRemoteAdapter) Put(ctx context.Context, path string, data []byte, ifMatch string) (string, error) {
	return a.client.Put(ctx, path, data, ifMatch)
}

func (a *webdavRemoteAdapter) List(ctx context.Context, dir string) ([]opmlbackup.ListEntry, error) {
	entries, err := a.client.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	result := make([]opmlbackup.ListEntry, len(entries))
	for i, e := range entries {
		result[i] = opmlbackup.ListEntry{
			Path:         e.Path,
			Size:         e.Size,
			LastModified: e.LastModified,
			IsDir:        e.IsDir,
		}
	}
	return result, nil
}

func (a *webdavRemoteAdapter) Delete(ctx context.Context, path string) error {
	return a.client.Delete(ctx, path)
}

func (a *webdavRemoteAdapter) MkcolAll(ctx context.Context, path string) error {
	return a.client.MkcolAll(ctx, path)
}
