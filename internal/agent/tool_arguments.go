package agent

import (
	"context"
	"encoding/json"
	"strings"
)

// unmarshalNoArguments is only for tools whose input schema has no fields.
// Providers may emit no argument text for these tools instead of an empty JSON
// object. Nonempty input still goes through JSON validation.
func unmarshalNoArguments(_ context.Context, arguments string) (any, error) {
	input := &struct{}{}
	if strings.TrimSpace(arguments) != "" {
		if err := json.Unmarshal([]byte(arguments), &input); err != nil {
			return nil, err
		}
	}
	return input, nil
}
