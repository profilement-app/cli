---
name: add
description: Propose the project in the current folder as a new project in Profilement — its tools, frameworks, libraries, services and Git topics, checked by the user on Profilement's review page before anything is added. Use when the user wants to add this project to Profilement, their CV or skill profile.
argument-hint: "[project name]"
allowed-tools: Bash(profilement *)
---

Read `${CLAUDE_PLUGIN_ROOT}/procedure.md` and follow it for **add**: a new project in Profilement.

If the user gave a name ($ARGUMENTS), use it as the project's name instead of asking for one.
