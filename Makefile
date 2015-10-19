IMAGE_UPTODATE=.image.uptodate
EXEC=arriba

.PHONY: docker clean

$(EXEC): main.go
	go get ./...
	go build -ldflags "-extldflags \"-static\" -linkmode=external" ./...

docker: $(IMAGE_UPTODATE)

$(IMAGE_UPTODATE): Dockerfile $(EXEC) ca-certificates.crt
	docker build -t 2opremio/arriba .

ca-certificates.crt: /etc/ssl/certs/ca-certificates.crt
	cp $< $@

clean:
	rm -f $(IMAGE_UPTODATE) $(EXEC) ca-certificates.crt
