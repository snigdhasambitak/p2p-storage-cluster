TARGETS=chord


all: $(TARGETS)

chord:
	go build -o chord main.go

clean:
	-rm -f $(TARGETS)

fmt: go fmt

.PHONY: all chord
