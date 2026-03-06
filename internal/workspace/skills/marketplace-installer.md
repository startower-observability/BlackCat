---
name: Marketplace Installer
tags: [marketplace, skills, install, manage, npx]
requires:
  bins: [npx]
---

# Marketplace Installer

Install, update, and manage skills from the marketplace directory.

## Prerequisites

- `npx` binary (comes with Node.js)
- Access to `~/.blackcat/marketplace/` directory

## Capabilities

- Install skills from npm packages
- Install skills from GitHub repos
- Update installed marketplace skills
- List installed skills
- Remove skills
- Validate skill format

## Marketplace Directory Structure

```
~/.blackcat/
├── marketplace/
│   ├── skill-name-1.md      # Downloaded/installed skills
│   ├── skill-name-2.md
│   └── ...
└── config.yaml
```

Built-in skills live in the application bundle and cannot be modified.
Marketplace skills override or extend built-in capabilities.

## How to Use

### Install from npm

```bash
# Install a skill published as npm package
npx @blackcat/skill-<name> install

# Example:
npx @blackcat/skill-aws install
```

### Install from GitHub

```bash
# Direct install from GitHub repo
npx degit github:username/blackcat-skill-name ~/.blackcat/marketplace/skill-name.md

# Or with full URL:
npx degit https://github.com/username/repo/skill.md ~/.blackcat/marketplace/my-skill.md
```

### Manual install

```bash
# Create marketplace directory
mkdir -p ~/.blackcat/marketplace

# Download skill file
curl -L -o ~/.blackcat/marketplace/custom-skill.md \
  https://raw.githubusercontent.com/user/repo/main/skill.md

# Or copy local file
cp ./my-skill.md ~/.blackcat/marketplace/
```

### Update a skill

```bash
# Re-run install command (usually overwrites)
npx @blackcat/skill-<name> install

# Or manually replace:
curl -L -o ~/.blackcat/marketplace/skill.md \
  https://example.com/updated-skill.md
```

### List installed marketplace skills

```bash
# List all .md files in marketplace
ls -la ~/.blackcat/marketplace/*.md

# With details:
for f in ~/.blackcat/marketplace/*.md; do
  echo "=== $(basename $f) ==="
  head -20 "$f" | grep -E "^(name:|tags:)"
done
```

### Remove a skill

```bash
# Remove specific skill
rm ~/.blackcat/marketplace/skill-name.md

# Remove all marketplace skills
rm ~/.blackcat/marketplace/*.md
```

### Validate skill format

```bash
# Check YAML frontmatter
npx js-yaml ~/.blackcat/marketplace/skill.md > /dev/null && echo "Valid YAML"

# Check required fields
node -e "
const fs = require('fs');
const content = fs.readFileSync(process.argv[1], 'utf8');
const match = content.match(/^---\n([\s\S]*?)\n---/);
if (!match) { console.log('Missing frontmatter'); process.exit(1); }
const yaml = match[1];
if (!yaml.includes('name:')) { console.log('Missing name'); process.exit(1); }
if (!yaml.includes('tags:')) { console.log('Missing tags'); process.exit(1); }
console.log('Valid skill format');
" ~/.blackcat/marketplace/skill.md
```

## Skill File Format Requirements

All marketplace skills must include:

1. **YAML frontmatter** between `---` markers
2. **name**: Short, descriptive skill name
3. **tags**: Array of searchable tags
4. **requires** (optional): Binaries and env vars needed

```markdown
---
name: My Custom Skill
tags: [custom, utility, api]
requires:
  bins: [curl, jq]
  env: [API_KEY]
---

# My Custom Skill

Description of what this skill does.

## Capabilities

- Thing it can do
- Another thing

## How to Use

Example commands here.

## Notes

Important caveats here.
```

## After Installation

Skills in the marketplace directory are loaded on daemon startup.

```bash
# Restart to pick up new skills
blackcat restart

# Or full daemon stop/start
blackcat stop && blackcat start
```

## Creating Publishable Skills

To share a skill:

1. Create a git repo with your `skill-name.md` file
2. Include a README with installation instructions
3. Tag releases for version management
4. Publish to npm (optional):

```json
{
  "name": "@blackcat/skill-your-name",
  "version": "1.0.0",
  "bin": {
    "install": "./install.js"
  },
  "files": ["skill.md", "install.js"]
}
```

## Notes

- Skill names should be unique (marketplace overrides built-in if same name)
- Keep skills under 150 lines for fast loading
- Test all example commands before publishing
- Use semantic versioning for published skills
- Document breaking changes in skill file
