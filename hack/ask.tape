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

Type "ask -p recorded-review 'Review the token parser change and summarize the evidence.'"
Enter
Wait+Screen /protect session lookup/
Sleep 3s

Type "go test -v ./internal/auth"
Enter
Wait+Screen /PASS/
Sleep 2s
