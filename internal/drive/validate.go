package drive

import "regexp"

// driveIDPattern matches real Google Drive file/folder IDs: URL-safe
// base64-ish alphanumerics plus '-' and '_'. Validating input against this
// before it touches a Redis key, a Drive API URL, or an error message
// closes off cache-key injection and keeps us from ever forwarding
// attacker-controlled path segments upstream.
var driveIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{10,100}$`)

// IsValidFileID reports whether s looks like a real Drive file/folder ID.
func IsValidFileID(s string) bool {
	return driveIDPattern.MatchString(s)
}
