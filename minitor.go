package main

import (
	"flag"
	"fmt"
	"github.com/nxadm/tail"
	"log"
	"os"
	"strings"
)

func main() {
	// log level = send
	var logFile string
	var fileSaveDir string
	log.SetOutput(os.Stdout)
	flag.StringVar(&logFile, "l", "", "Ipfs-search log to read")
	flag.StringVar(&fileSaveDir, "d", "./data", "Output path for recorded peer")
	flag.Parse()
	if logFile == "" {
		fmt.Errorf("please specifiy ipfs-search log")
		os.Exit(-1)
	}
	t, err := tail.TailFile(logFile, tail.Config{Follow: true})
	log.Printf("Started tailing file %s, saving dir %s", logFile, fileSaveDir)
	if err != nil {
		panic(err)
	}
	// process log
	for line := range t.Lines {
		text := line.Text
		if text == "" {
			continue
		}
		if !strings.Contains(text, "decision/engine.go") {
			continue
		}
		// get index
		// 2022-08-03T23:15:32.439-0400    WARN    send    decision/engine.go:706
		// Cid QmcxBHPosFDEA424nekX7sscXdEAe2GZ8A48Xh3iVsPt7R wanted from peer 12D3KooWQkTeaV32KPbNGbpF4NgyJk2hgvLJRDcf5tUB2wbef2Fy
		index := strings.Index(text, "Cid")
		if index == -1 {
			log.Printf("Failed find Cid in %s", text)
			continue
		}
		newTextArray := strings.Split(text[index:], " ")
		cid := newTextArray[1]
		requestedPeer := newTextArray[5]
		log.Printf("Got cid %s request from %s", cid, requestedPeer)
	}
}
