package api

import (
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

//SshConnect returns ssh client and session for specified host
func SshConnect(resource *OpenAmResource, port string) (*ssh.Client, *ssh.Session, error) {

	sshConfig := &ssh.ClientConfig{
		User: resource.Username,
		Auth: []ssh.AuthMethod{ssh.Password(resource.Password)},
	}

	serverString := resource.Hostname + ":" + port

	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	client, err := ssh.Dial("tcp", serverString, sshConfig)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, session, nil
}

// SftpConnect returns sftp client for existing ssh client
func SftpConnect(sshClient *ssh.Client) (*sftp.Client, error) {
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	return sftpClient, nil
}
