package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func shell(script string) []string {
	return []string{"/bin/sh", "-c", script}
}

func TestRunCmd(t *testing.T) {
	t.Run("exit code propagated", func(t *testing.T) {
		require.Equal(t, 7, RunCmd(shell("exit 7"), nil))
	})

	t.Run("env is set and replaced", func(t *testing.T) {
		t.Setenv("FOO", "old")
		env := Environment{"FOO": {Value: "new"}}
		require.Equal(t, 0, RunCmd(shell(`test "$FOO" = new`), env))
	})

	t.Run("env is removed", func(t *testing.T) {
		t.Setenv("UNSET", "should be removed")
		env := Environment{"UNSET": {NeedRemove: true}}
		require.Equal(t, 0, RunCmd(shell(`test -z "${UNSET+x}"`), env))
	})

	t.Run("command not found", func(t *testing.T) {
		require.Equal(t, exitCodeInternal, RunCmd([]string{"/definitely/not/exists"}, nil))
	})

	t.Run("empty command", func(t *testing.T) {
		require.Equal(t, exitCodeInternal, RunCmd(nil, nil))
	})
}

func TestBuildEnv(t *testing.T) {
	base := []string{"KEEP=1", "REPLACE=old", "REMOVE=x"}
	env := Environment{
		"REPLACE": {Value: "new"},
		"REMOVE":  {NeedRemove: true},
		"ADDED":   {Value: ""},
	}
	require.ElementsMatch(t, []string{"KEEP=1", "REPLACE=new", "ADDED="}, buildEnv(base, env))
}
