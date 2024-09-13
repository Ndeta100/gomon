package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ndeta100/gomon/config"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// ModifiedStatus holds information about a file's change status
type ModifiedStatus struct {
	Path   string `yaml:"path"`
	Status string `yaml:"status"` // "Created", "Modified", or "Deleted"
}

var fileHashes = map[string]string{}

func WatchHandler([]string) {
	cwd, _ := os.Getwd()
	configFilePath := filepath.Join(cwd, config.CONFIG_FILE_NAME)

	info, err := os.Stat(configFilePath)
	if os.IsNotExist(err) || info.IsDir() {
		config.CreateDefaultConfig(config.CONFIG_FILE_NAME)
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		fmt.Printf("Could not read the config file: %s\n", configFilePath)
		return
	}

	var configSettings config.Config
	err = yaml.Unmarshal(data, &configSettings)
	if err != nil {
		fmt.Println("Could not parse the config file:", err)
		return
	}

	configSettings.Delay = 500
	if len(configSettings.IncludePaths) == 0 {
		configSettings.IncludePaths = append(configSettings.IncludePaths, ".")
	}
	if len(configSettings.WatchFileTypes) == 0 {
		configSettings.WatchFileTypes = append(configSettings.WatchFileTypes, "*")
	}

	var wg sync.WaitGroup
	for _, watchPath := range configSettings.IncludePaths {
		if len(watchPath) == 0 {
			continue
		}
		fullPath := filepath.Join(cwd, watchPath)
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			fmt.Println("Starting watcher for:", path)
			WatchFilePath(path, configSettings)
		}(fullPath)
	}
	wg.Wait()
}

func WatchFilePath(watchPath string, cfg config.Config) {
	for {
		var modified []ModifiedStatus
		pathsChecked := []string{watchPath}
		for _, path := range ListDirContents(watchPath, cfg.WatchFileTypes, cfg.ExcludePaths) {
			fileHash := calculateFileHash(path)
			_, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					modified = append(modified, ModifiedStatus{Path: path, Status: "Deleted"})
					delete(fileHashes, path)
					continue
				}
				fmt.Println("Error reading file:", path, err)
				continue
			}

			previousHash, exists := fileHashes[path]
			if !exists {
				modified = append(modified, ModifiedStatus{Path: path, Status: "Created"})
			} else if previousHash != fileHash {
				modified = append(modified, ModifiedStatus{Path: path, Status: "Edited"})
			}
			fileHashes[path] = fileHash
			pathsChecked = append(pathsChecked, path)
		}

		for path := range fileHashes {
			if !slices.Contains(pathsChecked, path) {
				modified = append(modified, ModifiedStatus{Path: path, Status: "Deleted"})
				delete(fileHashes, path)
			}
		}

		if len(modified) > 0 {
			modifiedJson, _ := json.MarshalIndent(modified, "", " ")
			fmt.Println("Modified Files:", string(modifiedJson))
		}

		time.Sleep(time.Millisecond * time.Duration(cfg.Delay))
	}
}

func ListDirContents(path string, allowedFileTypes []string, ignorePaths []string) []string {
	allowAllTypes := slices.Contains(allowedFileTypes, "*")
	var contents []string

	files, err := os.ReadDir(path)
	if err != nil {
		fmt.Println("Error reading directory:", path, err)
		return []string{}
	}

	for _, file := range files {
		fullPath := filepath.Join(path, file.Name())
		if slices.Contains(ignorePaths, fullPath) {
			continue
		}

		if file.IsDir() {
			contents = append(contents, ListDirContents(fullPath, allowedFileTypes, ignorePaths)...)
		} else {
			if allowAllTypes {
				contents = append(contents, fullPath)
			} else {
				for _, extension := range allowedFileTypes {
					if filepath.Ext(fullPath) == extension {
						contents = append(contents, fullPath)
					}
				}
			}
		}
	}
	return contents
}

func calculateFileHash(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func InitHandler(args []string) {
	cwd, _ := os.Getwd()
	useForce := slices.Contains(args, "-force")
	configFilePath := filepath.Join(cwd, config.CONFIG_FILE_NAME)
	info, err := os.Stat(configFilePath)

	exists := err == nil

	if !exists || useForce {
		config.CreateDefaultConfig(configFilePath)
		return
	}

	if info.IsDir() {
		fmt.Println("The config path should not be a directory.")
		return
	}

	fmt.Println("The config file already exists. Use the -force flag to override the file.")
}
