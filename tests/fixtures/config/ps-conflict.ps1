# sample PowerShell profile fixture
Set-PSReadLineOption -PredictionViewStyle ListView

# a foreign session entry that must block Shuttle's managed block
oh-my-posh init pwsh --config "$env:POSH_THEMES_PATH\jandedobbeleer.omp.json" | Invoke-Expression
