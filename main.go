package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
)

var (
	outputFolder  string
	numThreads    int
	useCase       string
	includeBinary bool
	pattern       string
	mutex         sync.Mutex
)

const (
	redColor    = "\033[31m"
	greenColor  = "\033[32m"
	yellowColor = "\033[33m"
	resetColor  = "\033[0m"
)

func init() {
	flag.StringVar(&outputFolder, "o", "extracted", "Output folder name")
	flag.IntVar(&numThreads, "t", 1, "Number of threads")
	flag.StringVar(&useCase, "g", "-Hario", "Grep arguments")
	flag.StringVar(&pattern, "p", "", "Specific pattern to grep")
	flag.BoolVar(&includeBinary, "b", false, "Include binary files in the search")

	// Set a custom Usage function to display help
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Println("Options:")
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// Display help if no arguments are provided
	if flag.NFlag() == 0 {
		flag.Usage()
		return
	}

	if _, err := os.Stat(outputFolder); os.IsNotExist(err) {
		os.Mkdir(outputFolder, os.ModePerm)
	}

	var wg sync.WaitGroup
	fileContent, err := ioutil.ReadFile("patterns.json")
	if err != nil {
		printColor(redColor,"Error reading patterns file:", err)
		return
	}

	var patterns map[string]string
	err = json.Unmarshal(fileContent, &patterns)
	if err != nil {
		printColor(redColor,"Error decoding patterns:", err)
		return
	}

	if pattern == "" {
		for name, patternString := range patterns {
			wg.Add(1)
			go grepPattern(name, patternString, &wg)
		}
	} else {
		patternString, exists := patterns[pattern]
		if !exists {
			printColor(yellowColor,"Pattern '%s' not found in the JSON file\n", pattern)
			return
		}
		wg.Add(1)
		go grepPattern(pattern, patternString, &wg)
	}

	wg.Wait()
}

func grepPattern(name, pattern string, wg *sync.WaitGroup) {
	defer wg.Done()

	printColor(resetColor,"Searching for pattern: %s\n", name)

	args := []string{useCase}

	if includeBinary {
		args = append(args, "--binary-files=without-match")
	}
	args = append(args, "-E")

	args = append(args, pattern)

	cmd := exec.Command("grep", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				printColor(yellowColor, "%s pattern not found in this directory.\n", name)
			} else {
				printColor(redColor, "Error executing grep for pattern %s (exit code %d): %s\n", name, exitErr.ExitCode(), exitErr.Stderr)
			}
		} else {
			printColor(redColor, "Error executing grep for pattern %s: %v\n", name, err)
		}
		return
	}

	if len(output) > 0 {
		outputFileName := fmt.Sprintf("%s/%s.txt", outputFolder, name)
		err := ioutil.WriteFile(outputFileName, output, os.ModePerm)
		if err != nil {
			printColor(redColor, "Error writing output for pattern %s: %v\n", name, err)
			return
		}

		printColor(greenColor, "%s pattern search complete. Output saved to %s\n", name, outputFileName)
	} else {
		printColor(yellowColor, "Pattern %s not found.\n", name)
	}
}

func printColor(colorCode, format string, a ...interface{}) {
	mutex.Lock()
	fmt.Printf(colorCode+format+resetColor, a...)
	mutex.Unlock()
}
