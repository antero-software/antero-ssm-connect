package dbeaver

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/antero-software/antero-ssm-connect/internal/aws"
)

// Every network profile and connection this tool writes is tagged with one of
// these prefixes, so re-syncing can find and replace its own entries without
// touching anything the user configured by hand in DBeaver.
const (
	connectionKeyPrefix = "antero-"
	profileKeyPrefix    = "antero-np-"
)

// NetworkProfile is a reusable AWS SSM tunnel definition (one per SSM-managed
// EC2 "bastion" instance), matching DBeaver's own "network-profiles" concept —
// many connections in the same VPC share the same profile.
type NetworkProfile struct {
	Key        string
	Name       string
	InstanceID string
	Region     string
}

// Connection is a single DBeaver data source, tunneled through a NetworkProfile.
type Connection struct {
	Key            string
	Name           string
	Provider       string
	Driver         string
	Host           string
	Port           string
	Database       string
	URL            string
	AuthModel      string
	NetworkProfile string
}

type engineTarget struct {
	provider  string
	driver    string
	authModel string
	database  string
	// buildURL is empty for providers (e.g. Redis) that don't use a JDBC URL.
	buildURL func(host, port, database string) string
}

// targetFor maps an engine (as returned by aws.DetectEngineByPort) to the exact
// provider/driver ids DBeaver Enterprise/Ultimate uses. These were confirmed
// against a real data-sources.json rather than guessed, since a wrong driver
// id produces a connection DBeaver can't open.
func targetFor(engine string) (engineTarget, bool) {
	switch engine {
	case "PostgreSQL":
		return engineTarget{
			provider:  "postgresql",
			driver:    "postgres-jdbc",
			authModel: "native",
			database:  "postgres",
			buildURL: func(host, port, database string) string {
				return fmt.Sprintf("jdbc:postgresql://%s:%s/%s", host, port, database)
			},
		}, true
	case "MySQL":
		return engineTarget{
			provider:  "mysql-ee",
			driver:    "mysql8",
			authModel: "native",
			database:  "",
			buildURL: func(host, port, database string) string {
				return fmt.Sprintf("jdbc:mysql://%s:%s/%s", host, port, database)
			},
		}, true
	case "Redis":
		return engineTarget{
			provider:  "redis",
			driver:    "redis_jedis",
			authModel: "native",
			database:  "0",
		}, true
	default:
		// SQL Server, Oracle, MongoDB, Memcached: no confirmed driver id yet.
		return engineTarget{}, false
	}
}

// BuildPlan turns discovered EC2 instances and databases into the network
// profiles + connections to sync into DBeaver. Each database is tunneled
// through the SSM-managed instance in its own VPC; databases in a VPC with no
// such instance, or whose engine has no confirmed DBeaver driver, are skipped.
func BuildPlan(profile string, instances []aws.Instance, dbs []aws.DB, region string) (profiles []NetworkProfile, conns []Connection, skipped []string) {
	candidatesByVPC := map[string][]aws.Instance{}
	for _, inst := range instances {
		if inst.VpcID == "" {
			continue
		}
		candidatesByVPC[inst.VpcID] = append(candidatesByVPC[inst.VpcID], inst)
	}

	var vpcOrder []string
	for vpc := range candidatesByVPC {
		vpcOrder = append(vpcOrder, vpc)
	}
	sort.Strings(vpcOrder)

	profileByVPC := map[string]NetworkProfile{}
	for _, vpc := range vpcOrder {
		inst := pickBastionInstance(candidatesByVPC[vpc])
		np := NetworkProfile{
			Key:        networkProfileKey(profile, inst.ID),
			Name:       networkProfileName(profile, inst),
			InstanceID: inst.ID,
			Region:     region,
		}
		profileByVPC[vpc] = np
	}

	usedProfiles := map[string]bool{}
	for _, db := range dbs {
		engine := aws.DetectEngineByPort(db.Port)
		target, ok := targetFor(engine)
		if !ok {
			skipped = append(skipped, fmt.Sprintf("%s (%s): no confirmed DBeaver driver for this engine", db.Endpoint, engine))
			continue
		}

		np, ok := profileByVPC[db.VpcID]
		if !ok {
			skipped = append(skipped, fmt.Sprintf("%s (%s): no SSM-managed EC2 instance found in its VPC", db.Endpoint, engine))
			continue
		}

		database := target.database
		var url string
		if target.buildURL != nil {
			url = target.buildURL(db.Endpoint, db.Port, database)
		}

		conns = append(conns, Connection{
			Key:            connectionKey(profile, db.Endpoint, db.Port),
			Name:           connectionName(profile, engine, db),
			Provider:       target.provider,
			Driver:         target.driver,
			Host:           db.Endpoint,
			Port:           db.Port,
			Database:       database,
			URL:            url,
			AuthModel:      target.authModel,
			NetworkProfile: np.Key,
		})
		usedProfiles[np.Key] = true
	}

	for _, vpc := range vpcOrder {
		np := profileByVPC[vpc]
		if usedProfiles[np.Key] {
			profiles = append(profiles, np)
		}
	}

	sort.Slice(conns, func(i, j int) bool { return conns[i].Name < conns[j].Name })
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, conns, skipped
}

// pickBastionInstance prefers a dedicated NAT/bastion instance over other EC2
// instances in the same VPC (e.g. ECS container instances), since those churn
// during instance refreshes/deployments and would silently invalidate the
// SSM instance ID baked into a network profile.
func pickBastionInstance(candidates []aws.Instance) aws.Instance {
	best := candidates[0]
	bestScore := bastionScore(best.Name)
	for _, c := range candidates[1:] {
		if score := bastionScore(c.Name); score < bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

func bastionScore(name string) int {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "nat"), strings.Contains(lower, "bastion"):
		return 0
	default:
		return 1
	}
}

func connectionName(profile, engine string, db aws.DB) string {
	return fmt.Sprintf("%s · %s %s · %s", profile, engine, db.Role, shortHost(db.Endpoint))
}

func networkProfileName(profile string, inst aws.Instance) string {
	label := inst.Name
	if label == "" {
		label = inst.ID
	}
	return fmt.Sprintf("antero-%s-%s", profile, label)
}

func shortHost(endpoint string) string {
	if idx := strings.Index(endpoint, "."); idx > 0 {
		return endpoint[:idx]
	}
	return endpoint
}

func connectionKey(profile, endpoint, port string) string {
	sum := sha1.Sum([]byte(profile + "|" + endpoint + "|" + port))
	return connectionKeyPrefix + hex.EncodeToString(sum[:])[:12]
}

func networkProfileKey(profile, instanceID string) string {
	sum := sha1.Sum([]byte(profile + "|" + instanceID))
	return profileKeyPrefix + hex.EncodeToString(sum[:])[:12]
}

func (np NetworkProfile) toJSON() map[string]interface{} {
	return map[string]interface{}{
		"name": np.Name,
		"handlers": map[string]interface{}{
			"aws_ssm": map[string]interface{}{
				"type":          "TUNNEL",
				"enabled":       true,
				"save-password": true,
				"properties": map[string]interface{}{
					"ssm.instance.id":     np.InstanceID,
					"ssm.instance.region": np.Region,
					"ssm.document":        "",
				},
			},
		},
	}
}

func (c Connection) toJSON(folder string) map[string]interface{} {
	cfg := map[string]interface{}{
		"host":     c.Host,
		"port":     c.Port,
		"database": c.Database,
		"type":     "dev",
	}
	if c.URL != "" {
		cfg["url"] = c.URL
	}
	if c.AuthModel != "" {
		cfg["auth-model"] = c.AuthModel
	}
	if c.NetworkProfile != "" {
		cfg["config-profile"] = c.NetworkProfile
		cfg["handlers"] = map[string]interface{}{
			"aws_ssm": map[string]interface{}{
				"type":    "TUNNEL",
				"enabled": true,
			},
		}
	}
	return map[string]interface{}{
		"provider":      c.Provider,
		"driver":        c.Driver,
		"name":          c.Name,
		"folder":        folder,
		"save-password": false,
		"configuration": cfg,
	}
}

// ResolveConfigPath finds DBeaver's data-sources.json for the "General" project.
// It prefers a workspace that already exists on disk, falling back to the
// current default install layout if DBeaver has never been run.
func ResolveConfigPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home directory: %w", err)
	}

	var roots []string
	switch runtime.GOOS {
	case "darwin":
		roots = []string{
			filepath.Join(home, "Library", "DBeaverData"),
			filepath.Join(home, ".dbeaver4"),
			filepath.Join(home, ".dbeaver"),
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		roots = []string{filepath.Join(appData, "DBeaverData")}
	default:
		xdg := os.Getenv("XDG_DATA_HOME")
		if xdg == "" {
			xdg = filepath.Join(home, ".local", "share")
		}
		roots = []string{
			filepath.Join(xdg, "DBeaverData"),
			filepath.Join(home, ".dbeaver4"),
			filepath.Join(home, ".dbeaver"),
		}
	}

	for _, root := range roots {
		candidate := filepath.Join(root, "workspace6", "General", ".dbeaver", "data-sources.json")
		if _, err := os.Stat(filepath.Dir(candidate)); err == nil {
			return candidate, nil
		}
	}

	return filepath.Join(roots[0], "workspace6", "General", ".dbeaver", "data-sources.json"), nil
}

// Sync merges the given network profiles and connections into DBeaver's
// data-sources.json, replacing any entries previously written by this tool
// and leaving everything else (the user's own connections, profiles,
// folders, credentials) untouched.
func Sync(path, folderName string, profiles []NetworkProfile, conns []Connection) (connectionsWritten, profilesWritten int, err error) {
	root := map[string]json.RawMessage{}

	data, readErr := os.ReadFile(path)
	switch {
	case readErr == nil:
		if uerr := json.Unmarshal(data, &root); uerr != nil {
			return 0, 0, fmt.Errorf("existing DBeaver config at %s is not valid JSON: %w", path, uerr)
		}
	case !os.IsNotExist(readErr):
		return 0, 0, readErr
	}

	connections, err := mergeSection(root, "connections", connectionKeyPrefix)
	if err != nil {
		return 0, 0, fmt.Errorf("could not parse existing connections in %s: %w", path, err)
	}
	for _, c := range conns {
		connections[c.Key] = c.toJSON(folderName)
	}

	networkProfiles, err := mergeSection(root, "network-profiles", profileKeyPrefix)
	if err != nil {
		return 0, 0, fmt.Errorf("could not parse existing network profiles in %s: %w", path, err)
	}
	for _, np := range profiles {
		networkProfiles[np.Key] = np.toJSON()
	}

	folders := map[string]interface{}{}
	if raw, ok := root["folders"]; ok {
		if uerr := json.Unmarshal(raw, &folders); uerr != nil {
			return 0, 0, fmt.Errorf("could not parse existing folders in %s: %w", path, uerr)
		}
	}
	if _, exists := folders[folderName]; !exists {
		folders[folderName] = map[string]interface{}{}
	}

	for key, value := range map[string]interface{}{
		"connections":      connections,
		"network-profiles": networkProfiles,
		"folders":          folders,
	} {
		raw, merr := json.Marshal(value)
		if merr != nil {
			return 0, 0, merr
		}
		root[key] = raw
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return 0, 0, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return 0, 0, err
	}
	if err := os.WriteFile(path, out, 0600); err != nil {
		return 0, 0, err
	}

	return len(conns), len(profiles), nil
}

// mergeSection reads an existing top-level JSON object section and strips out
// any entries this tool previously wrote (identified by keyPrefix), so fresh
// entries can be added without leaving stale or duplicate ones behind.
func mergeSection(root map[string]json.RawMessage, section, keyPrefix string) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	if raw, ok := root[section]; ok {
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
	}
	for key := range result {
		if strings.HasPrefix(key, keyPrefix) {
			delete(result, key)
		}
	}
	return result, nil
}
