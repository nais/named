package api

import (
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func SshConnect(user, pass, host, port string) (*ssh.Client, *ssh.Session, error) {

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password(pass)},
	}

	serverString := host + ":" + port

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

func SftpConnect(sshClient *ssh.Client) (*sftp.Client, error) {
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	return sftpClient, nil
}
