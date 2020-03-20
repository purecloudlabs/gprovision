package main

import (
	"flag"
	"gprovision/pkg/log"
	"gprovision/pkg/oss/pblog/server"
)

var dbfile string

func main() {
	log.AddConsoleLog(0)
	log.FlushMemLog()
	flag.StringVar(&dbfile, "db", "./pb.db", "path to database")
	server.Flags()
	srvr := server.NewServer(dbfile)
	log.Logf("starting server on %s...", server.Port)

	srvr.Serve()
}
