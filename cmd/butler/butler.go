package main

import (
    "bytes"
    "compress/gzip"
    "errors"
	"flag"
    "fmt"
    //"io"
	"net"
	"os"
    "path/filepath"
    //"regexp"
    "slices"
    "strconv"
    "strings"
)

// the below will end up in another file and be imported but I want to name it server and 
// the codecrafters repo names the main file server.go and I can't mess with that while
// completing the challenge.

// usage

// s := makeHttpServer(...args)
// s.addRoute(path,handler)
// ...
// s.Run()

// or create s with static path instead to serve static content

//type server interface {
//    Run() (int, error)
//    AddRoute(r route ) (int, error)
//} // overkill for now but may extend

type httpServer struct {
    // an http server based on net.Listen
    // takes address and port strings (for now) as well as
    // chunkSize - in bytes of read and writes to the connection
    // staticPath - the location of optional static content to serve
    // default behaviour is to serve the files ending in .html
    // accoring to their relative filepaths within staticPath
    
    address string
    port string
    chunkSize int
    staticPath string
    listener net.Listener
    routes []route
}

func makeHttpServer(a string, p string, c int, sp string) *httpServer {
    if a == "" {
        a = "0.0.0.0"
    }
    if p == "" {
        p = "4221"
    }
    if c == 0 {
        c = 1024
    }
    s := httpServer{address: a, port: p, chunkSize: c, staticPath: sp, routes: make([]route,0) }
    return &s
}

func (s *httpServer) Listen() (net.Listener, error) {
    l, err := net.Listen("tcp", s.address + ":" + s.port)

    return l, err
}

// func readRequest(c net.Conn) request 

func (s *httpServer) Run() (int, error) {
    
	l, err := s.Listen()
    defer l.Close()

	if err != nil {
		fmt.Println("Failed to bind to port 4221")
            os.Exit(1) // we'll want to do something better than this 
	}
    for {
        conn, err := l.Accept()
        defer conn.Close()
        if err != nil {
		    fmt.Println("Error accepting connection: ", err.Error())
            os.Exit(1)
	    }
        
        go func(c net.Conn) {
            req := make([]byte, 0)
            intermediary := make([]byte, s.chunkSize)
            length := 0
            for {

                reqBytes, err := c.Read(intermediary)
                if err != nil { // read doesn't give error when stream ends it seems
                    // we should do something better here for unexpected errors
                    fmt.Println("Error reading request:", err.Error())
                    break
                }
                req = slices.Concat(req, intermediary[:reqBytes])
                length += reqBytes
                if reqBytes < s.chunkSize {
                    break
                }
            }


            r := s.buildRequestFromRaw(req)
            

            resp := r.handler(r)
            encodings := strings.Split(r.headers["Accept-Encoding"], ",")
            for i := range len(encodings) {
                encoding := strings.Trim(encodings[i], " ")
                if encoding == "gzip" {
                    resp.AddHeader("Content-Encoding: gzip")
                    resp.body = gzComp(resp.body)
                    fmt.Println(resp.body)
                    resp.headers["Content-Length"] = strconv.Itoa(len(resp.body))
                    break
                } // add else ifs for any other encodings to be supported
            }
            // -----------------------------------------------------
            fmt.Println("Response:")
            fmt.Println(string(resp.RawBytes()))
            fmt.Println("End Response\n")
            c.Write(resp.RawBytes())
            c.Close()

        }(conn)
    }
            
}

func gzComp(b string) string {
    buf := new(bytes.Buffer)
    compWriter := gzip.NewWriter(buf)
    compWriter.Write([]byte(b))
    compWriter.Close()
    str := buf.String()
    return str
}

func (s *httpServer) staticHandler(r request) response {
    //fmt.Println("Not Implemented!")
    if s.staticPath == "" {
        return response{version: "HTTP/1.1", statusCode: 404, statusText: "Not Found"}
    } else { 
        return response{version: "HTTP/1.1", statusCode: 505, statusText: "Not Implimented"}
    }

}

//func (s *httpServer) matchPath() route {

//}

func (s httpServer) buildRequestFromRaw(b []byte) request {
    rqln, hdrs, body := parseRequest(b[:]) // do I need to specify the length?
    method, target, version := parseRequestLine(rqln)
    hdmap := make(map[string]string)
    for i := 0; i < len(hdrs); i++ {
        fmt.Println(string(hdrs[i]))
        hdmap[strings.Split(string(hdrs[i]), ": ")[0]] = strings.Split(string(hdrs[i]), ": ")[1]
    }
    
    qp := make(map[string]string)
    pp := make(map[string]string)

    params := make([]string, 0)
    if strings.Contains(string(target),"?") {
        params = strings.Split(strings.Split(string(target), "?")[1], ",")
    }

    for i := 0; i < len(params); i++ {
        param := strings.Split(params[i], "=")
        qp[param[0]] = param[1]
    }

    // match against route

    noQuery := strings.Split(string(target), "?")[0]
    pathParts := strings.Split(noQuery, "/")
    
    indexes := make([]int, 0)
    match := -1
    for i := 0; i < len(s.routes); i++ {
        route := s.routes[i].path
        routeParts := strings.Split(route, "/")
        if len(routeParts) != len(pathParts) {
            continue
        }
        b := 0
        for j := 0; j < len(routeParts); j++ {
            if strings.Contains(routeParts[j], "{") {
                indexes = append(indexes, j)
                continue
            } else if routeParts[j] != pathParts[j] {
                b++
                indexes = make([]int, 0)
            }
        }

        if b == 0 {
            match = i
            break
        }


    }
    handler := s.staticHandler
    if match >= 0 {
        // we have a matching route, at index i of s.routes
        // we will also have potentially some indexes for pathParams
        handler = s.routes[match].handler

        for i := 0; i < len(indexes); i++ {
            index := indexes[i]
            paramKey := strings.Split(s.routes[match].path, "/")[index]
            paramKey = paramKey[1:len(paramKey) - 1]
            paramVal := strings.Split(noQuery, "/")[index]
            pp[paramKey] = paramVal 
        }
    }

    return request{raw: b, reqLine: rqln, headers: hdmap, body: body, method: string(method[:]), target: string(target[:]), version: string(version[:]), pathParams: pp, queryParams: qp, handler: handler}

}

type request struct {
    raw []byte
    
    reqLine []byte
    headers map[string]string
    body []byte
    method string
    target string
    version string

    pathParams map[string]string
    queryParams map[string]string

    handler
}

type response struct {
    //raw []byte
    
    version string
    statusCode int
    statusText string
    
    headers map[string]string
    body string 
}

func (r response) RawString() string {
    statusLine := r.version + " " + strconv.Itoa(r.statusCode) + " " + r.statusText + "\r\n"
    headers := ""
    for name, value := range r.headers {
        headers += name + ": " + value + "\r\n"
    }
     headers += "\r\n"
    //fmt.Println(statusLine + headers + r.body)
    return statusLine + headers + r.body
}

func (r response) RawBytes() []byte {
    return []byte(r.RawString())
}

func (r *response) AddHeader(h string) error {
    // do some validation
    if r.headers == nil {
        r.headers = make(map[string]string)
    }
    r.headers[strings.Split(h,":")[0]] = strings.Split(h, ":")[1]

    return nil
}


type handler func(request) response // functions passed by user to map to routes

type route struct {
    path string
    handler handler
}

func (s *httpServer) AddRoute(r string, h handler) {
    rt := route{path: r, handler: h}
    s.routes = append(s.routes, rt)
}


func parseRequest(h []byte) ([]byte, [][]byte, []byte) {
    // split http request into 3 parts
    // requesta line
    // headers
    // body
    
    parts := bytes.Split(h, []byte("\r\n"))
    last := len(parts) - 1
    reqLine := parts[0]
    headers := parts[1:last-1]
    body := parts[last]

    return reqLine, headers, body

}

func parseRequestLine(l []byte) ([]byte, []byte, []byte) {
    // split the reqline part of an http request into
    // method
    // target
    // httpVersion
    
    parts := bytes.Split(l, []byte(" "))
    method := parts[0]
    target := parts[1]
    version := parts[2]

    return method, target, version

}

func getHeader(h map[string]string, n string) string {
    return h[n]
    // we might modify error handling for this
    // or we might just remove this func
}

func return200(r request) response {
    return response{version: "HTTP/1.1", statusCode: 200, statusText: "OK"}
}

type serveDir struct {
    path string
}

func (p serveDir) returnFile(r request) response {
    //fmt.Println(r.pathParams)
    fn := r.pathParams["filename"]
    path := filepath.Join(p.path, fn)
    _, err := os.Stat(path)
    if r.method == "GET" {
        if errors.Is(err,os.ErrNotExist) {
            return response{version: "HTTP/1.1", statusCode: 404, statusText: "Not Found"}
        }
        bd, err := os.ReadFile(path)
        if err != nil {
            return response{version: "HTTP/1.1", statusCode: 500, statusText: "Server Error"}
        }

        resp := response{version: "HTTP/1.1", statusCode: 200, statusText: "OK", body: string(bd)}
    
        length := strconv.Itoa(len(bd))

        resp.AddHeader("Content-Type: application/octet-stream")
        resp.AddHeader("Content-Length: " + length)

        return resp
    } else if r.method == "POST" {
        fmt.Println("file contents:\n", string(r.body))
        err := os.WriteFile(path, r.body, 0666)
        if err != nil {
            fmt.Println("Error savin file to server:\n", err)
            return response{version: "HTTP/1.1", statusCode: 500, statusText: "Server Error"}
        }
        
        return response{version: "HTTP/1.1", statusCode: 201, statusText: "Created"}
    } else {

        return response{version: "HTTP/1.1", statusCode: 405, statusText: "Method Not Allowed"}
    }
}

func returnUserAgent(r request) response {
    ua := getHeader(r.headers,"User-Agent")
    length := strconv.Itoa(len(ua))
    resp := response{version: "HTTP/1.1", statusCode: 200, statusText: "OK", body: string(ua)}
    resp.AddHeader("Content-Type: text/plain")
    resp.AddHeader("Content-Length: " + length)

    return resp
}

func returnEcho(r request) response {
    echo := r.pathParams["echo"]
    length := strconv.Itoa(len(echo))
    resp := response{version: "HTTP/1.1", statusCode: 200, statusText: "OK", body: echo}
    resp.AddHeader("Content-Type: text/plain")
    resp.AddHeader("Content-Length: " + length)


    return resp
}

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")
    dir := flag.String("directory", "", "directory which hosts files to serve")
    flag.Parse()

    s := makeHttpServer("127.0.0.1", "4221", 1024, "")
    s.AddRoute("/",return200)
    s.AddRoute("/files/{filename}", serveDir{path: *dir}.returnFile)
    s.AddRoute("/user-agent", returnUserAgent)
    s.AddRoute("/echo/{echo}", returnEcho)
    s.Run()
}
