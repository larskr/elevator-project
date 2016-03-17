all: elevator

elevator: *.go network/*.go elev/*.go
	go build -o $@

get-pkgs:
	go get github.com/BurntSushi/toml
