package workcli

import (
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/gh-xj/work-cli/internal/work"

	appio "github.com/gh-xj/work-cli/internal/io"
)

func emitJSON(w io.Writer, payload map[string]any) error {
	payload["schema_version"] = "v1"
	return appio.WriteJSON(w, payload)
}

func printRecord(w io.Writer, value any) error {
	_, err := fmt.Fprintln(w, recordLine(value))
	return err
}

func printRecords(w io.Writer, values any, empty string) error {
	v := reflect.ValueOf(values)
	if v.Kind() != reflect.Slice {
		return printRecord(w, values)
	}
	if v.Len() == 0 {
		_, err := fmt.Fprintln(w, empty)
		return err
	}
	for i := 0; i < v.Len(); i++ {
		if _, err := fmt.Fprintln(w, recordLine(v.Index(i).Interface())); err != nil {
			return err
		}
	}
	return nil
}

func recordLine(value any) string {
	id := fieldString(value, "ID", "Id")
	state := fieldString(value, "State", "Status")
	priority := fieldString(value, "Priority")
	title := fieldString(value, "Title", "Name")

	parts := make([]string, 0, 4)
	for _, part := range []string{id, state, priority, title} {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return fmt.Sprint(value)
	}
	return strings.Join(parts, "\t")
}

func fieldString(value any, names ...string) string {
	v := reflect.ValueOf(value)
	for v.IsValid() && v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return ""
	}
	for _, name := range names {
		f := v.FieldByName(name)
		if !f.IsValid() || !f.CanInterface() {
			continue
		}
		switch f.Kind() {
		case reflect.String:
			return f.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return fmt.Sprint(f.Interface())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return fmt.Sprint(f.Interface())
		}
	}
	return ""
}

func workspacePayload(storePath string, item work.WorkItem) map[string]any {
	if strings.TrimSpace(item.Type) == "" || strings.TrimSpace(item.ID) == "" {
		return nil
	}
	return map[string]any{
		"path":       filepath.Join(storePath, "spaces", item.ID),
		"type":       item.Type,
		"scaffolded": true,
	}
}
