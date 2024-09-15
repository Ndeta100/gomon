package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/ndeta100/gomon/config"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"time"
)

// ModifiedStatus holds information about a file's change status
type ModifiedStatus struct {
	Path   string `yaml:"path"`
	Status string `yaml:"status"` // "Created", "Modified", or "Deleted"
}

var fileHashes = map[string]string{}
var appProcess *os.Process

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
				// File is new (Created)
				modified = append(modified, ModifiedStatus{Path: path, Status: "Created"})
			} else if previousHash != fileHash {
				// File content changed (Edited)
				modified = append(modified, ModifiedStatus{Path: path, Status: "Edited"})
			}
			fileHashes[path] = fileHash
			pathsChecked = append(pathsChecked, path)
		}

		// Check if any files have been deleted
		for path := range fileHashes {
			if !slices.Contains(pathsChecked, path) {
				modified = append(modified, ModifiedStatus{Path: path, Status: "Deleted"})
				delete(fileHashes, path)
			}
		}

		// Only trigger restart if any files were "Edited"
		for _, mod := range modified {
			if mod.Status == "Edited" {
				fmt.Println("Modified Files:", mod)
				restartApp(cfg) // Trigger restart only if Edited
				break           // Exit loop after restart
			}
		}

		time.Sleep(time.Millisecond * time.Duration(cfg.Delay))
	}
}

func restartApp(cfg config.Config) {
	// Kill the existing app process if it's running
	if appProcess != nil {
		fmt.Println("Killing existing app process...")

		// Check if the process is still running using syscall.Signal(0)
		if err := appProcess.Signal(syscall.Signal(0)); err == nil {
			err = appProcess.Kill()
			if err != nil {
				fmt.Println("Error killing app:", err)
				return
			}
			appProcess.Wait() // Wait for the process to fully terminate
			appProcess = nil
		} else {
			fmt.Println("App process is already finished.")
		}
	}

	// Add a small delay to ensure the process is fully terminated
	time.Sleep(500 * time.Millisecond)
	// Remove the old binary if it exists
	binaryPath := "./bin/app"
	if _, err := os.Stat(binaryPath); err == nil {
		fmt.Println("Removing old binary...")
		err := os.Remove(binaryPath)
		if err != nil {
			fmt.Println("Error removing old binary:", err)
			return
		}
		fmt.Println("Old binary removed.")
	}
	// Run pre-commands (specified in the config)
	for _, cmd := range cfg.PreCommands {
		runCommand(cmd.Command, cmd.Args, cfg)
	}

	// Run the main build command
	for _, cmd := range cfg.Commands {
		if cmd.Command == "go" && slices.Contains(cmd.Args, "build") {
			fmt.Printf("Running build command: %s %v\n", cmd.Command, cmd.Args)
			runCommand(cmd.Command, cmd.Args, cfg)

			// After the build, check if the binary exists
			stat, err := os.Stat("./bin/app")
			if err != nil {
				fmt.Println("Error getting binary stats:", err)
				return
			}
			fmt.Printf("Binary last modified: %v\n", stat.ModTime())
		}
	}

	// Run the binary (./bin/app)
	for _, cmd := range cfg.Commands {
		if cmd.Command == "./bin/app" {
			fmt.Printf("Running main app: %s %v\n", cmd.Command, cmd.Args)
			runCommandWithTracking(cmd.Command, cmd.Args, cfg)
		}
	}

	// Run post-commands (if any)
	for _, cmd := range cfg.PostCommands {
		runCommand(cmd.Command, cmd.Args, cfg)
	}
}

func runCommandWithTracking(command string, args []string, cfg config.Config) {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting command %s: %v\n", command, err)
		return
	}

	// Store the process for future termination
	appProcess = cmd.Process
	fmt.Printf("Started process %s with PID %d\n", command, appProcess.Pid)

	go func() {
		err = cmd.Wait()
		if err != nil {
			fmt.Printf("Process %s finished with error: %v\n", command, err)
		}
	}()
}

func runCommand(command string, args []string, cfg config.Config) {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err = cmd.Start(); err != nil {
		fmt.Println("Error starting command:", err)
		return
	}
	// Store the process of the main app command (from config) for tracking and killing later
	for _, appCmd := range cfg.Commands {
		if command == appCmd.Command {
			appProcess = cmd.Process
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Command finished with error:", err)
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
