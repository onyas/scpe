package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	"scpe"
	"runtime"
)

const prev = "-parent-"

var (
	Build      = "devel"
	V          = flag.Bool("version", false, "show version")
	H          = flag.Bool("help", false, "show help")
	FromServer = flag.String("fromServer", "", "name the source file, only work with ToLocal")
	ToLocal    = flag.String("toLocal", "", "name the target file, only work with FromServer")
	FromLocal  = flag.String("fromLocal", "", "name the source file, only work with ToServer")
	ToServer   = flag.String("toServer", "", "name the target file, only work with FromLocal")

	log = scpe.GetLogger()

	templates = &promptui.SelectTemplates{
		Label:    "✨ {{ . | green}}",
		Active:   "➤ {{ .Name | cyan }} {{if .Host}}{{if .User}}{{.User | faint}}{{`@` | faint}}{{end}}{{.Host | faint}}{{end}}",
		Inactive: "  {{.Name | faint}} {{if .Host}}{{if .User}}{{.User | faint}}{{`@` | faint}}{{end}}{{.Host | faint}}{{end}}",
	}
)

func main() {

	if parseArgsFail() {
		return
	}

	scpType := 0
	if len(*FromServer) != 0 && len(*ToLocal) != 0 {
		scpType = 0
	}
	if len(*ToServer) != 0 && len(*FromLocal) != 0 {
		scpType = 1
	}

	err := scpe.LoadConfig()
	if err != nil {
		log.Error("load config error", err)
		os.Exit(1)
	}

	node := choose(nil, scpe.GetConfig())
	if node == nil {
		return
	}

	client := scpe.NewClient(node)
	if scpType == 0 {
		client.StartScp(*FromServer, *ToLocal, scpType)
	} else {
		client.StartScp(*FromLocal, *ToServer, scpType)
	}
}

func parseArgsFail() bool {
	flag.Parse()
	if !flag.Parsed() {
		return true
	}

	if *H {
		flag.Usage()
		return true
	}

	if *V {
		fmt.Println("scpe - scp client enhanced for automatic execute command you want")
		fmt.Println("  git version:", Build)
		fmt.Println("  go version :", runtime.Version())
		return true
	}

	fmt.Print("fromLocal" + *FromLocal)

	if len(*FromServer) == 0 && len(*FromLocal) == 0 {
		flag.Usage()
		return true
	}

	if *ToLocal == "" && *ToServer == "" {
		flag.Usage()
		return true
	}

	if *FromServer != "" && *FromLocal != "" {
		flag.Usage()
		return true
	}

	if *ToLocal != "" && *ToServer != "" {
		flag.Usage()
		return true
	}
	return false
}

func choose(parent, trees []*scpe.Node) *scpe.Node {
	prompt := promptui.Select{
		Label:     "select host",
		Items:     trees,
		Templates: templates,
		Size:      20,
		Searcher: func(input string, index int) bool {
			node := trees[index]
			content := fmt.Sprintf("%s %s %s", node.Name, node.User, node.Host)
			if strings.Contains(input, " ") {
				for _, key := range strings.Split(input, " ") {
					key = strings.TrimSpace(key)
					if key != "" {
						if !strings.Contains(content, key) {
							return false
						}
					}
				}
				return true
			}
			if strings.Contains(content, input) {
				return true
			}
			return false
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		return nil
	}

	node := trees[index]
	if len(node.Children) > 0 {
		first := node.Children[0]
		if first.Name != prev {
			first = &scpe.Node{Name: prev}
			node.Children = append(node.Children[:0], append([]*scpe.Node{first}, node.Children...)...)
		}
		return choose(trees, node.Children)
	}

	if node.Name == prev {
		return choose(nil, parent)
	}

	return node
}
