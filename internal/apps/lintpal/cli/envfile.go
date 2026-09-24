package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var ErrInvalidEnvFile = errors.New("invalid env file")
var ErrEnvFileLimit = errors.New("env file limit exceeded")

const maxEnvFileBytes = 64 << 10

var envKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// LoadEnv loads local configuration without changing the process environment.
// An absent default file is allowed; an explicitly selected file is required.
func LoadEnv(ctx context.Context, cwd, explicitPath string, disabled bool) (LookupEnv, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if disabled && explicitPath != "" {
		return nil, ErrInvalidEnvFile
	}
	if disabled {
		return LayeredLookup(os.LookupEnv, nil), nil
	}
	path := explicitPath
	boundary := cwd
	if path == "" {
		command := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--show-toplevel")
		command.Stderr = io.Discard
		root, err := command.Output()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return LayeredLookup(os.LookupEnv, nil), nil
		}
		boundary = strings.TrimSpace(string(root))
		path = filepath.Join(boundary, ".env")
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	data, err := readEnvFile(ctx, path, boundary, explicitPath == "")
	if err != nil {
		return nil, err
	}
	values, err := parseEnvFile(data)
	if err != nil {
		return nil, err
	}
	return LayeredLookup(os.LookupEnv, values), nil
}

// LayeredLookup gives an existing process setting precedence over file data.
func LayeredLookup(process LookupEnv, file map[string]string) LookupEnv {
	copyFile := make(map[string]string, len(file))
	for key, value := range file {
		copyFile[key] = value
	}
	return func(key string) (string, bool) {
		if process != nil {
			if value, ok := process(key); ok {
				return value, true
			}
		}
		value, ok := copyFile[key]
		return value, ok
	}
}

func readEnvFile(ctx context.Context, path, boundary string, optional bool) ([]byte, error) {
	clean := filepath.Clean(path)
	boundary, err := filepath.Abs(boundary)
	if err != nil {
		return nil, ErrInvalidEnvFile
	}
	relative, err := filepath.Rel(boundary, clean)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		boundary = filepath.Dir(clean)
	}
	for part := clean; ; part = filepath.Dir(part) {
		info, err := os.Lstat(part)
		if err != nil {
			if optional && part == clean && errors.Is(err, os.ErrNotExist) {
				return nil, nil
			}
			return nil, ErrInvalidEnvFile
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInvalidEnvFile
		}
		if part == clean && !info.Mode().IsRegular() {
			return nil, ErrInvalidEnvFile
		}
		if part == boundary {
			break
		}
	}
	file, err := os.Open(clean)
	if err != nil {
		return nil, ErrInvalidEnvFile
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxEnvFileBytes+1))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, ErrInvalidEnvFile
	}
	if len(data) > maxEnvFileBytes {
		return nil, ErrEnvFileLimit
	}
	return data, nil
}

func parseEnvFile(data []byte) (map[string]string, error) {
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, ErrInvalidEnvFile
	}
	values := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !envKey.MatchString(key) {
			return nil, ErrInvalidEnvFile
		}
		if _, duplicate := values[key]; duplicate {
			return nil, ErrInvalidEnvFile
		}
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, `"`) {
			decoded, err := strconv.Unquote(value)
			if err != nil {
				return nil, ErrInvalidEnvFile
			}
			value = decoded
		} else if strings.HasPrefix(value, "'") {
			if len(value) < 2 || !strings.HasSuffix(value, "'") {
				return nil, ErrInvalidEnvFile
			}
			value = value[1 : len(value)-1]
		}
		if strings.ContainsAny(value, "\r\n") {
			return nil, ErrInvalidEnvFile
		}
		values[key] = value
	}
	if scanner.Err() != nil {
		return nil, ErrInvalidEnvFile
	}
	return values, nil
}
