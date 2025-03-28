package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
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
	logrus.WithField("file", file).Info("LoadFromYaml")
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

func GetDefaultConfigDirPath(component string) (string, error) {
	defaultCfgPath := filepath.Join(consts.DefaultPodCfgPath, component)
	_, err := os.Stat(defaultCfgPath)
	if err != nil {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return "", fmt.Errorf("get curr file path failed")
		}
		// 获取当前文件的目录

		defaultCfgPath = filepath.Join(filepath.Dir(curFile), component)
	}
	return defaultCfgPath, nil
}
