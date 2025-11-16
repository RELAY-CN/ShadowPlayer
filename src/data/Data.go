package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var GlobalConfig, _ = fetchConfig()

type Config struct {
	Port int32 `json:"port"`
}

func fetchConfig() (Config, error) {
	config := Config{
		Port: 5123,
	}

	exePath, err := os.Executable()
	if err != nil {
		return Config{}, fmt.Errorf("无法获得可执行路径: %v", err)
	}

	exeDir := filepath.Dir(exePath)
	jsonFilePath := filepath.Join(exeDir, "config.json")

	if fileExists(jsonFilePath) {
		fileData, err := os.ReadFile(jsonFilePath)
		if err != nil {
			return Config{}, fmt.Errorf("无法读取JSON文件: %v", err)
		}

		if err := json.Unmarshal(fileData, &config); err != nil {
			return Config{}, fmt.Errorf("未能解析JSON: %v", err)
		}

		fmt.Println("加载现有配置:")
	} else {
		fmt.Println("生成的新配置:")
		jsonData, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return Config{}, fmt.Errorf("未能序列化JSON: %v", err)
		}
		if err := os.WriteFile(jsonFilePath, jsonData, 0644); err != nil {
			return Config{}, fmt.Errorf("无法写入JSON文件: %v", err)
		}
	}

	fmt.Printf("Port: %d\n", config.Port)
	fmt.Printf("保存路径: %s\n", jsonFilePath)

	return config, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
