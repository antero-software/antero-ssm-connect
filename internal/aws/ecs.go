package aws

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// ECSCluster represents an ECS cluster
type ECSCluster struct {
	ARN  string
	Name string
}

// ECSContainer holds the name and docker runtime ID of a container in a task
type ECSContainer struct {
	Name      string
	RuntimeID string // docker container ID
}

// ECSTask represents a running ECS task with its containers
type ECSTask struct {
	TaskARN              string
	TaskID               string
	ServiceName          string
	Containers           []ECSContainer
	Status               string
	ContainerInstanceARN string // empty for Fargate tasks
}

// FetchECSClusters returns all ECS clusters for the given profile
func FetchECSClusters(profile string) ([]ECSCluster, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("load config failed: %w", err)
	}

	ecsClient := ecs.NewFromConfig(cfg)

	var arns []string
	paginator := ecs.NewListClustersPaginator(ecsClient, &ecs.ListClustersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			if isExpiredToken(err) {
				if err := EnsureSSOLogin(profile); err != nil {
					return nil, fmt.Errorf("SSO login failed: %w", err)
				}
				cfg, _ = config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profile))
				ecsClient = ecs.NewFromConfig(cfg)
				paginator = ecs.NewListClustersPaginator(ecsClient, &ecs.ListClustersInput{})
				continue
			}
			return nil, fmt.Errorf("list clusters failed: %w", err)
		}
		arns = append(arns, page.ClusterArns...)
	}

	if len(arns) == 0 {
		return nil, nil
	}

	desc, err := ecsClient.DescribeClusters(context.TODO(), &ecs.DescribeClustersInput{
		Clusters: arns,
	})
	if err != nil {
		return nil, fmt.Errorf("describe clusters failed: %w", err)
	}

	var result []ECSCluster
	for _, c := range desc.Clusters {
		result = append(result, ECSCluster{
			ARN:  *c.ClusterArn,
			Name: *c.ClusterName,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// FetchECSTasks returns all running tasks in the given cluster
func FetchECSTasks(profile, clusterARN string) ([]ECSTask, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("load config failed: %w", err)
	}

	ecsClient := ecs.NewFromConfig(cfg)

	var taskARNs []string
	paginator := ecs.NewListTasksPaginator(ecsClient, &ecs.ListTasksInput{
		Cluster:       &clusterARN,
		DesiredStatus: "RUNNING",
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("list tasks failed: %w", err)
		}
		taskARNs = append(taskARNs, page.TaskArns...)
	}

	if len(taskARNs) == 0 {
		return nil, nil
	}

	var result []ECSTask
	for i := 0; i < len(taskARNs); i += 100 {
		end := i + 100
		if end > len(taskARNs) {
			end = len(taskARNs)
		}

		desc, err := ecsClient.DescribeTasks(context.TODO(), &ecs.DescribeTasksInput{
			Cluster: &clusterARN,
			Tasks:   taskARNs[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describe tasks failed: %w", err)
		}

		for _, t := range desc.Tasks {
			taskID := path.Base(*t.TaskArn)

			// group looks like "service:<service-name>" or "family:<task-def>"
			serviceName := ""
			if t.Group != nil {
				parts := strings.SplitN(*t.Group, ":", 2)
				if len(parts) == 2 {
					serviceName = parts[1]
				} else {
					serviceName = *t.Group
				}
			}

			var containers []ECSContainer
			for _, c := range t.Containers {
				if c.Name == nil {
					continue
				}
				runtimeID := ""
				if c.RuntimeId != nil {
					runtimeID = *c.RuntimeId
				}
				containers = append(containers, ECSContainer{
					Name:      *c.Name,
					RuntimeID: runtimeID,
				})
			}

			containerInstanceARN := ""
			if t.ContainerInstanceArn != nil {
				containerInstanceARN = *t.ContainerInstanceArn
			}

			status := ""
			if t.LastStatus != nil {
				status = *t.LastStatus
			}

			result = append(result, ECSTask{
				TaskARN:              *t.TaskArn,
				TaskID:               taskID,
				ServiceName:          serviceName,
				Containers:           containers,
				Status:               status,
				ContainerInstanceARN: containerInstanceARN,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ServiceName < result[j].ServiceName
	})
	return result, nil
}

// FetchEC2InstanceFromContainerInstance resolves a container instance ARN to an EC2 instance ID
func FetchEC2InstanceFromContainerInstance(profile, clusterARN, containerInstanceARN string) (string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profile))
	if err != nil {
		return "", fmt.Errorf("load config failed: %w", err)
	}

	ecsClient := ecs.NewFromConfig(cfg)

	desc, err := ecsClient.DescribeContainerInstances(context.TODO(), &ecs.DescribeContainerInstancesInput{
		Cluster:            &clusterARN,
		ContainerInstances: []string{containerInstanceARN},
	})
	if err != nil {
		return "", fmt.Errorf("describe container instances failed: %w", err)
	}
	if len(desc.ContainerInstances) == 0 || desc.ContainerInstances[0].Ec2InstanceId == nil {
		return "", fmt.Errorf("EC2 instance ID not found for container instance")
	}

	return *desc.ContainerInstances[0].Ec2InstanceId, nil
}

func isExpiredToken(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "token expired") ||
		strings.Contains(msg, "InvalidGrantException") ||
		strings.Contains(msg, "failed to read cached SSO token file") ||
		strings.Contains(msg, "failed to refresh cached credentials")
}
