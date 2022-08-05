package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/nxadm/tail"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

type fileInfo struct {
	FileName string `json:"filename"`
	FileType string `json:"filetype"`
}

//type peerInfo struct {
//	peerID string
//	ipAddr []string
//}
type response struct {
	PeerResponses []*peerData `json:"Responses"`
}
type peerData struct {
	ID           string   `json:"ID"`
	Addresses    []string `json:"Addrs"`
	AccessedTime []string
}

func getPeerInfo(peerID string) *peerData {
	//http://127.0.0.1:5001/api/v0/dht/findpeer?arg=12D3KooWGsoKixZ7yhHBEHDt6v6sdK3eASsRY7uThATuwSnxyJXZ&verbose=false
	req, err := http.NewRequest("POST",
		fmt.Sprintf("http://127.0.0.1:5001/api/v0/dht/findpeer?arg=%s&verbose=false", peerID),
		nil)
	if err != nil {
		log.Printf(err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	// set timeout
	client.Timeout = time.Second * 15
	resp, err := client.Do(req)
	if err != nil {
		log.Printf(err.Error())
		return nil
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	peerRes := response{}
	err = json.Unmarshal(body, &peerRes)
	if err != nil {
		log.Printf("Error at unmarshal response %s \n%s", err, string(body))
		return nil
	}
	// get peer ID
	for _, peerInfo := range peerRes.PeerResponses {
		if peerInfo.ID == peerID {
			log.Printf("Peer %s has address %s", peerInfo.ID, peerInfo.Addresses)
			return peerInfo
		}
	}
	return nil
}
func savePeerInfo(filePath string, peerInfo *peerData) {
	peerDataBytes, _ := json.Marshal(peerInfo)
	err := ioutil.WriteFile(filePath, peerDataBytes, os.ModePerm)
	if err != nil {
		log.Printf("Failed to write peerData %s", peerInfo.ID)
		return
	}
	log.Printf("Saved peer info %s", peerInfo.ID)
}

func removeDuplicateValues(stringSlice []string) []string {
	keys := make(map[string]bool)
	var list []string

	// If the key(values of the slice) is not equal
	// to the already present value in new slice (list)
	// then we append it. else we jump on another element.
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
func processPeerInfo(peerID string, info *fileInfo, outDir string) {
	fileTypePath := path.Join(outDir, info.FileType, info.FileName)
	err := os.MkdirAll(fileTypePath, os.ModePerm)
	if err != nil {
		log.Printf("Failed create file folder %s", err)
		return
	}
	peerInfo := getPeerInfo(peerID)
	if peerInfo == nil {
		return
	}
	// store json
	peerFile := path.Join(fileTypePath, fmt.Sprintf("%s.json", peerID))
	if _, err := os.Stat(peerFile); err == nil {
		// case file exist
		existPeerInfo := peerData{}
		fileData, err := ioutil.ReadFile(peerFile)
		if err != nil {
			log.Printf("Failed to open exist peerInfo %s", peerID)
			savePeerInfo(peerFile, peerInfo)
			return
		}
		err = json.Unmarshal(fileData, &existPeerInfo)
		if err != nil {
			log.Printf("Failed to umarshal existed peer info %s", string(fileData))
			savePeerInfo(peerFile, peerInfo)
			return
		}
		mergedAddress := append(peerInfo.Addresses, existPeerInfo.Addresses...)
		peerInfo.Addresses = removeDuplicateValues(mergedAddress)
		peerInfo.AccessedTime = append(peerInfo.AccessedTime, existPeerInfo.AccessedTime...)
	}
	peerInfo.AccessedTime = append(peerInfo.AccessedTime, time.Now().String())
	// save update or untouched peerInfo
	savePeerInfo(peerFile, peerInfo)
}
func main() {
	// log level = send
	var logFile string
	var fileSaveDir string
	var databaseFile string
	log.SetOutput(os.Stdout)
	flag.StringVar(&logFile, "l", "", "log to read")
	flag.StringVar(&databaseFile, "i", "", "input database")
	flag.StringVar(&fileSaveDir, "d", "./data", "Output path for recorded peer")
	flag.Parse()

	// create dir
	err := os.MkdirAll(fileSaveDir, os.ModePerm)
	if err != nil {
		log.Printf("Failed create dir %s", err)
		os.Exit(-1)
	}
	db, err := ioutil.ReadFile(databaseFile)
	if err != nil {
		log.Printf("Failed load database json")
		os.Exit(-1)
	}
	var database map[string]*fileInfo
	err = json.Unmarshal(db, &database)
	if err != nil {
		log.Println(err)
	}
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
		// 12D3KooWGsoKixZ7yhHBEHDt6v6sdK3eASsRY7uThATuwSnxyJXZ
		index := strings.Index(text, "Cid")
		if index == -1 {
			log.Printf("Failed find Cid in %s", text)
			continue
		}
		newTextArray := strings.Split(text[index:], " ")
		cid := newTextArray[1]
		requestedPeer := newTextArray[5]
		log.Printf("Got cid %s request from %s", cid, requestedPeer)
		if val, ok := database[cid]; ok {
			// do something
			processPeerInfo(requestedPeer, val, fileSaveDir)
		}
	}
}
