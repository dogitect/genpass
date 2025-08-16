package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
)

const (
	// TypeHyphenated represents hyphenated string format.
	TypeHyphenated = "hyphenated"
	// TypeCompact represents compact string format.
	TypeCompact = "compact"

	lowerChars = "abcdefghijklmnopqrstuvwxyz"
	upperChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits     = "0123456789"

	maxGenerationCount = 32
)

var (
	// version is set during build time using ldflags.
	version = "0.0.1"

	flagType    = flag.String("type", TypeHyphenated, "Output format (hyphenated|compact)")
	flagCount   = flag.Int("count", 1, "Number of strings to generate (1-32)")
	flagHelp    = flag.Bool("help", false, "Show help message")
	flagVersion = flag.Bool("version", false, "Show version information")
)

func main() {
	// Custom usage function for better help formatting.
	flag.Usage = usage
	flag.Parse()

	// Handle version flag.
	if *flagVersion {
		fmt.Printf("genpass %s\n", version)
		return
	}

	// Handle help flag.
	if *flagHelp {
		flag.Usage()
		return
	}

	// Check for unexpected arguments.
	if len(flag.Args()) > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected argument: %s\n", flag.Args()[0])
		fmt.Fprintf(os.Stderr, "Use --help for usage information.\n")
		os.Exit(1)
	}

	// Validate count.
	if *flagCount < 1 || *flagCount > maxGenerationCount {
		fmt.Fprintf(os.Stderr, "Error: string count must be between 1 and %d\n", maxGenerationCount)
		os.Exit(1)
	}

	// Validate output type.
	if *flagType != TypeHyphenated && *flagType != TypeCompact {
		fmt.Fprintf(os.Stderr, "Error: invalid output type: %q (must be %q or %q)\n",
			*flagType, TypeHyphenated, TypeCompact)
		os.Exit(1)
	}

	// Generate secure strings.
	for i := 0; i < *flagCount; i++ {
		result, err := generateString(*flagType)
		if err != nil {
			// Avoid logging detailed error information that might contain sensitive data
			fmt.Fprintf(os.Stderr, "Error: failed to generate string\n")
			os.Exit(1)
		}
		fmt.Println(result)
	}
}

// generateString generates a string of the specified type.
func generateString(outputType string) (string, error) {
	switch outputType {
	case TypeHyphenated:
		return generateHyphenatedString()
	case TypeCompact:
		return generateCompactString()
	default:
		return "", fmt.Errorf("unsupported output type: %q", outputType)
	}
}

// generateHyphenatedString generates string in format: jaszeM-xizqox-7cafri.
func generateHyphenatedString() (string, error) {
	var parts []string

	for i := 0; i < 3; i++ {
		part, err := generateRandomString(6, lowerChars+upperChars+digits)
		if err != nil {
			return "", fmt.Errorf("generating part %d: %w", i+1, err)
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, "-"), nil
}

// generateCompactString generates string in format: VQ4noP8j1Y2eRaz.
func generateCompactString() (string, error) {
	return generateRandomString(15, lowerChars+upperChars+digits)
}

// generateRandomString generates a random string of specified length from charset.
func generateRandomString(length int, charset string) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length must be positive, got %d", length)
	}
	if len(charset) == 0 {
		return "", fmt.Errorf("charset cannot be empty")
	}

	result := make([]byte, length)
	charsetLength := big.NewInt(int64(len(charset)))

	for i := range result {
		randomIndex, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			// Clear partial result before returning error to avoid potential leaks
			for j := 0; j < i; j++ {
				result[j] = 0
			}
			return "", fmt.Errorf("generating random number: %w", err)
		}
		result[i] = charset[randomIndex.Int64()]
	}

	return string(result), nil
}

// usage prints the usage message.
func usage() {
	fmt.Printf("genpass - Secure Random String Generator\n\n")
	fmt.Printf("Generate cryptographically secure random strings for authentication,\n")
	fmt.Printf("tokens, and other security applications.\n\n")

	fmt.Printf("USAGE:\n")
	fmt.Printf("  genpass [options]\n\n")

	fmt.Printf("OPTIONS:\n")
	fmt.Printf("  --type string     Format type: hyphenated or compact (default: TypeHyphenated)\n")
	fmt.Printf("  --count int       Number of strings to generate, 1-32 (default: 1)\n")
	fmt.Printf("  --version         Show version information\n")
	fmt.Printf("  --help            Show this help message\n\n")

	fmt.Printf("FORMATS:\n")
	fmt.Printf("  hyphenated        18 chars in format: ABC123-def456-GHI789\n")
	fmt.Printf("  compact           15 chars in format: ABC123def456GHI\n\n")

	fmt.Printf("EXAMPLES:\n")
	fmt.Printf("  genpass                    # One hyphenated string\n")
	fmt.Printf("  genpass --type compact     # One compact string\n")
	fmt.Printf("  genpass --count 5          # Five hyphenated strings\n")
	fmt.Printf("  genpass -t compact -c 3    # Three compact strings\n\n")
}
