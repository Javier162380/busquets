package busquets

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// PaginationToken holds pagination state (can be extended for cursor-based pagination).
type PaginationToken struct {
	Offset int64 `json:"offset"`
}

// EncodePaginationToken encodes pagination state to base64.
func EncodePaginationToken(offset int64) string {
	token := PaginationToken{Offset: offset}
	data, err := json.Marshal(token)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

// DecodePaginationToken decodes base64 token to pagination state.
func DecodePaginationToken(encoded string) (*PaginationToken, error) {
	if encoded == "" {
		return &PaginationToken{Offset: 0}, nil
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	var token PaginationToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	if token.Offset < 0 {
		token.Offset = 0
	}

	return &token, nil
}
