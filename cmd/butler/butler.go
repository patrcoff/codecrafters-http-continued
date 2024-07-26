package main

import (
    "errors"
	"flag"
    "fmt"
	"os"
    "path/filepath"
    "strconv"

    "github.com/patrcoff/codecrafters-http-continued/pkg/server"
)


func return200(r server.Request) server.Response {
    return server.Response{Version: "HTTP/1.1", StatusCode: 200, StatusText: "OK"}
}

type serveDir struct {
    path string
}

func (p serveDir) returnFile(r server.Request) server.Response {
    //fmt.Println(r.pathParams)
    fn := r.PathParams["filename"]
    path := filepath.Join(p.path, fn)
    _, err := os.Stat(path)
    if r.Method == "GET" {
        if errors.Is(err,os.ErrNotExist) {
            return server.Response{Version: "HTTP/1.1", StatusCode: 404, StatusText: "Not Found"}
        }
        bd, err := os.ReadFile(path)
        if err != nil {
            return server.Response{Version: "HTTP/1.1", StatusCode: 500, StatusText: "Server Error"}
        }

        resp := server.Response{Version: "HTTP/1.1", StatusCode: 200, StatusText: "OK", Body: string(bd)}
    
        length := strconv.Itoa(len(bd))

        resp.AddHeader("Content-Type: application/octet-stream")
        resp.AddHeader("Content-Length: " + length)

        return resp
    } else if r.Method == "POST" {
        fmt.Println("file contents:\n", string(r.Body))
        err := os.WriteFile(path, r.Body, 0666)
        if err != nil {
            fmt.Println("Error savin file to server:\n", err)
            return server.Response{Version: "HTTP/1.1", StatusCode: 500, StatusText: "Server Error"}
        }
        
        return server.Response{Version: "HTTP/1.1", StatusCode: 201, StatusText: "Created"}
    } else {

        return server.Response{Version: "HTTP/1.1", StatusCode: 405, StatusText: "Method Not Allowed"}
    }
}

func returnUserAgent(r server.Request) server.Response {
    ua := server.GetHeader(r.Headers,"User-Agent")
    length := strconv.Itoa(len(ua))
    resp := server.Response{Version: "HTTP/1.1", StatusCode: 200, StatusText: "OK", Body: string(ua)}
    resp.AddHeader("Content-Type: text/plain")
    resp.AddHeader("Content-Length: " + length)

    return resp
}

func returnEcho(r server.Request) server.Response {
    echo := r.PathParams["echo"]
    length := strconv.Itoa(len(echo))
    resp := server.Response{Version: "HTTP/1.1", StatusCode: 200, StatusText: "OK", Body: echo}
    resp.AddHeader("Content-Type: text/plain")
    resp.AddHeader("Content-Length: " + length)


    return resp
}

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")
    dir := flag.String("directory", "", "directory which hosts files to serve")
    flag.Parse()

    s := server.MakeHttpServer("127.0.0.1", "4221", 1024, "")
    s.AddRoute("/",return200)
    s.AddRoute("/files/{filename}", serveDir{path: *dir}.returnFile)
    s.AddRoute("/user-agent", returnUserAgent)
    s.AddRoute("/echo/{echo}", returnEcho)
    s.Run()
}
