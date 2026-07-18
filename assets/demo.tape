# Regenerate with: vhs assets/demo.tape (needs vhs, ttyd, ffmpeg and a built ./hyprsunset-tui)
Output assets/demo.gif

Set FontSize 15
Set Width 1000
Set Height 420
Set Padding 20
Set Theme "Catppuccin Mocha"

Type "./hyprsunset-tui"
Enter
Sleep 1.5s

# Simple panel: move around and adjust values
Right@250ms 4
Sleep 500ms
Down
Right@250ms 3
Sleep 500ms
Down
Left@250ms 2
Sleep 1s

# Advanced panel: select fields and tweak the profile
Tab
Sleep 750ms
Down@300ms 2
Right@250ms 4
Sleep 500ms
Down
Right@250ms 2
Sleep 2s
