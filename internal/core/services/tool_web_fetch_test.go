package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSSRFTarget(t *testing.T) {
	blocked := []string{
		"http://localhost/foo",
		"http://127.0.0.1:8080/api",
		"http://0.0.0.0/path",
		"http://[::1]/bar",
		"http://169.254.169.254/latest/meta-data",
		"http://metadata.google.internal/computeMetadata",
		"ftp://example.com/file", // non-HTTP scheme
		"file:///etc/passwd",
		"http://10.0.0.1/internal",   // private IP
		"http://192.168.1.1/admin",   // private IP
		"http://172.16.0.1/internal", // private IP
	}

	for _, url := range blocked {
		t.Run("blocked_"+url, func(t *testing.T) {
			assert.True(t, isSSRFTarget(url), "should block: %s", url)
		})
	}

	allowed := []string{
		"https://example.com",
		"https://api.github.com/repos",
		"http://www.google.com/search?q=test",
		"https://docs.python.org/3/",
	}

	for _, url := range allowed {
		t.Run("allowed_"+url, func(t *testing.T) {
			assert.False(t, isSSRFTarget(url), "should allow: %s", url)
		})
	}
}

func TestExtractTextFromHTML(t *testing.T) {
	html := `<html><head><style>body{color:red}</style></head>
	<body>
	<script>alert('xss')</script>
	<h1>Hello World</h1>
	<p>This is a <b>test</b> paragraph.</p>
	<nav>Navigation links</nav>
	</body></html>`

	text := extractTextFromHTML(html)

	assert.Contains(t, text, "Hello World")
	assert.Contains(t, text, "test")
	assert.Contains(t, text, "paragraph")
	assert.NotContains(t, text, "alert")
	assert.NotContains(t, text, "color:red")
	assert.NotContains(t, text, "<script>")
	assert.NotContains(t, text, "<h1>")
}
