package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func main() {
	credentials := flag.String("credentials", "", "service-account JSON")
	fileID := flag.String("file", "", "Drive file ID")
	revisionID := flag.String("revision", "", "revision ID to download")
	output := flag.String("output", "", "destination path for revision download")
	flag.Parse()
	if *credentials == "" || *fileID == "" {
		log.Fatal("-credentials and -file are required")
	}
	srv, err := drive.NewService(context.Background(), option.WithCredentialsFile(*credentials))
	if err != nil {
		log.Fatal(err)
	}
	if *revisionID != "" {
		if *output == "" {
			log.Fatal("-output is required with -revision")
		}
		resp, err := srv.Revisions.Get(*fileID, *revisionID).Download()
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		out, err := os.Create(*output)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := io.Copy(out, resp.Body); err != nil {
			out.Close()
			log.Fatal(err)
		}
		if err := out.Close(); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("downloaded revision %s to %s\n", *revisionID, *output)
		return
	}
	revisions, err := srv.Revisions.List(*fileID).
		Fields("revisions(id,mimeType,modifiedTime,size,keepForever,originalFilename)").
		Do()
	if err != nil {
		log.Fatal(err)
	}
	for _, revision := range revisions.Revisions {
		fmt.Printf("id=%s modified=%s mime=%s size=%d original=%q keep=%v\n",
			revision.Id, revision.ModifiedTime, revision.MimeType, revision.Size,
			revision.OriginalFilename, revision.KeepForever)
	}
}
