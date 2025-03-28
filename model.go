// model.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// processModel handles the export of a single model
func (a *App) processModel(modelFull string) {
	modelNameProvided := strings.Split(modelFull, ":")[0]
	modelTag := ""
	if strings.Contains(modelFull, ":") {
		modelTag = strings.Split(modelFull, ":")[1]
	}

	// Find the model directory
	var modelDirCandidates []string
	err := filepath.Walk(filepath.Join(a.OllamaBaseDir, "manifests/registry.ollama.ai/library/"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && strings.Contains(path, modelNameProvided) {
			modelDirCandidates = append(modelDirCandidates, path)
		}
		return nil
	})
	if err != nil {
		errorExit(fmt.Sprintf("Error finding model directory: %v", err))
	}

	if len(modelDirCandidates) == 0 {
		fmt.Printf("ERROR: No model found matching '%s' in %s/manifests/registry.ollama.ai/library/\n", modelNameProvided, a.OllamaBaseDir)
		fmt.Println("Available models:")
		cmd := exec.Command("ls", "-la", filepath.Join(a.OllamaBaseDir, "manifests/registry.ollama.ai/library/"))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		return
	}

	sort.Strings(modelDirCandidates)
	modelDir := modelDirCandidates[0]
	modelName := strings.ReplaceAll(strings.TrimPrefix(modelDir, filepath.Join(a.OllamaBaseDir, "manifests/registry.ollama.ai/library/")), "/", ":")

	debugPrint(fmt.Sprintf("Model directory found: %s", modelDir), a.Debug)
	debugPrint(fmt.Sprintf("Model name found: %s", modelName), a.Debug)

	// Get the latest tag if no tag is provided
	if modelTag == "" {
		debugPrint(fmt.Sprintf("No tag provided for %s, trying to get the latest tag", modelName), a.Debug)
		files, err := os.ReadDir(modelDir)
		if err != nil {
			errorExit(fmt.Sprintf("Error reading model directory: %v", err))
		}
		if len(files) > 0 {
			modelTag = files[len(files)-1].Name()
			debugPrint(fmt.Sprintf("Latest tag found: %s", modelTag), a.Debug)
		} else {
			fmt.Printf("ERROR: No tag found for model %s\n", modelName)
			return
		}
	}

	// Check if the model directory exists
	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		fmt.Printf("ERROR: Model %s does not exist in %s\n", modelName, modelDir)
		return
	}

	// Check if the manifest file exists for the specific tag
	if _, err := os.Stat(filepath.Join(modelDir, modelTag)); os.IsNotExist(err) {
		fmt.Printf("ERROR: Tag %s for model %s does not exist\n", modelTag, modelName)
		fmt.Println("Available tags:")
		cmd := exec.Command("ls", "-la", modelDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		return
	}

	// Create destination directory for the model
	modelNameDest := strings.ReplaceAll(modelName, ":", "/")
	if err := os.MkdirAll(filepath.Join(a.OutputDir, "models/manifests/registry.ollama.ai/library/", modelNameDest), 0755); err != nil {
		errorExit(fmt.Sprintf("Failed to create destination directory: %v", err))
	}

	// Copy the model-specific manifest
	if err := copyFile(filepath.Join(modelDir, modelTag), filepath.Join(a.OutputDir, "models/manifests/registry.ollama.ai/library/", modelNameDest, modelTag)); err != nil {
		fmt.Println("ERROR: Failed to copy manifest.")
		return
	}

	// Extract all SHA256 hashes from the manifest
	manifestContent, err := os.ReadFile(filepath.Join(modelDir, modelTag))
	if err != nil {
		fmt.Println("ERROR: Failed to read manifest file.")
		return
	}

	re := regexp.MustCompile(`sha256:[a-f0-9]+`)
	blobData := re.FindAllString(string(manifestContent), -1)

	if len(blobData) == 0 {
		fmt.Println("ERROR: No blob references found in the manifest.")
		return
	}

	var blobs []string
	for _, blob := range blobData {
		blobs = append(blobs, strings.TrimPrefix(blob, "sha256:"))
	}
	sort.Strings(blobs)
	blobs = unique(blobs)

	// Copy required blobs
	copiedCount := 0
	failedCount := 0

	for _, blob := range blobs {
		blobPath := filepath.Join(a.OllamaBaseDir, "blobs/sha256-"+blob)
		destPath := filepath.Join(a.OutputDir, "models/blobs/sha256-"+blob)

		debugPrint(fmt.Sprintf("Verifying blob: %s", blobPath), a.Debug)

		if _, err := os.Stat(blobPath); err == nil {
			debugPrint("Blob found, copying...", a.Debug)
			if err := copyFile(blobPath, destPath); err == nil {
				copiedCount++
			} else {
				failedCount++
			}
		} else {
			failedCount++
		}
	}
	fmt.Printf("  Copied %d blobs, failed to copy %d blobs for %s:%s\n", copiedCount, failedCount, modelName, modelTag)
	if failedCount > 0 {
		fmt.Println("  WARNING: Some blobs were not copied successfully.")
	}
	if copiedCount > 0 {
		fmt.Printf("  Model %s:%s exported successfully\n", modelName, modelTag)
	} else {
		fmt.Printf("  ERROR: No blobs could be exported for %s:%s\n", modelName, modelTag)
	}
	fmt.Println("")
}
