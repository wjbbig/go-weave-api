package go_weave_api

type Option func(*Weave)

func WithPlugin() Option {
	return func(weave *Weave) {
		weave.enablePlugin = true
	}
}

func WithWeaveMtu(mtu int) Option {
	return func(weave *Weave) {
		weave.mtu = mtu
	}
}

func WithResume() Option {
	return func(weave *Weave) {
		weave.resume = true
	}
}

func WithHost(host string) Option {
	return func(weave *Weave) {
		weave.host = host
	}
}

func WithLogLevel(level string) Option {
	return func(weave *Weave) {
		weave.logLevel = level
	}
}

func WithToken(token string) Option {
	return func(weave *Weave) {
		weave.token = token
	}
}

func WithProxy() Option {
	return func(weave *Weave) {
		weave.enableProxy = true
	}
}

func WithName(name string) Option {
	return func(weave *Weave) {
		weave.name = name
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
		weave.dns.Address = address
	}
}

func WithHostnameFromLabel(labelKey string) Option {
	return func(weave *Weave) {
		weave.hostnameFromLabel = labelKey
	}
}

func WithHostnameMatch(match string) Option {
	return func(weave *Weave) {
		weave.hostnameMatch = match
	}
}

func WithHostnameReplacement(replacement string) Option {
	return func(weave *Weave) {
		weave.hostnameReplacement = replacement
	}
}

func WithPeers(peers ...string) Option {
	return func(weave *Weave) {
		weave.peers = peers
	}
}

func WithoutDNS() Option {
	return func(weave *Weave) {
		weave.withoutDNS = true
	}
}

func WithTrustedSubnets(cidrs string) Option {
	return func(weave *Weave) {
		weave.trustedSubnets = cidrs
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

func NoMulticastRoute() Option {
	return func(weave *Weave) {
		weave.noMultiRouter = true
	}
}

func NoFastdp() Option {
	return func(weave *Weave) {
		weave.disableFastDP = true
	}
}
