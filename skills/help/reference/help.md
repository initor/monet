# Monet Command Reference

## Skills are the commands

Every monet skill is typed as `/monet:<name>` in Claude Code. Each one is a dir in the plugin's `skills/` dir, with a `SKILL.md` whose frontmatter gives its `name`, `description`, and the tools it may use. There is no separate commands dir: the skill is the command.

Available skills are discovered at runtime from the plugin's `skills/` dir.

## The monet agent

The monet agent (`monet:monet`) matches a request to the right skill and dispatches it as `monet:<name>`. You don't need to type a skill yourself: ask monet, and it routes for you.

## Usage

**Direct invocation:**
- `/monet:help`: show this reference
- `/monet:status`: show agent status and skill inventory
- `/monet:status skills`: show the skill inventory (argument passed directly)

**Agent dispatch (through the monet agent):**
- Ask monet to "review this code": routes to `monet:review-code`
- Ask monet to "review my changes": routes to `monet:review-local`
- Ask monet to "paint this": routes to `monet:paint`

## Skill features

Monet skills support four capabilities:

1. **Slash invocation:** each skill in the plugin's `skills/` dir is typed as `/monet:<name>`.
2. **Interactive input:** a skill can list `AskUserQuestion` in its `allowed-tools` to gather input when no argument is given.
3. **Argument passing:** a skill declares `argument-hint` in its frontmatter and reads the arguments from `$ARGUMENTS` in its body.
4. **Bundled references:** a skill keeps long reference content in its own `reference/` dir and reads it from `${CLAUDE_SKILL_DIR}` when a step needs it. A skill that needs another skill's instructions loads it with the Skill tool as `monet:<name>`.
