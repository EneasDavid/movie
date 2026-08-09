package drive

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Category mirrors a Netflix-style "row": a named shelf of videos. Each
// direct subfolder of the configured root becomes one category; videos
// sitting directly in the root land in a synthetic "Catálogo" category.
type Category struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Items []File `json:"items"`
}

type Catalog struct {
	Categories  []Category `json:"categories"`
	GeneratedAt time.Time  `json:"generatedAt"`
}

// BuildCatalog lists the root folder, then fetches every subfolder's
// contents concurrently — this is the "multithreading" that keeps catalog
// build latency close to that of the single slowest subfolder instead of
// the sum of all of them, which matters a lot once there are a dozen+
// categories. Concurrency is still bounded by the Client's own semaphore
// (DriveMaxInFlight), so this can't overwhelm the Drive API quota.
func (c *Client) BuildCatalog(ctx context.Context, rootFolderID string) (*Catalog, error) {
	children, err := c.listChildren(ctx, rootFolderID)
	if err != nil {
		return nil, err
	}

	var folders []File
	var rootVideos []File
	for _, f := range children {
		switch {
		case f.IsFolder:
			folders = append(folders, f)
		case isVideo(f.MimeType):
			rootVideos = append(rootVideos, f)
		}
	}

	categories := make([]Category, 0, len(folders)+1)
	if len(rootVideos) > 0 {
		categories = append(categories, Category{ID: "root", Title: "Catálogo", Items: rootVideos})
	}

	type result struct {
		cat Category
		ok  bool
	}
	results := make([]result, len(folders))
	var wg sync.WaitGroup
	for i, folder := range folders {
		wg.Add(1)
		go func(i int, folder File) {
			defer wg.Done()
			items, err := c.listChildren(ctx, folder.ID)
			if err != nil {
				// A single broken/unshared subfolder shouldn't take down
				// the whole catalog — skip it, keep the rest.
				return
			}
			videos := make([]File, 0, len(items))
			for _, it := range items {
				if !it.IsFolder && isVideo(it.MimeType) {
					videos = append(videos, it)
				}
			}
			if len(videos) > 0 {
				results[i] = result{cat: Category{ID: folder.ID, Title: folder.Name, Items: videos}, ok: true}
			}
		}(i, folder)
	}
	wg.Wait()

	for _, res := range results {
		if res.ok {
			categories = append(categories, res.cat)
		}
	}

	return &Catalog{Categories: categories, GeneratedAt: time.Now()}, nil
}

func isVideo(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/")
}
