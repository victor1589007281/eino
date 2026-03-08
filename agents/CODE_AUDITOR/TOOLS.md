# TOOLS.md - Local Notes

Skills define _how_ tools work. This file is for _your_ specifics — the stuff that's unique to your setup.

## What Goes Here

Things like:

- Camera names and locations
- SSH hosts and aliases
- Preferred voices for TTS
- Speaker/room names
- Device nicknames
- Anything environment-specific

## Examples

```markdown
### Cameras

- living-room → Main area, 180° wide angle
- front-door → Entrance, motion-triggered

### SSH

- home-server → 192.168.1.100, user: admin

### TTS

- Preferred voice: "Nova" (warm, slightly British)
- Default speaker: Kitchen HomePod
```

## Why Separate?

Skills are shared. Your setup is yours. Keeping them apart means you can update skills without losing your notes, and share skills without leaking your infrastructure.

---

Add whatever helps you do your job. This is your cheat sheet.

## Installed Skills (Global)

**Location:** `~/.openclaw/skills/`

| Skill | Command/Usage |
|-------|---------------|
| `find-skills` | Search for new skills via ClawHub |
| `clawsec` | Security audits via `clawsec-suite` |
| `proactive-agent` | WAL Protocol → SESSION-STATE.md |
| `secure-install` | `node bin/safe-install.js <path> --yes` |

## ClawSec Sub-Skills

**Location:** `~/.openclaw/skills/clawsec/skills/`

- `clawsec-suite` — Main security suite manager
- `soul-guardian` — File integrity monitoring
- `clawsec-feed` — Security advisories
- `openclaw-audit-watchdog` — Audit monitoring

## Configuration

- **Skills Config:** `~/.openclaw/openclaw.json` → `skills.load.extraDirs`
- **Gateway:** systemd service, port 18789
- **Workspace:** `~/.openclaw/workspace/`
