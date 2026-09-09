Require ask
Require git

Set Shell "bash"
Set Theme "Catppuccin Macchiato"
Set FontSize 18
Set Width 1440
Set Height 520
Set Padding 24
Set Framerate 20
Set TypingSpeed 65ms
Set WaitTimeout 330s

Hide
Type "export PS1='$ '; clear"
Enter
Show
Type "git show --stat --oneline HEAD"
Enter
Wait+Screen /validate core and provider release archives/
Sleep 8s
Type "git show HEAD |"
Enter
Type "  ask -m gpt-5.6-luna 'Explain this fix in four sentences. Use relative file paths.' && echo 'Review complete'"
Enter
Wait+Screen /which agent\?/
Sleep 5s
Enter
Wait+Screen /(?m)^Review complete/
Sleep 20s
