// Package project locates the agentmod project that governs a directory.
// A directory is inside a project when it, or one of its ancestors, contains
// the marker file .agentmod/agentmod.toml. The nearest marker wins, so nested
// projects shadow outer ones.
package project

import (
	"errors"
	"os"
	"path/filepath"
)

// Marker path components relative to a project root.
const (
	DirName        = ".agentmod"
	ConfigFileName = "agentmod.toml"
)

// ErrNotFound is returned by Discover when no ancestor up to the filesystem
// root contains .agentmod/agentmod.toml.
var ErrNotFound = errors.New("not inside an agentmod project (no .agentmod/agentmod.toml here or in any parent directory)")

// Project describes a discovered agentmod project.
type Project struct {
	Root        string // absolute directory containing .agentmod/
	AgentmodDir string // Root/.agentmod
	ConfigPath  string // Root/.agentmod/agentmod.toml
}

// Discover walks from startDir upward to the filesystem root and returns the
// nearest enclosing project. startDir may be relative; it is made absolute
// without resolving symlinks, so activation follows the path the user is
// actually in. The marker must be a regular file: a directory named
// agentmod.toml, or a bare .agentmod/ without the config, does not activate.
// Unreadable ancestors are skipped rather than treated as errors.
func Discover(startDir string) (*Project, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	for {
		root := at(dir)
		if root != nil {
			return root, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir { // filesystem root reached
			return nil, ErrNotFound
		}
		dir = parent
	}
}

// at reports the project rooted exactly at dir, or nil if dir has no valid
// marker.
func at(dir string) *Project {
	agentmodDir := filepath.Join(dir, DirName)
	configPath := filepath.Join(agentmodDir, ConfigFileName)
	info, err := os.Stat(configPath)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return &Project{
		Root:        dir,
		AgentmodDir: agentmodDir,
		ConfigPath:  configPath,
	}
}
