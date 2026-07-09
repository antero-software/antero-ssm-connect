# antero-ssm-connect

⚡ A powerful CLI to port-forward into private **RDS**, **Aurora**, and **ElastiCache** (Redis, Memcached) endpoints through EC2 instances using AWS SSM Session Manager — fully interactive, no SSH required.

## Features
- ☁️ Interactive profile / EC2 / database selection (SSO-aware)
- 🚀 Quick connect via `--profile` and `--filter`
- 🔐 SSM-based secure access (no open ports or bastion hosts)
- 🔄 Port-forward RDS, Aurora, Redis, Memcached — all in one tool
- 🧵 Background port-forwarding (non-blocking, persistent)
- 🔢 Tracks active sessions by PID
- 📋 List active tunnels with `--list`
- ❌ Kill specific tunnels with `--kill <pid>`
- 💥 Kill all tunnels with `--kill-all`
- 🧹 Automatically cleans up dead sessions
- ⚠️ Prevents local port conflicts
- 🗄️ Centralized DBeaver connections via `--dbeaver` — native AWS SSM tunnel, same connections for the whole team (DBeaver paid editions only)

## Installation

```bash
brew tap antero-software/antero-ssm-connect
brew install antero-ssm-connect
```

## Requirements

Make sure the following are installed and configured before using:

- ✅ [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html) — Install Guide  
- ✅ [session-manager-plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) — Required for SSM sessions 
- ✅ AWS SSO login configured in `~/.aws/config`
- ✅ SSM agent running on EC2 instances
- ✅ Target EC2 instances must:
- Be registered in SSM (visible in aws ssm describe-instance-information)
  - Have the AmazonSSMManagedInstanceCore policy
  - Have internal access to the target DB (RDS/ElastiCache)

### You can verify SSO with:
```bash
aws sso login --profile your-profile
aws sts get-caller-identity --profile your-profile
```

## Example Commands

### 🧩 Start SSM shell session to EC2 instance (no port-forward)
```bash
aws-ssm-connect --ssm
```

### 🚀 Port-forward to RDS or ElastiCache on a custom local port
```bash
aws-ssm-connect --db-port-forward
```

### 📋 List all active tunnels (by PID)
```bash
aws-ssm-connect --list
```

### 💥 Kill all active sessions
```bash
aws-ssm-connect --kill-all
```

### 🗄️ Centralize DBeaver connections for the team
```bash
aws-ssm-connect --dbeaver --profile your-profile
```
Requires **DBeaver Enterprise/Ultimate/Lite/Team Edition** — the native AWS SSM tunnel handler
this relies on isn't in DBeaver Community.

Discovers every RDS/Aurora/ElastiCache database visible to the profile, matches each one to the
SSM-managed EC2 instance in its VPC, and writes them straight into DBeaver's own
`data-sources.json`:
- One **network profile** per bastion EC2 instance (`ssm.instance.id` + region), shared by every
  database behind it.
- One **connection** per database, pointing at its real endpoint/port with DBeaver's native
  `AWS SSM` tunnel handler enabled — DBeaver opens the tunnel itself on connect, no separate
  `--db-port-forward` process required.

Supported engines: PostgreSQL, MySQL/MariaDB, Redis. SQL Server/Oracle/MongoDB/Memcached are
skipped — no confirmed DBeaver driver id for them yet.

The one manual step per network profile (not per connection): open it once in DBeaver under
*Network configurations → AWS SSM* and pick a **Credentials** method (e.g. "AWS profile"). That
lives in DBeaver's encrypted credential store, which this tool intentionally doesn't touch —
instance ID and region are already filled in.

Close DBeaver (or refresh the Database Navigator afterwards) before running this, and pass
`--dbeaver-path` to point at a non-default `data-sources.json` if auto-detection picks the
wrong workspace.
