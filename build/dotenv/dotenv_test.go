package dotenv

import (
	"reflect"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "basic",
			input: "FOO=bar\nBAZ=qux\n",
			want: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
		{
			name: "whitespace_and_comments",
			input: `
# comment
	FOO=bar   # trailing comment
BAZ = qux  # another
QUUX=abc#not a comment
`,
			want: map[string]string{
				"FOO":  "bar",
				"BAZ":  "qux",
				"QUUX": "abc#not a comment",
			},
		},
		{
			name:  "export_prefix",
			input: "export KEY=value\nX=1  # trailing comment\n",
			want: map[string]string{
				"KEY": "value",
				"X":   "1",
			},
		},
		{
			name:  "export_whitespace",
			input: "export\tKEY=value\nexport    X=1\nexportY=should_not_match\n",
			want: map[string]string{
				"KEY":     "value",
				"X":       "1",
				"exportY": "should_not_match",
			},
		},
		{
			name: "double_and_single_quotes",
			input: `
A="quoted # not comment"
B='single quoted # not comment'
C="line1\nline2\tTabbed\\Backslash\"Quote"
D='escapes \n are literal \\ and # not comment'
`,
			want: map[string]string{
				"A": "quoted # not comment",
				"B": "single quoted # not comment",
				"C": "line1\nline2\tTabbed\\Backslash\"Quote",
				"D": "escapes \\n are literal \\\\ and # not comment",
			},
		},
		{
			name:  "bom",
			input: "\uFEFFFOO=bar\n",
			want: map[string]string{
				"FOO": "bar",
			},
		},
		{
			name:    "unterminated_double_quote",
			input:   `FOO="bar`,
			wantErr: true,
		},
		{
			name:    "unterminated_single_quote",
			input:   "FOO='bar",
			wantErr: true,
		},
		{
			name:  "unknown_escape_is_preserved",
			input: `FOO="bar\qbaz"`,
			want: map[string]string{
				"FOO": `bar\qbaz`,
			},
		},
		{
			name: "malformed_empty_key_is_skipped",
			input: `
=value
OK=1
`,
			want: map[string]string{
				"OK": "1",
			},
		},
		{
			name: "ignore_after_closing_quote",
			input: `
A="val" # comment
B='val'    # comment
`,
			want: map[string]string{
				"A": "val",
				"B": "val",
			},
		},
		{
			name: "multiline_double_quoted_physical",
			input: `
MULTI="line1
line2
line3"
`,
			want: map[string]string{
				"MULTI": "line1\nline2\nline3",
			},
		},
		{
			name: "multiline_single_quoted_physical",
			input: `
S='line1
line2\q'
`,
			want: map[string]string{
				"S": "line1\nline2\\q",
			},
		},
		{
			name: "multiline_with_trailing_comment",
			input: `
A="v1
v2"   # trailing comment
`,
			want: map[string]string{
				"A": "v1\nv2",
			},
		},
		{
			name: "pem_like_certificate",
			input: `
TLS_CERT="-----BEGIN CERTIFICATE-----
ABC
DEF
-----END CERTIFICATE-----"
`,
			want: map[string]string{
				"TLS_CERT": "-----BEGIN CERTIFICATE-----\nABC\nDEF\n-----END CERTIFICATE-----",
			},
		},
		{
			name: "unterminated_multiline_at_eof",
			input: `
X="line1
line2
`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none (result: %#v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("mismatch:\n  got:  %#v\n  want: %#v", got, tc.want)
			}
		})
	}
}
