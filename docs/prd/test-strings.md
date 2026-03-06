# Test PRD - String Utilities

## Summary
Create string utility functions for common operations.

## Requirements
- Function: `reverse(s string) string` - reverses a string
- Function: `truncate(s string, maxLen int) string` - truncates with ellipsis

## Acceptance Criteria
- Reverse works on empty and non-empty strings
- Truncate handles edge cases (empty, shorter than maxLen)
- Unit tests included

## Technical Notes
- Place in `pkg/strings/utils.go`
- Tests in `pkg/strings/utils_test.go`
