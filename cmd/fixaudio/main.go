// Command fixaudio automates the E-AC-3-audio fix end-to-end against a
// live Google Drive catalog folder: list every video (root + one level of
// subfolders, mirroring internal/drive/catalog.go's category structure),
// download each one, inspect its audio codec with ffprobe, remux
// (video copy + AAC audio, via ffmpeg — see scripts/fix-audio-codec.sh for
// the same logic as a standalone script) anything incompatible, and
// upload the fixed content back onto the SAME Drive file ID.
//
// Reusing the file ID instead of delete+recreate matters: the catalog
// cache, thumbnail cache, and any user's "continuar assistindo" progress
// are all keyed by fileId, so an in-place content replacement needs no
// follow-up cleanup anywhere else in the app.
//
// WHY THIS RUNS LOCALLY, NOT INSIDE THE DEPLOYED APP:
// Same constraint as scripts/fix-audio-codec.sh — Vercel's Go runtime has
// no ffmpeg and no execution budget for transcoding a feature-length
// file, so this has to run wherever ffmpeg actually lives (your machine).
// What THIS tool adds over the plain script is not having to manually
// download/upload each file yourself — it drives the Drive API directly.
//
// SETUP (one-time, in Google Cloud Console — see README.md for the full
// walkthrough):
//  1. Create a Service Account on the same project as your Drive API key.
//  2. Create a JSON key for it and download it.
//  3. Share the catalog's root Drive folder with the service account's
//     email (…@…iam.gserviceaccount.com) as Editor — a service account
//     has no access to your files until you explicitly share with it,
//     same as sharing with any other Google account.
//
// USAGE:
//
//	go run ./cmd/fixaudio -credentials=/path/to/service-account.json -folder=<DRIVE_FOLDER_ID>
//
// Add -dry-run to see what would be fixed without changing anything.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const videoMimePrefix = "video/"

func main() {
	credentials := flag.String("credentials", "", "path to the service account JSON key file (required)")
	folderID := flag.String("folder", os.Getenv("GOOGLE_DRIVE_FOLDER_ID"), "root Drive folder ID (defaults to $GOOGLE_DRIVE_FOLDER_ID)")
	dryRun := flag.Bool("dry-run", false, "list what would be fixed without downloading/uploading anything")
	forceRemux := flag.Bool("force-remux", false, "rewrite compatible MP4 files as streaming-friendly fragmented MP4")
	flag.Parse()

	if *credentials == "" {
		log.Fatal("missing -credentials (path to the service account JSON key file)")
	}
	if *folderID == "" {
		log.Fatal("missing -folder (root Drive folder ID) and $GOOGLE_DRIVE_FOLDER_ID is not set")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Fatal("ffmpeg not found on PATH — install with 'brew install ffmpeg'")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		log.Fatal("ffprobe not found on PATH — install with 'brew install ffmpeg'")
	}

	ctx := context.Background()
	srv, err := drive.NewService(ctx, option.WithCredentialsFile(*credentials))
	if err != nil {
		log.Fatalf("drive.NewService: %v", err)
	}

	videos, err := collectVideos(srv, *folderID)
	if err != nil {
		log.Fatalf("listing catalog: %v", err)
	}
	log.Printf("found %d video(s) across the root folder and its subfolders", len(videos))

	tmpDir, err := os.MkdirTemp("", "fixaudio-*")
	if err != nil {
		log.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	var fixed, skipped, failed int
	for i, v := range videos {
		log.Printf("[%d/%d] %s (%s)", i+1, len(videos), v.Name, v.Id)

		if err := processOne(ctx, srv, v, tmpDir, *dryRun, *forceRemux); err != nil {
			if err == errSkip {
				skipped++
				continue
			}
			log.Printf("  FAIL: %v", err)
			failed++
			continue
		}
		fixed++
	}

	log.Printf("done. fixed=%d skipped=%d failed=%d", fixed, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

var errSkip = fmt.Errorf("skip")

// collectVideos mirrors internal/drive/catalog.go's BuildCatalog: root
// folder's own videos, plus one level of subfolders' videos. Kept
// separate (not imported from internal/drive) since that package is built
// around the read-only API-key client, not an OAuth-authenticated
// *drive.Service — different auth model, not worth forcing into one
// interface for a tool that runs once in a while.
func collectVideos(srv *drive.Service, rootFolderID string) ([]*drive.File, error) {
	children, err := listChildren(srv, rootFolderID)
	if err != nil {
		return nil, err
	}

	var videos []*drive.File
	var folders []*drive.File
	for _, f := range children {
		switch {
		case f.MimeType == "application/vnd.google-apps.folder":
			folders = append(folders, f)
		case strings.HasPrefix(f.MimeType, videoMimePrefix):
			videos = append(videos, f)
		}
	}

	for _, folder := range folders {
		items, err := listChildren(srv, folder.Id)
		if err != nil {
			log.Printf("  skipping subfolder %q: %v", folder.Name, err)
			continue
		}
		for _, it := range items {
			if strings.HasPrefix(it.MimeType, videoMimePrefix) {
				videos = append(videos, it)
			}
		}
	}
	return videos, nil
}

func listChildren(srv *drive.Service, folderID string) ([]*drive.File, error) {
	var out []*drive.File
	call := srv.Files.List().
		Q(fmt.Sprintf("'%s' in parents and trashed = false", folderID)).
		Fields("nextPageToken, files(id, name, mimeType)").
		PageSize(1000)

	return out, call.Pages(context.Background(), func(page *drive.FileList) error {
		out = append(out, page.Files...)
		return nil
	})
}

// processOne downloads one file, checks its audio codec, remuxes it if
// needed, and uploads the result back onto the same file ID. Returns
// errSkip (not a real error) when the file is already fine.
func processOne(ctx context.Context, srv *drive.Service, f *drive.File, tmpDir string, dryRun, forceRemux bool) error {
	inPath := filepath.Join(tmpDir, f.Id+"-in"+filepath.Ext(f.Name))

	if err := downloadFile(ctx, srv, f.Id, inPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer os.Remove(inPath)

	codecs, err := probeCodecs(inPath)
	if err != nil {
		return fmt.Errorf("ffprobe: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(f.Name))
	videoCompatible := codecs.Video == "h264" && (codecs.PixelFormat == "yuv420p" || codecs.PixelFormat == "yuvj420p")
	audioCompatible := codecs.Audio == "aac" || codecs.Audio == "mp3"
	containerCompatible := ext == ".mp4" || ext == ".m4v"
	if videoCompatible && audioCompatible && containerCompatible && !forceRemux {
		log.Printf("  skip: already video=%s profile=%s pix_fmt=%s level=%d audio=%s in %s", codecs.Video, codecs.VideoProfile, codecs.PixelFormat, codecs.VideoLevel, codecs.Audio, ext)
		return errSkip
	}

	log.Printf("  video=%s profile=%s pix_fmt=%s level=%d audio=%s container=%s -> writing streaming-friendly h264/yuv420p/aac/mp4", codecs.Video, codecs.VideoProfile, codecs.PixelFormat, codecs.VideoLevel, codecs.Audio, ext)
	if dryRun {
		log.Printf("  (dry-run, not uploading)")
		return errSkip
	}

	outPath := filepath.Join(tmpDir, f.Id+"-out.mp4")
	if err := convertForBrowsers(inPath, outPath, videoCompatible, audioCompatible); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	defer os.Remove(outPath)

	newName := strings.TrimSuffix(f.Name, filepath.Ext(f.Name)) + ".mp4"
	if err := uploadInPlace(ctx, srv, f.Id, newName, outPath); err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	log.Printf("  ok: uploaded as %s (same file ID, catalog/progress links stay valid)", newName)
	return nil
}

func downloadFile(ctx context.Context, srv *drive.Service, fileID, destPath string) error {
	resp, err := srv.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.ReadFrom(resp.Body)
	return err
}

type mediaCodecs struct {
	Video        string
	Audio        string
	VideoProfile string
	PixelFormat  string
	VideoLevel   int
}

func probeCodecs(path string) (mediaCodecs, error) {
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_type,codec_name,profile,pix_fmt,level", "-of", "json", path).Output()
	if err != nil {
		return mediaCodecs{}, err
	}
	var probe struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			Profile     string `json:"profile"`
			PixelFormat string `json:"pix_fmt"`
			Level       int    `json:"level"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return mediaCodecs{}, err
	}
	var codecs mediaCodecs
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			if codecs.Video == "" {
				codecs.Video = stream.CodecName
				codecs.VideoProfile = stream.Profile
				codecs.PixelFormat = stream.PixelFormat
				codecs.VideoLevel = stream.Level
			}
		case "audio":
			if codecs.Audio == "" {
				codecs.Audio = stream.CodecName
			}
		}
	}
	if codecs.Video == "" || codecs.Audio == "" {
		return mediaCodecs{}, fmt.Errorf("missing video or audio stream (video=%q audio=%q)", codecs.Video, codecs.Audio)
	}
	return codecs, nil
}

func convertForBrowsers(inPath, outPath string, copyVideo, copyAudio bool) error {
	args := []string{"-y", "-v", "error", "-i", inPath, "-map", "0:v:0", "-map", "0:a:0"}
	if copyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-pix_fmt", "yuv420p")
	}
	if copyAudio {
		args = append(args, "-c:a", "copy")
	} else {
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}
	// Fragmented MP4 keeps the initialization metadata tiny. Large
	// feature-length files otherwise build multi-megabyte moov atoms that
	// mobile Safari may abandon before metadata/duration becomes available.
	args = append(args, "-movflags", "+frag_keyframe+empty_moov+default_base_moof", outPath)
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func uploadInPlace(ctx context.Context, srv *drive.Service, fileID, newName, contentPath string) error {
	f, err := os.Open(contentPath)
	if err != nil {
		return err
	}
	defer f.Close()

	update := &drive.File{Name: newName, MimeType: "video/mp4"}
	_, err = srv.Files.Update(fileID, update).Media(f).Context(ctx).Do()
	return err
}
