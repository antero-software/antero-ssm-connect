package cmd

import "fmt"

var Version = "dev"

func ShowVersion() {
	fmt.Println("antero-ssm-connect version:", Version)
}
