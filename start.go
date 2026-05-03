package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: go run start.go <PORT>\n")
		os.Exit(1)
	}
	port := os.Args[1]

	cmd := exec.Command("docker", "compose", "up", "--build", "-d")
	cmd.Env = append(os.Environ(), "PORT="+port)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Stock market service available at http://localhost:%s\n", port)
}
