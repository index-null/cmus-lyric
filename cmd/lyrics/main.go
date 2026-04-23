package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/index-null/cmus-lyric/internal/cover"
	"github.com/index-null/cmus-lyric/internal/player"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("lyrics %s (%s) built at %s\n", version, commit, date)
			return
		case "cover":
			p := tea.NewProgram(cover.NewModel(), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				log.Print(err)
				os.Exit(1)
			}
			return
		}
	}

	p := tea.NewProgram(player.NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
