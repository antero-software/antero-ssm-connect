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
