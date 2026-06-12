package extensions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func fingerprintPackageDir(dir string) (string, error) {
	files := []string{}
	if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return "", fmt.Errorf("fingerprint extension package: %w", err)
	}
	sort.Strings(files)

	hash := sha256.New()
	for _, rel := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		info, err := os.Lstat(path)
		if err != nil {
			return "", fmt.Errorf("fingerprint extension file %s: %w", rel, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("fingerprint extension file %s: not a regular file", rel)
		}
		if _, err := fmt.Fprintf(hash, "%s\n%d\n%d\n", rel, info.Mode().Perm()&0o111, info.Size()); err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("open extension file %s: %w", rel, err)
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", fmt.Errorf("read extension file %s: %w", rel, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close extension file %s: %w", rel, closeErr)
		}
		if _, err := hash.Write([]byte{0}); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
