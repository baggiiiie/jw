package main

import (
	"fmt"
	"os"

	"jenkins-monitor/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
