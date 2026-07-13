package cmd

import (
	"fmt"

	"github.com/antero-software/antero-ssm-connect/internal/aws"
	"github.com/antero-software/antero-ssm-connect/internal/dbeaver"
)

// SyncDBeaverConfig discovers databases and their SSM-managed EC2 instances
// for the given AWS profile, then writes matching connections + network
// profiles into DBeaver's own data-sources.json — using DBeaver's native AWS
// SSM tunnel handler, so no separate CLI tunnel process is needed and every
// team member ends up with the same connections.
func SyncDBeaverConfig(profile, pathOverride string) error {
	if err := aws.EnsureSSOLogin(profile); err != nil {
		return fmt.Errorf("SSO login failed: %w", err)
	}

	instances, err := aws.FetchInstances(profile)
	if err != nil {
		return fmt.Errorf("failed to fetch EC2 instances: %w", err)
	}

	dbs, err := aws.FetchDBs(profile)
	if err != nil {
		return fmt.Errorf("failed to fetch databases: %w", err)
	}
	if len(dbs) == 0 {
		fmt.Println("No databases found for this profile.")
		return nil
	}

	region, err := aws.ResolveRegion(profile)
	if err != nil {
		return fmt.Errorf("failed to resolve AWS region: %w", err)
	}

	path, err := dbeaver.ResolveConfigPath(pathOverride)
	if err != nil {
		return fmt.Errorf("failed to resolve DBeaver config path: %w", err)
	}

	profiles, conns, skipped := dbeaver.BuildPlan(profile, instances, dbs, region)
	if len(conns) == 0 {
		fmt.Println("No databases could be matched to an SSM-managed EC2 instance with a supported engine (PostgreSQL, MySQL/MariaDB, Redis).")
		return nil
	}

	connWritten, profilesWritten, err := dbeaver.Sync(path, fmt.Sprintf("Antero SSM (%s)", profile), profiles, conns)
	if err != nil {
		return fmt.Errorf("failed to update DBeaver config: %w", err)
	}

	fmt.Printf("\n✅ Synced %d connection(s) across %d network profile(s) into DBeaver:\n📄 %s\n\n", connWritten, profilesWritten, path)
	for _, c := range conns {
		fmt.Printf("   🛢️  %s → %s:%s (via SSM)\n", c.Name, c.Host, c.Port)
	}
	if len(skipped) > 0 {
		fmt.Printf("\n⚠️  Skipped %d database(s):\n", len(skipped))
		for _, s := range skipped {
			fmt.Printf("   • %s\n", s)
		}
	}

	fmt.Println("\nℹ️  Restart DBeaver (or refresh the Database Navigator) to see the new connections.")
	fmt.Println("ℹ️  Each network profile needs AWS credentials chosen once in DBeaver (Network configurations → AWS SSM → Credentials) — instance ID/region are already filled in.")
	return nil
}
