package drive

import (
	"context"
	"path/filepath"
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
	var rootFiles []File
	for _, f := range children {
		switch {
		case f.IsFolder:
			folders = append(folders, f)
		default:
			rootFiles = append(rootFiles, f)
		}
	}
	rootVideos := videosWithFolderCovers(rootFiles)

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
			videos := videosWithFolderCovers(items)
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

func isImage(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

// videosWithFolderCovers treats image files beside videos as explicit
// artwork. A single image covers every video in that folder; with several
// images, matching basenames ("Movie.mp4" + "Movie.jpg") win. This keeps
// cover management entirely in Drive without adding an admin UI.
func videosWithFolderCovers(files []File) []File {
	videos := make([]File, 0, len(files))
	images := make([]File, 0, len(files))
	for _, f := range files {
		switch {
		case !f.IsFolder && isVideo(f.MimeType):
			videos = append(videos, f)
		case !f.IsFolder && isImage(f.MimeType) && f.ThumbnailURL != "":
			images = append(images, f)
		}
	}

	byBase := make(map[string]string, len(images))
	for _, image := range images {
		byBase[mediaBaseName(image.Name)] = image.ThumbnailURL
	}
	for i := range videos {
		if cover := byBase[mediaBaseName(videos[i].Name)]; cover != "" {
			videos[i].ThumbnailURL = cover
		} else if len(images) == 1 {
			videos[i].ThumbnailURL = images[0].ThumbnailURL
		}
	}
	return videos
}

func mediaBaseName(name string) string {
	ext := filepath.Ext(name)
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(name, ext)))
}
