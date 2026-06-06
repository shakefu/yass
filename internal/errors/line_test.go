package errors

import (
	"testing"
)

func TestFormatPath(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		cwd      string
		want     string
	}{
		{
			name:     "empty file path returns yass",
			filePath: "",
			cwd:      "/home/user",
			want:     "yass",
		},
		{
			name:     "file directly in cwd returns basename",
			filePath: "/home/user/foo.yass.yaml",
			cwd:      "/home/user",
			want:     "foo.yass.yaml",
		},
		{
			name:     "file in subdirectory of cwd returns relative path",
			filePath: "/home/user/sub/dir/foo.yass.yaml",
			cwd:      "/home/user",
			want:     "sub/dir/foo.yass.yaml",
		},
		{
			name:     "file outside cwd returns absolute path",
			filePath: "/other/path/foo.yass.yaml",
			cwd:      "/home/user",
			want:     "/other/path/foo.yass.yaml",
		},
		{
			name:     "file one level up returns absolute path",
			filePath: "/home/foo.yass.yaml",
			cwd:      "/home/user",
			want:     "/home/foo.yass.yaml",
		},
		{
			name:     "nested subdirectory path",
			filePath: "/home/user/a/b/c/d/spec.yass.yaml",
			cwd:      "/home/user",
			want:     "a/b/c/d/spec.yass.yaml",
		},
		{
			name:     "cwd with trailing slash",
			filePath: "/home/user/spec.yass.yaml",
			cwd:      "/home/user/",
			want:     "spec.yass.yaml",
		},
		{
			name:     "same path as cwd is outside (it is cwd itself)",
			filePath: "/home/user",
			cwd:      "/home/user",
			want:     "/home/user",
		},
		{
			name:     "similar prefix but not under cwd",
			filePath: "/home/username/foo.yass.yaml",
			cwd:      "/home/user",
			want:     "/home/username/foo.yass.yaml",
		},
		{
			name:     "root cwd with file in subdirectory",
			filePath: "/foo/bar.yass.yaml",
			cwd:      "/",
			want:     "foo/bar.yass.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPath(tt.filePath, tt.cwd)
			if got != tt.want {
				t.Errorf("FormatPath(%q, %q) = %q, want %q", tt.filePath, tt.cwd, got, tt.want)
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		line    int
		code    string
		message string
		want    string
	}{
		{
			name:    "with line number",
			file:    "foo.yass.yaml",
			line:    42,
			code:    "yass.yaml.malformed",
			message: "YAML well-formedness error",
			want:    "foo.yass.yaml:42: [yass.yaml.malformed] YAML well-formedness error\n",
		},
		{
			name:    "without line number",
			file:    "foo.yass.yaml",
			line:    0,
			code:    "yass.yaml.empty_file",
			message: "empty file",
			want:    "foo.yass.yaml: [yass.yaml.empty_file] empty file\n",
		},
		{
			name:    "line number 1",
			file:    "spec.yass.yaml",
			line:    1,
			code:    "yass.yaml.has_bom",
			message: "file begins with a UTF-8 BOM",
			want:    "spec.yass.yaml:1: [yass.yaml.has_bom] file begins with a UTF-8 BOM\n",
		},
		{
			name:    "message with newline replaced",
			file:    "yass",
			line:    0,
			code:    "yass.internal.uncaught",
			message: "something went\nwrong here",
			want:    "yass: [yass.internal.uncaught] something went wrong here\n",
		},
		{
			name:    "message with carriage return replaced",
			file:    "yass",
			line:    0,
			code:    "yass.internal.uncaught",
			message: "something went\rwrong here",
			want:    "yass: [yass.internal.uncaught] something went wrong here\n",
		},
		{
			name:    "message with multiple newlines",
			file:    "test.yass.yaml",
			line:    5,
			code:    "yass.yaml.malformed",
			message: "line one\nline two\nline three",
			want:    "test.yass.yaml:5: [yass.yaml.malformed] line one line two line three\n",
		},
		{
			name:    "yass as file for no associated file",
			file:    "yass",
			line:    0,
			code:    "yass.argv.no_subcommand",
			message: "no subcommand given",
			want:    "yass: [yass.argv.no_subcommand] no subcommand given\n",
		},
		{
			name:    "relative path with subdirectory",
			file:    "sub/dir/foo.yass.yaml",
			line:    10,
			code:    "yass.spec.no_name",
			message: "spec document missing spec key",
			want:    "sub/dir/foo.yass.yaml:10: [yass.spec.no_name] spec document missing spec key\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatError(tt.file, tt.line, tt.code, tt.message)
			if got != tt.want {
				t.Errorf("FormatError(%q, %d, %q, %q) = %q, want %q",
					tt.file, tt.line, tt.code, tt.message, got, tt.want)
			}
		})
	}
}
