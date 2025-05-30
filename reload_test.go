package reload

import (
	"net/http"
	"testing"
)

func TestExpectingDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers http.Header
		want    bool
	}{
		{
			name: "websocket",
			headers: http.Header{
				"Connection":     {"upgrade"},
				"Upgrade":        {"websocket"},
				"Accept":         {"*/*"},
				"Sec-Fetch-Dest": {"empty"},
			},
			want: false,
		},
		{
			name: "SSE",
			headers: http.Header{
				"Connection":     {"keep-alive"},
				"Accept":         {"text/event-stream"},
				"Sec-Fetch-Dest": {"empty"},
			},
			want: false,
		},
		{
			name: "regular html request",
			headers: http.Header{
				"Connection":     {"keep-alive"},
				"Accept":         {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
				"Sec-Fetch-Dest": {"document"},
			},
			want: true,
		},
		{
			name: "regular css request",
			headers: http.Header{
				"Connection":     {"keep-alive"},
				"Accept":         {"text/css,*/*;q=0.1"},
				"Sec-Fetch-Dest": {"style"},
			},
			want: false,
		},
		{
			name: "regular js request",
			headers: http.Header{
				"Connection":     {"keep-alive"},
				"Accept":         {"*/*"},
				"Sec-Fetch-Dest": {"script"},
			},
			want: false,
		},
		{
			name: "html without sec-fetch-dest",
			headers: http.Header{
				"Connection": {"keep-alive"},
				"Accept":     {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
			},
			want: true,
		},
		{
			name: "css without sec-fetch-dest",
			headers: http.Header{
				"Connection": {"keep-alive"},
				"Accept":     {"text/css,*/*;q=0.1"},
			},
			want: false,
		},
		{
			name: "js without sec-fetch-dest",
			headers: http.Header{
				"Connection": {"keep-alive"},
				"Accept":     {"*/*"},
			},
			want: true, // yes, we can't distinguish it from "accept all"
		},
		{
			name: "accept all",
			headers: http.Header{
				"Connection": {"keep-alive"},
				"Accept":     {"*/*"},
			},
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := expectingDocument(test.headers); got != test.want {
				t.Errorf("expectingDocument() = %v, want %v", got, test.want)
			}
		})
	}
}
