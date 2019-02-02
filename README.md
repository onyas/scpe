# scpe


scp client enhanced for automatic execute command you want.


## install

use `go get`

```
go get -u github.com/onyas/scpe/cmd/scpe
```

or download binary from [releases](//github.com/onyas/scpe/releases).

## config

put config file in `~/.scpe` or `~/.scpe.yml` or `~/.scpe.yaml` or `./.scpe` or `./.scpe.yml` or `./.scpe.yaml`.

config example:

```yaml
- { name: dev server fully configured, user: appuser, host: 192.168.8.35, port: 22, password: 123456 }
- { name: dev server with key path, user: appuser, host: 192.168.8.35, port: 22, keypath: /root/.ssh/id_rsa }
- { name: dev server with passphrase key, user: appuser, host: 192.168.8.35, port: 22, keypath: /root/.ssh/id_rsa, passphrase: abcdefghijklmn}
- { name: dev server without port, user: appuser, host: 192.168.8.35 }
- { name: dev server without user, host: 192.168.8.35 }
- { name: dev server without password, host: 192.168.8.35 }
- { name: ⚡️ server with emoji name, host: 192.168.8.35 }

# server group 1
- name: server group 1
  children:
  - { name: server 1, user: root, host: 192.168.1.2 }
  - { name: server 2, user: root, host: 192.168.1.3 }
  - { name: server 3, user: root, host: 192.168.1.4 }

# server group 2
- name: server group 2
  children:
  - { name: server 1, user: root, host: 192.168.2.2 }
  - { name: server 2, user: root, host: 192.168.3.3 }
  - { name: server 3, user: root, host: 192.168.4.4 }

# before-cp-callback-shells
- name: dev server fully configured
  user: appuser
  host: 192.168.8.35
  port: 22
  password: 123456
  before-cp-callback-shells:
  - {cmd: 2}
  - {delay: 1500, cmd: 0}
  - {cmd: 'echo 1'}
  
 ```
