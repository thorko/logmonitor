package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/MiLk/kingpin"
	config "github.com/micro/go-config"
)

var (
	command     string
	key         string
	optConfPath string
)

func main() {
	kingpin.Flag("command", "Command to run: get, list (list all keys), reset").PlaceHolder("get").Short('x').Required().StringVar(&command)
	kingpin.Flag("key", "key to get or reset").Short('k').StringVar(&key)
	kingpin.Flag("configfile", "Path to config file").PlaceHolder("/etc/logmonitor/config.yaml").Default("/etc/logmonitor/config.yaml").Short('c').StringVar(&optConfPath)
	kingpin.CommandLine.HelpFlag.Hidden()
	kingpin.Parse()

	config.LoadFile(optConfPath)
	conf := config.Map()

	//fmt.Printf("%+v", patterns)
	//os.Exit(0)
	rr := make(map[string]string)
	for k, v := range conf["Patterns"].(map[string]interface{}) {
		rr[k] = v.(string)
	}

	if command == "list" {
		for u := range rr {
			fmt.Printf("%s\n", u)
		}
		os.Exit(0)
	}

	address := conf["Daemon"].(map[string]interface{})["Listen"].(string)

	switch command {
	case "get":
		if len(key) == 0 {
			fmt.Printf("Key is required")
			os.Exit(1)
		} else {
			conn, err := net.Dial("tcp", address)
			if err != nil {
				fmt.Printf("Couldn't connect to %s, err: %s\n", address, err)
				os.Exit(1)
			} else {
				if key == "all" {
					fmt.Fprintf(conn, "get "+key+"\n")
					response, err := bufio.NewReader(conn).ReadString(';')
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					} else {
						fmt.Printf("%s", strings.Trim(response, ";"))
					}
				} else {
					fmt.Fprintf(conn, "get "+key+"\n")
					response, err := bufio.NewReader(conn).ReadString('\n')
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					} else {
						fmt.Printf("%s", response)
					}
				}
			}
		}
	case "reset":
		conn, err := net.Dial("tcp", address)
		if err != nil {
			fmt.Printf("Couldn't connect to %s, err: %s\n", address, err)
			os.Exit(1)
		} else {
			fmt.Fprintf(conn, "reset "+key+"\n")
			response, err := bufio.NewReader(conn).ReadString('\n')
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			} else {
				fmt.Println(response)
			}
		}
	}
}
