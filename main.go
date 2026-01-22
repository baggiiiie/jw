package main

import (
	"fmt"
	"os"

	_ "jenkins-monitor/cmd"
	"jenkins-monitor/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
