package cmd

import (
	"syscall"

	"github.com/nlink-jp/lagent/internal/sandbox"
)

// currentProject is the project directory every command resolves
// (ADR-0035): the working directory as the kernel reports it, then
// symlinks resolved. os.Getwd would return $PWD, whose spelling is
// whatever the shell was given — the same directory reached in another
// Unicode form was a different project to every record keyed by it.
func currentProject() (string, error) {
	cwd, err := syscall.Getwd()
	if err != nil {
		return "", err
	}
	return sandbox.ResolveWriteDir(cwd)
}
