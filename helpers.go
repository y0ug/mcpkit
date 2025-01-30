package mcpkit

import (
	"context"
	"fmt"
)

func FetchAll[T any](
	ctx context.Context,
	fetch func(ctx context.Context, cursor *string) ([]T, *string, error),
) ([]T, error) {
	var allItems []T
	var cursor *string

	for {
		items, nextCursor, err := fetch(ctx, cursor)
		if err != nil {
			return nil, fmt.Errorf("fetch failed: %w", err)
		}

		allItems = append(allItems, items...)

		if nextCursor == nil {
			break
		}

		cursor = nextCursor
	}

	return allItems, nil
}
