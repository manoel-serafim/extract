package main

import (
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

// Define the patterns as a map
var patterns = map[string]string{

	"string":          "\"[^\"]+\"",
	"ip":              "(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])",
	"aws-keys":        "([^A-Z0-9]|^)(AKIA|A3T|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{12,}",
	"base64":          "([^A-Za-z0-9+/]|^)(eyJ|YTo|Tzo|PD[89]|aHR0cHM6L|aHR0cDo|rO0)[%a-zA-Z0-9+/]+={0,2}",
	"cors":            "Access-Control-Allow",
	"debug-pages":     "(Application-Trace|Routing Error|DEBUG\"? ?[=:] ?True|Caused by:|stack trace:|Microsoft .NET Framework|Traceback|[0-9]:in `|#!/us|WebApplicationException|java\\.lang\\.|phpinfo|swaggerUi|on line [0-9]|SQLSTATE)",
	"http-auth":       "[a-z0-9_/\\.:-]+@[a-z0-9-]+\\.[a-z0-9.-]+",
	"json-sec":        "(\\\\?\"|&quot;|%22)[a-z0-9_-]*(api[_-]?key|S3|aws_|secret|passw|auth)[a-z0-9_-]*(\\\\?\"|&quot;|%22): ?(\\\\?\"|&quot;|%22)[^\"&]+(\\\\?\"|&quot;|%22)",
	"meg-headers":     "^< [a-z0-9_\\-]+: .*",
	"php-sinks":       "[^a-z0-9_](system|exec|popen|pcntl_exec|eval|create_function|unserialize|file_exists|md5_file|filemtime|filesize|assert) ?\\(",
	"s3-buckets":      "[a-z0-9.-]+\\.s3\\.amazonaws\\.com|[a-z0-9.-]+\\.s3-[a-z0-9-]\\.amazonaws\\.com|[a-z0-9.-]+\\.s3-website[.-](eu|ap|us|ca|sa|cn)|//s3\\.amazonaws\\.com/[a-z0-9._-]+|//s3-[a-z0-9-]+\\.amazonaws\\.com/[a-z0-9._-]+",
	"secrets":         "(aws_access|aws_secret|api[_-]?key|ListBucketResult|S3_ACCESS_KEY|Authorization:|RSA PRIVATE|Index of|aws_|secret|ssh-rsa AA)",
	"urls":            "https?://[^\"\\'> ]+",
	"dom-xss-sources": "document\\.URL|document\\.documentURI|document\\.URLUnencoded|document\\.baseURI|location|document\\.cookie|document\\.referrer|window\\.name|history\\.pushState|history\\.replaceState|localStorage|sessionStorage|IndexedDB|Database",
	"dom-xss-sinks":   "document\\.write|window\\.location|document\\.cookie|eval|document\\.domain|WebSocket|[a-zA-Z]+\\.src|postMessage|setRequestHeader|FileReader\\.readAsText|ExecuteSql|sessionStorage\\.setItem|document\\.evaluate|JSON\\.parse|[a-zA-Z]+\\.setAttribute|RegExp",
	"user-content":    "https?://[^\\s]+\\.(?:pdf|exe|csv|docx|jpg|png|mp3|mp4|zip|txt|html|ppt|xls|xlsx|pptx|odt|ods|odp|rtf|wav|avi|flv|mov|mpg)",
	"dev-content":     "\\w+:\\/\\/[^\\s]*\\.(js|c|go|java|cpp|h|hpp|cs|py|php|rb|pl|swift|scala|groovy|ts|css|html|xml|json|yaml|yml|ini|sql|asm|sh|bash|ps1|bat|perl|r|lua|coffee|dart|kotlin|md|markdown|rmd|rst|tex|csharp|ts|scss|less|sass)",
}

func init() {
	flag.StringVar(&outputFolder, "o", "extracted", "Output folder name")
	flag.IntVar(&numThreads, "t", 1, "Number of threads")
	flag.StringVar(&useCase, "g", "-hario", "Grep arguments")
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

	if pattern == "" {
		for name, patternString := range patterns {
			wg.Add(1)
			go grepPattern(name, patternString, &wg)
		}
	} else {
		patternString, exists := patterns[pattern]
		if !exists {
			printColor(yellowColor, "Pattern '%s' not found in the local dictionary\n", pattern)
			return
		}
		wg.Add(1)
		go grepPattern(pattern, patternString, &wg)
	}

	wg.Wait()
}

func grepPattern(name, pattern string, wg *sync.WaitGroup) {
	defer wg.Done()

	printColor(resetColor, "Searching for pattern: %s\n", name)

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
		outputFileName := fmt.Sprintf("%s/%s", outputFolder, name)
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
