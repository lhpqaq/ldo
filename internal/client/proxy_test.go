package client

import (
	"runtime"
	"testing"
)

func clearProxyEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"HTTPS_PROXY", "https_proxy",
		"HTTP_PROXY", "http_proxy",
		"NO_PROXY", "no_proxy",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
}

func TestResolveSystemProxy_HTTPSPreferred(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7890")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:8888")

	proxyURL, source, err := resolveSystemProxy("https://linux.do")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if proxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected proxy url: %s", proxyURL)
	}
	if source != "HTTPS_PROXY" {
		t.Fatalf("unexpected source: %s", source)
	}
}

func TestResolveSystemProxy_FallbackToHTTPProxy(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:8888")

	proxyURL, source, err := resolveSystemProxy("https://linux.do")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if proxyURL != "http://127.0.0.1:8888" {
		t.Fatalf("unexpected proxy url: %s", proxyURL)
	}
	if source != "HTTP_PROXY" {
		t.Fatalf("unexpected source: %s", source)
	}
}

func TestResolveSystemProxy_UseLowercaseWhenUppercaseMissing(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("https_proxy", "http://127.0.0.1:7890")

	proxyURL, source, err := resolveSystemProxy("https://linux.do")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if proxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected proxy url: %s", proxyURL)
	}
	if source != "https_proxy" && source != "HTTPS_PROXY" {
		t.Fatalf("unexpected source: %s", source)
	}
}

func TestResolveSystemProxy_UppercaseWinsOverLowercase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows 环境变量名大小写不敏感，无法验证大小写优先级")
	}

	clearProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7890")
	t.Setenv("https_proxy", "http://127.0.0.1:9999")

	proxyURL, source, err := resolveSystemProxy("https://linux.do")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if proxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected proxy url: %s", proxyURL)
	}
	if source != "HTTPS_PROXY" {
		t.Fatalf("unexpected source: %s", source)
	}
}

func TestResolveSystemProxy_TrimWhitespace(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "   http://127.0.0.1:7890   ")

	proxyURL, source, err := resolveSystemProxy("https://linux.do")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if proxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("unexpected proxy url: %s", proxyURL)
	}
	if source != "HTTPS_PROXY" {
		t.Fatalf("unexpected source: %s", source)
	}
}

func TestResolveSystemProxy_EmptyConfig(t *testing.T) {
	clearProxyEnv(t)

	proxyURL, source, err := resolveSystemProxy("https://linux.do")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if proxyURL != "" {
		t.Fatalf("expected empty proxy url, got: %s", proxyURL)
	}
	if source != "" {
		t.Fatalf("expected empty source, got: %s", source)
	}
}

func TestResolveSystemProxy_InvalidProxyURL(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "://bad")

	_, _, err := resolveSystemProxy("https://linux.do")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveSystemProxy_InvalidBaseURL(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7890")

	_, _, err := resolveSystemProxy("linux.do")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
