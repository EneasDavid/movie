package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv reads a .env file (if present) and applies KEY=VALUE lines to
// the process environment, without overriding anything already set —
// real env vars (as set by Vercel in production) always win. This is a
// local-dev convenience only; deliberately hand-rolled instead of pulling
// in a dependency for ~20 lines of parsing. Missing file is not an error:
// production has no .env and shouldn't need one.
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, alreadySet := os.LookupEnv(key); !alreadySet {
			_ = os.Setenv(key, value)
		}
	}
}
