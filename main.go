package main

import (
	"fmt"
	"os"

	"ghprs/cmd"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("ghprs v1.0.0")
	},
}

var helloCmd = &cobra.Command{
	Use:   "hello [name]",
	Short: "Say hello to someone",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := "World"
		if len(args) > 0 {
			name = args[0]
		}
		fmt.Printf("Hello, %s!\n", name)
	},
}

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cmd.RootCmd.AddCommand(versionCmd)
	cmd.RootCmd.AddCommand(helloCmd)
}
