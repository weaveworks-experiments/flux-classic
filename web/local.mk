WEB_SRC:=$(shell find web/src -type f)
WEB_BUILD:=$(shell find web/build -type f)
WEB_STATIC:=web/src/index.html

$(call image_stamp,web): $(WEB_STATIC) $(WEB_BUILD) web/build/app.js web/webpack.production.config.js

$(call image_stamp,webbuild): web/package.json web/.babelrc web/.eslintrc web/.eslintignore web/webpack.production.config.js

#web/build/app.js: $(call image_stamp,webbuild)
#	$(call run_build_container,webbuild,,web,npm run build)

web/build/app.js: $(call image_stamp,webbuild)
	mkdir -p web/build
	docker run --rm -v $(shell pwd)/web/src:/build/src \
		-v $(shell pwd)/web/build:/build/build \
		$(call docker_tag,webbuild) npm run build

.PHONY: clean-web
clean:: clean-web

clean-web::
	rm -f $(WEB_BUILD)
