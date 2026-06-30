package util

import "strings"

// SplitPath splits a file path into directory and base name (without extension).
//
// Input:
//
//	"/path/to/song.mp3" → dir="/path/to", name="song", ok=true
//	"/path/to/song"     → ok=false (no extension)
//	"song.mp3"          → ok=false (no directory)
//
// Returns ("", "", false) if the path lacks "/" or ".".
func SplitPath(path string) (dir, name string, ok bool) {
	dotIdx := strings.LastIndex(path, ".")
	slashIdx := strings.LastIndex(path, "/")
	if dotIdx < 0 || slashIdx < 0 {
		return "", "", false
	}
	return path[:slashIdx], path[slashIdx+1 : dotIdx], true
}
