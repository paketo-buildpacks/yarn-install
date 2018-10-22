package yarn

import (
	"os"
	"os/exec"
)

type Yarn struct{}

func (n *Yarn) Install(dir string) error {
	return n.runCommand(dir, "install")
}

func (n *Yarn) Rebuild(dir string) error {
	return n.runCommand(dir, "rebuild")
}

func (n *Yarn) runCommand(dir string, args ...string) error {
	cmd := exec.Command("yarn", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
