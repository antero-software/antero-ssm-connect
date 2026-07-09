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
	ecs := flag.Bool("ecs", false, "Start interactive bash session on an ECS container")
	cluster := flag.String("cluster", "", "ECS cluster name or ARN (optional, skips cluster selection)")
	help := flag.Bool("help", false, "Show usage information")
	version := flag.Bool("version", false, "Show version")
	dbPortForward := flag.Bool("db-port-forward", false, "Start port-forward to DB proxy via EC2")
	dbeaverSync := flag.Bool("dbeaver", false, "Generate/update DBeaver connections for discovered databases")
	dbeaverPath := flag.String("dbeaver-path", "", "Override path to DBeaver's data-sources.json (optional)")
	flag.Parse()

	if *ssm || *ecs || *dbPortForward || *dbeaverSync || (*profile == "" && *filter != "") {
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
	case *dbeaverSync:
		if err := cmd.SyncDBeaverConfig(*profile, *dbeaverPath); err != nil {
			log.Fatalf("DBeaver sync failed: %v", err)
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
	case *ecs:
		if *profile != "" {
			if err := cmd.StartECSSession(*profile, *cluster); err != nil {
				log.Fatalf("ECS session failed: %v", err)
			}
		} else {
			if err := cmd.StartECSSessionWithPrompt(*cluster); err != nil {
				log.Fatalf("ECS session failed: %v", err)
			}
		}
	default:
		if err := cmd.Interactive(); err != nil {
			panic(err)
		}
	}
}
