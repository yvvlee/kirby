// Package safefile opens an explicit path through an os.Root anchored at its parent.
package safefile

import (
	"os"
	"path/filepath"
)

func Open(path string) (*os.File, error) {
	return OpenFile(path, os.O_RDONLY, 0)
}

func OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	directory, name := split(path)
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.OpenFile(name, flag, perm)
}

func ReadFile(path string) ([]byte, error) {
	directory, name := split(path)
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFile(name)
}

func split(path string) (string, string) {
	clean := filepath.Clean(path)
	directory, name := filepath.Split(clean)
	if directory == "" {
		directory = "."
	}
	return directory, name
}
