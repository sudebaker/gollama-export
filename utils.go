// utils.go
package main

import (
	"fmt"
	"os/exec"
	"strings"
)

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
