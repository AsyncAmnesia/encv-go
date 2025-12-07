//go:build !windows
// +build !windows

package main

import "github.com/spf13/cobra"

func addPlatformSpecificCommands_register(rootCmd *cobra.Command) {
	// 空实现
}
