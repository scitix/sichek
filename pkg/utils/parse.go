package utils

import (
	"encoding/json"
	"os"
	"strconv"

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

func ParseStringToFloat(str string) float64 {
	num, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0
	}
	return num
}

func ParseBoolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func CheckSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
