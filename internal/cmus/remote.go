package cmus

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Track struct {
	Position int
	File     string
	Duration int
	Artist   string
	Title    string
	Album    string
	Status   string
}

func Remote() Track {
	output, err := querySocket()
	if err != nil {
		return remoteExec()
	}
	return ParseStatus(output)
}

func querySocket() (string, error) {
	path := socketPath()
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		return "", err
	}

	if _, err := conn.Write([]byte("status\n")); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return buf.String(), nil
}

func socketPath() string {
	if p := os.Getenv("CMUS_SOCKET"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		p := filepath.Join(xdg, "cmus-socket")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cmus", "socket")
}

func remoteExec() Track {
	cmd := exec.Command("cmus-remote", "-Q")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return Track{Status: "stopped"}
	}
	return ParseStatus(out.String())
}

func ParseStatus(output string) Track {
	info := strings.Split(output, "\n")
	if len(info) < 1 || len(info[0]) < 1 {
		return Track{Status: "stopped"}
	}

	parts := strings.Fields(info[0])
	if len(parts) < 2 {
		return Track{Status: "stopped"}
	}

	track := Track{Status: parts[1]}
	for _, line := range info {
		switch {
		case strings.HasPrefix(line, "file "):
			track.File = line[5:]
		case strings.HasPrefix(line, "duration "):
			track.Duration, _ = strconv.Atoi(line[9:])
		case strings.HasPrefix(line, "position "):
			track.Position, _ = strconv.Atoi(line[9:])
		case strings.HasPrefix(line, "tag artist "):
			track.Artist = line[11:]
		case strings.HasPrefix(line, "tag title "):
			track.Title = line[10:]
		case strings.HasPrefix(line, "tag album "):
			track.Album = line[10:]
		}
	}
	return track
}
