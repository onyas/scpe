package scpe

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"syscall"
	"time"
	"github.com/pkg/sftp"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"sync"
	"github.com/kballard/go-shellquote"
)

var (
	DefaultCiphers = []string{
		"aes128-ctr",
		"aes192-ctr",
		"aes256-ctr",
		"aes128-gcm@openssh.com",
		"chacha20-poly1305@openssh.com",
		"arcfour256",
		"arcfour128",
		"arcfour",
		"aes128-cbc",
		"3des-cbc",
		"blowfish-cbc",
		"cast128-cbc",
		"aes192-cbc",
		"aes256-cbc",
	}
)

type Client interface {
	StartScp(sourceFile string, targetFile string, scpType int)
}

type defaultClient struct {
	clientConfig *ssh.ClientConfig
	node         *Node
}

func NewClient(node *Node) Client {
	u, err := user.Current()
	if err != nil {
		l.Error(err)
		return nil
	}

	var authMethods []ssh.AuthMethod

	var pemBytes []byte
	if node.KeyPath == "" {
		pemBytes, err = ioutil.ReadFile(path.Join(u.HomeDir, ".ssh/id_rsa"))
	} else {
		pemBytes, err = ioutil.ReadFile(node.KeyPath)
	}
	if err != nil {
		l.Error(err)
	} else {
		var signer ssh.Signer
		if node.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(node.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(pemBytes)
		}
		if err != nil {
			l.Error(err)
		} else {
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	password := node.password()

	if password != nil {
		authMethods = append(authMethods, password)
	}

	authMethods = append(authMethods, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, 0, len(questions))
		for i, q := range questions {
			fmt.Print(q)
			if echos[i] {
				scan := bufio.NewScanner(os.Stdin)
				if scan.Scan() {
					answers = append(answers, scan.Text())
				}
				err := scan.Err()
				if err != nil {
					return nil, err
				}
			} else {
				b, err := terminal.ReadPassword(syscall.Stdin)
				if err != nil {
					return nil, err
				}
				fmt.Println()
				answers = append(answers, string(b))
			}
		}
		return answers, nil
	}))

	config := &ssh.ClientConfig{
		User:            node.user(),
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 10,
	}

	config.SetDefaults()
	config.Ciphers = append(config.Ciphers, DefaultCiphers...)

	return &defaultClient{
		clientConfig: config,
		node:         node,
	}
}

func (c *defaultClient) StartScp(sourceFile string, targetPath string, scpType int) {
	host := c.node.Host
	port := c.node.port()
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), c.clientConfig)
	if err != nil {
		l.Error(err)
		return
	}
	defer client.Close()

	l.Infof("connect server ssh -p %d %s@%s version: %s\n", port, c.node.user(), host, string(client.ServerVersion()))

	session, err := client.NewSession()
	if err != nil {
		l.Error(err)
		return
	}
	defer session.Close()

	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		l.Error(err)
		return
	}
	defer terminal.Restore(fd, state)

	w, h, err := terminal.GetSize(fd)
	if err != nil {
		l.Error(err)
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	err = session.RequestPty("xterm", h, w, modes)
	if err != nil {
		l.Error(err)
		return
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		l.Error(err)
		return
	}

	err = session.Shell()
	if err != nil {
		l.Error(err)
		return
	}

	// then callback before scp
	for i := range c.node.BeforeCpCallbackShells {
		shell := c.node.BeforeCpCallbackShells[i]
		time.Sleep(shell.Delay * time.Millisecond)
		stdinPipe.Write([]byte(shell.Cmd + "\r"))
	}

	// scp
	if scpType == 1 {
		err = copyFromLocalToServer(sourceFile, targetPath, stdinPipe)
		if err != nil {
			l.Error(err)
			return
		}
	} else if scpType == 0 {
		err = copyFromServerToLocal(sourceFile, targetPath, client)
		if err != nil {
			l.Error(err)
			return
		}
		fmt.Fprint(stdinPipe, "\003")// ctrl+c
	}

	// callback after scp
	for i := range c.node.AfterCpCallbackShells {
		shell := c.node.AfterCpCallbackShells[i]
		time.Sleep(shell.Delay * time.Millisecond)
		stdinPipe.Write([]byte(shell.Cmd + "\r"))
	}

	// change stdin to user
	go func() {
		_, err = io.Copy(stdinPipe, os.Stdin)
		l.Error(err)
		session.Close()
	}()

	// interval get terminal size
	// fix resize issue
	go func() {
		var (
			ow = w
			oh = h
		)
		for {
			cw, ch, err := terminal.GetSize(fd)
			if err != nil {
				break
			}

			if cw != ow || ch != oh {
				err = session.WindowChange(ch, cw)
				if err != nil {
					break
				}
				ow = cw
				oh = ch
			}
			time.Sleep(time.Second)
		}
	}()

	// send keepalive
	go func() {
		for {
			time.Sleep(time.Second * 10)
			client.SendRequest("keepalive@openssh.com", false, nil)
		}
	}()

	session.Wait()
}
func copyFromServerToLocal(filePath, destinationPath string, client *ssh.Client) error {
	l.Infof("new sftp client")
	// open an SFTP session over an existing ssh connection.
	sftp, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftp.Close()

	l.Infof("sftp open file %s", filePath)
	// Open the source file
	srcFile, err := sftp.Open(filePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	l.Infof("create file in local %s", destinationPath+path.Base(filePath))
	// Create the destination file
	dstFile, err := os.Create(destinationPath + path.Base(filePath))
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy the file
	srcFile.WriteTo(dstFile)
	l.Infof("download file success!\n")

	return nil
}

func copyFromLocalToServer(filePath, destinationPath string, pipe io.WriteCloser) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	s, err := f.Stat()
	if err != nil {
		return err
	}
	return copy(s.Size(), s.Mode().Perm(), path.Base(filePath), f, destinationPath, pipe)
}

func copy(size int64, mode os.FileMode, fileName string, contents io.Reader, destination string, pipe io.WriteCloser) error {
	cmd := shellquote.Join("scp", "-t", destination)
	pipe.Write([]byte(cmd + "\r"))

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		fmt.Fprintf(pipe, "C%#o %d %s\n", mode, size, fileName)
		io.Copy(pipe, contents)
		fmt.Fprint(pipe, "\x00")
		fmt.Fprint(pipe, "\003")
		wg.Done()
	}()

	wg.Wait()
	return nil
}
