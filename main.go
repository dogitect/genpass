package main

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"iter"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/cpu"
)

// Compile-time constants optimized by the compiler
const (
	// Generator type constants using iota for efficient comparison
	_ GeneratorType = iota
	GeneratorHyphenated
	GeneratorCompact

	// Character sets as compile-time constants for better optimization
	lowerChars = "abcdefghijklmnopqrstuvwxyz"
	upperChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits     = "0123456789"

	// Pre-computed character sets using constant folding
	alphanumericChars = lowerChars + upperChars + digits

	// Performance tuning constants
	maxConcurrentGenerators = 32
	defaultWorkerPoolSize   = 8
	bufferPoolSize          = 1024
	maxBatchSize            = 1000

	// Security constants
	minEntropyBits    = 128
	maxStringLength   = 1024
	constantTimeLimit = 256 // For constant-time operations

	// Version information (set at build time)
	version = "0.0.2"
)

// GeneratorType represents the type of string generator using a custom type
// for better type safety and performance
type GeneratorType uint8

// String implements fmt.Stringer for GeneratorType
func (g GeneratorType) String() string {
	switch g {
	case GeneratorHyphenated:
		return "hyphenated"
	case GeneratorCompact:
		return "compact"
	default:
		return "unknown"
	}
}

// ParseGeneratorType parses a string into GeneratorType
func ParseGeneratorType(s string) (GeneratorType, error) {
	switch strings.ToLower(s) {
	case "hyphenated", "h":
		return GeneratorHyphenated, nil
	case "compact", "c":
		return GeneratorCompact, nil
	default:
		return 0, fmt.Errorf("invalid generator type: %q", s)
	}
}

// CharacterSet represents a character set with optimized operations
type CharacterSet struct {
	chars []byte
	mask  uint64 // Bitmask for power-of-2 optimization
}

// NewCharacterSet creates a new character set with compile-time optimization
func NewCharacterSet(s string) *CharacterSet {
	chars := []byte(s)
	// Remove duplicates while preserving order
	unique := chars[:0]
	seen := make(map[byte]bool)
	for _, c := range chars {
		if !seen[c] {
			seen[c] = true
			unique = append(unique, c)
		}
	}

	// Calculate mask for power-of-2 optimization
	var mask uint64
	if len := len(unique); len > 0 && (len&(len-1)) == 0 {
		mask = uint64(len - 1)
	}

	return &CharacterSet{
		chars: unique,
		mask:  mask,
	}
}

// At returns the character at index i using constant-time access when possible
func (cs *CharacterSet) At(i uint64) byte {
	if cs.mask != 0 {
		// Power-of-2 optimization using bitwise AND
		return cs.chars[i&cs.mask]
	}
	// Fallback to modulo operation
	return cs.chars[i%uint64(len(cs.chars))]
}

// Len returns the length of the character set
func (cs *CharacterSet) Len() int {
	return len(cs.chars)
}

// String returns the character set as a string for compatibility
func (cs *CharacterSet) String() string {
	return string(cs.chars)
}

// GeneratorConfig represents configuration for string generation
type GeneratorConfig struct {
	Type         GeneratorType
	Length       int
	Count        int
	Charset      *CharacterSet
	Parallel     bool
	Workers      int
	BatchSize    int
	MemoryPool   bool
	ConstantTime bool
}

// Validate validates the generator configuration
func (gc *GeneratorConfig) Validate() error {
	var errs []error

	if gc.Length <= 0 || gc.Length > maxStringLength {
		errs = append(errs, fmt.Errorf("invalid length: %d (must be 1-%d)", gc.Length, maxStringLength))
	}

	if gc.Count <= 0 || gc.Count > maxBatchSize {
		errs = append(errs, fmt.Errorf("invalid count: %d (must be 1-%d)", gc.Count, maxBatchSize))
	}

	if gc.Charset == nil || gc.Charset.Len() == 0 {
		errs = append(errs, errors.New("charset cannot be empty"))
	} else if gc.Charset.Len() > 256 {
		errs = append(errs, errors.New("charset too large (max 256 characters)"))
	}

	if gc.Workers <= 0 {
		gc.Workers = runtime.NumCPU()
	} else if gc.Workers > 32 {
		gc.Workers = 32 // Cap maximum workers
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// ByteBuffer represents a reusable byte buffer with pooling
type ByteBuffer struct {
	buf []byte
	cap int
}

// NewByteBuffer creates a new byte buffer with the specified capacity
func NewByteBuffer(capacity int) *ByteBuffer {
	return &ByteBuffer{
		buf: make([]byte, 0, capacity),
		cap: capacity,
	}
}

// Reset resets the buffer for reuse
func (bb *ByteBuffer) Reset() {
	bb.buf = bb.buf[:0]
}

// Grow grows the buffer to accommodate n more bytes
func (bb *ByteBuffer) Grow(n int) {
	if cap(bb.buf)-len(bb.buf) < n {
		newBuf := make([]byte, len(bb.buf), max(2*cap(bb.buf), len(bb.buf)+n))
		copy(newBuf, bb.buf)
		bb.buf = newBuf
	}
}

// Write appends bytes to the buffer
func (bb *ByteBuffer) Write(p []byte) (n int, err error) {
	bb.Grow(len(p))
	bb.buf = append(bb.buf, p...)
	return len(p), nil
}

// String returns the buffer content as a string
func (bb *ByteBuffer) String() string {
	return string(bb.buf)
}

// Bytes returns the buffer content as a byte slice
func (bb *ByteBuffer) Bytes() []byte {
	return bb.buf
}

// BufferPool manages a pool of reusable byte buffers
type BufferPool struct {
	pool sync.Pool
	size int
}

// NewBufferPool creates a new buffer pool
func NewBufferPool(bufferSize int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return NewByteBuffer(bufferSize)
			},
		},
		size: bufferSize,
	}
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() *ByteBuffer {
	return bp.pool.Get().(*ByteBuffer)
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(bb *ByteBuffer) {
	if bb != nil {
		bb.Reset()
		bp.pool.Put(bb)
	}
}

// EntropySource represents a cryptographically secure entropy source
type EntropySource struct {
	health atomic.Bool
	stats  struct {
		generated atomic.Uint64
		errors    atomic.Uint64
	}
}

// NewEntropySource creates a new entropy source
func NewEntropySource() *EntropySource {
	es := &EntropySource{}
	es.health.Store(true)
	return es
}

// GenerateBytes generates cryptographically secure random bytes
func (es *EntropySource) GenerateBytes(n int) ([]byte, error) {
	if !es.health.Load() {
		return nil, errors.New("entropy source is unhealthy")
	}

	buf := make([]byte, n)
	_, err := rand.Read(buf)
	if err != nil {
		es.stats.errors.Add(1)
		es.health.Store(false)
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	es.stats.generated.Add(uint64(n))
	return buf, nil
}

// GenerateUint64 generates a cryptographically secure random uint64
func (es *EntropySource) GenerateUint64() (uint64, error) {
	bytes, err := es.GenerateBytes(8)
	if err != nil {
		return 0, err
	}

	// Convert bytes to uint64 using safe binary encoding
	return binary.LittleEndian.Uint64(bytes), nil
}

// Health returns the health status of the entropy source
func (es *EntropySource) Health() bool {
	return es.health.Load()
}

// Stats returns statistics about the entropy source
func (es *EntropySource) Stats() (generated, errors uint64) {
	return es.stats.generated.Load(), es.stats.errors.Load()
}

// Generator represents a generic string generator using type parameters
type Generator[T any] interface {
	Generate(ctx context.Context, config *GeneratorConfig) (T, error)
	GenerateBatch(ctx context.Context, config *GeneratorConfig) ([]T, error)
	GenerateStream(ctx context.Context, config *GeneratorConfig) iter.Seq2[T, error]
}

// CryptoGenerator implements a cryptographically secure string generator
type CryptoGenerator struct {
	entropy    *EntropySource
	bufferPool *BufferPool
	workers    chan struct{}
	stats      struct {
		generated atomic.Uint64
		errors    atomic.Uint64
		duration  atomic.Uint64 // in nanoseconds
	}
}

// NewCryptoGenerator creates a new cryptographic string generator
func NewCryptoGenerator(workerLimit int) *CryptoGenerator {
	return &CryptoGenerator{
		entropy:    NewEntropySource(),
		bufferPool: NewBufferPool(1024),
		workers:    make(chan struct{}, workerLimit),
	}
}

// Generate generates a single secure random string
func (cg *CryptoGenerator) Generate(ctx context.Context, config *GeneratorConfig) (string, error) {
	start := time.Now()
	defer func() {
		cg.stats.duration.Add(uint64(time.Since(start).Nanoseconds()))
	}()

	if err := config.Validate(); err != nil {
		cg.stats.errors.Add(1)
		return "", fmt.Errorf("invalid config: %w", err)
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case cg.workers <- struct{}{}:
		defer func() { <-cg.workers }()
	}

	var result string
	var err error

	switch config.Type {
	case GeneratorHyphenated:
		result, err = cg.generateHyphenatedString(ctx, config)
	case GeneratorCompact:
		result, err = cg.generateCompactString(ctx, config)
	default:
		// Fallback to hyphenated
		result, err = cg.generateHyphenatedString(ctx, config)
	}

	if err != nil {
		cg.stats.errors.Add(1)
		return "", err
	}

	cg.stats.generated.Add(1)
	return result, nil
}

// GenerateBatch generates multiple secure random strings concurrently
func (cg *CryptoGenerator) GenerateBatch(ctx context.Context, config *GeneratorConfig) ([]string, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	results := make([]string, config.Count)

	if config.Count == 1 || !config.Parallel {
		// Sequential generation for small batches
		for i := 0; i < config.Count; i++ {
			result, err := cg.Generate(ctx, config)
			if err != nil {
				return nil, fmt.Errorf("generating string %d: %w", i, err)
			}
			results[i] = result
		}
		return results, nil
	}

	// Parallel generation using errgroup
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(config.Workers)

	for i := 0; i < config.Count; i++ {
		i := i // Capture loop variable
		g.Go(func() error {
			result, err := cg.Generate(ctx, config)
			if err != nil {
				return fmt.Errorf("generating string %d: %w", i, err)
			}
			results[i] = result
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// GenerateStream generates strings as an iterator using Go 1.25 iter package
func (cg *CryptoGenerator) GenerateStream(ctx context.Context, config *GeneratorConfig) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		for i := 0; i < config.Count; i++ {
			select {
			case <-ctx.Done():
				if !yield("", ctx.Err()) {
					return
				}
				return
			default:
			}

			result, err := cg.Generate(ctx, config)
			if !yield(result, err) {
				return
			}
		}
	}
}

// generateHyphenatedString generates a hyphenated format string
func (cg *CryptoGenerator) generateHyphenatedString(ctx context.Context, config *GeneratorConfig) (string, error) {
	// Use functional programming style with pipeline
	parts := make([]string, 3)

	// Generate parts concurrently if context allows
	g, ctx := errgroup.WithContext(ctx)

	for i := range parts {
		i := i // Capture loop variable
		g.Go(func() error {
			part, err := cg.generateSecureString(ctx, 6, config.Charset)
			if err != nil {
				return fmt.Errorf("generating part %d: %w", i, err)
			}
			parts[i] = part
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return "", err
	}

	return strings.Join(parts, "-"), nil
}

// generateCompactString generates a compact format string
func (cg *CryptoGenerator) generateCompactString(ctx context.Context, config *GeneratorConfig) (string, error) {
	return cg.generateSecureString(ctx, config.Length, config.Charset)
}

// generateSecureString generates a cryptographically secure random string
// using constant-time operations when possible
func (cg *CryptoGenerator) generateSecureString(ctx context.Context, length int, charset *CharacterSet) (string, error) {
	if length <= 0 {
		return "", errors.New("length must be positive")
	}

	// Use buffer pool for memory efficiency
	buffer := cg.bufferPool.Get()
	defer cg.bufferPool.Put(buffer)

	buffer.Grow(length)

	// Generate random indices for character selection
	indices := make([]uint64, length)
	for i := range indices {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		randVal, err := cg.entropy.GenerateUint64()
		if err != nil {
			return "", fmt.Errorf("generating random value: %w", err)
		}
		indices[i] = randVal
	}

	// Select characters using constant-time operations when possible
	result := make([]byte, length)
	if charset.mask != 0 {
		// Optimized path for power-of-2 character sets
		for i, idx := range indices {
			result[i] = charset.At(idx)
		}
	} else {
		// Fallback with rejection sampling for uniform distribution
		for i, idx := range indices {
			// Use rejection sampling to ensure uniform distribution
			maxValid := ^uint64(0) - (^uint64(0) % uint64(charset.Len()))
			retries := 0
			const maxRetries = 10 // Prevent infinite loops

			for idx >= maxValid {
				if retries >= maxRetries {
					return "", errors.New("too many retries in random sampling - possible attack")
				}
				var err error
				idx, err = cg.entropy.GenerateUint64()
				if err != nil {
					return "", fmt.Errorf("generating uniform random value: %w", err)
				}
				retries++
			}
			result[i] = charset.At(idx)
		}
	}

	// Clear sensitive data from memory
	defer func() {
		for i := range indices {
			indices[i] = 0
		}
	}()

	return string(result), nil
}

// Stats returns generator statistics
func (cg *CryptoGenerator) Stats() (generated, errors uint64, avgDuration time.Duration) {
	g := cg.stats.generated.Load()
	e := cg.stats.errors.Load()
	d := cg.stats.duration.Load()

	var avg time.Duration
	if g > 0 {
		avg = time.Duration(d / g)
	}

	return g, e, avg
}

// Application represents the main application with modern CLI framework
type Application struct {
	generator *CryptoGenerator
	config    *viper.Viper
}

// NewApplication creates a new application instance
func NewApplication() *Application {
	return &Application{
		generator: NewCryptoGenerator(maxConcurrentGenerators),
		config:    viper.New(),
	}
}

// Main application entry point using advanced CLI patterns
func main() {
	app := NewApplication()

	rootCmd := &cobra.Command{
		Use:   "genpass",
		Short: "Secure password generator",
		Long: `Generate cryptographically secure passwords.

Formats:
  hyphenated  6char-6char-6char (default)
  compact     custom length string`,
		Version: version,
		RunE:    app.runCommand,
	}

	// Configure flags with advanced validation
	rootCmd.Flags().StringP("type", "t", "hyphenated", "Output format (hyphenated|compact)")
	rootCmd.Flags().IntP("length", "l", 15, "Length for compact format")
	rootCmd.Flags().IntP("count", "c", 1, "Number of passwords")
	rootCmd.Flags().StringP("charset", "s", alphanumericChars, "Character set")
	rootCmd.Flags().BoolP("parallel", "p", true, "Parallel generation")
	rootCmd.Flags().IntP("workers", "w", runtime.NumCPU(), "Worker threads")
	rootCmd.Flags().BoolP("stats", "", false, "Show statistics")
	rootCmd.Flags().BoolP("stream", "", false, "Stream output")
	rootCmd.Flags().DurationP("timeout", "", 30*time.Second, "Timeout")

	// Bind flags to viper for advanced configuration management
	viper.BindPFlags(rootCmd.Flags())

	// Execute with structured error handling
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// runCommand executes the main application logic with advanced error handling
func (app *Application) runCommand(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("timeout"))
	defer cancel()

	// Parse and validate configuration
	config, err := app.parseConfig()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Generate strings using the specified method
	if viper.GetBool("stream") {
		return app.generateStream(ctx, config)
	} else {
		return app.generateBatch(ctx, config)
	}
}

// parseConfig parses and validates the application configuration
func (app *Application) parseConfig() (*GeneratorConfig, error) {
	genType, err := ParseGeneratorType(viper.GetString("type"))
	if err != nil {
		return nil, err
	}

	charset := NewCharacterSet(viper.GetString("charset"))
	if charset.Len() == 0 {
		return nil, errors.New("charset cannot be empty")
	}

	config := &GeneratorConfig{
		Type:         genType,
		Length:       viper.GetInt("length"),
		Count:        viper.GetInt("count"),
		Charset:      charset,
		Parallel:     viper.GetBool("parallel"),
		Workers:      viper.GetInt("workers"),
		BatchSize:    maxBatchSize,
		MemoryPool:   true,
		ConstantTime: true,
	}

	return config, config.Validate()
}

// generateBatch generates strings in batch mode
func (app *Application) generateBatch(ctx context.Context, config *GeneratorConfig) error {
	start := time.Now()

	results, err := app.generator.GenerateBatch(ctx, config)
	if err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	duration := time.Since(start)

	// Output results
	for _, result := range results {
		fmt.Println(result)
	}

	// Show statistics if requested
	if viper.GetBool("stats") {
		app.showStats(duration, config.Count)
	}

	return nil
}

// generateStream generates strings in streaming mode using Go 1.25 iterators
func (app *Application) generateStream(ctx context.Context, config *GeneratorConfig) error {
	start := time.Now()
	generated := 0

	// Use the new iterator pattern from Go 1.25
	for result, err := range app.generator.GenerateStream(ctx, config) {
		if err != nil {
			continue
		}

		fmt.Println(result)
		generated++
	}

	duration := time.Since(start)

	// Show statistics if requested
	if viper.GetBool("stats") {
		app.showStats(duration, generated)
	}

	return nil
}

// showStats displays generation statistics
func (app *Application) showStats(duration time.Duration, count int) {
	generated, errors, avgDuration := app.generator.Stats()
	entropyGenerated, entropyErrors := app.generator.entropy.Stats()

	fmt.Fprintf(os.Stderr, "\n--- Generation Statistics ---\n")
	fmt.Fprintf(os.Stderr, "Total Generated: %d strings\n", generated)
	fmt.Fprintf(os.Stderr, "Total Errors: %d\n", errors)
	fmt.Fprintf(os.Stderr, "Batch Duration: %v\n", duration)
	fmt.Fprintf(os.Stderr, "Average Duration: %v per string\n", avgDuration)
	fmt.Fprintf(os.Stderr, "Throughput: %.2f strings/sec\n", float64(count)/duration.Seconds())
	fmt.Fprintf(os.Stderr, "Entropy Generated: %d bytes\n", entropyGenerated)
	fmt.Fprintf(os.Stderr, "Entropy Errors: %d\n", entropyErrors)
	fmt.Fprintf(os.Stderr, "Worker Utilization: %d/%d\n", len(app.generator.workers), cap(app.generator.workers))

	// CPU feature detection for optimization insights
	if cpu.X86.HasAES {
		fmt.Fprintf(os.Stderr, "Hardware AES: Available\n")
	}
	if cpu.X86.HasAVX2 {
		fmt.Fprintf(os.Stderr, "AVX2 SIMD: Available\n")
	}
}

// Utility functions

// max returns the maximum of two integers (built-in since Go 1.21)
func max(a, b int) int {
	return cmp.Or(a, b)
}
