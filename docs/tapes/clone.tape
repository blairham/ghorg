# Regenerate docs/images/clone.png:
#   GHORG_GITHUB_TOKEN=$(gh auth token) vhs docs/tapes/clone.tape
# with a ghorg built from this checkout first on PATH. Clones go to
# $GHORG_ABSOLUTE_PATH_TO_CLONE_TO, which should start empty.
Set Shell "bash"
Set FontSize 15
Set Width 1100
Set Height 640
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms

Hide
Type "export PS1='$ ' GHORG_CONFIG=none GHORG_COLOR=enabled && clear"
Enter
Show

Type "ghorg clone blairham --clone-type=user --match-regex='^(tuikit|stevedore|localiam|dockmaster|k4a|ghorg)$'"
Sleep 500ms
Enter
Sleep 8s
Screenshot docs/images/clone.png
Sleep 1s
