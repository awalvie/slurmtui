Output dev/screenshots/screenshots.gif
Set Shell bash
Set FontSize 28
Set FontFamily "JetBrains Mono,DejaVu Sans Mono"
Set LineHeight 1.4
Set Width 2500
Set Height 1500
Set Padding 40
Set Theme "Dracula"
Set WindowBar Colorful
Set WindowBarSize 60
Set TypingSpeed 0

Hide
Type "export PS1=; clear; slurmtui --context dev --readonly"
Enter
Sleep 4s
Show
Screenshot assets/screenshots/overview.png
Sleep 200ms

Type "d"
Sleep 1s
Screenshot assets/screenshots/describe.png
Sleep 200ms
Escape
Sleep 500ms

Type "?"
Sleep 1s
Screenshot assets/screenshots/help.png
Sleep 200ms
Escape
Sleep 300ms
Type "q"
Sleep 1s

Hide
Type "clear; slurmtui --context dev"
Enter
Sleep 4s
Down
Sleep 300ms
Type "D"
Sleep 1s
Show
Screenshot assets/screenshots/drain.png
Sleep 200ms
Escape
Sleep 500ms
Type "q"
Sleep 1s

Hide
Type "clear; env -u SLURM_JWT SLURMTUI_CONFIG_DIR=$SCREENSHOTS_EMPTY_CONFIG slurmtui"
Enter
Sleep 2s
Show
Screenshot assets/screenshots/setup.png
Sleep 200ms
Ctrl+C
Sleep 500ms
