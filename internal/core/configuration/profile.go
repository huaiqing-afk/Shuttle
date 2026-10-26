package configuration

const ProfileBlockStart = "# >>> shuttle managed (do not edit) >>>"
const ProfileBlockEnd = "# <<< shuttle managed (do not edit) <<<"

// ProfileBlock enables history prediction without installing or loading a prompt theme.
const ProfileBlock = ProfileBlockStart + "\n" +
	"if (Get-Module -ListAvailable PSReadLine) {\n" +
	"    Set-PSReadLineOption -PredictionSource History\n" +
	"}\n" + ProfileBlockEnd
