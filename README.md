# sgx-container-runtime

A wrapper of [runc](https://github.com/opencontainers/runc) adding devices and sockets to all containers which want Intel SGX supported.

# Installation

```
make && make install
```

# Docker Engine setup

## systemd

```
sudo mkdir -p /etc/systemd/system/docker.service.d
sudo tee /etc/systemd/system/docker.service.d/override.conf <<EOF
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd --host=fd:// --add-runtime=sgx=/usr/local/bin/sgx-container-runtime
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker
```

## daemon configuration file

```
sudo tee /etc/docker/daemon.json <<EOF
{
    "runtimes": {
        "sgx": {
            "path": "/usr/local/bin/sgx-container-runtime",
            "runtimeArgs": []
        }
    }
}
EOF
sudo pkill -SIGHUP dockerd
```

You can optionally reconfigure the default runtime by adding the following to `/etc/docker/daemon.json`:

```
"default-runtime": "sgx"
```

## command line

```
sudo dockerd --add-runtime=sgx=/usr/local/bin/sgx-container-runtime [...]
```
