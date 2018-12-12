package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alecthomas/template"
	"github.com/thejerf/abtime"
	"github.com/thejerf/sphyraena/elements/handlers"
	"github.com/thejerf/sphyraena/elements/handlers/dirserve"
	"github.com/thejerf/sphyraena/identity/auth/enticate/clauses"
	"github.com/thejerf/sphyraena/identity/auth/enticate/samples"
	"github.com/thejerf/sphyraena/identity/session"
	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/router"
	"github.com/thejerf/sphyraena/secret"
	"github.com/thejerf/sphyraena/sphyrw"
	"github.com/thejerf/suture"
)

var baseloc = flag.String("base", ".", "base location of the sample site")
var bind = flag.String("bind", ":10020", "bind specification for the server")
var username = flag.String("username", "user", "username for the site")
var password = flag.String("password", "password", "password for the site")

var templates *template.Template

func main() {
	flag.Parse()

	if len(*baseloc) > 0 && (*baseloc)[len(*baseloc)-1] == '/' {
		*baseloc = (*baseloc)[:len(*baseloc)-1]
	}

	var err error
	templates, err = template.ParseGlob(*baseloc + "/templates/*.tmpl")
	if err != nil {
		fmt.Printf("Could not parse templates from %s: %v\n", *baseloc+"/templates", err)
		os.Exit(1)
	}

	m := http.NewServeMux()

	supervisor := suture.NewSimple("sphyraena supervisor")
	sessionIDGenerator := session.NewSessionIDGenerator(128, nil)
	supervisor.Add(sessionIDGenerator)
	secretGenerator := secret.NewGenerator(128)
	supervisor.Add(secretGenerator)

	ramSessionServer := session.NewRAMServer(
		sessionIDGenerator, secretGenerator,
		&session.RAMSessionSettings{time.Minute * 180, abtime.NewRealTime()})
	r := router.New(request.NewSphyraenaState(ramSessionServer, nil))

	r.AddLocationForward("/public/", &dirserve.FileSystemServer{
		FileSystem:          http.Dir(*baseloc + "/public/"),
		ShowFile:            dirserve.StandardWebFiles,
		ServeSubdirectories: false,
		Index:               false,
		LegalMask:           os.FileMode(0777),
		BypassSendFile:      true,
	})

	hardCoded := samples.NewHardcodedAuth()
	hardCoded.AddUser(*username, *password)
	cookieAuth, _ := clauses.NewCookieAuth(
		router.NewRouteBlock(&router.ForwardClause{request.HandlerFunc(Login)}),
		hardCoded,
	)
	r.Add(cookieAuth)
	r.AddLocationForward("/samplerest", request.HandlerFunc(handlers.CounterOut))
	//	r.AddLocationForward("/socket/", sockjs.StreamingRESTHandler())
	r.AddLocationForward("/", request.HandlerFunc(Index))

	m.Handle("/", r)
	server := &http.Server{
		Addr:           *bind,
		MaxHeaderBytes: 1 << 20,
		Handler:        m,
	}

	fmt.Printf("Serving https://%s\n", *bind)
	go func() {
		err := server.ListenAndServeTLS("cert.pem", "key.pem")
		if err != nil {
			fmt.Printf("No longer serving: %v\n", err)
			panic(err)
		}
	}()

	supervisor.Serve()
}

type IndexType struct {
	Title    string
	StreamID string
}

func Index(rw *sphyrw.SphyraenaResponseWriter, req *request.Request) {
	sID, _ := req.StreamID()

	err2 := templates.ExecuteTemplate(rw, "index.tmpl",
		IndexType{"Index", string(sID)})

	if err2 != nil {
		fmt.Printf("Error while trying to build index page: %v\n", err2)
	}
}
