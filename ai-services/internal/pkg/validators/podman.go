package validators

import (
	"fmt"
	"os/exec"
)

func Podman() (string, error) {
	path, err := exec.LookPath("podman")
	if err != nil {
		return "", fmt.Errorf("podman is not installed or not found in PATH, error: %v", err)
	}
	return path, nil
}
