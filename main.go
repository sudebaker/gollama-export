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
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

var (
	ollamaBaseDir = flag.String("o", "/var/lib/ollama", "Ollama models directory")
	outputDir     = flag.String("d", "./ollama-export", "Destination directory for exported models")
	debug         = flag.Bool("debug", false, "Enable debug messages")
)

func debugPrint(msg string) {
	if *debug {
		fmt.Println("[DEBUG]", msg)
	}
}

func errorExit(msg string) {
	fmt.Println("ERROR:", msg)
	os.Exit(1)
}

func usage() {
	fmt.Println("Usage: goexport-ollama [-o|--ollama-dir <directory>] [-d|--output-dir <directory>] [model1[:tag1] model2[:tag2] ...]")
	fmt.Println("  -o, --ollama-dir <directory> : Ollama base directory (default: /var/lib/ollama)")
	fmt.Println("  -d, --output-dir <directory> : Output directory for export (default: ./ollama-export)")
	fmt.Println("  model1[:tag1] model2[:tag2] ... : List of models to export (if not specified, all are exported)")
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

func getOllamaModelsWithTags() ([]string, error) {
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
		parts := strings.Split(line, ":")
		if len(parts) >= 2 {
			tag := strings.TrimSpace(parts[1])
			tag = strings.Split(tag, " ")[0]
			models = append(models, fmt.Sprintf("%s:%s", parts[0], tag))
		}
	}
	return models, nil
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

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("error copying file: %v", err)
	}
	return nil
}

func main() {
	flag.Usage = usage
	flag.Parse()

	// Check if the required directories exist
	if _, err := os.Stat(filepath.Join(*ollamaBaseDir, "manifests")); os.IsNotExist(err) {
		errorExit(fmt.Sprintf("Directory %s/manifests not found", *ollamaBaseDir))
	}
	if _, err := os.Stat(filepath.Join(*ollamaBaseDir, "blobs")); os.IsNotExist(err) {
		errorExit(fmt.Sprintf("Directory %s/blobs not found", *ollamaBaseDir))
	}
	debugPrint("Source directories verified correctly")
	debugPrint(fmt.Sprintf("OLLAMA_BASE_DIR: %s", *ollamaBaseDir))
	debugPrint(fmt.Sprintf("OUTPUT_DIR: %s", *outputDir))

	// Create destination directories
	if err := os.MkdirAll(filepath.Join(*outputDir, "models/manifests/registry.ollama.ai/library"), 0755); err != nil {
		errorExit(fmt.Sprintf("Failed to create destination directory: %v", err))
	}
	if err := os.MkdirAll(filepath.Join(*outputDir, "models/blobs"), 0755); err != nil {
		errorExit(fmt.Sprintf("Failed to create destination directory: %v", err))
	}
	debugPrint("Destination directories created correctly")

	// Determine models to export
	var modelsToExport []string
	if flag.NArg() > 0 {
		modelsToExport = flag.Args()
		fmt.Printf("Exporting specified models: %s\n", strings.Join(modelsToExport, " "))
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
		modelNameProvided := strings.Split(modelFull, ":")[0]
		modelTag := ""
		if strings.Contains(modelFull, ":") {
			modelTag = strings.Split(modelFull, ":")[1]
		}

		// Find the model directory
		var modelDirCandidates []string
		err := filepath.Walk(filepath.Join(*ollamaBaseDir, "manifests/registry.ollama.ai/library/"), func(path string, info os.FileInfo, err error) error {
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
			fmt.Printf("ERROR: No model found matching '%s' in %s/manifests/registry.ollama.ai/library/\n", modelNameProvided, *ollamaBaseDir)
			fmt.Println("Available models:")
			cmd := exec.Command("ls", "-la", filepath.Join(*ollamaBaseDir, "manifests/registry.ollama.ai/library/"))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
			continue
		}

		sort.Strings(modelDirCandidates)
		modelDir := modelDirCandidates[0]
		modelName := strings.ReplaceAll(strings.TrimPrefix(modelDir, filepath.Join(*ollamaBaseDir, "manifests/registry.ollama.ai/library/")), "/", ":")

		debugPrint(fmt.Sprintf("Model directory found: %s", modelDir))
		debugPrint(fmt.Sprintf("Model name found: %s", modelName))

		// Get the latest tag if no tag is provided
		if modelTag == "" {
			debugPrint(fmt.Sprintf("No tag provided for %s, trying to get the latest tag", modelName))
			files, err := os.ReadDir(modelDir)
			if err != nil {
				errorExit(fmt.Sprintf("Error reading model directory: %v", err))
			}
			if len(files) > 0 {
				modelTag = files[len(files)-1].Name()
				debugPrint(fmt.Sprintf("Latest tag found: %s", modelTag))
			} else {
				fmt.Printf("ERROR: No tag found for model %s\n", modelName)
				continue
			}
		}

		fmt.Printf("Processing model: %s, tag: %s\n", modelName, modelTag)

		// Check if the model directory exists
		if _, err := os.Stat(modelDir); os.IsNotExist(err) {
			fmt.Printf("ERROR: Model %s does not exist in %s\n", modelName, modelDir)
			continue
		}

		debugPrint(fmt.Sprintf("Model directory found: %s", modelDir))

		// Check if the manifest file exists for the specific tag
		if _, err := os.Stat(filepath.Join(modelDir, modelTag)); os.IsNotExist(err) {
			fmt.Printf("ERROR: Tag %s for model %s does not exist\n", modelTag, modelName)
			fmt.Println("Available tags:")
			cmd := exec.Command("ls", "-la", modelDir)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
			continue
		}

		debugPrint(fmt.Sprintf("Manifest found: %s", filepath.Join(modelDir, modelTag)))

		// Create destination directory for the model
		modelNameDest := strings.ReplaceAll(modelName, ":", "/")
		if err := os.MkdirAll(filepath.Join(*outputDir, "models/manifests/registry.ollama.ai/library/", modelNameDest), 0755); err != nil {
			errorExit(fmt.Sprintf("Failed to create destination directory: %v", err))
		}

		// Copy the model-specific manifest
		fmt.Printf("  Copying manifest for %s:%s\n", modelName, modelTag)
		if err := copyFile(filepath.Join(modelDir, modelTag), filepath.Join(*outputDir, "models/manifests/registry.ollama.ai/library/", modelNameDest, modelTag)); err != nil {
			fmt.Println("ERROR: Failed to copy manifest.")
			continue
		}

		// Extract all SHA256 hashes from the manifest
		fmt.Printf("  Identifying required blobs for %s:%s\n", modelName, modelTag)
		manifestContent, err := os.ReadFile(filepath.Join(modelDir, modelTag))
		if err != nil {
			fmt.Println("ERROR: Failed to read manifest file.")
			continue
		}

		re := regexp.MustCompile(`sha256:[a-f0-9]+`)
		blobData := re.FindAllString(string(manifestContent), -1)

		if len(blobData) == 0 {
			fmt.Println("ERROR: No blob references found in the manifest.")
			continue
		}

		var blobs []string
		for _, blob := range blobData {
			blobs = append(blobs, strings.TrimPrefix(blob, "sha256:"))
		}
		sort.Strings(blobs)
		blobs = unique(blobs)

		// Copy required blobs
		blobCount := len(blobs)
		fmt.Printf("  Copying %d blobs for %s:%s\n", blobCount, modelName, modelTag)

		copiedCount := 0
		failedCount := 0

		bar := progressbar.NewOptions(blobCount,
			progressbar.OptionSetDescription(fmt.Sprintf("Copying blobs for %s:%s", modelName, modelTag)),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
			progressbar.OptionSetPredictTime(false),
			progressbar.OptionSetItsString("blob"),
			progressbar.OptionThrottle(65*time.Millisecond),
		)

		for _, blob := range blobs {
			blobPath := filepath.Join(*ollamaBaseDir, "blobs/sha256-"+blob)
			destPath := filepath.Join(*outputDir, "models/blobs/sha256-"+blob)

			debugPrint(fmt.Sprintf("Verifying blob: %s", blobPath))

			if _, err := os.Stat(blobPath); err == nil {
				debugPrint("Blob found, copying...")
				if err := copyFile(blobPath, destPath); err == nil {
					//fmt.Printf("    Copied: sha256-%s\n", blob)
					copiedCount++
				} else {
					fmt.Printf("    ERROR: Failed to copy blob sha256-%s\n", blob)
					failedCount++
				}
			} else {
				fmt.Printf("    ERROR: Blob sha256-%s not found\n", blob)
				failedCount++
			}
			bar.Add(1)
		}
		bar.Finish()

		fmt.Printf("  Results for %s:%s - %d blobs copied, %d failures\n", modelName, modelTag, copiedCount, failedCount)

		if copiedCount > 0 {
			fmt.Printf("  Model %s:%s exported successfully\n", modelName, modelTag)
		} else {
			fmt.Printf("  ERROR: No blobs could be exported for %s:%s\n", modelName, modelTag)
		}
		fmt.Println("")
	}

	// Check if anything was exported
	if _, err := os.ReadDir(filepath.Join(*outputDir, "models/blobs/")); err == nil {
		if len(getFilesInDirectory(filepath.Join(*outputDir, "models/blobs/"))) == 0 {
			fmt.Println("WARNING: No blobs were exported.")
			fmt.Println("Verifying original blob file structure:")
			cmd := exec.Command("find", filepath.Join(*ollamaBaseDir, "blobs"), "-type", "f", "|", "head", "-n", "5")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		}
	}

	// Compress the export
	fmt.Println("Compressing export...")
	cmd := exec.Command("tar", "-czvf", "ollama-export.tar.gz", "-C", *outputDir, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		errorExit(fmt.Sprintf("Failed to compress export: %v", err))
	}

	fmt.Println("===================================================")
	fmt.Println("Export completed: ollama-export.tar.gz")
	fmt.Println("To import on the destination system:")
	fmt.Println("1. Decompress with: tar -xzvf ollama-export.tar.gz -C /destination/path")
	fmt.Println("2. Copy the files to the Docker container: docker cp /destination/path/. [ollama-container]:/root/.ollama/")
	fmt.Println("3. Restart the container: docker restart [ollama-container]")
	fmt.Println("===================================================")
	fmt.Println("Export finished.")
}

func unique(strSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range strSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func getFilesInDirectory(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files
}
