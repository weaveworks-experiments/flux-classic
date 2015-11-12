WEB_BABELGEN:=$(patsubst %.babel,%.js,$(shell find web/gen -name '*.babel'))
WEB_LESSGEN:=$(patsubst %.less,%.css,$(shell find web/gen -name '*.less'))
WEB_GEN:=$(WEB_LESSGEN) $(WEB_BABELGEN)
WEB_STATIC:=web/index.html web/res/*.css web/res/*.js

web.bin: $(shell find web -name '*.go')

docker/.web.done: $(WEB_STATIC) $(WEB_GEN)

$(WEB_GEN): docker/.webbuild.done

%.css: %.less
	$(call run_build_container,webbuild,,,,lessc $< $@)

%.js: %.babel
	$(call run_build_container,webbuild,,,,babel $< -o $@)

.PHONY: clean-web
clean: clean-web

clean-web:
	rm -f $(WEB_GEN)
