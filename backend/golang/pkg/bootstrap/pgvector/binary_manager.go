package pgvector

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
)

// BinaryManager handles downloading and managing pgvector-enabled PostgreSQL binaries.
type BinaryManager struct {
	logger   *log.Logger
	cacheDir string
	version  string
	baseURL  string
}

// BinaryInfo contains metadata about a binary package.
type BinaryInfo struct {
	Platform     string
	Architecture string
	URL          string
	SHA256       string
	Size         int64
}

// NewBinaryManager creates a new pgvector binary manager.
func NewBinaryManager(logger *log.Logger, cacheDir string) *BinaryManager {
	if cacheDir == "" {
		homeDir, _ := os.UserHomeDir()
		cacheDir = filepath.Join(homeDir, ".cache", "enchanted-twin", "pgvector")
	}

	return &BinaryManager{
		logger:   logger,
		cacheDir: cacheDir,
		version:  "16.4-pgvector0.7.4", // PostgreSQL 16.4 with pgvector 0.7.4
		baseURL:  "https://github.com/EternisAI/pgvector-binaries/releases/download",
	}
}

// GetBinariesPath returns the path to pgvector-enabled PostgreSQL binaries
// It follows this priority:
// 1. Check if binaries are bundled with the application
// 2. Check cache for downloaded binaries
// 3. Download binaries if needed
// 4. Fall back to standard embedded-postgres if pgvector binaries unavailable.
func (bm *BinaryManager) GetBinariesPath(ctx context.Context) (string, bool, error) {
	platform := bm.getPlatform()
	arch := bm.getArchitecture()

	bm.logger.Debug("Looking for pgvector binaries",
		"platform", platform,
		"architecture", arch,
		"version", bm.version)

	// 1. Check for bundled binaries (distributed with application)
	bundledPath := bm.getBundledBinariesPath(platform, arch)
	if bm.binariesExist(bundledPath) {
		bm.logger.Info("Using bundled pgvector binaries", "path", bundledPath)
		return bundledPath, true, nil
	}

	// 2. Check cache for downloaded binaries
	cachedPath := bm.getCachedBinariesPath(platform, arch)
	if bm.binariesExist(cachedPath) {
		bm.logger.Info("Using cached pgvector binaries", "path", cachedPath)
		return cachedPath, true, nil
	}

	// 3. Try to download binaries
	downloadedPath, err := bm.downloadBinaries(ctx, platform, arch)
	if err != nil {
		bm.logger.Warn("Failed to download pgvector binaries, falling back to standard PostgreSQL", "error", err)
		return "", false, nil
	}

	bm.logger.Info("Downloaded and cached pgvector binaries", "path", downloadedPath)
	return downloadedPath, true, nil
}

// getBundledBinariesPath returns the path where bundled binaries should be located.
func (bm *BinaryManager) getBundledBinariesPath(platform, arch string) string {
	// Look for binaries bundled with the application
	execDir, err := os.Executable()
	if err != nil {
		return ""
	}
	appDir := filepath.Dir(execDir)
	return filepath.Join(appDir, "binaries", fmt.Sprintf("%s-%s", platform, arch))
}

// getCachedBinariesPath returns the path where cached binaries should be located.
func (bm *BinaryManager) getCachedBinariesPath(platform, arch string) string {
	return filepath.Join(bm.cacheDir, bm.version, fmt.Sprintf("%s-%s", platform, arch))
}

// binariesExist checks if PostgreSQL binaries exist at the given path.
func (bm *BinaryManager) binariesExist(path string) bool {
	if path == "" {
		return false
	}

	// Check for key PostgreSQL executables
	binDir := filepath.Join(path, "bin")
	requiredBins := []string{"postgres", "initdb", "pg_ctl"}

	if runtime.GOOS == "windows" {
		for i, bin := range requiredBins {
			requiredBins[i] = bin + ".exe"
		}
	}

	for _, bin := range requiredBins {
		binPath := filepath.Join(binDir, bin)
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			return false
		}
	}

	// Check for pgvector extension
	extDir := filepath.Join(path, "share", "postgresql", "extension")
	vectorControl := filepath.Join(extDir, "vector.control")
	if _, err := os.Stat(vectorControl); os.IsNotExist(err) {
		return false
	}

	return true
}

// downloadBinaries downloads and extracts pgvector binaries for the current platform.
func (bm *BinaryManager) downloadBinaries(ctx context.Context, platform, arch string) (string, error) {
	binaryInfo := bm.getBinaryInfo(platform, arch)
	if binaryInfo == nil {
		return "", fmt.Errorf("no pgvector binaries available for %s-%s", platform, arch)
	}

	// Create cache directory
	cachePath := bm.getCachedBinariesPath(platform, arch)
	if err := os.MkdirAll(cachePath, 0o755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Download the archive
	archivePath := filepath.Join(bm.cacheDir, fmt.Sprintf("postgresql-%s-%s-%s.tar.gz", bm.version, platform, arch))

	bm.logger.Info("Downloading pgvector binaries",
		"url", binaryInfo.URL,
		"size", binaryInfo.Size,
		"destination", archivePath)

	if err := bm.downloadFile(ctx, binaryInfo.URL, archivePath); err != nil {
		return "", fmt.Errorf("failed to download binaries: %w", err)
	}

	// Verify checksum
	if err := bm.verifyChecksum(archivePath, binaryInfo.SHA256); err != nil {
		if removeErr := os.Remove(archivePath); removeErr != nil {
			bm.logger.Error("Failed to remove archive after checksum verification failure", "error", removeErr)
		}
		return "", fmt.Errorf("checksum verification failed: %w", err)
	}

	// Extract the archive
	if err := bm.extractTarGz(archivePath, cachePath); err != nil {
		if removeErr := os.Remove(archivePath); removeErr != nil {
			bm.logger.Error("Failed to remove archive after extraction failure", "error", removeErr)
		}
		return "", fmt.Errorf("failed to extract binaries: %w", err)
	}

	// Clean up the archive
	if err := os.Remove(archivePath); err != nil {
		bm.logger.Error("Failed to remove archive after successful extraction", "error", err)
	}

	bm.logger.Info("Successfully downloaded and extracted pgvector binaries", "path", cachePath)
	return cachePath, nil
}

// downloadFile downloads a file from URL to the specified path with progress logging.
func (bm *BinaryManager) downloadFile(ctx context.Context, url, filepath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			bm.logger.Error("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			bm.logger.Error("Failed to close output file", "error", err)
		}
	}()

	// Copy with progress logging for large files
	contentLength := resp.ContentLength
	if contentLength > 10*1024*1024 { // > 10MB
		bm.logger.Info("Downloading large file, this may take a moment...", "size_mb", contentLength/(1024*1024))
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

// verifyChecksum verifies the SHA256 checksum of a file.
func (bm *BinaryManager) verifyChecksum(filepath, expectedSHA256 string) error {
	if expectedSHA256 == "" {
		bm.logger.Warn("No checksum provided, skipping verification")
		return nil
	}

	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			bm.logger.Error("Failed to close file", "error", err)
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actualSHA256 := fmt.Sprintf("%x", hash.Sum(nil))
	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actualSHA256)
	}

	return nil
}

// extractTarGz extracts a .tar.gz file to the specified directory.
func (bm *BinaryManager) extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			bm.logger.Error("Failed to close file", "error", err)
		}
	}()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func() {
		if err := gzr.Close(); err != nil {
			bm.logger.Error("Failed to close gzip reader", "error", err)
		}
	}()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(destDir, header.Name)

		// Security check: ensure path is within destDir
		if !strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}

			outFile, err := os.Create(path)
			if err != nil {
				return err
			}

			_, err = io.Copy(outFile, tr)
			if closeErr := outFile.Close(); closeErr != nil {
				bm.logger.Error("Failed to close output file", "error", closeErr)
			}
			if err != nil {
				return err
			}

			if err := os.Chmod(path, os.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}

	return nil
}

// getPlatform returns the current platform identifier.
func (bm *BinaryManager) getPlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}

// getArchitecture returns the current architecture identifier.
func (bm *BinaryManager) getArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "386":
		return "i386"
	default:
		return runtime.GOARCH
	}
}

// getBinaryInfo returns download information for the specified platform and architecture.
func (bm *BinaryManager) getBinaryInfo(platform, arch string) *BinaryInfo {
	// This would typically be loaded from a manifest file or API
	// For now, we'll define the available binaries inline
	binaries := map[string]*BinaryInfo{
		"darwin-amd64": {
			Platform:     "darwin",
			Architecture: "amd64",
			URL:          fmt.Sprintf("%s/v%s/postgresql-%s-darwin-amd64.tar.gz", bm.baseURL, bm.version, bm.version),
			SHA256:       "",               // Would be filled in production
			Size:         45 * 1024 * 1024, // ~45MB
		},
		"darwin-arm64": {
			Platform:     "darwin",
			Architecture: "arm64",
			URL:          fmt.Sprintf("%s/v%s/postgresql-%s-darwin-arm64.tar.gz", bm.baseURL, bm.version, bm.version),
			SHA256:       "",               // Would be filled in production
			Size:         45 * 1024 * 1024, // ~45MB
		},
		"linux-amd64": {
			Platform:     "linux",
			Architecture: "amd64",
			URL:          fmt.Sprintf("%s/v%s/postgresql-%s-linux-amd64.tar.gz", bm.baseURL, bm.version, bm.version),
			SHA256:       "",               // Would be filled in production
			Size:         50 * 1024 * 1024, // ~50MB
		},
		"linux-arm64": {
			Platform:     "linux",
			Architecture: "arm64",
			URL:          fmt.Sprintf("%s/v%s/postgresql-%s-linux-arm64.tar.gz", bm.baseURL, bm.version, bm.version),
			SHA256:       "",               // Would be filled in production
			Size:         50 * 1024 * 1024, // ~50MB
		},
		"windows-amd64": {
			Platform:     "windows",
			Architecture: "amd64",
			URL:          fmt.Sprintf("%s/v%s/postgresql-%s-windows-amd64.tar.gz", bm.baseURL, bm.version, bm.version),
			SHA256:       "",               // Would be filled in production
			Size:         55 * 1024 * 1024, // ~55MB
		},
	}

	key := fmt.Sprintf("%s-%s", platform, arch)
	return binaries[key]
}
