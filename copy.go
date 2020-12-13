package media_shrinker

import (
	"fmt"
	"os"
	"io"
)

func copyFile(inputPath string, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != err {
		return fmt.Errorf("Copy failed, could not create output file: %w", err)
	}
	defer outFile.Close()
	inputFile, err := os.Open(inputPath)
	if err != err {
		return fmt.Errorf("Copy failed, could not open input file: %w", err)
	}
	defer inputFile.Close()
	_, err = io.Copy(outFile, inputFile)
	return err
}
