package xsmem

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/xs-memory/xs-memory/internal/storage"
)

// Export writes the store as a tar.gz archive. See design §6.1.
func (s *Store) Export(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Save manifest before export.
	if err := storage.WriteManifest(s.path, s.manifest); err != nil {
		return fmt.Errorf("smem: export write manifest: %w", err)
	}

	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.WalkDir(s.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip the LOCK file.
		if d.Name() == "LOCK" {
			return nil
		}

		rel, err := filepath.Rel(s.path, path)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

// Import restores a store from a tar.gz archive. See design §6.1.
func Import(path string, r io.Reader) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("smem: import mkdir: %w", err)
	}

	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("smem: import gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("smem: import tar: %w", err)
		}

		target := filepath.Join(path, header.Name)

		// Prevent path traversal.
		if !strings.HasPrefix(target, filepath.Clean(path)+string(os.PathSeparator)) && target != filepath.Clean(path) {
			return fmt.Errorf("smem: import: invalid path %q", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}
