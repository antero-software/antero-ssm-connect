package cmd

import (
	"fmt"
	"log"

	"github.com/antero-software/antero-ssm-connect/internal/aws"
	"github.com/antero-software/antero-ssm-connect/internal/tunnel"
	"github.com/antero-software/antero-ssm-connect/internal/ui"
	"github.com/manifoldco/promptui"
)

type Action string

const (
	ActionSSMSession  Action = "🧩 Start a shell session (SSM)"
	ActionECSSession  Action = "🐳 Connect to an ECS container"
	ActionPortForward Action = "🔌 Port-forward to a database"
	ActionList        Action = "📋 List active sessions"
	ActionKillAll     Action = "❌ Kill all sessions"
	ActionExit        Action = "🔚 Exit"
)

// Interactive entrypoint — shows menu
func Interactive() error {
	action, err := promptMainAction()
	if err != nil {
		// graceful cancel on Ctrl+C
		fmt.Println("\n👋 Cancelled.")
		return nil
	}

	switch action {
	case ActionSSMSession:
		return StartSSMSessionWithPrompt()
	case ActionECSSession:
		return StartECSSessionWithPrompt("")
	case ActionPortForward:
		return runPortForward()
	case ActionList:
		ListSessions()
		return nil
	case ActionKillAll:
		KillAllSessions()
		return nil
	case ActionExit:
		fmt.Println("👋 Goodbye!")
		return nil
	default:
		return fmt.Errorf("unknown action")
	}
}

// Menu
func promptMainAction() (Action, error) {
	options := []Action{
		ActionSSMSession,
		ActionECSSession,
		ActionPortForward,
		ActionList,
		ActionKillAll,
		ActionExit,
	}

	prompt := promptui.Select{
		Label: "What would you like to do?",
		Items: options,
	}

	_, result, err := prompt.Run()
	return Action(result), err
}

// Port-forward wizard (moved from old Interactive)
func runPortForward() error {
	profiles, err := aws.FetchProfiles()
	if err != nil {
		return fmt.Errorf("load profiles failed: %w", err)
	}

	profile, err := ui.PromptProfile(profiles)
	if err != nil {
		return fmt.Errorf("profile prompt failed: %w", err)
	}

	if err := aws.EnsureSSOLogin(profile); err != nil {
		return fmt.Errorf("SSO login failed: %w", err)
	}

	instances, err := aws.FetchInstances(profile)
	if err != nil {
		return fmt.Errorf("fetch instances failed: %w", err)
	}

	instance, err := ui.PromptInstance(instances)
	if err != nil {
		return fmt.Errorf("instance prompt failed: %w", err)
	}

	dbs, err := aws.FetchDBs(profile)
	if err != nil {
		return fmt.Errorf("fetch dbs failed: %w", err)
	}

	filtered := ui.FilterDBsByVPC(dbs, instance.VpcID)
	if len(filtered) == 0 {
		fmt.Println("No databases found in the same VPC.")
		return nil
	}

	db, err := ui.PromptDatabase(filtered)
	if err != nil {
		return fmt.Errorf("database prompt failed: %w", err)
	}

	err = tunnel.WriteLastSelection(&tunnel.LastSelection{
		Profile:      profile,
		InstanceName: instance.Name,
		InstanceID:   instance.ID,
		DBEndpoint:   db.Endpoint,
		DBPort:       db.Port,
	})
	if err != nil {
		log.Printf("⚠️ failed to save last selection: %v", err)
	}

	localPort := db.Port

	return tunnel.StartPortForward(profile, instance.Name, instance.ID, db.Endpoint, db.Port, localPort)
}
