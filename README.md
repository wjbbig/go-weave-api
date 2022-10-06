# go-weave-api
go-weave-api is a simple weave client that helps users deploy weave nodes locally or remotely. Most of the code is written based on weave scripts. More detail can be found in this [repository](https://github.com/weaveworks/weave.git).
# requirement
go-weave-api requires **go-1.18** or later. This api is writing based on weave-2.8.1, 
there is no verification that the previous version of weave can be used.
# how-to-use
Start with local machine:
~~~go
func main() {
    w, err := NewWeaveNode("127.0.0.1",WithProxy(), WithPlugin())
	if err != nil {
		panic(err)
	}
	defer w.Close()
	
	if err = w.Launch(); err != nil {
		panic(err)
	}

	status, err := w.Status()
	if err != nil {
		panic(err)
	}
	fmt.Println(status.Overview)

	w.Stop()
}
~~~
Start with remote docker node:
~~~go
func main() {
    w, err := NewWeaveNode("192.168.0.111", WithDockerPort(2375),
		WithTLS("./cacert.pem", "./cert.pem", "./key.pem"),
		WithProxy(), WithPlugin())
    if err != nil {
        panic(err)
    }
    defer w.Close()
    
    if err = w.Launch(); err != nil {
        panic(err)
    }
    
    status, err := w.Status()
    if err != nil {
        panic(err)
    }
    fmt.Println(status.Overview)
    
    w.Stop()
}
~~~

# License
This repository is distributed under the [Apache License Version 2.0](https://www.apache.org/licenses/LICENSE-2.0.html) found in the [LICENSE](./LICENSE) file.