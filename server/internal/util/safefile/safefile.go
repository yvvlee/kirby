// Package safefile opens an explicit path through an os.Root anchored at its parent.
package safefile

import (
	"os"
	"path/filepath"
)

// Open anchors file access at the supplied path parent.
//
// Parameters: path is the explicit file to open.
// Returns: a file handle owned by the caller.
// Throws: a filesystem resolution or open error.
func Open(path string) (*os.File, error) {
	return OpenFile(path, os.O_RDONLY, 0)
}

// OpenFile prevents the final file component from escaping its opened parent.
//
// Parameters: path is the target; flag selects access; perm applies when creating it.
// Returns: a file handle owned by the caller.
// Throws: a filesystem resolution or open error.
func OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	directory, name := split(path)
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.OpenFile(name, flag, perm)
}

// ReadFile confines the final component to its parent while reading contents.
//
// Parameters: path is the explicit file to read.
// Returns: the file contents.
// Throws: a filesystem resolution, open, or read error.
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
