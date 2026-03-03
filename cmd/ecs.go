package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/antero-software/antero-ssm-connect/internal/aws"
	"github.com/antero-software/antero-ssm-connect/internal/ui"
	"github.com/manifoldco/promptui"
)

// StartECSSession lets the user pick a cluster → task → container,
// then SSMs into the underlying EC2 host and runs docker exec.
// If clusterFilter is non-empty the cluster selection prompt is skipped.
func StartECSSession(profile, clusterFilter string) error {
	var selectedCluster aws.ECSCluster

	if clusterFilter != "" {
		selectedCluster = aws.ECSCluster{ARN: clusterFilter, Name: clusterFilter}
	} else {
		clusters, err := aws.FetchECSClusters(profile)
		if err != nil {
			return fmt.Errorf("fetch clusters failed: %w", err)
		}
		if len(clusters) == 0 {
			return fmt.Errorf("no ECS clusters found for profile %s", profile)
		}

		clusterOptions := make([]string, len(clusters))
		for i, c := range clusters {
			clusterOptions[i] = c.Name
		}

		clusterPrompt := promptui.Select{
			Label: fmt.Sprintf("Select ECS cluster (profile: %s)", profile),
			Items: clusterOptions,
			Searcher: func(input string, index int) bool {
				return strings.Contains(strings.ToLower(clusterOptions[index]), strings.ToLower(input))
			},
		}

		clusterIdx, _, err := clusterPrompt.Run()
		if err != nil {
			return fmt.Errorf("cluster selection cancelled: %w", err)
		}

		selectedCluster = clusters[clusterIdx]
	}

	fmt.Printf("\n⏳ Fetching running tasks in cluster: %s\n", selectedCluster.Name)

	tasks, err := aws.FetchECSTasks(profile, selectedCluster.ARN)
	if err != nil {
		return fmt.Errorf("fetch tasks failed: %w", err)
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no running tasks found in cluster %s", selectedCluster.Name)
	}

	// Pick task
	taskOptions := make([]string, len(tasks))
	for i, t := range tasks {
		containerNames := make([]string, len(t.Containers))
		for j, c := range t.Containers {
			containerNames[j] = c.Name
		}
		taskOptions[i] = fmt.Sprintf("%-40s  task: %s  [%s]",
			t.ServiceName, shortID(t.TaskID), strings.Join(containerNames, ", "))
	}

	taskPrompt := promptui.Select{
		Label: "Select ECS task",
		Items: taskOptions,
		Size:  20,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(taskOptions[index]), strings.ToLower(input))
		},
	}

	taskIdx, _, err := taskPrompt.Run()
	if err != nil {
		return fmt.Errorf("task selection cancelled: %w", err)
	}

	selectedTask := tasks[taskIdx]

	if selectedTask.ContainerInstanceARN == "" {
		return fmt.Errorf("this task runs on Fargate — direct docker exec is not supported (no underlying EC2 host)")
	}

	// Pick container (skip prompt if only one)
	selectedContainer := selectedTask.Containers[0]
	if len(selectedTask.Containers) > 1 {
		containerNames := make([]string, len(selectedTask.Containers))
		for i, c := range selectedTask.Containers {
			containerNames[i] = c.Name
		}
		containerPrompt := promptui.Select{
			Label: "Select container",
			Items: containerNames,
		}
		containerIdx, _, err := containerPrompt.Run()
		if err != nil {
			return fmt.Errorf("container selection cancelled: %w", err)
		}
		selectedContainer = selectedTask.Containers[containerIdx]
	}

	if selectedContainer.RuntimeID == "" {
		return fmt.Errorf("container '%s' has no runtime ID — it may not be fully started yet", selectedContainer.Name)
	}

	fmt.Printf("\n⏳ Resolving EC2 host for task %s...\n", shortID(selectedTask.TaskID))

	ec2InstanceID, err := aws.FetchEC2InstanceFromContainerInstance(profile, selectedCluster.ARN, selectedTask.ContainerInstanceARN)
	if err != nil {
		return fmt.Errorf("resolve EC2 instance failed: %w", err)
	}

	fmt.Printf("✅ Connecting to container '%s' via %s\n\n", selectedContainer.Name, ec2InstanceID)

	awsPath, err := exec.LookPath("aws")
	if err != nil {
		return fmt.Errorf("aws CLI not found in PATH: %w", err)
	}

	dockerCmd := fmt.Sprintf("docker exec -it %s /bin/bash", selectedContainer.RuntimeID)

	return syscall.Exec(awsPath, []string{
		"aws", "ssm", "start-session",
		"--profile", profile,
		"--target", ec2InstanceID,
		"--document-name", "AWS-StartInteractiveCommand",
		"--parameters", fmt.Sprintf(`{"command":["%s"]}`, dockerCmd),
	}, os.Environ())
}

func StartECSSessionWithPrompt(clusterFilter string) error {
	profiles, err := aws.FetchProfiles()
	if err != nil {
		return err
	}

	profile, err := ui.PromptProfile(profiles)
	if err != nil {
		return err
	}

	return StartECSSession(profile, clusterFilter)
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
