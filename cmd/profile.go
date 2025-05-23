package cmd

import (
	"fmt"

	"github.com/antero-software/antero-ssm-connect/internal/aws"
	"github.com/antero-software/antero-ssm-connect/internal/ui"
)

func SelectProfileIfEmpty(profile *string) error {
	if *profile != "" {
		return nil
	}

	profiles, err := aws.FetchProfiles()
	if err != nil {
		return fmt.Errorf("failed to load AWS profiles: %w", err)
	}
	if len(profiles) == 0 {
		return fmt.Errorf("no AWS profiles found")
	}

	selected, err := ui.PromptProfile(profiles)
	if err != nil {
		return err
	}

	*profile = selected
	return nil
}
