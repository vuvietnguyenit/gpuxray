package app

import (
	"fmt"
	"log"
	"os"
)

var LIBCUDART_LOC = []string{
	"/usr/lib/x86_64-linux-gnu/libcudart.so",
	"/usr/local/cuda/lib64/libcudart.so",
	"/usr/lib64/libcudart.so",
	"/lib/x86_64-linux-gnu/libcudart.so",
	"/usr/lib/libcudart.so",
}

var LIBCUDA_LOC = []string{
	"/usr/lib/x86_64-linux-gnu/libcuda.so",
	"/usr/local/cuda/lib64/libcuda.so",
	"/usr/lib64/libcuda.so",
	"/lib/x86_64-linux-gnu/libcuda.so",
	"/usr/lib/libcuda.so",
}

func findCudaLibrary() string {
	if libPath := os.Getenv("LIBCUDART_PATH"); libPath != "" {
		if _, err := os.Stat(libPath); err == nil {
			log.Printf("Using CUDA library from LIBCUDART_PATH: %s", libPath)
			return libPath
		}
		log.Printf("Warning: LIBCUDART_PATH is set but file not found: %s", libPath)
	}

	if libPath := os.Getenv("LIBCUDA_PATH"); libPath != "" {
		if _, err := os.Stat(libPath); err == nil {
			log.Printf("Using CUDA library from LIBCUDA_PATH: %s", libPath)
			return libPath
		}
		log.Printf("Warning: LIBCUDA_PATH is set but file not found: %s", libPath)
	}

	// Check default locations
	var searchErrors []string
	for _, loc := range LIBCUDART_LOC {
		if _, err := os.Stat(loc); err == nil {
			return loc
		} else {
			searchErrors = append(searchErrors, fmt.Sprintf("  - %s: %v", loc, err))
		}
	}

	// If we get here, library was not found
	log.Println("CUDA library not found in default locations:")
	for _, errMsg := range searchErrors {
		log.Println(errMsg)
	}
	log.Println("\nPlease set one of the following environment variables to specify the CUDA library path:")
	log.Println("  export LIBCUDART_PATH=/path/to/libcudart.so")
	log.Println("  export LIBCUDA_PATH=/path/to/libcuda.so")
	log.Println("\nExample:")
	log.Println("  export LIBCUDART_PATH=/usr/local/cuda/lib64/libcudart.so")

	return ""
}
