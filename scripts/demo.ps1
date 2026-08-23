# End-to-end KoraDB demo (PowerShell).
# Builds the CLI and walks through schema -> collection -> insert -> query ->
# schema evolution against a fresh database file.

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$go = if (Get-Command go -ErrorAction SilentlyContinue) { "go" } else { "C:\Program Files\Go\bin\go.exe" }
& $go build -o bin\KoraDB.exe ./cmd/KoraDB

$db = Join-Path $env:TEMP ("KoraDB-demo-" + [guid]::NewGuid().ToString("N") + ".db")
$p  = Join-Path $root "bin\KoraDB.exe"

Write-Host "`n== 1. register schema ==" -ForegroundColor Cyan
& $p --db $db schema add user.proto .\examples\user.proto

Write-Host "`n== 2. create collection (key=email, index=city) ==" -ForegroundColor Cyan
& $p --db $db collection create users example.User --key-field=email --index=city

Write-Host "`n== 3. insert ==" -ForegroundColor Cyan
& $p --db $db insert users '{"name":"Alice","email":"alice@x.com","city":"NYC"}'
& $p --db $db insert users '{"name":"Bob","email":"bob@x.com","city":"LA"}'
& $p --db $db insert users '{"name":"Carol","email":"carol@x.com","city":"NYC"}'

Write-Host "`n== 4. query indexed field city==NYC ==" -ForegroundColor Cyan
& $p --db $db query users city == NYC

Write-Host "`n== 5. evolve schema (add age, roles) - no migration ==" -ForegroundColor Cyan
& $p --db $db schema add user.proto .\examples\user_v2.proto
Write-Host "-- old record still reads cleanly --"
& $p --db $db get users alice@x.com
Write-Host "-- new record uses new fields --"
& $p --db $db insert users '{"name":"Dave","email":"dave@x.com","city":"SF","age":42,"roles":["admin","ops"]}'
& $p --db $db get users dave@x.com

Write-Host "`nDemo DB: $db" -ForegroundColor DarkGray
