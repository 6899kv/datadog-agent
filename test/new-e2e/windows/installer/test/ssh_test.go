// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package installertest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/test/new-e2e/windows"
	"github.com/DataDog/datadog-agent/test/new-e2e/windows/hyperv"
	"github.com/DataDog/datadog-agent/test/new-e2e/windows/installer"

	"github.com/stretchr/testify/suite"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type testHost struct {
	host     string
	username string
	password string
	vmname   string
	snapshot string
}

type windowsInstallerSuite struct {
	suite.Suite

	target *testHost

	installer           string
	prevstableinstaller string

	sshclient *ssh.Client

	// test suite output dir
	suiteoutputdir string
	// individual test output dir
	testoutputdir string
}

func TestWindowsInstaller(t *testing.T) {

	// TODO: make all this configurable
	// TODO: use new-e2e/pulumi for provisioning
	prevstableinstaller := "ddagent-cli-7.43.1.msi"
	testinstaller := "datadog-agent-7.45.0-rc.3-1.x86_64.msi"

	hosts := []testHost{
		{
			host:     "172.23.224.26:22",
			username: "user",
			password: "user",
			vmname:   "Windows 10",
			snapshot: "ssh",
		},
		{
			host:     "172.23.238.202:22",
			username: "DDEV\\Administrator",
			password: "123!@#QWEqwe",
			vmname:   "Windows Server 2019",
			snapshot: "ddev-ssh",
		},
	}
	testhostid := 1

	suite.Run(t, &windowsInstallerSuite{
		target:              &hosts[testhostid],
		suiteoutputdir:      filepath.Join("./output", time.Now().Format(time.RFC3339)),
		prevstableinstaller: prevstableinstaller,
		installer:           testinstaller,
	})
}

func (s *windowsInstallerSuite) SetupSuite() {
	// create output dir
	os.MkdirAll(s.suiteoutputdir, os.ModePerm)
}

func (s *windowsInstallerSuite) TearDownSuite() {
	fmt.Printf("Output directory: %s\n", s.suiteoutputdir)
}

func (s *windowsInstallerSuite) SetupTest() {
	// create output dir
	s.testoutputdir = filepath.Join(s.suiteoutputdir, s.T().Name())
	os.MkdirAll(s.testoutputdir, os.ModePerm)

	// revert VM
	fmt.Println("Reverting VM")
	err := hyperv.RevertVM(s.target.vmname, s.target.snapshot)
	s.Require().NoError(err)

	// connect to SSH
	sshconfig := &ssh.ClientConfig{
		User: s.target.username,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.target.password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	sshclient, err := ssh.Dial("tcp", s.target.host, sshconfig)
	s.Require().NoError(err)

	s.sshclient = sshclient
	s.T().Cleanup(func() {
		fmt.Println("closing ssh")
		sshclient.Close()
	})

	output, err := windows.PsExec(s.sshclient, "ipconfig")
	s.Require().NoError(err)
	s.Require().NotEmpty(output)
	fmt.Println(output)
}

func (s *windowsInstallerSuite) TestDefaultInstall() {
	t, err := NewTester(s.sshclient)
	s.Require().NoError(err)

	err = t.InstallAgent(s.sshclient, s.installer, "",
		filepath.Join(s.testoutputdir, "install.log"))
	s.Require().NoError(err)

	s.Require().True(t.AssertExpectations(s.Assert(), s.sshclient))
}

func (s *windowsInstallerSuite) TestUninstall() {
	t, err := NewTester(s.sshclient)
	s.Require().NoError(err)

	err = t.InstallAgent(s.sshclient, s.installer, "",
		filepath.Join(s.testoutputdir, "install.log"))
	s.Require().NoError(err)

	s.Require().True(t.AssertExpectations(s.Assert(), s.sshclient))

	err = installer.UninstallAgent(s.sshclient,
		filepath.Join(s.testoutputdir, "uninstall.log"))
	s.Require().NoError(err)

	userexists, err := windows.LocalUserExists(s.sshclient, "ddagentuser")
	s.Require().NoError(err)
	s.Require().True(userexists, "user should still exist after uninstall")
}

func (s *windowsInstallerSuite) TestAllowClosedSourceArgs() {
	tcs := []struct {
		testname string
		args     string
		expected string
	}{
		{"AllowClosedSource1", "ALLOWCLOSEDSOURCE=1", installer.AllowClosedSourceYes},
		{"NpmFlag", "NPM=1", installer.AllowClosedSourceYes},
		{"ADDLOCAL_NPM", "ADDLOCAL=NPM", installer.AllowClosedSourceYes},
	}

	firstTest := true
	for _, npmEnabled := range []bool{false, true} {
		for _, tc := range tcs {
			var tcname string
			if npmEnabled {
				tcname = fmt.Sprintf("%s/NPMEnabled", tc.testname)
			} else {
				tcname = fmt.Sprintf("%s/NPMDisabled", tc.testname)
			}
			s.Run(tcname, func() {
				if !firstTest {
					s.SetupTest()
				}
				firstTest = false

				t, err := NewTester(s.sshclient,
					WithExpectedAllowClosedSource(tc.expected),
					WithExpectNPMRunning(npmEnabled))
				s.Require().NoError(err)

				err = setNetworkConfig(s.sshclient, npmEnabled)
				s.Require().NoError(err)

				err = t.InstallAgent(s.sshclient, s.installer, tc.args,
					filepath.Join(s.testoutputdir, "install.log"))
				s.Require().NoError(err)

				s.Require().True(t.AssertExpectations(s.Assert(), s.sshclient))
			})
		}
	}
}

func (s *windowsInstallerSuite) TestUpgradeWithNPM() {
	err := installer.InstallAgentWithDefaultUser(s.sshclient, s.prevstableinstaller, "ADDLOCAL=NPM",
		filepath.Join(s.testoutputdir, "install.log"))
	s.Require().NoError(err)

	err = setNetworkConfig(s.sshclient, true)
	s.Require().NoError(err)

	t, err := NewTester(s.sshclient,
		WithExpectedAllowClosedSource(installer.AllowClosedSourceYes),
		WithExpectNPMRunning(true))
	s.Require().NoError(err)

	err = t.InstallAgent(s.sshclient, s.installer, "",
		filepath.Join(s.testoutputdir, "upgrade.log"))
	s.Require().NoError(err)

	s.Require().True(t.AssertExpectations(s.Assert(), s.sshclient))
}

func (s *windowsInstallerSuite) TestDisableAllowClosedSource() {
	err := installer.InstallAgentWithDefaultUser(s.sshclient,
		s.prevstableinstaller, "ADDLOCAL=NPM",
		filepath.Join(s.testoutputdir, "install.log"))
	s.Require().NoError(err)

	err = setNetworkConfig(s.sshclient, true)
	s.Require().NoError(err)

	t, err := NewTester(s.sshclient,
		WithExpectedAllowClosedSource(installer.AllowClosedSourceNo),
		WithExpectNPMRunning(false))
	s.Require().NoError(err)

	err = t.InstallAgent(s.sshclient,
		s.installer, "ALLOWCLOSEDSOURCE=0",
		filepath.Join(s.testoutputdir, "upgrade.log"))
	s.Require().NoError(err)

	s.Require().True(t.AssertExpectations(s.Assert(), s.sshclient))
}

func (s *windowsInstallerSuite) TestUpgradeChangeUser() {
	err := installer.InstallAgentWithDefaultUser(s.sshclient,
		s.prevstableinstaller, "",
		filepath.Join(s.testoutputdir, "install.log"))
	s.Require().NoError(err)

	s.Require().True(AssertDefaultInstalledUser(s.Assert(), s.sshclient))

	username := "testuser"
	password := "123!@#QWEqwe"
	t, err := NewTester(s.sshclient,
		WithInstallUser(username),
		WithInstallPassword(password),
		WithExpectedAgentUserFromUsername(s.sshclient, username, password))
	s.Require().NoError(err)

	err = t.InstallAgent(s.sshclient,
		s.installer, "",
		filepath.Join(s.testoutputdir, "upgrade.log"))
	s.Require().NoError(err)

	s.Require().True(t.AssertExpectations(s.Assert(), s.sshclient))
}

func (s *windowsInstallerSuite) TestAgentUser() {
	hostinfo, err := windows.GetHostInfo(s.sshclient)
	s.Require().NoError(err)

	var domainpart string
	var servicedomainpart string
	if hostinfo.IsDomainController() {
		domainpart = windows.NetBIOSName(hostinfo.Domain)
		servicedomainpart = windows.NetBIOSName(hostinfo.Domain)
	} else {
		domainpart = windows.NetBIOSName(hostinfo.Hostname)
		servicedomainpart = "."
	}

	tcs := []struct {
		testname            string
		builtinaccount      bool
		username            string
		expecteddomain      string
		expecteduser        string
		expectedserviceuser string
	}{
		{"user_only", false, "testuser", domainpart, "testuser", fmt.Sprintf("%s\\testuser", servicedomainpart)},
		{"dotslash_user", false, ".\\testuser", domainpart, "testuser", fmt.Sprintf("%s\\testuser", servicedomainpart)},
		{"domain_user", false, fmt.Sprintf("%s\\testuser", domainpart), domainpart, "testuser", fmt.Sprintf("%s\\testuser", servicedomainpart)},
		{"LocalSystem", true, "LocalSystem", "NT AUTHORITY", "SYSTEM", "LocalSystem"},
		{"SYSTEM", true, "SYSTEM", "NT AUTHORITY", "SYSTEM", "LocalSystem"},
	}

	userpassword := "123!@#QWEqwe"
	for tc_i, tc := range tcs {
		s.Run(tc.testname, func() {
			if tc_i > 0 {
				s.SetupTest()
			}

			if hostinfo.IsDomainController() && !tc.builtinaccount {
				// user must exist on domain controllers
				err = windows.CreateLocalUser(s.sshclient, tc.expecteduser, userpassword)
				s.Require().NoError(err)
			}

			t, err := NewTester(s.sshclient,
				WithInstallUser(tc.username),
				WithInstallPassword(userpassword),
				WithExpectedAgentUser(tc.expecteddomain, tc.expecteduser, tc.expectedserviceuser))
			s.Require().NoError(err)

			err = t.InstallAgent(s.sshclient,
				s.installer, "",
				filepath.Join(s.testoutputdir, "install.log"))
			s.Require().NoError(err)

			s.Require().True(t.AssertExpectations(s.Assert(), s.sshclient))
		})
	}
}

func setNetworkConfig(client *ssh.Client, npmEnabled bool) error {
	sftpclient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("sftp connection failed: %v", err)
	}
	defer sftpclient.Close()

	configPath := installer.DefaultConfigPath
	err = sftpclient.MkdirAll(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config dir: %v", err)
	}
	err = windows.WriteFile(sftpclient,
		filepath.Join(configPath, "system-probe.yaml"),
		[]byte(fmt.Sprintf("network_config:\n  enabled: %v", npmEnabled)))
	if err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}
	return nil
}
