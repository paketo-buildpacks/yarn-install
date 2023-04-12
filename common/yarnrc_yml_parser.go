package common

import (
	"os"

	"gopkg.in/yaml.v2"
)

type YarnrcYmlParser struct{}

func NewYarnrcYmlParser() YarnrcYmlParser {
	return YarnrcYmlParser{}
}

func (y YarnrcYmlParser) Parse(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var rc struct {
		NodeLinker string `yaml:"nodeLinker"`
	}

	err = yaml.NewDecoder(file).Decode(&rc)
	if err != nil {
		return "", err
	}

	return rc.NodeLinker, nil
}
