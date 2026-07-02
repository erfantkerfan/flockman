package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetTestDB(t *testing.T) {
	t.Helper()
	db = nil
	DatabaseFile = filepath.Join(t.TempDir(), "test.sqlite3")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	oldStdout := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}

func Test_serviceAddListRm(t *testing.T) {
	resetTestDB(t)

	token := strings.TrimSpace(captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "add", "nginx"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("service add: %v", err)
		}
	}))
	if !isValidTokenFormat(token) {
		t.Fatalf("service add printed invalid token: %q", token)
	}

	listOutput := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "ls"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("service ls: %v", err)
		}
	})
	if !strings.Contains(listOutput, "nginx") {
		t.Errorf("service ls missing service name, got:\n%s", listOutput)
	}
	if !strings.Contains(listOutput, token) {
		t.Errorf("service ls missing token, got:\n%s", listOutput)
	}

	captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "rm", "nginx"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("service rm: %v", err)
		}
	})

	listAfterRm := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "ls"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("service ls after rm: %v", err)
		}
	})
	if strings.Contains(listAfterRm, "nginx") {
		t.Errorf("service ls still lists removed service:\n%s", listAfterRm)
	}
}

func Test_serviceRm_notFound(t *testing.T) {
	resetTestDB(t)

	rootCmd.SetArgs([]string{"service", "rm", "missing"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("service rm missing: expected error, got nil")
	}
	if !strings.Contains(err.Error(), `service "missing" not found`) {
		t.Errorf("service rm error = %q, want not found message", err.Error())
	}
}
