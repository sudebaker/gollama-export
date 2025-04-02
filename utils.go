// utils.go
package main

import (
	//"archive/tar"
	//"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	//"path/filepath"
	"strings"
)

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

// func getOllamaModels() ([]string, error) {
// 	cmd := exec.Command("ollama", "ls")
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return nil, fmt.Errorf("error executing ollama ls: %v\nOutput: %s", err, output)
// 	}

// 	lines := strings.Split(string(output), "\n")
// 	if len(lines) <= 1 {
// 		return nil, fmt.Errorf("no models found with 'ollama ls'")
// 	}

// 	models := []string{}
// 	for _, line := range lines[1:] {
// 		line = strings.TrimSpace(line)
// 		if line == "" {
// 			continue
// 		}
// 		parts := strings.Fields(line)
// 		if len(parts) >= 1 {
// 			models = append(models, parts[0])
// 		}
// 	}
// 	return models, nil
// }

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

// func createTarGz(source, target string) error {
// 	tarfile, err := os.Create(target)
// 	if err != nil {
// 		return err
// 	}
// 	defer tarfile.Close()

// 	gzipWriter := gzip.NewWriter(tarfile)
// 	defer gzipWriter.Close()

// 	tarWriter := tar.NewWriter(gzipWriter)
// 	defer tarWriter.Close()

// 	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return err
// 		}

// 		header, err := tar.FileInfoHeader(info, info.Name())
// 		if err != nil {
// 			return err
// 		}

// 		header.Name, err = filepath.Rel(source, path)
// 		if err != nil {
// 			return err
// 		}

// 		if err := tarWriter.WriteHeader(header); err != nil {
// 			return err
// 		}

// 		if !info.IsDir() {
// 			file, err := os.Open(path)
// 			if err != nil {
// 				return err
// 			}
// 			defer file.Close()

// 			_, err = io.Copy(tarWriter, file)
// 			if err != nil {
// 				return err
// 			}
// 		}

// 		return nil
// 	})
// }

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
