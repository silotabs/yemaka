package extensions

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateRunInput checks an extension's input against its manifest without
// launching the extension process. It is used by scheduler/job flows so invalid
// recurring work is stopped before it is enabled or run.
func (s *Store) ValidateRunInput(name string, input map[string]any) error {
	detail, err := s.Show(name)
	if err != nil {
		return err
	}
	if input == nil {
		input = map[string]any{}
	}
	if err := ValidateObjectAgainstSchema("input", detail.Manifest.InputSchema, input); err != nil {
		return fmt.Errorf("extension input for %s is invalid: %w%s", detail.Status.Name, err, inputSampleSuffix(detail.Manifest))
	}
	return nil
}

// ValidateKnownRunInput validates input when the extension currently exists.
// Missing extension names are left to the scheduler runner so review/test
// workflows can still create placeholder jobs for future generated extensions.
func (s *Store) ValidateKnownRunInput(name string, input map[string]any) error {
	if err := s.ValidateRunInput(name, input); err != nil {
		if errors.Is(err, ErrExtensionNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func inputSampleSuffix(manifest Manifest) string {
	sample := strings.TrimSpace(SampleInputJSONForManifest(manifest))
	if sample == "" || sample == "{}" {
		return ""
	}
	return "; provide input like " + sample
}
