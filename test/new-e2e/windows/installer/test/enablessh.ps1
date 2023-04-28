<#
.SYNOPSIS

Installs and configures OpenSSH Server on a Hyper-V VM

.PARAMETER VMName

Name of the Hyper-V VM

.PARAMETER SSHKeyPath

Optional path to SSH Public Key to add to administrators_authorized_keys

.PARAMETER Credential

Optional PSCredential for the Hyper-V VM PSSession. Will prompt if not provided.

.PARAMETER SnapshotName

Optional name of snapshot to create after completion

.EXAMPLE

PS> .\enablessh.ps1 -VMName "Windows Server 2019" -Credential (Get-Credential) -SnapshotName ssh

#>
param (
    [Parameter(Mandatory=$true)]
    [string]$VMName,
    [string]$SSHKeyPath,
    [string]$SnapshotName,
    [ValidateNotNull()]
    [System.Management.Automation.PSCredential]
    [System.Management.Automation.Credential()]
    $Credential = [System.Management.Automation.PSCredential]::Empty
)

if ($Credential -eq [System.Management.Automation.PSCredential]::Empty) {
    $Credential = Get-Credential
}

$s = New-PSSession -VMName $VMName -Credential $VMCreds

# Install OpenSSH Server
Invoke-Command -Session $s -ScriptBlock {
    Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
    Set-Service -Name sshd -StartupType 'Automatic'
    Start-Service -Name sshd

    # Wait for files to be populated
    while (!(Test-Path "$env:programdata\ssh\ssh_host_rsa_key")) {
        Start-Sleep 10
    }
}

if ($SSHKeyPath) {
    # Add SSH Key
    $sshkey = (Get-Content -Path $SSHKeyPath)
    Invoke-Command -Session $s -ScriptBlock { Add-Content -Path "$env:programdata\ssh\administrators_authorized_keys" -Value $Using:sshkey}
}

# Fix authorized_keys privs
Invoke-Command -Session $s -ScriptBlock {
    if (!(Test-Path "$env:programdata\ssh\administrators_authorized_keys")) {
        New-Item -ItemType File -Path "$env:programdata\ssh\administrators_authorized_keys"
    }
    get-acl "$env:programdata\ssh\ssh_host_rsa_key" | set-acl "$env:programdata\ssh\administrators_authorized_keys"
}

# Print connection info
Invoke-Command -Session $s -ScriptBlock { ipconfig }

if ($SnapshotName) {
    # Note: checkpoint-vm breaks the session
    Checkpoint-VM -Name $VMName -SnapshotName $SnapshotName
}

