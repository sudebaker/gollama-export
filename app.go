// app.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// App struct to hold application state
type App struct {
	OllamaBaseDir string
	OutputDir     string
	Debug         bool
}

// NewApp creates a new App instance
func NewApp(ollamaBaseDir, outputDir string, debug bool) *App {
	return &App{
		OllamaBaseDir: ollamaBaseDir,
		OutputDir:     outputDir,
		Debug:         debug,
	}
}

// Run executes the main application logic
func (a *App) Run() {
	// Check if the required directories exist
	if _, err := os.Stat(filepath.Join(a.OllamaBaseDir, "manifests")); os.IsNotExist(err) {
		errorExit(fmt.Sprintf("Directory %s/manifests not found", a.OllamaBaseDir))
	}
	if _, err := os.Stat(filepath.Join(a.OllamaBaseDir, "blobs")); os.IsNotExist(err) {
		errorExit(fmt.Sprintf("Directory %s/blobs not found", a.OllamaBaseDir))
	}
	debugPrint("Source directories verified correctly", a.Debug)
	debugPrint(fmt.Sprintf("OLLAMA_BASE_DIR: %s", a.OllamaBaseDir), a.Debug)
	debugPrint(fmt.Sprintf("OUTPUT_DIR: %s", a.OutputDir), a.Debug)

	// Create destination directories
	if err := os.MkdirAll(filepath.Join(a.OutputDir, "models/manifests/registry.ollama.ai/library"), 0755); err != nil {
		errorExit(fmt.Sprintf("Failed to create destination directory: %v", err))
	}
	if err := os.MkdirAll(filepath.Join(a.OutputDir, "models/blobs"), 0755); err != nil {
		errorExit(fmt.Sprintf("Failed to create destination directory: %v", err))
	}
	debugPrint("Destination directories created correctly", a.Debug)

	// Determine models to export
	var modelsToExport []string
	if *modelName != "" { // Si se especifica un modelo con la bandera -m
		modelsToExport = append(modelsToExport, *modelName)
		fmt.Printf("Exporting specified model: %s\n", *modelName)
	} else {
		fmt.Println("Exporting all available models in ollama:")
		var err error
		modelsToExport, err = getOllamaModelsWithTags()
		if err != nil {
			errorExit(err.Error())
		}
		fmt.Println(strings.Join(modelsToExport, " "))
	}

	// Process each model
	for _, modelFull := range modelsToExport {
		a.processModel(modelFull)
	}

	// Check if anything was exported
	if _, err := os.ReadDir(filepath.Join(a.OutputDir, "models/blobs/")); err == nil {
		if len(getFilesInDirectory(filepath.Join(a.OutputDir, "models/blobs/"))) == 0 {
			fmt.Println("WARNING: No blobs were exported.")
			fmt.Println("Verifying original blob file structure:")
			cmd := exec.Command("find", filepath.Join(a.OllamaBaseDir, "blobs"), "-type", "f", "|", "head", "-n", "5")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		}
	}

	// Compress the export
	fmt.Println("Compressing export...")
	outputFileName := "ollama-export.tar.gz"
	if *modelName != "" {
		outputFileName = fmt.Sprintf("ollama-export-%s.tar.gz", *modelName)
	}

	// Asegúrate de que el archivo tar.gz se cree dentro del directorio de salida
	outputFilePath := filepath.Join(a.OutputDir, outputFileName)

	err := createTarGz(a.OutputDir, outputFilePath)
	if err != nil {
		errorExit(fmt.Sprintf("Failed to compress export: %v", err))
	}

	fmt.Println("===================================================")
	fmt.Printf("Export completed: %s\n", outputFilePath)
	fmt.Println("To import on the destination system:")
	fmt.Println("1. Decompress with: tar -xzvf ollama-export.tar.gz -C /destination/path")
	fmt.Println("2. Copy the files to the Docker container: docker cp /destination/path/. [ollama-container]:/root/.ollama/")
	fmt.Println("3. Restart the container: docker restart [ollama-container]")
	fmt.Println("===================================================")
	fmt.Println("Export finished.")
}
