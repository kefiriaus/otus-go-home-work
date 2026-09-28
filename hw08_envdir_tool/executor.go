package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const exitCodeInternal = 111

// RunCmd runs a command + arguments (cmd) with environment variables from env.
func RunCmd(cmd []string, env Environment) (returnCode int) {
	if len(cmd) == 0 {
		fmt.Fprintln(os.Stderr, "envdir: no command to run")
		return exitCodeInternal
	}

	command := exec.CommandContext(context.Background(), cmd[0], cmd[1:]...) //nolint:gosec
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = buildEnv(os.Environ(), env)

	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "envdir:", err)
		return exitCodeInternal
	}

	return 0
}

func buildEnv(base []string, env Environment) []string {
	result := make([]string, 0, len(base)+len(env))
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		if _, ok := env[name]; ok {
			continue
		}
		result = append(result, kv)
	}

	for name, v := range env {
		if v.NeedRemove {
			continue
		}
		result = append(result, name+"="+v.Value)
	}

	return result
}
