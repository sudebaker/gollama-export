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
	// "encoding/json"
	"flag"
	"fmt"
	"io"
	// "net/http"
	// "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ollamaBaseDir = flag.String("ollama-dir", "/var/lib/ollama", "Directorio base de ollama")
	outputDir     = flag.String("output-dir", "./ollama-export", "Directorio de salida para la exportación")
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

// // Respuesta de la API para listar modelos
// type OllamaTagsResponse struct {
// 	Models []struct {
// 		Name       string    `json:"name"`
// 		ModifiedAt string    `json:"modified_at"`
// 		Size       int64     `json:"size"`
// 		Digest     string    `json:"digest"`
// 		Details    struct{} `json:"details"`
// 	} `json:"models"`
// }

// func getOllamaModelsFromAPI(url *url.URL) ([]string, error) {
// 	resp, err := http.Get("http://localhost:11434/api/tags")
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		return nil, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
// 	}

// 	var response OllamaTagsResponse
// 	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
// 		return nil, err
// 	}

// 	var models []string
// 	for _, model := range response.Models {
// 		models = append(models, model.Name)
// 	}

// 	return models, nil
// }

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
	// if no arguments are provided, export all models to defaults directories
	if flag.NArg() == 0 {
		models, err := getOllamaModels()
		if err != nil {
			errorExit(err.Error())
		}

		for _, model := range models {
			//modelName := strings.Split(model, ":")[0] // No longer needed, model name is already clean
			modelName := model
			modelDir := filepath.Join(*ollamaBaseDir, modelName)
			destFile := filepath.Join(*outputDir, fmt.Sprintf("%s.tar.gz", modelName))

			if _, err := os.Stat(modelDir); os.IsNotExist(err) {
				errorExit(fmt.Sprintf("Model %s does not exist in %s", modelName, *ollamaBaseDir))
			}

			if err := os.MkdirAll(*outputDir, 0755); err != nil {
				errorExit(fmt.Sprintf("Failed to create output directory: %v", err))
			}

			if err := createTarGz(modelDir, destFile); err != nil {
				errorExit(fmt.Sprintf("Failed to create tar.gz file: %v", err))
			}
		}
		fmt.Println("All models exported successfully.")
		return
	}
	// if arguments are provided, export the specified model
	modelName := flag.Arg(0)
	modelDir := filepath.Join(*ollamaBaseDir, modelName)
	destFile := filepath.Join(*outputDir, fmt.Sprintf("%s.tar.gz", modelName))
	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		errorExit(fmt.Sprintf("Model %s does not exist in %s", modelName, *ollamaBaseDir))
	}
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		errorExit(fmt.Sprintf("Failed to create output directory: %v", err))
	}
	if err := createTarGz(modelDir, destFile); err != nil {
		errorExit(fmt.Sprintf("Failed to create tar.gz file: %v", err))
	}
	fmt.Printf("Model %s exported successfully to %s\n", modelName, destFile)
}
