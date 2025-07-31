package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
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
func (a *App) Run(models ...string) {
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

	// Determine models to export
	var modelsToExport []string
	if len(models) > 0 {
		modelsToExport = models
		fmt.Printf("Exporting specified models: %s\n", strings.Join(models, " "))
	} else if *modelName != "" { // Si se especifica un modelo con la bandera -m
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

	// Iterate over models to export
	for _, modelFull := range modelsToExport {
		// Compress the export
		fmt.Printf("Compressing model: %s\n", modelFull)
		safeModelName := strings.ReplaceAll(modelFull, ":", "-")
		outputFileName := fmt.Sprintf("ollama-export-%s.tar.gz", safeModelName)
		outputFilePath := filepath.Join(a.OutputDir, outputFileName)

		modelNameParts := strings.Split(modelFull, ":")
		modelBaseName := modelNameParts[0]
		modelTag := "latest"
		if len(modelNameParts) > 1 {
			modelTag = modelNameParts[1]
		}

		manifestPath := filepath.Join(a.OllamaBaseDir, "manifests/registry.ollama.ai/library", modelBaseName, modelTag)

		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			fmt.Printf("WARNING: Manifest for model '%s' not found, skipping.\n", modelFull)
			continue
		}

		manifestFile, err := os.ReadFile(manifestPath)
		if err != nil {
			errorExit(fmt.Sprintf("Failed to read manifest file: %v", err))
		}

		var manifestData map[string]interface{}
		if err := json.Unmarshal(manifestFile, &manifestData); err != nil {
			errorExit(fmt.Sprintf("Failed to unmarshal manifest JSON: %v", err))
		}

		var blobHashes []string
		layers := manifestData["layers"].([]interface{})
		for _, layer := range layers {
			layerMap := layer.(map[string]interface{})
			digest := layerMap["digest"].(string)
			blobHashes = append(blobHashes, strings.Split(digest, ":")[1])
		}

		var filesToCompress []string
		for _, hash := range blobHashes {
			filesToCompress = append(filesToCompress, filepath.Join(a.OllamaBaseDir, "blobs", "sha256-"+hash))
		}
		filesToCompress = append(filesToCompress, manifestPath)

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Suffix = " Compressing..."
		s.Start()

		if err := createTarGz(outputFilePath, filesToCompress, a.OllamaBaseDir); err != nil {
			s.Stop()
			errorExit(fmt.Sprintf("Failed to create tar.gz archive: %v", err))
		}

		s.Stop()

		fmt.Println("===================================================")
		fmt.Printf("Export completed: %s\n", outputFilePath)
		fmt.Println("To import on the destination system:")
		fmt.Println("1. Decompress with: tar -xzvf ollama-export.tar.gz -C /destination/path")
		fmt.Println("2. Copy the files to the Docker container: docker cp /destination/path/. [ollama-container]:/root/.ollama/")
		fmt.Println("3. Register the models in the container(inside the container): echo \"FROM nombremodelo:tag\" > Modelfile")
		fmt.Println("4. ollama create --model [nombremodelo:tag] --file Modelfile")
		fmt.Println("===================================================")
		fmt.Println("Export finished.")
	}
}

func createTarGz(buf string, files []string, baseDir string) error {
	// Create output file
	outFile, err := os.Create(buf)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Create new gzip writer
	gz := gzip.NewWriter(outFile)
	defer gz.Close()

	// Create new tar writer
	tw := tar.NewWriter(gz)
	defer tw.Close()

	// Add files to tar archive
	for _, file := range files {
		if err := addFileToTar(tw, file, baseDir); err != nil {
			return err
		}
	}

	return nil
}

func addFileToTar(tw *tar.Writer, path string, baseDir string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create header
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	// Update header name to be relative to baseDir
	header.Name, err = filepath.Rel(baseDir, path)
	if err != nil {
		return err
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tw, file); err != nil {
		return err
	}

	return nil
}