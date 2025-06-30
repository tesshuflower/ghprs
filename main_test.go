package main

import (
	"testing"

	"ghprs/cmd"
)

func TestMainPackage(t *testing.T) {
	// Test that RootCmd is properly initialized
	if cmd.RootCmd == nil {
		t.Error("RootCmd should not be nil")
	}

	// Test that RootCmd has the expected name
	if cmd.RootCmd.Use != "ghprs" {
		t.Errorf("Expected RootCmd.Use to be 'ghprs', got '%s'", cmd.RootCmd.Use)
	}

	// Test that version command is available
	versionCmd, _, err := cmd.RootCmd.Find([]string{"version"})
	if err != nil {
		t.Errorf("Version command should be available: %v", err)
	}
	if versionCmd == nil {
		t.Error("Version command should not be nil")
	}
}
