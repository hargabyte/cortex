package cmd

import (
	_ "embed"
)

// reportSkillTemplate is the markdown content for the /report skill.
// This template is output by `cx report --init-skill` and should be
// saved to ~/.claude/commands/report.md for interactive report generation.
//
// The skill provides:
// - Interactive preference gathering (audience, format, focus)
// - Consistent report structure and naming
// - Multiple output formats (HTML, Markdown, terminal)
// - Diagram rendering options
//
//go:embed report_skill_template.md
var reportSkillTemplate string
