package utils

import (
	"encoding/json"
	"os"

	"sigs.k8s.io/yaml"
)

func JSON(c interface{}) (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func Yaml(c interface{}) (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func LoadFromYaml(file string, c interface{}) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		return err
	}
	return nil
}
