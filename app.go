// app.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

	// Iterate over models to export
	for _, modelFull := range modelsToExport {
		// Compress the export
		fmt.Printf("Compressing model: %s\n", modelFull)
		outputFileName := "ollama-export.tar.gz"
		// Reemplazar caracteres inválidos en el nombre del archivo
		safeModelName := strings.ReplaceAll(modelFull, ":", "-")
		outputFileName = fmt.Sprintf("ollama-export-%s.tar.gz", safeModelName)

		// Ruta completa para el archivo tar.gz
		outputFilePath := filepath.Join(a.OutputDir, outputFileName)

		// Crear el archivo tar.gz con los archivos específicos del modelo
		var cmd *exec.Cmd
		modelNameParts := strings.Split(modelFull, ":")
		modelBaseName := modelNameParts[0]
		modelTag := "latest"
		if len(modelNameParts) > 1 {
			modelTag = modelNameParts[1]
		}

		// Construir la ruta al manifiesto del modelo
		manifestPath := filepath.Join(a.OllamaBaseDir, "manifests/registry.ollama.ai/library", modelBaseName, modelTag)

		// Leer el archivo de manifiesto
		manifestFile, err := os.ReadFile(manifestPath)
		if err != nil {
			errorExit(fmt.Sprintf("Failed to read manifest file: %v", err))
		}

		// Analizar el JSON
		var manifest map[string]interface{}
		err = json.Unmarshal(manifestFile, &manifest)
		if err != nil {
			errorExit(fmt.Sprintf("Failed to unmarshal manifest JSON: %v", err))
		}

		// Extraer los hashes SHA256 de los blobs
		var blobHashes []string
		layers := manifest["layers"].([]interface{})
		for _, layer := range layers {
			layerMap := layer.(map[string]interface{})
			digest := layerMap["digest"].(string)
			blobHashes = append(blobHashes, strings.Split(digest, ":")[1]) // Remove "sha256:" prefix
		}

		// Construir la lista de archivos a incluir
		var blobFiles []string
		for _, hash := range blobHashes {
			blobFiles = append(blobFiles, filepath.Join(a.OllamaBaseDir, "blobs", "sha256-"+hash))
		}

		// Comando tar para incluir solo los archivos de blob
		tarArgs := []string{"-czvf", outputFilePath, "-C", a.OllamaBaseDir}

		// Add blobs to tar with correct paths
		for _, blobFile := range blobFiles {
			tarArgs = append(tarArgs, blobFile)
		}

		// Add manifest to tar with correct path
		tarArgs = append(tarArgs, manifestPath)

		cmd = exec.Command("tar", tarArgs...)

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Suffix = " Compressing..."
		s.Start()
		// Prefijo de rutas dentro del archivo tar
		for i := range tarArgs {
			if strings.Contains(tarArgs[i], "blobs") {
				tarArgs[i] = strings.Replace(tarArgs[i], a.OllamaBaseDir+"/", "ollama/", 1)
			} else if strings.Contains(tarArgs[i], "manifests") {
				tarArgs[i] = strings.Replace(tarArgs[i], a.OllamaBaseDir+"/", "ollama/", 1)
			}
		}

		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			errorExit(fmt.Sprintf("Failed to compress export: %v", err))
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
