package work

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type builtinWorkType struct {
	id    string
	files map[string]string
}

var builtinWorkTypes = []builtinWorkType{
	{
		id: "research",
		files: map[string]string{
			"type.yaml": joinLines(
				"schema_version: 1",
				"id: research",
				"description: Research workspace",
				"scaffold: scaffold",
			),
			"scaffold/README.md": joinLines(
				"# Research Work",
				"",
				"Scaffold version: 1",
				"",
				"Use this workspace for one bounded research question. Keep source material, notes, and findings together, then summarize the durable conclusion before closing the work item.",
			),
			"scaffold/RULES.md": joinLines(
				"# Research Rules",
				"",
				"- State the question and scope in `notes.md` before collecting material.",
				"- Put raw source material, excerpts, and captures under `raw/`.",
				"- Put synthesized pages and durable notes under `pages/`.",
				"- Summarize durable conclusions and open questions in `findings.md`.",
			),
			"scaffold/notes.md": joinLines(
				"# Notes",
				"",
				"Question:",
				"",
				"Scope:",
			),
			"scaffold/findings.md": joinLines(
				"# Findings",
				"",
				"## Summary",
				"",
				"## Evidence",
				"",
				"## Open Questions",
			),
			"scaffold/raw/.keep":   "",
			"scaffold/pages/.keep": "",
		},
	},
}

func joinLines(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

func (s *Store) installBuiltinWorkTypes() error {
	for _, preset := range builtinWorkTypes {
		if err := s.installBuiltinWorkType(preset); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) installBuiltinWorkType(preset builtinWorkType) error {
	root := filepath.Join(s.workTypesDir(), preset.id)
	if _, err := os.Stat(root); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(s.workTypesDir(), 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(s.workTypesDir(), "."+preset.id+".*.tmp")
	if err != nil {
		return err
	}
	stageLive := true
	defer func() {
		if stageLive {
			_ = os.RemoveAll(stage)
		}
	}()

	for rel, body := range preset.files {
		clean, err := cleanWorkTypeRelativePath(rel)
		if err != nil {
			return fmt.Errorf("builtin work type %q file %q: %w", preset.id, rel, err)
		}
		path := filepath.Join(stage, clean)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}

	if err := os.Rename(stage, root); err != nil {
		return err
	}
	stageLive = false
	return nil
}
