package drive

import "testing"

func TestIsVideoFileAcceptsMOVByMimeOrExtension(t *testing.T) {
	tests := []struct {
		file File
		want bool
	}{
		{File{Name: "filme.mov", MimeType: "video/quicktime"}, true},
		{File{Name: "filme.MOV", MimeType: "application/octet-stream"}, true},
		{File{Name: "capa.jpg", MimeType: "image/jpeg"}, false},
	}
	for _, test := range tests {
		if got := isVideoFile(test.file); got != test.want {
			t.Fatalf("isVideoFile(%q, %q) = %v, want %v", test.file.Name, test.file.MimeType, got, test.want)
		}
	}
}
