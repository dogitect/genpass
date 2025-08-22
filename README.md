# Genpass - Generate a random secure string

A secure string generator built with Go

## Install

```bash
go install github.com/dogitect/genpass@latest
```

## Usage

```bash
# Default (hyphenated format)
genpass

# Compact format
genpass -t compact -l 32

# Multiple passwords
genpass -c 5

# Show statistics
genpass -c 100 --stats
```

## License

MIT © dogitect
