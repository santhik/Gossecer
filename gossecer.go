package main

import (
	"github.com/nohupped/GoLogger"
	"Gossecer/modules"
	"flag"
	"sync"
	"os"
	"regexp"
	"strconv"
)
func main() {
	// Flags and Variable declaration
	mylogger := GoLogger.New("/var/log/gossecer.log")
	defer mylogger.Close()
	hostname, err := os.Hostname()
	modules.CheckError(err)
	configfileparam := flag.String("config", "/etc/gossec.conf", "The program's main configuration file")
	flag.Parse()
	// Main config file
	configfile, err := modules.GetConfig(*configfileparam)
	modules.CheckError(err)
	// Get OSSec's configfile
	OSSecConfGlobal, err := configfile.GetSection("ossec")
	modules.CheckError(err)
	ossecConf, err := OSSecConfGlobal.GetKey("ConfFile")
	modules.CheckError(err)

	// Get Redis Config file
	RedisConfGlobal, err := configfile.GetSection("redis")
	redisServer, err := RedisConfGlobal.GetKey("Server")
	modules.CheckError(err)
	redisPort, err := RedisConfGlobal.GetKey("Port")
	modules.CheckError(err)

	// Filters
	FiltersGlobal, err := configfile.GetSection("filters")
	modules.CheckError(err)
	filters_keys := FiltersGlobal.Keys()
	var regexps []*regexp.Regexp

	for _, i := range filters_keys {
		regexps = append(regexps, regexp.MustCompile(i.MustString("")))
	}

	// Expire
	ExpireGlobal, err := configfile.GetSection("expire")
	modules.CheckError(err)
	expire_keys := ExpireGlobal.KeysHash()

	keys := make([]modules.Key, 0)
	if len(expire_keys) == 0 {
		keys = nil
	} else {
		for k, v := range expire_keys {
			tmpkey := modules.Key{}
			tmpconvertedkey, err := strconv.Atoi(k)
			modules.CheckError(err)
			tmpconvertedval, err := strconv.Atoi(v)
			modules.CheckError(err)
			tmpkey[tmpconvertedkey] = tmpconvertedval
			keys = append(keys, tmpkey)
		}
	}
	//var expire []string


	// End of variable declaration

	mylogger.Info.Println("Parsing ", ossecConf.String())
	host, ip :=modules.GetOSSecConfig(ossecConf.MustString("/var/ossec/etc/ossec.conf"))

	mylogger.Info.Println("Starting UDP server on ", host, ip, "for", hostname)
	itemschan := make(chan *modules.Jsondata, 1024)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	// Starts a udp server on the respective IP/PORT extracted from ossec config, and writes the datagrams to the
	// channel itemschan, all from a separate goroutine
	go modules.StartUdpServer(host, ip, hostname, itemschan, wg)

	// The below closure will read continuously from the itemschan in another separate goroutine, normalize it, and
	// puts it to redis applying the normalizing filters and ttl.
	go func() {
		for ; ; {
			modules.PutToRedis(redisServer.MustString("localhost"), redisPort.MustString("6379"),
				regexps, keys, itemschan)
		}
	}()
	wg.Wait()
}