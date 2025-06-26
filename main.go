package main

import (
	"flag"
	"log"

	"github.com/antero-software/antero-ssm-connect/cmd"
)

func main() {
	// CLI flags
	profile := flag.String("profile", "", "AWS profile name")
	filter := flag.String("filter", "", "Instance name filter")
	port := flag.Int("port", 0, "Local port to bind (optional)")
	list := flag.Bool("list", false, "List active port-forward sessions")
	kill := flag.Int("kill", 0, "Kill a port-forward session by PID")
	killAll := flag.Bool("kill-all", false, "Kill all active port-forward sessions")
	ssm := flag.Bool("ssm", false, "Start standard SSM shell session to EC2")
	help := flag.Bool("help", false, "Show usage information")
	version := flag.Bool("version", false, "Show version")
	dbPortForward := flag.Bool("db-port-forward", false, "Start port-forward to DB proxy via EC2")
	flag.Parse()

	if *ssm || *dbPortForward || (*profile == "" && *filter != "") {
		if err := cmd.SelectProfileIfEmpty(profile); err != nil {
			log.Fatalf("profile selection failed: %v", err)
		}
	}

	// Command dispatch
	switch {
	case *help:
		cmd.ShowHelper()
	case *version:
		cmd.ShowVersion()
	case *list:
		cmd.ListSessions()
	case *kill != 0:
		cmd.KillSession(*kill)
	case *killAll:
		cmd.KillAllSessions()
	case *dbPortForward:
		if err := cmd.ConnectToDBProxy(*profile, *port); err != nil {
			log.Fatalf("DB port forward failed: %v", err)
		}
	case *profile != "" && *filter != "":
		cmd.QuickConnect(*profile, *filter, *port)
	case *ssm:
		if *profile != "" {
			err := cmd.StartSSMSession(*profile)
			if err != nil {
				log.Fatalf("SSM session failed: %v", err)
			}
		} else {
			if err := cmd.StartSSMSessionWithPrompt(); err != nil {
				log.Fatalf("SSM session failed: %v", err)
			}
		}
	default:
		if err := cmd.Interactive(); err != nil {
			panic(err)
		}
	}
}
