package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/nyxtom/broadcast/client/go/broadcast"
)

func main() {
	var ip = flag.String("h", "127.0.0.1", "broadcast server ip (default 127.0.0.1)")
	var port = flag.Int("p", 7331, "broadcast server port (default 7331)")
	var maxIdle = flag.Int("i", 1, "max idle client connections to pool from")

	flag.Parse()

	addr := *ip + ":" + strconv.Itoa(*port)
	c, err := broadcast.NewClient(*port, *ip, *maxIdle)
	if err != nil {
		fmt.Printf(err.Error())
		os.Exit(1)
	}

	SetCompletionHandler(completionHandler)
	setHistoryCapacity(100)

	reg, _ := regexp.Compile(`'.*?'|".*?"|\S+`)
	prompt := ""

	for {
		prompt = fmt.Sprintf("%s>", addr)

		cmd, err := line(prompt)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			return
		}

		cmds := reg.FindAllString(cmd, -1)
		if len(cmds) == 0 {
			continue
		} else {
			addHistory(cmd)

			args := make([]interface{}, len(cmds[1:]))
			for i := range args {
				item := strings.Trim(string(cmds[1+i]), "\"'")
				if a, err := strconv.Atoi(item); err == nil {
					args[i] = a
				} else if a, err := strconv.ParseFloat(item, 64); err == nil {
					args[i] = a
				} else if a, err := strconv.ParseBool(item); err == nil {
					args[i] = a
				} else if len(item) == 1 {
					b := []byte(item)
					args[i] = b[0]
				} else {
					args[i] = item
				}
			}

			cmd := cmds[0]
			if strings.ToLower(cmd) == "help" || cmd == "?" {
				printHelp(cmds)
			} else {
				reply, err := c.Do(cmds[0], args...)
				if err != nil {
					fmt.Printf("%s", err.Error())
				} else {
					printReply(cmd, reply)
				}

				fmt.Printf("\n")
			}
		}
	}
}

func printReply(cmd string, reply interface{}) {
	switch reply := reply.(type) {
	case int64:
		fmt.Printf("(integer) %d", reply)
	case float64:
		fmt.Printf("(float) %f", reply)
	case string:
		fmt.Printf("%s", reply)
	case []byte:
		fmt.Printf("%q", reply)
	case nil:
		fmt.Printf("(nil)")
	case error:
		fmt.Printf("%s", string(reply.Error()))
	case []interface{}:
		for i, v := range reply {
			fmt.Printf("%d) ", i+1)
			printReply(cmd, v)
			if i != len(reply)-1 {
				fmt.Printf("\n")
			}
		}
	}
}

func printGenericHelp() {
	msg :=
		`broadcast-cli
Type:   "help <command>" for help on <command>
    `
	fmt.Println(msg)
}

func printCommandHelp(arr []string) {
	fmt.Println()
	fmt.Printf("\t%s %s \n", arr[0], arr[1])
	fmt.Printf("\tGroup: %s \n", arr[2])
	fmt.Println()
}

func printHelp(cmds []string) {
	args := cmds[1:]
	if len(args) == 0 {
		printGenericHelp()
	} else if len(args) > 1 {
		fmt.Println()
	} else {
		cmd := strings.ToUpper(args[0])
		for i := 0; i < len(helpCommands); i++ {
			if helpCommands[i][0] == cmd {
				printCommandHelp(helpCommands[i])
			}
		}
	}
}

func completionHandler(in string) []string {
	var keywords []string
	for _, i := range helpCommands {
		if strings.HasPrefix(i[0], strings.ToUpper(in)) {
			keywords = append(keywords, i[0])
		}
	}
	return keywords
}
