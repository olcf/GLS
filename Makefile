all:
	mkdir -p attr_check/lib
	g++ -lgpfs -c attr_check/attr_check.cpp -o attr_check/lib/libattr_check.a
	/usr/local/go/bin/go mod tidy
	/usr/local/go/bin/go build -o gls .

rpm:
	VERSION=1.2.0 ARCH=$$(arch) RELEASE=$$(git rev-parse --short HEAD) envsubst < build/nfpm-template.yaml > build/nfpm.yaml
	nfpm -f build/nfpm.yaml pkg --packager rpm

install:
	/usr/bin/install ./gls /usr/local/bin	

clean:
	rm -rf attr_check/lib ./gls ./*.rpm build/nfpm.yaml
