; CySec.env Inno Setup Script
; Builds the desktop installer for Windows

[Setup]
AppName=CySec.env
AppVersion=0.1.0
AppPublisher=CySec.env Team
AppPublisherURL=https://github.com/cysec-env/cysec
DefaultDirName={pf}\CySec.env
DefaultGroupName=CySec.env
UninstallDisplayIcon={app}\bin\cysec.exe
Compression=lzma2
SolidCompression=yes
OutputDir=..\..\build
OutputBaseFilename=CySec.env-Setup
; SetupIconFile=cysec.ico
WizardStyle=modern
ChangesEnvironment=yes

; Allow installation for all users (requires admin rights)
PrivilegesRequired=admin

[Files]
; The core binary
Source: "..\..\cysec.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
; Pre-packaged registry entries
Source: "..\..\registry\*"; DestDir: "{app}\registry"; Flags: ignoreversion recursesubdirs createallsubdirs
; Default config
; Source: "..\..\config.yaml.default"; DestDir: "{app}\config"; DestName: "config.yaml"; Flags: onlyifdoesntexist

[Dirs]
Name: "{app}\bin"
Name: "{app}\registry\tools"
Name: "{app}\config"
Name: "{app}\workspace"; Flags: uninsneveruninstall

[Icons]
; CySec Terminal Shortcut
Name: "{group}\CySec Terminal"; Filename: "{app}\bin\cysec.exe"; WorkingDir: "{app}\workspace"; Comment: "Launch CySec.env Managed Environment"; IconFilename: "{app}\bin\cysec.exe"
; Optional Desktop Shortcut
Name: "{commondesktop}\CySec Terminal"; Filename: "{app}\bin\cysec.exe"; WorkingDir: "{app}\workspace"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional icons:"; Flags: unchecked
Name: "addtopath"; Description: "Add cysec to system PATH"; GroupDescription: "Environment:"

[Registry]
; Add to system PATH if task is selected
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Control\Session Manager\Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}\bin"; Tasks: addtopath; Check: NeedsAddPath(ExpandConstant('{app}\bin'))

[Run]
; Run the setup provisioning step post-install
Filename: "{app}\bin\cysec.exe"; Parameters: "setup --full --yes"; Description: "Provision the Full Security Environment (~110 tools)"; Flags: postinstall

[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_LOCAL_MACHINE,
    'SYSTEM\CurrentControlSet\Control\Session Manager\Environment',
    'Path', OrigPath)
  then begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;

procedure RemovePath(Param: string);
var
  OrigPath: string;
begin
  if RegQueryStringValue(HKEY_LOCAL_MACHINE,
    'SYSTEM\CurrentControlSet\Control\Session Manager\Environment',
    'Path', OrigPath)
  then begin
    StringChangeEx(OrigPath, ';' + Param, '', True);
    StringChangeEx(OrigPath, Param + ';', '', True);
    StringChangeEx(OrigPath, Param, '', True);
    RegWriteExpandStringValue(HKEY_LOCAL_MACHINE,
      'SYSTEM\CurrentControlSet\Control\Session Manager\Environment',
      'Path', OrigPath);
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
  begin
    RemovePath(ExpandConstant('{app}\bin'));
  end;
end;
