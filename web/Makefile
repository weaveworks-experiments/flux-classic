PKG:=github.com/squaremo/weevil
STATIC_FILES:=res/*.js res/*.css res/main.css index.html
SRC:=*.go

.PHONY: image clean run

image: docker/.server.done

clean:
	rm -f weevil docker/.server.uptodate
	rm -f build

.%.done: Dockerfile.%
	rm -rf build-container
	mkdir build-container
	cp -pr $^ build-container
	docker build -t weevil/$(*F) -f build-container/$(<F) build-container
	rm -rf build-container
	touch $@

docker/.server.done: weevil $(STATIC_FILES)

weevil: docker/.build.done docker/build-in-container.sh $(SRC)
	rm -rf build/src/$(PKG)
	mkdir -p build/src/$(PKG)
	cp -pr $(SRC) build/src/$(PKG)
	docker run --rm -v $$PWD/build:/go \
	    -v $$PWD/docker/build-in-container.sh:/build.sh \
	    --workdir=/go/src/$(PKG) -e GOPATH=/go weevil/build sh /build.sh
	cp build/bin/weevil $@

res/main.css: res/main.less
	lessc res/main.less res/main.css

run: docker/.server.done
	docker run --rm -v `pwd`/res:/home/weevil/res \
	  -p 7070:7070 weevil/server
