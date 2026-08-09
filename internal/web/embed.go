// Package web embeds the entire static frontend (HTML/CSS/JS) directly
// into the compiled Go binary via go:embed. This is deliberate: it means
// the server never depends on the working directory it happens to be
// launched from — identical behavior locally (`go run ./cmd/server`) and
// on Vercel — and there is exactly one artifact to deploy.
package web

import "embed"

//go:embed static
var FS embed.FS
