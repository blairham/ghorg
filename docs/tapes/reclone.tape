# Regenerate docs/images/reclone.png:
#   GHORG_GITHUB_TOKEN=$(gh auth token) vhs docs/tapes/reclone.tape
# with a ghorg built from this checkout first on PATH, GHORG_RECLONE_PATH
# pointing at a reclone.yaml, and GHORG_ABSOLUTE_PATH_TO_CLONE_TO set (run
# clone.tape first, so some entries pull and some clone). Without
# GHORG_RECLONE_ENV_CONFIG_ONLY, reclone clears every GHORG_ variable before
# each entry and clones into ~/ghorg instead.
Set Shell "bash"
Set FontSize 15
Set Width 1100
Set Height 1000
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms

Hide
Type "export PS1='$ ' GHORG_CONFIG=none GHORG_COLOR=enabled GHORG_RECLONE_ENV_CONFIG_ONLY=true && clear"
Enter
Show

Type "ghorg reclone"
Sleep 300ms
Enter
Sleep 10s
Screenshot docs/images/reclone.png
Sleep 1s
