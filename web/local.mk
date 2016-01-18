WEB_SRC:=$(shell find web/src -type f)
WEB_STATIC:=web/src/index.html
webpack_config:=webpack.production.config.js

$(call image_stamp,web): $(WEB_STATIC)
$(call image_stamp,web): web/build/assets.tar

$(call image_stamp,webbuild): web/package.json web/.babelrc web/.eslintrc web/.eslintignore

web/build/assets.tar: $(call image_stamp,webbuild) web/$(webpack_config) $(WEB_SRC)
	mkdir -p web/build/assets
	docker run --rm -v $(shell pwd)/web/src:/build/src \
		-v $$PWD/web/$(webpack_config):/build/$(webpack_config) \
		-v $$PWD/web/build:/build/build \
		$(call docker_tag,webbuild) npm run build
	tar cWvf $@ -C web/build/assets .

.PHONY: clean-web
clean:: clean-web

clean-web::
	rm -rf ./web/build
