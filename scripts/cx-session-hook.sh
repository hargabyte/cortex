#!/bin/bash
# cx-session-hook.sh - Claude Code session start hook for Cortex
#
# This script runs at the start of each Claude Code session to:
# 1. Check if cx is available
# 2. Display codebase stats if a cx database exists
# 3. Remind the AI to use cx before exploring code

# Check if cx is in PATH
if ! command -v cx &> /dev/null; then
    echo "⚠️  cx not found in PATH - install from github.com/hargabyte/cortex"
    exit 0
fi

# Check if we're in a directory with a cx database
if [ -d ".cx" ] && [ -f ".cx/cortex.db" ]; then
    # Get stats
    STATS=$(cx db info 2>/dev/null | grep -E "Entities:|Dependencies:" | tr '\n' ' ')

    echo "🧠 Cortex (cx) available - USE IT BEFORE EXPLORING"
    echo "   Graph: $STATS"
    echo ""
    echo "   BEFORE exploring code:  cx context --smart \"your task\""
    echo "   BEFORE editing files:   cx safe <file>"
    echo "   To find code:           cx find <name> | cx show <entity>"
    echo "   Project overview:       cx map"
else
    echo "🧠 Cortex (cx) available"
    echo "   Run 'cx scan' to build the code graph for this project"
fi
