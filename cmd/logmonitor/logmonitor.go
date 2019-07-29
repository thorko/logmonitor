package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"

	"strconv"
	"syscall"
	"time"

	"github.com/MiLk/kingpin"
	"github.com/h2so5/goback/regexp"
	"github.com/hpcloud/tail"
	config "github.com/micro/go-config"
	"github.com/rapidloop/skv"
)

var (
	command     string
	optConfPath string
)

func incrementKey(dbFile, key string) bool {
	// open db
	store, err := skv.Open(dbFile)
	if err != nil {
		log.Fatalf("Couldn't open db file: %s", err)
		return false

	}

	var old int
	err = store.Get(key, &old)
	if err != nil {
		// key empty will initialise it
		store.Put(key, 1)
		log.Printf("key: %s => %d", key, 1)
		store.Close()
		return true
	}
	old++
	log.Printf("key: %s => %d", key, old)
	err = store.Put(key, old)
	if err != nil {
		log.Printf("Couldn't save key: %s, err: %s", key, err)
	}
	store.Close()
	return true
}

// resetting all counter
func resetAllCounter(dbFile string, counter map[string]string) error {
	// open db
	store, err := skv.Open(dbFile)
	if err != nil {
		log.Fatalf("Couldn't open db file: %s", err)
		return err

	}

	for u := range counter {
		err = store.Put(u, 0)
		if err != nil {
			store.Close()
			return err
		}
	}
	store.Close()
	return nil
}

func handleConnection(c net.Conn, dbFile string, rr map[string]string) {
	log.Printf("Serving %s\n", c.RemoteAddr().String())
	for {
		netData, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			log.Println(err)
			return
		}

		store, err := skv.Open(dbFile)
		if err != nil {
			c.Write([]byte(fmt.Sprintf("Couldn't open db file\n")))

		}
		request := strings.Fields(string(netData))

		if len(request) < 2 {
			c.Write([]byte(string("get <key>\n")))
		} else {
			value := 0
			if request[0] == "get" {
				if request[1] == "all" {
					// loop through all keys in config
					log.Printf("Get all keys")
					ts := time.Now().Unix()
					for key, _ := range rr {
						store.Get(key, &value)
						// write influxdb_line
						c.Write([]byte(fmt.Sprintf("logstats,pattern=%s count=%d %d\n", key, value, ts)))
					}
					c.Write([]byte(fmt.Sprintf(";")))
				} else {
					log.Printf("get key: %s", request[1])
					store.Get(request[1], &value)
					c.Write([]byte(fmt.Sprintf("%d\n", value)))
				}
			}
			if request[0] == "reset" {
				log.Printf("resetting key: %s", request[1])
				store.Put(request[1], 0)
				c.Write([]byte(fmt.Sprintf("success\n")))
			}
		}

		store.Close()
		c.Close()
	}

}

func main() {
	kingpin.Flag("command", "Command to run: stop, reload, reset").Short('x').StringVar(&command)
	kingpin.Flag("configfile", "Path to config file").PlaceHolder("/etc/logmonitor/config.yaml").Default("/etc/logmonitor/config.yaml").Short('c').StringVar(&optConfPath)
	kingpin.CommandLine.HelpFlag.Hidden()
	kingpin.Parse()

	config.LoadFile(optConfPath)
	conf := config.Map()

	logfile := conf["Log"].(map[string]interface{})["File"].(string)
	file, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()
	log.SetOutput(file)

	pidFile := conf["Daemon"].(map[string]interface{})["Pid"].(string)
	dbFile := conf["DB"].(map[string]interface{})["File"].(string)

	var pid []byte
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		pid = []byte(fmt.Sprintf("%d", os.Getpid()))
		ioutil.WriteFile(pidFile, pid, 0664)
	} else {
		if piddata, err := ioutil.ReadFile(pidFile); err == nil {
			if len(piddata) != 0 {
				pid = piddata
			}
		} else {
			log.Printf("Couldn't read pidfile: %s", pidFile)
			os.Exit(1)
		}
	}

	if command == "reload" {
		pp, _ := strconv.Atoi(string(pid))
		syscall.Kill(pp, syscall.SIGHUP)
		os.Exit(0)
	}

	if command == "stop" {
		pp, _ := strconv.Atoi(string(pid))
		syscall.Kill(pp, syscall.SIGTERM)
		os.Exit(0)
	}
	if command == "reset" {
		pp, _ := strconv.Atoi(string(pid))
		syscall.Kill(pp, syscall.SIGUSR1)
		os.Exit(0)
	}

	log.Printf("Pid: %s", pid)

	// make a map of patterns
	rr := make(map[string]string)
	for k, v := range conf["Patterns"].(map[string]interface{}) {
		rr[k] = v.(string)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	signal.Notify(ch, syscall.SIGUSR1)
	signal.Notify(ch, syscall.SIGINT)
	signal.Notify(ch, syscall.SIGTERM)

	go func() {
		for {
			s := <-ch
			switch s {
			case syscall.SIGHUP:
				conf = config.Map()
				log.Println("Config reloaded")
			case syscall.SIGUSR1:
				resetAllCounter(dbFile, rr)
				log.Println("Resetted all counter")
			case syscall.SIGTERM:
				log.Println("Stopping daemon")
				os.Remove(pidFile)
				time.Sleep(2 * time.Second)
				os.Exit(0)
			}
		}
	}()

	var l net.Listener
	log.Printf("Listening on %s", conf["Daemon"].(map[string]interface{})["Listen"].(string))
	l, err = net.Listen("tcp", conf["Daemon"].(map[string]interface{})["Listen"].(string))
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				log.Printf("Error connecting: %s", err)
				return
			}
			handleConnection(c, dbFile, rr)
		}

	}()

	seek := &tail.SeekInfo{Whence: io.SeekEnd}

	log.Printf("Reading: %s", conf["Watch"].(map[string]interface{})["File"].(string))
	t, err := tail.TailFile(conf["Watch"].(map[string]interface{})["File"].(string), tail.Config{Location: seek, Follow: true})
	for line := range t.Lines {
		for key, pattern := range rr {
			if r, _ := regexp.MatchString(pattern, line.Text); r {
				incrementKey(dbFile, key)
			}
		}
	}
}
