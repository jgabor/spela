//go:build !dev && !production && !bindings

package gui

import "fmt"

func Run() error {
	return fmt.Errorf("binary was not built with GUI support; use 'mage build' or 'mage dev'")
}
