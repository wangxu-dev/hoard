//go:build !darwin

package app

import "github.com/egoist/mygo"

func restartForUpdate(_ string) error  { mygo.App.Relaunch(); return nil }
func showUpdatedWindow(_ *mygo.Window) {}
