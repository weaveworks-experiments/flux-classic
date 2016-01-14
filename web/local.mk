WEB_SRC:=$(shell find web/src -type f)
WEB_BUILD:=$(shell find web/build -type f)
WEB_STATIC:=web/src/index.html

$(call image_stamp,web): $(WEB_STATIC) $(WEB_BUILD) web/build/app.js web/webpack.production.config.js

$(call image_stamp,webbuild): web/package.json

web/build/app.js: $(call image_stamp,webbuild)
	$(call run_build_container,webbuild,,web,npm run build)

.PHONY: clean-web
clean:: clean-web

clean-web::
	rm -f $(WEB_BUILD)
