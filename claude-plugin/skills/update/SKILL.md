---
name: update
description: Propose an update of one of the user's Profilement projects from the current folder — the tools, frameworks, libraries, services and Git topics it has now, checked by the user on Profilement's review page before anything changes. Use when the user wants to update this project in Profilement.
argument-hint: "[project code]"
allowed-tools: Bash(profilement *)
---

Read `${CLAUDE_PLUGIN_ROOT}/procedure.md` and follow it for **update**: an update of one of the user's projects.

If the user gave a project code ($ARGUMENTS), use that project instead of asking which one.
