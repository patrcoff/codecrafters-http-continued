package server

import (
    "bytes"
    "compress/gzip"
    //"errors"
	//"flag"
    "fmt"
    //"io"
	"net"
	"os"
    //"path/filepath"
    //"regexp"
    "slices"
    "strconv"
    "strings"
    //"testing"
)


type HttpServer struct {
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

func MakeHttpServer(a string, p string, c int, sp string) *HttpServer {
    if a == "" {
        a = "0.0.0.0"
    }
    if p == "" {
        p = "4221"
    }
    if c == 0 {
        c = 1024
    }
    s := HttpServer{address: a, port: p, chunkSize: c, staticPath: sp, routes: make([]route,0) }
    return &s
}

func (s *HttpServer) Listen() (net.Listener, error) {
    l, err := net.Listen("tcp", s.address + ":" + s.port)

    return l, err
}

// func readRequest(c net.Conn) Request 

func (s *HttpServer) Run() (int, error) {
    
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
                    fmt.Println("Error reading Request:", err.Error())
                    break
                }
                req = slices.Concat(req, intermediary[:reqBytes])
                length += reqBytes
                if reqBytes < s.chunkSize {
                    break
                }
            }


            r := s.BuildRequestFromRaw(req)
            

            resp := r.Handler(r)
            encodings := strings.Split(r.Headers["Accept-Encoding"], ",")
            for i := range len(encodings) {
                encoding := strings.Trim(encodings[i], " ")
                if encoding == "gzip" {
                    resp.AddHeader("Content-Encoding: gzip")
                    resp.Body = GzComp(resp.Body)
                    resp.Headers["Content-Length"] = strconv.Itoa(len(resp.Body))
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

func GzComp(b string) string {
    buf := new(bytes.Buffer)
    compWriter := gzip.NewWriter(buf)
    compWriter.Write([]byte(b))
    compWriter.Close()
    str := buf.String()
    return str
}

func (s *HttpServer) StaticHandler(r Request) Response {
    //fmt.Println("Not Implemented!")
    if s.staticPath == "" {
        return Response{Version: "HTTP/1.1", StatusCode: 404, StatusText: "Not Found"}
    } else { 
        return Response{Version: "HTTP/1.1", StatusCode: 505, StatusText: "Not Implimented"}
    }

}

//func (s *HttpServer) matchPath() route {

//}

func (s HttpServer) BuildRequestFromRaw(b []byte) Request {
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
    Handler := s.StaticHandler
    if match >= 0 {
        // we have a matching route, at index i of s.routes
        // we will also have potentially some indexes for pathParams
        Handler = s.routes[match].Handler

        for i := 0; i < len(indexes); i++ {
            index := indexes[i]
            paramKey := strings.Split(s.routes[match].path, "/")[index]
            paramKey = paramKey[1:len(paramKey) - 1]
            paramVal := strings.Split(noQuery, "/")[index]
            pp[paramKey] = paramVal 
        }
    }

    return Request{raw: b, reqLine: rqln, Headers: hdmap, Body: body, Method: string(method[:]), Target: string(target[:]), Version: string(version[:]), PathParams: pp, QueryParams: qp, Handler: Handler}

}

type Request struct {
    raw []byte
    
    reqLine []byte
    Headers map[string]string
    Body []byte
    Method string
    Target string
    Version string

    PathParams map[string]string
    QueryParams map[string]string

    Handler
}

type Response struct {
    //raw []byte
    
    Version string
    StatusCode int
    StatusText string
    
    Headers map[string]string
    Body string 
}

func (r Response) RawString() string {
    statusLine := r.Version + " " + strconv.Itoa(r.StatusCode) + " " + r.StatusText + "\r\n"
    headers := ""
    for name, value := range r.Headers {
        headers += name + ": " + value + "\r\n"
    }
     headers += "\r\n"
    //fmt.Println(statusLine + headers + r.body)
    return statusLine + headers + r.Body
}

func (r Response) RawBytes() []byte {
    return []byte(r.RawString())
}

func (r *Response) AddHeader(h string) error {
    // do some validation
    if r.Headers == nil {
        r.Headers = make(map[string]string)
    }
    r.Headers[strings.Split(h,":")[0]] = strings.Split(h, ":")[1]

    return nil
}


type Handler func(Request) Response // functions passed by user to map to routes

type route struct {
    path string
    Handler Handler
}

func (s *HttpServer) AddRoute(r string, h Handler) {
    rt := route{path: r, Handler: h}
    s.routes = append(s.routes, rt)
}


func parseRequest(h []byte) ([]byte, [][]byte, []byte) {
    // split http Request into 3 parts
    // Requesta line
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
    // split the reqline part of an http Request into
    // method
    // target
    // httpVersion
    
    parts := bytes.Split(l, []byte(" "))
    method := parts[0]
    target := parts[1]
    version := parts[2]

    return method, target, version

}

func GetHeader(h map[string]string, n string) string {
    return h[n]
    // we might modify error handling for this
    // or we might just remove this func
}

