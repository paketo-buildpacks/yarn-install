package yarninstall

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type ProjectPathParser struct{}

func NewProjectPathParser() ProjectPathParser {
	return ProjectPathParser{}
}

func (p ProjectPathParser) Get(path string) (string, error) {
	customProjPath := os.Getenv("BP_NODE_PROJECT_PATH")
	if customProjPath == "" {
		return path, nil
	}

	customProjPath = filepath.Clean(customProjPath)
	path = filepath.Join(path, customProjPath)
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("expected value derived from BP_NODE_PROJECT_PATH [%s] to be an existing directory", path)
		} else {
			return "", err
		}
	}
	return path, nil
}
