---
name: Capability Evolver
tags: [skills, evolution, self-improvement, extension]
requires:
  bins: [node]
---

# Capability Evolver

Self-evolution engine allowing the agent to propose, draft, and register new skills based on encountered gaps.

## Prerequisites

- `node` binary (for validation and file operations)
- Access to write to `~/.blackcat/marketplace/` directory

## Protocol: Proposing a New Skill

When encountering a capability gap:

1. **Identify the gap**: What task cannot be done with current skills?
2. **Define requirements**:
   - What problem does it solve?
   - What binary/environment variables are needed?
   - What is the simplest working example?
3. **Draft the skill** following the skill format
4. **Place in marketplace** directory
5. **Notify user** to restart daemon to pick up the new skill

## Skill File Format

```markdown
---
name: Skill Name Here
tags: [tag1, tag2, category]
requires:
  bins: [binary1, binary2]      # optional
  env: [ENV_VAR_1, ENV_VAR_2]   # optional
---

# Skill Name Here

Brief description of what this skill enables.

## Prerequisites

List required binaries and environment variables.

## Capabilities

- Bullet list of what this skill can do
- Each capability should be actionable

## How to Use

Provide concrete examples:

```bash
# Example command
binary command --flag value
```

```python
# Example code
import something
result = something.do()
```

## Notes

- Important caveats
- Rate limits
- Common pitfalls
```

## Drafting a New Skill

### Step 1: Create skill file

```bash
mkdir -p ~/.blackcat/marketplace
cat > ~/.blackcat/marketplace/my-new-skill.md << 'EOF'
---
name: My New Skill
tags: [custom, utility]
requires:
  bins: [curl]
---

# My New Skill

Description here...
EOF
```

### Step 2: Validate format

```bash
node -e "
const fs = require('fs');
const content = fs.readFileSync(process.argv[1], 'utf8');
const hasFrontMatter = content.startsWith('---');
const hasName = content.includes('name:');
const hasTags = content.includes('tags:');
console.log('Valid:', hasFrontMatter && hasName && hasTags);
" ~/.blackcat/marketplace/my-new-skill.md
```

### Step 3: Register with user

Message to user:
"New skill 'My New Skill' drafted. Restart the blackcat daemon to activate: `blackcat restart`"

## Skill Evolution Checklist

Before proposing a skill, verify:

- [ ] Skill solves a real, recurring problem
- [ ] No existing skill already covers this
- [ ] Required binaries are commonly available
- [ ] At least one working example is provided
- [ ] Error handling is documented
- [ ] File is under 150 lines
- [ ] Follows existing skill format

## Marketplace Directory Structure

```
~/.blackcat/
├── marketplace/
│   ├── custom-skill-1.md
│   ├── custom-skill-2.md
│   └── ...
└── config.yaml
```

## Auto-Discovery

The daemon loads skills from:
1. Built-in: `internal/workspace/skills/*.md`
2. Marketplace: `~/.blackcat/marketplace/*.md` (user-created)

Restart required after adding marketplace skills.

## Notes

- Skill names should be descriptive but concise
- Tags help with skill discovery
- One skill per file
- Keep examples copy-paste ready
- Test commands before including them
