package updater

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Updater orchestrates version checking and binary updates.
type Updater struct {
	Client     *GitHubClient
	CurrentVer Version
	GOOS       string
	GOARCH     string
}

// CheckResult contains the result of a version check.
type CheckResult struct {
	Current     Version
	Latest      Version
	UpdateAvail bool
}

// Check verifies if a new version is available.
func (u *Updater) Check() (CheckResult, error) {
	latest, err := u.Client.FetchLatestVersion()
	if err != nil {
		return CheckResult{}, fmt.Errorf("checking version: %w", err)
	}

	return CheckResult{
		Current:     u.CurrentVer,
		Latest:      latest,
		UpdateAvail: latest.NewerThan(u.CurrentVer),
	}, nil
}

// Update downloads and installs the latest version.
func (u *Updater) Update() (CheckResult, error) {
	result, err := u.Check()
	if err != nil {
		return CheckResult{}, err
	}
	if !result.UpdateAvail {
		return result, nil
	}

	if err := u.performUpdate(result.Latest); err != nil {
		return CheckResult{}, err
	}

	return result, nil
}

// performUpdate executes the atomic binary replacement.
func (u *Updater) performUpdate(ver Version) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("determining binary path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	dir := filepath.Dir(execPath)

	// Download archive to a temp file in the same directory for atomic rename.
	archiveFile, err := os.CreateTemp(dir, "envctl-archive-*.tar.gz")
	if err != nil {
		return fmt.Errorf("creating temp archive file: %w", err)
	}
	archivePath := archiveFile.Name()
	defer func() { _ = os.Remove(archivePath) }()

	body, err := u.Client.DownloadArchive(ver, u.GOOS, u.GOARCH)
	if err != nil {
		_ = archiveFile.Close()
		return fmt.Errorf("downloading: %w", err)
	}

	if _, err := io.Copy(archiveFile, body); err != nil {
		_ = archiveFile.Close()
		_ = body.Close()
		return fmt.Errorf("writing archive: %w", err)
	}
	_ = body.Close()
	_ = archiveFile.Close()

	// Verify checksum.
	if err := u.verifyChecksum(ver, archivePath); err != nil {
		return err
	}

	// Extract binary to a temp file in the same directory.
	tempBin, err := os.CreateTemp(dir, "envctl-bin-*")
	if err != nil {
		return fmt.Errorf("creating temp binary file: %w", err)
	}
	tempBinPath := tempBin.Name()
	_ = tempBin.Close()
	defer func() { _ = os.Remove(tempBinPath) }()

	if err := extractBinary(archivePath, tempBinPath); err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}

	// Atomic replacement.
	if err := os.Rename(tempBinPath, execPath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}
	if err := os.Chmod(execPath, 0o755); err != nil {
		return fmt.Errorf("chmod binary: %w", err)
	}

	return nil
}

// verifyChecksum verifies the SHA256 checksum of the archive.
func (u *Updater) verifyChecksum(ver Version, archivePath string) error {
	checksums, err := u.Client.FetchChecksums(ver)
	if err != nil {
		return fmt.Errorf("fetching checksums: %w", err)
	}

	archiveName := ArchiveFilename(u.GOOS, u.GOARCH)

	expectedHash, ok := checksums[archiveName]
	if !ok {
		return fmt.Errorf("checksum not found for %s", archiveName)
	}

	f, err := os.Open(archivePath) //nolint:gosec // archivePath is a temp file we just created
	if err != nil {
		return fmt.Errorf("opening archive for checksum: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("computing checksum: %w", err)
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// extractBinary extracts the "envctl" file from a tar.gz archive.
func extractBinary(archivePath, destPath string) error {
	f, err := os.Open(archivePath) //nolint:gosec // archivePath is a temp file we just created
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("opening gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == binaryName {
			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec // destPath is controlled temp directory
			if err != nil {
				return err
			}
			// Limit copy to prevent zip bombs.
			if _, err := io.Copy(out, io.LimitReader(tr, 200*1024*1024)); err != nil { //nolint:gosec // bounded by LimitReader
				_ = out.Close()
				return err
			}
			return out.Close()
		}
	}

	return fmt.Errorf("binary %q not found in archive", binaryName)
}
