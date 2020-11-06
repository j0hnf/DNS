/*
 * John Fitzpatrick
 * @j0hn__f
 * 2020-04-14
 *
 * resolves dns fast... very fast using DoH
 *
 * build: $ go build resolve_dns-over-https.go
 * usage: $ ./resolve_dns-over-https <path-to-hostslist> <num-workers>
 *
 */

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
)

type DOHreply struct {
	Status   int  `json:"Status"`
	TC       bool `json:"TC"`
	RD       bool `json:"RD"`
	RA       bool `json:"RA"`
	AD       bool `json:"AD"`
	CD       bool `json:"CD"`
	Question []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
	} `json:"Question"`
	Answer []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Answer"`
}

func resolve(outputChannel chan string, hn chan string, done chan bool) {
	for true { // i don't know why but it sems to want to have this loop in order to work... dunno why
		hostname := <-hn
		ips := resolveDOH(hostname)
		for _, ip := range ips {
			outputChannel <- string(hostname + " " + ip.String())
		}
		done <- true
	}
}

//resolve hostname using dns over https
func resolveDOH(hostname string) []net.IP {

	// TODO: parallelise the below so the v6 lookup does not wait for the v4 lookup to return and process
	ip4 := resolveDOHipv4(hostname)
	ip6 := resolveDOHipv6(hostname)

	ips := append(ip4, ip6...)

	return ips

}

func resolveDOHipv4(hostname string) []net.IP {
	url := "https://dns.google/resolve?name=" + hostname + "&type=A&do=0" //do=0 says don't bother with dnssec stuff - i'm not using it right now

	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	var dohResponse DOHreply

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&dohResponse)
	if err != nil {
		fmt.Println("????? DNS: FAILED TO HANDLE JSON RESPONSE PROPERLY")
	}

	ips := []net.IP{}

	for _, answer := range dohResponse.Answer {

		if answer.Type == 1 {
			// we have an IPv4 address
			ips = append(ips, net.ParseIP(answer.Data))
		}
	}

	return ips
}
func resolveDOHipv6(hostname string) []net.IP {
	url := "https://dns.google/resolve?name=" + hostname + "&type=AAAA&do=0" //do=0 says don't bother with dnssec stuff - i'm not using it right now

	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	var dohResponse DOHreply

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&dohResponse)
	if err != nil {
		fmt.Println("????? DNS: FAILED TO HANDLE JSON RESPONSE PROPERLY")
	}

	ips := []net.IP{}

	for _, answer := range dohResponse.Answer {

		if answer.Type == 28 {
			// we have an IPv6 address
			ips = append(ips, net.ParseIP(answer.Data))
		}
	}

	return ips
}

/* output channel writer - anything writen to output channel ends up here for printing */
func outputData(outputChannel chan string) {

	for {
		data := <-outputChannel
		fmt.Println(data)
	}
}

func main() {

	outputChannel := make(chan string)
	go outputData(outputChannel)

	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open file")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var hostsList []string

	for scanner.Scan() {
		hostsList = append(hostsList, scanner.Text())
	}

	q := make(chan string)
	done := make(chan bool)

	numWorkers := 10

	// if there are fewer hosts to lookup than 10 don't start ten workers!!! (it'll deadlock)
	if len(hostsList) < 10 {
		numWorkers = len(hostsList)
	}

	// however, if there is a second argument use this to specify number of threads
	if len(os.Args) > 2 {

		numWorkers, err = strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalln("number of threads must be an integer\nExiting...\n")
		}

	}
	numberOfJobs := len(hostsList)

	for i := 0; i < numWorkers; i++ {
		go resolve(outputChannel, q, done)
		//fmt.Printf("finished worker %d\n", i)
	}

	for j := 0; j < numberOfJobs; j++ {
		hostname := hostsList[j]
		go func(host string) {
			//fmt.Println("called anon function with args: " + host)
			q <- host
			//fmt.Println("queued" + host)

		}(hostname)
	}

	//fmt.Printf("(i) loaded up %d jobs for execution\n", numberOfJobs)
	//fmt.Printf("(i) distributed %d jobs amongst %d workers\n\n", numberOfJobs, numWorkers)
	for c := 0; c < numberOfJobs; c++ {
		<-done
		//fmt.Println("got a done message")
	}

	//outputChannel <- "\nFinished!\n"

}
