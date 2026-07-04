package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ezerfernandes/repomni/internal/editor"
	"gopkg.in/yaml.v3"
)

// editYAMLConfig serializes seed to YAML, opens it in an external editor with
// header prepended as guidance comments, and parses the edited result back into
// a value of type T. The result is validated by strict decoding, so a syntax
// error or an unknown/misspelled key fails the edit instead of writing a config
// that silently drops the mistyped field.
func editYAMLConfig[T any](seed T, header string) (*T, error) {
	data, err := yaml.Marshal(seed)
	if err != nil {
		return nil, fmt.Errorf("serialize config: %w", err)
	}

	edited, err := editor.EditContentOrAbort(header+string(data), ".yaml")
	if err != nil {
		return nil, fmt.Errorf("edit config: %w", err)
	}

	// KnownFields(true) rejects keys the target struct doesn't define, so a
	// typo like `enable:` (instead of `enabled:`) or a mis-indented key surfaces
	// as an error rather than silently unmarshaling to a zero value. A buffer
	// left with only comments/whitespace decodes as io.EOF; treat that as an
	// abort rather than a corrupt (all-zero) config.
	dec := yaml.NewDecoder(strings.NewReader(edited))
	dec.KnownFields(true)
	var out T
	if err := dec.Decode(&out); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, editor.ErrAborted
		}
		return nil, fmt.Errorf("invalid config after editing: %w", err)
	}
	return &out, nil
}
