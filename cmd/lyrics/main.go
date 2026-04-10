package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/index-null/cmus-lyric/internal/player"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("lyrics %s (%s) built at %s\n", version, commit, date)
		return
	}

	p := tea.NewProgram(player.NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
