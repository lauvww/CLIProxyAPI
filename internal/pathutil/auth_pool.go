package pathutil

import (
	"path"
	"strings"
)

// NormalizePath trims and cleans auth-pool style paths while keeping a stable
// separator style for Windows-looking inputs.
func NormalizePath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	slashNormalized := strings.ReplaceAll(trimmed, "\\", "/")
	cleaned := cleanSlashPath(slashNormalized)
	if cleaned == "" || cleaned == "." {
		return ""
	}

	if shouldUseWindowsSeparators(trimmed, cleaned) {
		cleaned = strings.ReplaceAll(cleaned, "/", "\\")
		if isWindowsDriveRoot(cleaned) {
			return cleaned + "\\"
		}
		return cleaned
	}

	return cleaned
}

// NormalizeCompareKey returns a stable comparison key for auth-pool paths.
func NormalizeCompareKey(value string) string {
	normalized := NormalizePath(value)
	if normalized == "" {
		return ""
	}

	key := strings.ReplaceAll(normalized, "\\", "/")
	if key != "/" {
		key = strings.TrimRight(key, "/")
	}

	if isWindowsStyleKey(key) {
		return strings.ToLower(key)
	}

	return key
}

// PathsEqual compares auth-pool paths using a stable normalized key.
func PathsEqual(left string, right string) bool {
	leftKey := NormalizeCompareKey(left)
	if leftKey == "" {
		return false
	}
	return leftKey == NormalizeCompareKey(right)
}

// IsWithinScope reports whether filePath is equal to or nested beneath scopePath.
func IsWithinScope(filePath string, scopePath string) bool {
	fileKey := NormalizeCompareKey(filePath)
	scopeKey := NormalizeCompareKey(scopePath)
	if fileKey == "" || scopeKey == "" {
		return false
	}
	if fileKey == scopeKey {
		return true
	}
	return strings.HasPrefix(fileKey, scopeKey+"/")
}

func cleanSlashPath(value string) string {
	if value == "" {
		return ""
	}

	if hasWindowsDrivePrefix(value) {
		prefix := value[:2]
		remainder := strings.TrimPrefix(value[2:], "/")
		if remainder == "" {
			return prefix + "/"
		}
		return prefix + "/" + path.Clean("/" + remainder)[1:]
	}

	if strings.HasPrefix(value, "//") {
		trimmed := strings.TrimLeft(value, "/")
		parts := strings.Split(trimmed, "/")
		if len(parts) >= 2 {
			prefix := "//" + parts[0] + "/" + parts[1]
			remainder := strings.Join(parts[2:], "/")
			if remainder == "" {
				return prefix
			}
			return prefix + "/" + strings.TrimPrefix(path.Clean("/"+remainder), "/")
		}
	}

	return path.Clean(value)
}

func hasWindowsDrivePrefix(value string) bool {
	if len(value) < 2 {
		return false
	}
	letter := value[0]
	return ((letter >= 'a' && letter <= 'z') || (letter >= 'A' && letter <= 'Z')) && value[1] == ':'
}

func isWindowsDriveRoot(value string) bool {
	return len(value) == 2 && hasWindowsDrivePrefix(value)
}

func shouldUseWindowsSeparators(original string, cleaned string) bool {
	return strings.Contains(original, "\\") || hasWindowsDrivePrefix(original) || hasWindowsDrivePrefix(cleaned)
}

func isWindowsStyleKey(value string) bool {
	return hasWindowsDrivePrefix(value) || strings.HasPrefix(value, "//")
}
