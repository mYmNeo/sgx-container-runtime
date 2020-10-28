.PHONY: all clean

all:
	CGO_ENABLED=0 go build $(LDFLAGS) .

install:
	install -m 755 sgx-container-runtime /usr/local/bin/

clean:
	rm -f ./sgx-container-runtime
