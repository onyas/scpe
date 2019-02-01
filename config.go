package scpe

import (
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os/user"
	"path"
	"time"
)

type Node struct {
	Name           string           `yaml:"name"`
	Host           string           `yaml:"host"`
	User           string           `yaml:"user"`
	Port           int              `yaml:"port"`
	KeyPath        string           `yaml:"keypath"`
	Passphrase     string           `yaml:"passphrase"`
	Password       string           `yaml:"password"`
	BeforeCpCallbackShells []*CallbackShell `yaml:"before-cp-callback-shells"`
	AfterCpCallbackShells []*CallbackShell `yaml:"after-cp-callback-shells"`
	Children       []*Node          `yaml:"children"`
}

type CallbackShell struct {
	Cmd   string        `yaml:"cmd"`
	Delay time.Duration `yaml:"delay"`
}

func (n *Node) String() string {
	return n.Name
}

func (n *Node) user() string {
	if n.User == "" {
		return "root"
	}
	return n.User
}

func (n *Node) port() int {
	if n.Port <= 0 {
		return 22
	}
	return n.Port
}

func (n *Node) password() ssh.AuthMethod {
	if n.Password == "" {
		return nil
	}
	return ssh.Password(n.Password)
}

var (
	config []*Node
)

func GetConfig() []*Node {
	return config
}

func LoadConfig() error {
	b, err := LoadConfigBytes(".scpe", ".scpe.yml", ".scpe.yaml")
	if err != nil {
		return err
	}
	var c []*Node
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return err
	}

	config = c

	return nil
}

func LoadConfigBytes(names ...string) ([]byte, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	// homedir
	for i := range names {
		scpe, err := ioutil.ReadFile(path.Join(u.HomeDir, names[i]))
		if err == nil {
			return scpe, nil
		}
	}
	// relative
	for i := range names {
		scpe, err := ioutil.ReadFile(names[i])
		if err == nil {
			return scpe, nil
		}
	}
	return nil, err
}