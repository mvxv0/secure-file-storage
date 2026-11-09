package env

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Load ucitava .env fajl iz trenutnog ili nadredjenih direktorijuma (za monorepo strukturu).
func Load() {
	candidates := []string{
		".env",
		filepath.Join("..", ".env"),
		filepath.Join("..", "..", ".env"),
	}

	for _, path := range candidates {
		if loadFile(path) {
			return
		}
	}
}

func loadFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return true
}

func GetString(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return val
	}
	return fallback
}

func GetInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return parsed
		}
	}
	return fallback
}

func GetInt64(key string, fallback int64) int64 {
	if val, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

