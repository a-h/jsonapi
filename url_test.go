package jsonapi

import "testing"

func TestJoin(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		paths       []string
		query       map[string]string
		fragment    string
		expected    string
		expectedErr string
	}{
		{
			name:        "no base URL, no path, no query",
			baseURL:     "",
			paths:       nil,
			query:       nil,
			expectedErr: ErrEmptyURL.Error(),
		},
		{
			name:     "no path, no query",
			baseURL:  "http://example.com",
			paths:    nil,
			query:    nil,
			expected: "http://example.com",
		},
		{
			name:     "path, no query",
			baseURL:  "http://example.com",
			paths:    []string{"foo", "bar"},
			query:    nil,
			expected: "http://example.com/foo/bar",
		},
		{
			name:     "path, query",
			baseURL:  "http://example.com",
			paths:    []string{"foo", "bar"},
			query:    map[string]string{"baz": "qux"},
			expected: "http://example.com/foo/bar?baz=qux",
		},
		{
			name:     "query, no path",
			baseURL:  "http://example.com",
			paths:    nil,
			query:    map[string]string{"baz": "qux"},
			expected: "http://example.com?baz=qux",
		},
		{
			name:     "path - escaping test",
			baseURL:  "http://example.com",
			paths:    []string{"foo", "bar baz"},
			query:    nil,
			expected: "http://example.com/foo/bar%20baz",
		},
		{
			name:     "query - escaping test",
			baseURL:  "http://example.com",
			paths:    nil,
			query:    map[string]string{"baz": "qux quux"},
			expected: "http://example.com?baz=qux+quux",
		},
		{
			name:        "invalid base URL",
			baseURL:     "f8967f8",
			paths:       nil,
			query:       nil,
			expectedErr: ErrMissingScheme.Error(),
		},
		{
			name:     "fragment",
			baseURL:  "http://example.com",
			paths:    []string{"foo", "bar"},
			query:    map[string]string{"baz": "qux"},
			fragment: "fragment",
			expected: "http://example.com/foo/bar?baz=qux#fragment",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual, err := URL(test.baseURL).Path(test.paths...).Query(test.query).Fragment(test.fragment).String()
			if (err == nil && test.expectedErr != "") || (err != nil && err.Error() != test.expectedErr) {
				t.Errorf("expected error %v, got %v", test.expectedErr, err)
			}
			if actual != test.expected {
				t.Errorf("expected %s, got %s", test.expected, actual)
			}
		})
	}
}
