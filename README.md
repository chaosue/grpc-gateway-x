grpc-gateway-x
---------------
grpc-gateway-x provides a proxy for (front) clients accessing GRPC servers/services without need to know where the backend server are, 
and a reverse-proxy for GRPC servers to expose their services and response to the incoming requests.    

### features
* proxy with GRPC-WEB protocol for web/js/ts front.
* proxy with GRPC protocol for app front or any backend app.
* auto service-discovery via consul with full GRPC request method name, so multi-clustered services can be reverse-proxied. 
* you can also explicitly specify the backend address in the configuration file. this will disable auto service-discovery.
* for more configurable features, please refer to the `config.example.yaml` file.
 
### build
`go build`

### run
`./grcp-gateway-x --config config.yaml`

### proxy with auto service discovery usecase
#### prerequisites
* the services must be registered per the rules of `github.com/go-kratos/kratos/contrib/registry`
* the registered service names must the prefix part of the full path in GRPC requests, e.g.     
   `/com.yourcorp.yourproj.grpc.Service1/Method1`, `com.yourcorp.yourproj.grpc.Service1` should be the registered service name.