Require ask

Set Shell "zsh"
Set Theme "Catppuccin Macchiato"
Set FontSize 18
Set Width 1200
Set Height 520
Set Padding 24
Set Framerate 20
Set TypingSpeed 35ms

Sleep 1s

Type "ask -p demo 'Review the token parser change and summarize the evidence.'"
Enter
Wait+Screen /Focused parser tests pass/
Sleep 3s
