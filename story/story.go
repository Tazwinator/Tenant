// Package story embeds the script of Act 1.
//
// Spoilers live in act1/. The format is described in internal/script.
package story

import (
	"embed"
	"io/fs"
)

//go:embed act1
var files embed.FS

// Act1 is the free, open-source first act.
func Act1() fs.FS {
	sub, err := fs.Sub(files, "act1")
	if err != nil {
		panic(err)
	}
	return sub
}
