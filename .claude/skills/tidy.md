---
name: tidy
description: Switch to main, pull latest, and remove merged branches
user_invocable: true
---

Tidy up the local repository:

1. Switch to the `main` branch
2. Pull the latest changes from origin
3. Delete all local branches that have already been merged into main (except main itself)

Run these steps using the Bash tool. After completing, show a summary of what was cleaned up (branches deleted, if any).
