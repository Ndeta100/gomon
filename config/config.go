package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

const CONFIG_FILE_NAME = "config.yaml"

type Command struct {
	Args    []string `yaml:"args"`
	Command string   `yaml:"command"`
}

type Config struct {
	WatchFileTypes []string  `yaml:"watch_file_types"`
	IncludePaths   []string  `yaml:"include_paths"`
	ExcludePaths   []string  `yaml:"exclude_paths"`
	Delay          int       `yaml:"delay"`
	Commands       []Command `yaml:"commands"`
	LogLevel       string    `yaml:"log_level"`
	Debounce       bool      `yaml:"debounce"`
	NotifyOnChange bool      `yaml:"notify_on_change"`
	PreCommands    []Command `yaml:"pre_commands"`
	PostCommands   []Command `yaml:"post_commands"`
}

func CreateDefaultConfig(yamlFilename string) Config {
	defaultConfig := Config{
		WatchFileTypes: []string{"*.go", "*.html"},
		IncludePaths:   []string{"./src", "./templates"},
		ExcludePaths:   []string{"./build", "./vendors"},
		Commands: []Command{
			{Command: "go build", Args: []string{"-o", "bin/app"}},
			{Command: "./bin/app", Args: []string{}},
		},
		Delay:          500,
		LogLevel:       "info",
		Debounce:       true,
		NotifyOnChange: true,
		PreCommands:    []Command{{Command: "echo", Args: []string{"Running pre-build commands..."}}},
		PostCommands:   []Command{{Command: "echo", Args: []string{"App restarted successfully!"}}},
	}
	//Check if yaml file exist
	if _, err := os.Stat(yamlFilename); os.IsNotExist(err) {
		//if the config file does not exist, write the default yaml to the path
		data, err := yaml.Marshal(defaultConfig)
		if err != nil {
			log.Fatal(err)
		}
		err = os.WriteFile(yamlFilename, data, 0644)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Default configuration file created:", yamlFilename)
	} else {
		//data, err := os.ReadFile(yamlFilename)
		fmt.Println("Default configuration file found:", yamlFilename)
	}

	return defaultConfig
}

//func CreateDefaultConfig(filePath string) {
//	defaultConfig := `
//watch_file_types:
//  - "*.go"
//  - "*.html"
//include_paths:
//  - "./src"
//  - "./templates"
//exclude_paths:
//  - "./vendor"
//  - "./build"
//commands:
//  - command: "go build"
//    args: ["-o", "bin/app"]
//  - command: "./bin/app"
//    args: []
//delay: 1000            # Delay in ms
//log_level: "info"      # Logging level: debug, info, warn, error
//debounce: true         # Avoid rapid multiple restarts
//notify_on_change: true # Log when file changes are detected
//pre_commands:
//  - command: "echo"
//    args: ["Running pre-build commands..."]
//post_commands:
//  - command: "echo"
//    args: ["App restarted successfully!"]
//`
//	if _, err := os.Stat(filePath); os.IsNotExist(err) {
//		err := os.WriteFile(filePath, []byte(defaultConfig), 0644)
//		if err != nil {
//			return
//		}
//		fmt.Println("Default configuration file created:", filePath)
//	} else {
//		fmt.Println("Configuration file already exists:", filePath)
//	}
//}
