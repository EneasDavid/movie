package handlers

import "testing"

func TestBrowserVideoMime(t *testing.T) {
	tests := []struct {
		name, driveMime, want string
	}{
		{"filme.mov", "application/octet-stream", "video/quicktime"},
		{"filme.MOV", "video/mp4", "video/quicktime"},
		{"filme.mp4", "application/octet-stream", "video/mp4"},
		{"filme.webm", "application/octet-stream", "video/webm"},
		{"sem-extensao", "video/custom", "video/custom"},
		{"arquivo.bin", "application/octet-stream", ""},
	}
	for _, test := range tests {
		if got := browserVideoMime(test.name, test.driveMime); got != test.want {
			t.Errorf("browserVideoMime(%q, %q) = %q, want %q", test.name, test.driveMime, got, test.want)
		}
	}
}
