@echo off
rem Who runs the tests: user name, admin or not (integrity level), Windows build.
whoami
whoami /groups | findstr /i /c:"BUILTIN\Administrators" /c:"Mandatory Label"
ver
reg query "HKLM\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" /v pv 2>nul
reg query "HKCU\Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" /v pv 2>nul
