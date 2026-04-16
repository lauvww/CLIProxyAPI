package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	usagePersistenceDirName    = "use"
	usagePersistenceFileName   = "usage-statistics"
	usagePersistenceFileExt    = ".json"
	usagePersistencePayloadVer = 1
	usagePersistenceDefaultTag = "default"
	usagePersistenceWindowsDir = `D:\CLIProxyAPI\use`
	usagePersistenceDirEnv     = "CLIPROXYAPI_USAGE_DIR"
)

type persistedUsagePayload struct {
	Version    int                `json:"version"`
	ExportedAt time.Time          `json:"exported_at"`
	AuthPool   string             `json:"auth_pool,omitempty"`
	Usage      StatisticsSnapshot `json:"usage"`
}

// LoadRecentSnapshot loads usage statistics from disk and keeps only records within
// the most recent dayWindow days.
func LoadRecentSnapshot(configPath string, authPool string, now time.Time, dayWindow int) (StatisticsSnapshot, string, bool, error) {
	path := ResolvePersistenceFilePath(configPath, authPool)
	raw, errRead := os.ReadFile(path)
	if errRead != nil {
		if errors.Is(errRead, os.ErrNotExist) {
			return StatisticsSnapshot{}, path, false, nil
		}
		return StatisticsSnapshot{}, path, false, fmt.Errorf("read usage persistence file: %w", errRead)
	}

	var payload persistedUsagePayload
	if errUnmarshal := json.Unmarshal(raw, &payload); errUnmarshal != nil {
		return StatisticsSnapshot{}, path, false, fmt.Errorf("parse usage persistence file: %w", errUnmarshal)
	}
	if payload.Version != 0 && payload.Version != usagePersistencePayloadVer {
		return StatisticsSnapshot{}, path, false, fmt.Errorf("unsupported usage persistence version: %d", payload.Version)
	}

	if now.IsZero() {
		now = time.Now().UTC()
	}
	if dayWindow <= 0 {
		dayWindow = 7
	}
	since := now.UTC().AddDate(0, 0, -dayWindow)
	filtered := filterSnapshotSince(payload.Usage, since)
	return filtered, path, true, nil
}

// SaveSnapshot writes usage statistics to disk in JSON format.
func SaveSnapshot(configPath string, authPool string, snapshot StatisticsSnapshot, exportedAt time.Time) (string, error) {
	path := ResolvePersistenceFilePath(configPath, authPool)
	if exportedAt.IsZero() {
		exportedAt = time.Now().UTC()
	}

	filtered := snapshot
	normalizedPool := normalizeAuthPool(authPool)
	if normalizedPool != "" {
		filtered = FilterSnapshotByAuthPool(snapshot, normalizedPool)
	}

	payload := persistedUsagePayload{
		Version:    usagePersistencePayloadVer,
		ExportedAt: exportedAt.UTC(),
		AuthPool:   normalizedPool,
		Usage:      filtered,
	}

	data, errMarshal := json.MarshalIndent(payload, "", "  ")
	if errMarshal != nil {
		return path, fmt.Errorf("marshal usage persistence payload: %w", errMarshal)
	}

	dir := filepath.Dir(path)
	if errMkdir := os.MkdirAll(dir, 0o755); errMkdir != nil {
		return path, fmt.Errorf("create usage persistence directory: %w", errMkdir)
	}

	if errWrite := os.WriteFile(path, data, 0o644); errWrite != nil {
		return path, fmt.Errorf("write usage persistence file: %w", errWrite)
	}
	return path, nil
}

// ResolvePersistenceFilePath builds the usage persistence file path from config path and auth pool.
func ResolvePersistenceFilePath(configPath string, authPool string) string {
	baseDir := resolvePersistenceBaseDir(configPath)
	fileName := resolvePersistenceFileName(authPool)
	return filepath.Join(baseDir, fileName)
}

// ListAuthPools returns the distinct normalized auth pool keys present in a snapshot.
func ListAuthPools(snapshot StatisticsSnapshot) []string {
	seen := make(map[string]struct{})
	pools := make([]string, 0)

	for _, apiSnapshot := range snapshot.APIs {
		for _, modelSnapshot := range apiSnapshot.Models {
			for _, detail := range modelSnapshot.Details {
				authPool := normalizeAuthPool(detail.AuthPool)
				if authPool == "" {
					continue
				}
				if _, ok := seen[authPool]; ok {
					continue
				}
				seen[authPool] = struct{}{}
				pools = append(pools, authPool)
			}
		}
	}

	sort.Strings(pools)
	return pools
}

func resolvePersistenceBaseDir(configPath string) string {
	if overrideDir := strings.TrimSpace(os.Getenv(usagePersistenceDirEnv)); overrideDir != "" {
		return overrideDir
	}
	if runtime.GOOS == "windows" {
		return usagePersistenceWindowsDir
	}
	baseDir := ""
	if configPath != "" {
		baseDir = filepath.Dir(configPath)
	}
	if baseDir == "" || baseDir == "." {
		if wd, errWd := os.Getwd(); errWd == nil && wd != "" {
			baseDir = wd
		}
	}
	if baseDir == "" {
		baseDir = "."
	}
	return filepath.Join(baseDir, usagePersistenceDirName)
}

func resolvePersistenceFileName(authPool string) string {
	normalizedPool := normalizeAuthPool(authPool)
	if normalizedPool == "" {
		return usagePersistenceFileName + "." + usagePersistenceDefaultTag + usagePersistenceFileExt
	}
	hash := sha256.Sum256([]byte(strings.ToLower(normalizedPool)))
	shortHash := hex.EncodeToString(hash[:8])
	return usagePersistenceFileName + "." + shortHash + usagePersistenceFileExt
}

func filterSnapshotSince(snapshot StatisticsSnapshot, since time.Time) StatisticsSnapshot {
	result := StatisticsSnapshot{
		APIs:           make(map[string]APISnapshot),
		RequestsByDay:  make(map[string]int64),
		RequestsByHour: make(map[string]int64),
		TokensByDay:    make(map[string]int64),
		TokensByHour:   make(map[string]int64),
	}

	sinceUTC := since.UTC()

	for apiName, apiSnapshot := range snapshot.APIs {
		filteredAPI := APISnapshot{
			Models: make(map[string]ModelSnapshot),
		}

		for modelName, modelSnapshot := range apiSnapshot.Models {
			filteredModel := ModelSnapshot{}
			for _, detail := range modelSnapshot.Details {
				if detail.Timestamp.IsZero() {
					continue
				}
				timestamp := detail.Timestamp.UTC()
				if timestamp.Before(sinceUTC) {
					continue
				}

				tokens := normaliseTokenStats(detail.Tokens)
				if detail.LatencyMs < 0 {
					detail.LatencyMs = 0
				}
				detail.Timestamp = timestamp
				detail.Tokens = tokens
				detail = normalizeRequestDetailDimensions(detail)

				filteredModel.Details = append(filteredModel.Details, detail)
				filteredModel.TotalRequests++
				filteredModel.TotalTokens += tokens.TotalTokens

				filteredAPI.TotalRequests++
				filteredAPI.TotalTokens += tokens.TotalTokens

				result.TotalRequests++
				if detail.Failed {
					result.FailureCount++
				} else {
					result.SuccessCount++
				}
				result.TotalTokens += tokens.TotalTokens

				dayKey := timestamp.Format("2006-01-02")
				hourKey := formatHour(timestamp.Hour())
				result.RequestsByDay[dayKey]++
				result.RequestsByHour[hourKey]++
				result.TokensByDay[dayKey] += tokens.TotalTokens
				result.TokensByHour[hourKey] += tokens.TotalTokens
			}

			if filteredModel.TotalRequests > 0 {
				filteredAPI.Models[modelName] = filteredModel
			}
		}

		if filteredAPI.TotalRequests > 0 {
			result.APIs[apiName] = filteredAPI
		}
	}

	return result
}
