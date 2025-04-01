// main.go
package main

/// An application to export and compress ollama models
/// The app gets some args from the command line (model to export, ollama directory, destination directory)
/// It then checks if the model exists in the ollama directory
/// If it does, it compresses the model and saves it to the destination directory
/// If it doesn't, it prints an error message
/// The app also checks if the destination directory exists and creates it if it doesn't
/// If no arguments are provided, the app export all models to defaults directories

import (
	"flag"
	"fmt"
	"os"
)

var (
	ollamaBaseDir = flag.String("o", "/var/lib/ollama", "Ollama models directory")
	outputDir     = flag.String("d", "./ollama-export", "Destination directory for exported models")
	debug         = flag.Bool("debug", false, "Enable debug messages")
	modelName     = flag.String("m", "", "Model to export (optional)") // New flag for model selection
)

func main() {
	flag.Usage = usage
	// Add -h and --help flags
	flag.Bool("h", false, "Show this help message")
	flag.Bool("help", false, "Show this help message")

	flag.Parse()

	// Check if -h or --help is present
	for _, arg := range os.Args {
		if arg == "-h" || arg == "--help" {
			usage()
			return
		}
	}

	// Initialize the application
	app := NewApp(*ollamaBaseDir, *outputDir, *debug)
	app.Run()
}

func debugPrint(msg string, debug bool) {
	if debug {
		fmt.Println("[DEBUG]", msg)
	}
}

func errorExit(msg string) {
	fmt.Println("ERROR:", msg)
	os.Exit(1)
}

func usage() {
	fmt.Println("Usage: goexport-ollama [OPTIONS] [model1[:tag1] model2[:tag2] ...]")
	fmt.Println("  -o, --ollama-dir <directory> : Ollama base directory (default: /var/lib/ollama)")
	fmt.Println("  -d, --output-dir <directory> : Output directory for export (default: ./ollama-export)")
	fmt.Println("  -h, --help                   : Show this help message")
	fmt.Println("  --debug                      : Enable debug messages")
	fmt.Println("  model1[:tag1] model2[:tag2] ... : List of models to export (if not specified, all are exported)")
	os.Exit(0)
}
