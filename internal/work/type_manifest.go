package work

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var workTypePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type workTypeManifest struct {
	SchemaVersion int    `yaml:"schema_version"`
	ID            string `yaml:"id"`
	Description   string `yaml:"description,omitempty"`
	Scaffold      string `yaml:"scaffold,omitempty"`

	root string
}

func (s *Store) readWorkTypeManifest(typeID string) (workTypeManifest, error) {
	id, err := normalizeWorkType(typeID)
	if err != nil {
		return workTypeManifest{}, err
	}

	path := filepath.Join(s.workTypesDir(), id, "type.yaml")
	var manifest workTypeManifest
	if err := readYAMLFile(path, &manifest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return workTypeManifest{}, fmt.Errorf("%w: work type %q", ErrNotFound, id)
		}
		return workTypeManifest{}, fmt.Errorf("read work type manifest %s: %w", path, err)
	}
	manifest.root = filepath.Dir(path)

	if manifest.SchemaVersion != 1 {
		return workTypeManifest{}, fmt.Errorf("work type %q has unsupported schema_version %d", id, manifest.SchemaVersion)
	}
	if strings.TrimSpace(manifest.ID) != id {
		return workTypeManifest{}, fmt.Errorf("work type %q id mismatch: manifest id %q", id, manifest.ID)
	}
	if manifest.Scaffold != "" {
		if _, err := cleanWorkTypeRelativePath(manifest.Scaffold); err != nil {
			return workTypeManifest{}, fmt.Errorf("work type %q scaffold: %w", id, err)
		}
	}

	return manifest, nil
}

func normalizeWorkType(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", nil
	}
	if !workTypePattern.MatchString(id) {
		return "", fmt.Errorf("invalid work type %q: use kebab-case ids", raw)
	}
	return id, nil
}

func cleanWorkTypeRelativePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("path is empty")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("%q must be relative", raw)
	}
	clean := filepath.Clean(trimmed)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%q escapes the work type directory", raw)
	}
	return clean, nil
}

func (m workTypeManifest) scaffoldPath() (string, bool, error) {
	if strings.TrimSpace(m.Scaffold) == "" {
		return "", false, nil
	}
	clean, err := cleanWorkTypeRelativePath(m.Scaffold)
	if err != nil {
		return "", false, err
	}
	return filepath.Join(m.root, clean), true, nil
}

func (s *Store) createWorkItemSpaceLocked(id string, manifest workTypeManifest) (string, error) {
	finalPath := s.workItemSpacePath(id)
	if _, err := os.Stat(finalPath); err == nil {
		return "", fmt.Errorf("%w: %s", ErrAlreadyExists, finalPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if err := os.MkdirAll(s.spacesDir(), 0o755); err != nil {
		return "", err
	}
	stagePath, err := os.MkdirTemp(s.spacesDir(), "."+id+".*.tmp")
	if err != nil {
		return "", err
	}
	stageLive := true
	defer func() {
		if stageLive {
			_ = os.RemoveAll(stagePath)
		}
	}()

	if scaffoldPath, ok, err := manifest.scaffoldPath(); err != nil {
		return "", err
	} else if ok {
		if err := copyDirContents(scaffoldPath, stagePath); err != nil {
			return "", fmt.Errorf("copy scaffold for work item %s: %w", id, err)
		}
	}

	if err := os.Rename(stagePath, finalPath); err != nil {
		return "", err
	}
	stageLive = false
	return finalPath, nil
}

func copyDirContents(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := os.MkdirAll(dstPath, info.Mode().Perm()); err != nil {
				return err
			}
			if err := copyDirContents(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported scaffold entry %s", srcPath)
		}
		if err := copyFile(srcPath, dstPath, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
