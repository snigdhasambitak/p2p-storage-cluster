TARGETS=p2pchord

install


p2pchord:
	go build -o p2pchord main.go

clean:
	-rm -f $(TARGETS)

fmt: go fmt

.PHONY: all p2pchord
