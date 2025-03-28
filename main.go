package main

/// An application to export and compress ollama models
/// The app gets some args from the command line (model to export, ollama directory, destination directory)
/// It then checks if the model exists in the ollama directory
/// If it does, it compresses the model and saves it to the destination directory
/// If it doesn't, it prints an error message
/// The app also checks if the destination directory exists and creates it if it doesn't
/// If no arguments are provided, the app export all models to defaults directories

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ollamaBaseDir = flag.String("o", "/var/lib/ollama", "Ollama models directory")
	outputDir     = flag.String("d", "./ollama-export", "Destination directory for exported models")
	modelToExport = flag.String("m", "", "Model to export (optional, if not specified, all models will be exported)")
)

func errorExit(msg string) {
	fmt.Println("ERROR:", msg)
	os.Exit(1)
}

func getOllamaModels() ([]string, error) {
	cmd := exec.Command("ollama", "ls")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error executing ollama ls: %v\nOutput: %s", err, output)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) <= 1 {
		return nil, fmt.Errorf("no models found with 'ollama ls'")
	}

	models := []string{}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			models = append(models, parts[0])
		}
	}
	return models, nil
}

func findModelDirectory(baseDir, modelName string) (string, error) {
	var foundDir string
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && strings.Contains(path, modelName) {
			foundDir = path
			return filepath.SkipDir // Stop searching further
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	if foundDir == "" {
		return "", fmt.Errorf("model directory not found for %s", modelName)
	}

	return foundDir, nil
}

func createTarGz(source, target string) error {
	tarfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tarfile.Close()

	gzipWriter := gzip.NewWriter(tarfile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		header.Name, err = filepath.Rel(source, path)
		if err != nil {
			return err
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func main() {
	flag.Parse()

	// Check if the output directory exists and create it if it doesn't
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		errorExit(fmt.Sprintf("Failed to create output directory: %v", err))
	}

	// Export a specific model
	if *modelToExport != "" {
		modelDir, err := findModelDirectory(*ollamaBaseDir, *modelToExport)
		if err != nil {
			errorExit(fmt.Sprintf("Error finding model directory: %v", err))
		}

		destFile := filepath.Join(*outputDir, fmt.Sprintf("%s.tar.gz", *modelToExport))

		if err := createTarGz(modelDir, destFile); err != nil {
			errorExit(fmt.Sprintf("Failed to create tar.gz file: %v", err))
		}
		fmt.Printf("Model %s exported successfully to %s\n", *modelToExport, destFile)
		return
	}

	// Export all models
	models, err := getOllamaModels()
	if err != nil {
		errorExit(err.Error())
	}

	for _, model := range models {
		modelDir, err := findModelDirectory(*ollamaBaseDir, model)
		if err != nil {
			errorExit(fmt.Sprintf("Error finding model directory: %v", err))
		}

		destFile := filepath.Join(*outputDir, fmt.Sprintf("%s.tar.gz", model))

		if err := createTarGz(modelDir, destFile); err != nil {
			errorExit(fmt.Sprintf("Failed to create tar.gz file: %v", err))
		}
	}
	fmt.Println("All models exported successfully.")
}
