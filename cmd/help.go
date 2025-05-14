package cmd

import "fmt"

func ShowHelper() {
	fmt.Println(`
AWS SSM Tunnel CLI

Usage:
  antero-ssm-connect                                          # Interactive mode (prompts)
  antero-ssm-connect --profile <profile> --filter <keyword>   # Quick connect to database
  antero-ssm-connect --ssm --profile <profile>                # Start SSM shell session to EC2
  antero-ssm-connect --db-port-forward --profile <profile>    # Port-forward to a selected DB proxy via EC2
  antero-ssm-connect --list                                   # List active port-forward sessions
  antero-ssm-connect --kill <pid>                             # Kill a specific port-forward session by PID
  antero-ssm-connect --kill-all                               # Kill all active port-forward sessions
  antero-ssm-connect --help                                   # Show this helper message
  antero-ssm-connect --version                                # Show version

Flags:
--profile            AWS profile to use (e.g., dev, prod)
--filter             Filter for EC2 instance name (for DB tunneling)
--port               Local port override (optional)
--ssm                Start standard SSM shell session to EC2 instance
--db-port-forward    Port-forward to a selected RDS proxy via EC2
--list               Show active port-forward sessions
--kill               Kill a session by PID
--kill-all           Kill all active sessions
--version            Show version info
--help               Show this help message

Examples:
antero-ssm-connect --profile dev --filter prod-db
antero-ssm-connect --ssm --profile dev
antero-ssm-connect --db-port-forward --profile dev
antero-ssm-connect --kill 12345
antero-ssm-connect --list
`)
}
