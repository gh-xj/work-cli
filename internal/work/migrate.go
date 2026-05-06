package work

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Migrate applies safe, idempotent migrations for older work stores.
func (s *Store) Migrate(input MigrateInput) (MigrationResult, error) {
	result := MigrationResult{DryRun: input.DryRun}
	err := s.withMutationLock(func() error {
		if err := s.ensureInitialized(); err != nil {
			return err
		}
		inbox, err := s.migrateInboxItems(input.DryRun)
		if err != nil {
			return err
		}
		workItems, err := s.migrateWorkItems(input.DryRun)
		if err != nil {
			return err
		}
		result.InboxItems = inbox
		result.WorkItems = workItems
		return nil
	})
	return result, err
}

func (s *Store) migrateInboxItems(dryRun bool) (MigrationRecordResult, error) {
	return migrateRecordDir(s.inboxDir(), inboxIDPattern, func(path string) (bool, error) {
		var item InboxItem
		if err := readYAMLFile(path, &item); err != nil {
			return false, err
		}
		missing := item.SchemaVersion == 0
		if err := normalizeInboxItem(&item); err != nil {
			return false, err
		}
		if !missing {
			return false, nil
		}
		if dryRun {
			return true, nil
		}
		return true, writeYAMLFile(path, item)
	})
}

func (s *Store) migrateWorkItems(dryRun bool) (MigrationRecordResult, error) {
	return migrateRecordDir(s.itemsDir(), workIDPattern, func(path string) (bool, error) {
		var item WorkItem
		if err := readYAMLFile(path, &item); err != nil {
			return false, err
		}
		missing := item.SchemaVersion == 0
		if err := normalizeWorkItem(&item); err != nil {
			return false, err
		}
		if !missing {
			return false, nil
		}
		if dryRun {
			return true, nil
		}
		return true, writeYAMLFile(path, item)
	})
}

func migrateRecordDir(dir string, idPattern interface{ MatchString(string) bool }, migrate func(string) (bool, error)) (MigrationRecordResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return MigrationRecordResult{}, nil
		}
		return MigrationRecordResult{}, err
	}
	result := MigrationRecordResult{}
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" || !idPattern.MatchString(id) {
			continue
		}
		result.Scanned++
		changed, err := migrate(filepath.Join(dir, entry.Name()))
		if err != nil {
			return result, fmt.Errorf("migrate %s: %w", filepath.Join(dir, entry.Name()), err)
		}
		if changed {
			result.Changed++
		}
	}
	return result, nil
}

func normalizeInboxItem(item *InboxItem) error {
	version, err := normalizeRecordSchemaVersion("inbox item", item.SchemaVersion)
	if err != nil {
		return err
	}
	item.SchemaVersion = version
	return nil
}

func normalizeWorkItem(item *WorkItem) error {
	version, err := normalizeRecordSchemaVersion("work item", item.SchemaVersion)
	if err != nil {
		return err
	}
	item.SchemaVersion = version
	return nil
}

func normalizeRecordSchemaVersion(kind string, version int) (int, error) {
	if version == 0 {
		return CurrentRecordSchemaVersion, nil
	}
	if version != CurrentRecordSchemaVersion {
		return 0, fmt.Errorf("%s has unsupported schema_version %d", kind, version)
	}
	return version, nil
}
