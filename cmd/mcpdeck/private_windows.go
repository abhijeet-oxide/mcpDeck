//go:build windows

package main

// On Windows the per-user data folder under LOCALAPPDATA is already private.
func privateFiles()         {}
func chmodPrivate(_ string) {}
