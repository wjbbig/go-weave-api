package go_weave_api

type Option func(*Weave)

func WithPlugin() Option {
	return func(weave *Weave) {
		weave.enablePlugin = true
	}
}

func WithProxy() Option {
	return func(weave *Weave) {
		weave.enableProxy = true
	}
}

func WithNickname(nickname string) Option {
	return func(weave *Weave) {
		weave.nickname = nickname
	}
}

func WithPassword(password string) Option {
	return func(weave *Weave) {
		weave.password = password
	}
}

func WithDockerPort(port int) Option {
	return func(weave *Weave) {
		weave.local = false
		weave.dockerPort = port
	}
}

func WithTLS(cacertPath, certPath, keyPath string) Option {
	return func(weave *Weave) {
		weave.tlsVerify = true
		weave.clientTLS = &tlsCerts{
			cacertPath: cacertPath,
			certPath:   certPath,
			keyPath:    keyPath,
		}
	}
}

func WithIPRange(ipRange string) Option {
	return func(weave *Weave) {
		weave.ipRange = ipRange
	}
}

func WithIpAllocDefaultSubnet(ipRange string) Option {
	return func(weave *Weave) {
		weave.ipAllocDefaultSubnet = ipRange
	}
}

func WithPort(port int) Option {
	return func(weave *Weave) {
		weave.port = port
	}
}

func WithVersion(version string) Option {
	return func(weave *Weave) {
		weave.version = version
	}
}

func WithDNSAddress(address string) Option {
	return func(weave *Weave) {
		if weave.dns == nil {
			weave.dns = NewDNSServer(address, "weave.local.", false)
			weave.dns.weave = weave
			return
		}
		weave.dns.Address = address
	}
}

func NoRestart() Option {
	return func(weave *Weave) {
		weave.restartPolicy = "none"
	}
}

func NoDiscovery() Option {
	return func(weave *Weave) {
		weave.discovery = false
	}
}

func NoDNS() Option {
	return func(weave *Weave) {
		if weave.dns == nil {
			weave.dns = NewDNSServer("", "", true)
			return
		}
		weave.dns.Disabled = true
	}
}
