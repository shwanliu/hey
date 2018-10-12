// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command hey is an HTTP load generator.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"bytes"
	"io"
	"mime/multipart"
	gourl "net/url"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"time"
	"path/filepath"
	// "encoding/json"

	"github.com/rakyll/hey/requester"
)

const (
	headerRegexp = `^([\w-]+):\s*(.+)`
	authRegexp   = `^(.+):([^\s].+)`
	heyUA        = "hey/0.0.1"
)

var (
	m           = flag.String("m", "GET", "")
	headers     = flag.String("h", "", "")
	body        = flag.String("d", "", "")
	bodyFile    = flag.String("D", "", "")
	accept      = flag.String("A", "", "")
	contentType = flag.String("T", "text/html", "")
	authHeader  = flag.String("a", "", "")
	hostHeader  = flag.String("host", "", "")

	//两张图片的文件名的传入，文件名之间使用“,”隔开
	form_data_filename = flag.String("F","","")

	//人脸入库，请求参数为 {人脸库名称} {图片} {customId}
	add_face_form_data = flag.String("add","","")
	
	//人脸入库，请求参数为 {人脸库名称} {图片} {score} {topn}
	identify_face_form_data = flag.String("identify","","")

	output = flag.String("o", "", "")

	c = flag.Int("c", 50, "")
	n = flag.Int("n", 200, "")
	q = flag.Float64("q", 0, "")
	t = flag.Int("t", 20, "")
	z = flag.Duration("z", 0, "")

	h2   = flag.Bool("h2", false, "")
	cpus = flag.Int("cpus", runtime.GOMAXPROCS(-1), "")

	disableCompression = flag.Bool("disable-compression", false, "")
	disableKeepAlives  = flag.Bool("disable-keepalive", false, "")
	disableRedirects   = flag.Bool("disable-redirects", false, "")
	proxyAddr          = flag.String("x", "", "")
)

var usage = `Usage: hey [options...] <url>

Options:
  -n  Number of requests to run. Default is 200.
  -c  Number of requests to run concurrently. Total number of requests cannot
      be smaller than the concurrency level. Default is 50.
  -q  Rate limit, in queries per second (QPS). Default is no rate limit.
  -z  Duration of application to send requests. When duration is reached,
      application stops and exits. If duration is specified, n is ignored.
      Examples: -z 10s -z 3m.
  -o  Output type. If none provided, a summary is printed.
      "csv" is the only supported alternative. Dumps the response
      metrics in comma-separated values format.

  -m  HTTP method, one of GET, POST, PUT, DELETE, HEAD, OPTIONS.
  -H  Custom HTTP header. You can specify as many as needed by repeating the flag.
      For example, -H "Accept: text/html" -H "Content-Type: application/xml" .
  -t  Timeout for each request in seconds. Default is 20, use 0 for infinite.
  -A  HTTP Accept header.
  -d  HTTP request body.
  -D  HTTP request body from file. For example, /home/user/file.txt or ./file.txt.
  -T  Content-type, defaults to "text/html".
  -a  Basic authentication, username:password.
  -x  HTTP Proxy address as host:port.
  -h2 Enable HTTP/2.
  
  //使用-F 指定两个文件名，两个文件名之间用逗号隔开
  -F  form-data: filename,filename 

  -host	HTTP Host header.

  -disable-compression  Disable compression.
  -disable-keepalive    Disable keep-alive, prevents re-use of TCP
                        connections between different HTTP requests.
  -disable-redirects    Disable following of HTTP redirects
  -cpus                 Number of used cpu cores.
                        (default for current machine is %d cores)
`
// 对于多图片文件上传，该函数的使用是针对人脸引擎接口face_verify的压力测试，
func uploadMultipartFile(image string, imageother string )([]byte, string) {

		var b bytes.Buffer  
		w := multipart.NewWriter(&b)

		// Add your image file
		f, err := os.Open(image)
		if err != nil {
			fmt.Println("error open image")
		}
		defer f.Close()

		fw, err := w.CreateFormFile("image", filepath.Base(image))
		if err != nil {
			fmt.Println(" CreateFormFile for image error ")
		}

		if _, err = io.Copy(fw, f); err != nil {
			fmt.Println(" Io.Copy error ")
		}

		// Add the other image
		f, err = os.Open(imageother)
		if err != nil {
			fmt.Println(" open imageother error  ")
		}

		defer f.Close()

		fw, err = w.CreateFormFile("imageother", filepath.Base(imageother))
		if err != nil {
			fmt.Println(" CreateFormFile for imageother error ")
		}
		
		if _, err = io.Copy(fw, f); err != nil {
			fmt.Println(" Io.Copy error ")
		}

		// Don't forget to close the multipart writer
		w.Close()

		var b_ []byte
		b_ , _ = ioutil.ReadAll(&b)
		//使用 返回所需要的ContentType，里面会有boundary的参数
		return b_ , w.FormDataContentType()
}


// 该函数的使用是针对人脸引擎接口人脸入库的压力测试，
func upload_AddFace(name string, image string, customId string )([]byte, string) {

	var b bytes.Buffer  
	w := multipart.NewWriter(&b)

	// Add the facedb name
	if fw, err = w.CreateFormField("image"); err != nil {
        return
    }
		
	if _, err = fw.Write([]byte(name)); err != nil {
		return
	}

	// Add your image file
	f, err := os.Open(image)
	if err != nil {
		fmt.Println("error open image")
	}
	defer f.Close()

	fw, err := w.CreateFormFile("image", filepath.Base(image))
	if err != nil {
		fmt.Println(" CreateFormFile for image error ")
	}

	if _, err = io.Copy(fw, f); err != nil {
		fmt.Println(" Io.Copy error ")
	}

	// Add the customId
	if fw, err = w.CreateFormField("customId"); err != nil {
        return
    }
		
	if _, err = fw.Write([]byte(name)); err != nil {
		return
	}

	// Don't forget to close the multipart writer
	w.Close()

	var b_ []byte
	b_ , _ = ioutil.ReadAll(&b)
	//使用 返回所需要的ContentType，里面会有boundary的参数
	return b_ , w.FormDataContentType()
}


func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(usage, runtime.NumCPU()))
	}

	var hs headerSlice
	flag.Var(&hs, "H", "")

	flag.Parse()
	if flag.NArg() < 1 {
		usageAndExit("")
	}

	runtime.GOMAXPROCS(*cpus)
	num := *n
	conc := *c
	q := *q
	dur := *z

	if dur > 0 {
		num = math.MaxInt32
		if conc <= 0 {
			usageAndExit("-c cannot be smaller than 1.")
		}
	} else {
		if num <= 0 || conc <= 0 {
			usageAndExit("-n and -c cannot be smaller than 1.")
		}

		if num < conc {
			usageAndExit("-n cannot be less than -c.")
		}
	}

	url := flag.Args()[0]
	method := strings.ToUpper(*m)

	// set content-type change by shawnliu
	header := make(http.Header)

	// set any other additional headers
	if *headers != "" {
		usageAndExit("Flag '-h' is deprecated, please use '-H' instead.")
	}
	// set any other additional repeatable headers
	for _, h := range hs {
		match, err := parseInputWithRegexp(h, headerRegexp)
		if err != nil {
			usageAndExit(err.Error())
		}
		header.Set(match[1], match[2])
	}

	if *accept != "" {
		header.Set("Accept", *accept)
	}

	// set basic auth if set
	var username, password string
	if *authHeader != "" {
		match, err := parseInputWithRegexp(*authHeader, authRegexp)
		if err != nil {
			usageAndExit(err.Error())
		}
		username, password = match[1], match[2]
	}

	var bodyAll []byte
	if *body != "" {
		bodyAll = []byte(*body)
	}
	if *bodyFile != "" {
		slurp, err := ioutil.ReadFile(*bodyFile)
		if err != nil {
			errAndExit(err.Error())
		}
		bodyAll = slurp
	}

	// 分解出两个文件名，对应(image，imageother)
	var image,imageother string
	if *form_data_filename != "" {
		MultiFilename := strings.Split(*form_data_filename,",")
		fmt.Printf("upload imagefiles are  %q\n", MultiFilename)
		image = MultiFilename[0]
		imageother = MultiFilename[1]
		bodyAll, *contentType =  uploadMultipartFile(image,imageother)
	}

	var name_,image_,customId_ string
	if *add_face_form_data!= "" {
		MultiFilename := strings.Split(*add_face_form_data,",")
		fmt.Printf("upload imagefiles are  %q\n", MultiFilename)
		name_ = MultiFilename[0]
		image_= MultiFilename[1]
		customId_= MultiFilename[2]
		bodyAll, *contentType =  upload_AddFace(name_,image_,customId_)
	}
	
    //

	var proxyURL *gourl.URL
	if *proxyAddr != "" {
		var err error
		proxyURL, err = gourl.Parse(*proxyAddr)
		if err != nil {
			usageAndExit(err.Error())
		}
	}
	
	// 设置request的 "Content-Type",改变原本设定位置，因为form-data的post，需要 boundary=XXXXX
	header.Set("Content-Type", *contentType)

	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		usageAndExit(err.Error())
	}

	req.ContentLength = int64(len(bodyAll))
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}

	// set host header if set
	if *hostHeader != "" {
		req.Host = *hostHeader
	}

	ua := req.UserAgent()
	if ua == "" {
		ua = heyUA
	} else {
		ua += " " + heyUA
	}
	header.Set("User-Agent", ua)
	req.Header = header

	w := &requester.Work{
		Request:            req,
		RequestBody:        bodyAll,
		N:                  num,
		C:                  conc,
		QPS:                q,
		Timeout:            *t,
		DisableCompression: *disableCompression,
		DisableKeepAlives:  *disableKeepAlives,
		DisableRedirects:   *disableRedirects,
		H2:                 *h2,
		ProxyAddr:          proxyURL,
		Output:             *output,
	}
	w.Init()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		w.Stop()
	}()
	if dur > 0 {
		go func() {
			time.Sleep(dur)
			w.Stop()
		}()
	}
	w.Run()
}

func errAndExit(msg string) {
	fmt.Fprintf(os.Stderr, msg)
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func usageAndExit(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, msg)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	flag.Usage()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func parseInputWithRegexp(input, regx string) ([]string, error) {
	re := regexp.MustCompile(regx)
	matches := re.FindStringSubmatch(input)
	if len(matches) < 1 {
		return nil, fmt.Errorf("could not parse the provided input; input = %v", input)
	}
	return matches, nil
}

type headerSlice []string

func (h *headerSlice) String() string {
	return fmt.Sprintf("%s", *h)
}

func (h *headerSlice) Set(value string) error {
	*h = append(*h, value)
	return nil
}
