WEB_SRC:=$(shell find web/src -type f)
WEB_STATIC:=web/src/index.html
WEBPACK_CONFIG:=web/.babelrc web/.eslintrc web/.eslintignore web/webpack.production.config.js

$(call image_stamp,web): $(WEB_STATIC)
$(call image_stamp,web): web/build/assets.tar

$(call image_stamp,webbuild): web/package.json

web/build/assets.tar: $(call image_stamp,webbuild) web/package.json $(WEB_SRC) $(WEBPACK_CONFIG)
	mkdir -p web/build/assets
	docker run --rm -v $(shell pwd)/web/src:/webbuild/src:ro \
		$(foreach f,$(WEBPACK_CONFIG),-v $$PWD/$(f):/webbuild/$(notdir $(f)):ro) \
		-v $$PWD/web/build:/webbuild/build \
		$(call docker_tag,webbuild) npm run build
	tar cvf $@ -C web/build/assets .

.PHONY: clean-web
clean:: clean-web

clean-web::
	rm -rf ./web/build
