package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestIntegration(t *testing.T) {
	cmd := exec.Command("/bin/bash", "-c", "./test")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal("expected success but script failed")
	}
}
